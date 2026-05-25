package middleware

import (
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nodes/config"
	"nodes/utils"

	"github.com/gin-gonic/gin"
)

const (
	defaultBufSize   = 65536
	defaultFlushSec  = 5
	writeBufSize     = 64 * 1024
	maxBodyReadSize  = 2 * 1024
	maxBodyLogLen    = 2048
	archiveQueueSize = 64
	blockTimeout     = 100 * time.Millisecond
	maxPoolBufCap    = 8192
)

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return &b
	},
}

func putBuf(bp *[]byte) {
	if cap(*bp) > maxPoolBufCap {
		return
	}
	*bp = (*bp)[:0]
	bufPool.Put(bp)
}

var (
	infoWriter  *asyncWriter
	errorWriter *asyncWriter
	logOnce     sync.Once
)

type asyncWriter struct {
	ch            chan *[]byte
	archiveCh     chan string
	done          chan struct{}
	runDone       chan struct{}
	wg            sync.WaitGroup
	stopOnce      sync.Once
	dir           string
	prefix        string
	maxAge        int
	zip           bool
	encryptKey    []byte
	flushInterval time.Duration
	dropPolicy    string
	dropped       atomic.Int64

	file        *os.File
	buf         *bufio.Writer
	currentDate string
}

func newAsyncWriter(dir, prefix string, maxAge, chSize, flushSec int, zip bool, encryptKey, dropPolicy string, archiveWorkers int) *asyncWriter {
	_ = os.MkdirAll(dir, 0o755)
	if chSize <= 0 {
		chSize = defaultBufSize
	}
	if flushSec <= 0 {
		flushSec = defaultFlushSec
	}
	if archiveWorkers <= 0 {
		archiveWorkers = 1
	}
	if dropPolicy == "" {
		dropPolicy = "drop"
	}

	w := &asyncWriter{
		ch:            make(chan *[]byte, chSize),
		archiveCh:     make(chan string, archiveQueueSize),
		done:          make(chan struct{}),
		runDone:       make(chan struct{}),
		dir:           dir,
		prefix:        prefix,
		maxAge:        maxAge,
		zip:           zip,
		flushInterval: time.Duration(flushSec) * time.Second,
		dropPolicy:    dropPolicy,
	}
	if encryptKey != "" {
		key := make([]byte, 32)
		copy(key, []byte(encryptKey))
		w.encryptKey = key
	}

	if err := w.openCurrent(); err != nil {
		log.Printf("failed to open log file: %v", err)
	}
	w.archiveOldLogs()
	w.cleanup()

	w.wg.Add(1)
	go w.run()

	for i := 0; i < archiveWorkers; i++ {
		w.wg.Add(1)
		go w.archiveWorker()
	}
	return w
}

func (w *asyncWriter) writeLine(level, message string) {
	bp := bufPool.Get().(*[]byte)
	b := (*bp)[:0]
	b = append(b, level...)
	b = append(b, ' ')
	b = time.Now().AppendFormat(b, "2006/01/02 15:04:05")
	b = append(b, ' ')
	b = append(b, message...)
	b = append(b, '\n')
	*bp = b

	switch w.dropPolicy {
	case "block":
		select {
		case w.ch <- bp:
			return
		default:
		}

		t := time.NewTimer(blockTimeout)
		select {
		case w.ch <- bp:
			t.Stop()
		case <-t.C:
			putBuf(bp)
			w.dropped.Add(1)
		}
	default:
		select {
		case w.ch <- bp:
		default:
			putBuf(bp)
			w.dropped.Add(1)
		}
	}
}

func (w *asyncWriter) run() {
	defer func() {
		close(w.runDone)
		w.wg.Done()
	}()

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case bp := <-w.ch:
			w.writeToBuf(*bp)
			putBuf(bp)
			for len(w.ch) > 0 {
				bp := <-w.ch
				w.writeToBuf(*bp)
				putBuf(bp)
			}
		case <-ticker.C:
			w.flushBuf()
		case <-w.done:
			for {
				select {
				case bp := <-w.ch:
					w.writeToBuf(*bp)
					putBuf(bp)
				default:
					w.flushBuf()
					return
				}
			}
		}
	}
}

func (w *asyncWriter) writeToBuf(p []byte) {
	if w.file == nil {
		return
	}

	today := time.Now().Format("2006-01-02")
	if today != w.currentDate {
		w.flushBuf()
		oldPath := w.file.Name()
		_ = w.file.Close()
		select {
		case w.archiveCh <- oldPath:
		default:
			log.Printf("[log] archive queue full, skipping: %s", oldPath)
		}
		if err := w.openCurrent(); err != nil {
			log.Printf("failed to rotate log: %v", err)
		}
	}
	_, _ = w.buf.Write(p)
}

func (w *asyncWriter) flushBuf() {
	if w.buf != nil {
		_ = w.buf.Flush()
	}
}

func (w *asyncWriter) openCurrent() error {
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(w.dir, fmt.Sprintf("%s_%s.log", w.prefix, today))

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = f
	w.buf = bufio.NewWriterSize(f, writeBufSize)
	w.currentDate = today
	return nil
}

func (w *asyncWriter) archiveWorker() {
	defer w.wg.Done()
	for path := range w.archiveCh {
		w.archiveFile(path)
	}
}

func (w *asyncWriter) Stop() {
	w.stopOnce.Do(func() {
		close(w.done)
		<-w.runDone
		close(w.archiveCh)
		w.wg.Wait()
	})
}

func (w *asyncWriter) Stats() (dropped int64, queueLen int) {
	return w.dropped.Load(), len(w.ch)
}

func (w *asyncWriter) archiveFile(filePath string) {
	if w.zip {
		w.compressAndEncrypt(filePath)
		return
	}
	if len(w.encryptKey) > 0 {
		w.encryptOnly(filePath)
	}
}

func (w *asyncWriter) compressAndEncrypt(filePath string) {
	in, err := os.Open(filePath)
	if err != nil {
		log.Printf("open log file %s failed: %v", filePath, err)
		return
	}

	gzPath := filePath + ".gz"
	out, err := os.Create(gzPath)
	if err != nil {
		_ = in.Close()
		log.Printf("create gzip file %s failed: %v", gzPath, err)
		return
	}

	gw := gzip.NewWriter(out)
	gw.Name = filepath.Base(filePath)
	gw.ModTime = time.Now()

	_, copyErr := io.Copy(gw, in)
	_ = in.Close()
	_ = gw.Close()
	_ = out.Close()

	if copyErr != nil {
		_ = os.Remove(gzPath)
		log.Printf("compress log file %s failed: %v", filePath, copyErr)
		return
	}

	if len(w.encryptKey) > 0 {
		if w.encryptFile(gzPath, filePath+".gz.enc") {
			_ = os.Remove(gzPath)
			_ = os.Remove(filePath)
			return
		}
		log.Printf("encrypt gzip file %s failed, kept gzip fallback", gzPath)
		_ = os.Remove(filePath)
		return
	}

	_ = os.Remove(filePath)
}

func (w *asyncWriter) encryptOnly(filePath string) {
	if w.encryptFile(filePath, filePath+".enc") {
		_ = os.Remove(filePath)
	}
}

func (w *asyncWriter) encryptFile(inPath, outPath string) bool {
	data, err := os.ReadFile(inPath)
	if err != nil || len(data) == 0 {
		return false
	}

	encrypted, err := w.encrypt(data)
	if err != nil {
		log.Printf("encrypt log %s failed: %v", inPath, err)
		return false
	}
	if err := os.WriteFile(outPath, encrypted, 0o644); err != nil {
		log.Printf("write encrypted log %s failed: %v", outPath, err)
		return false
	}
	return true
}

func (w *asyncWriter) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(w.encryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (w *asyncWriter) archiveOldLogs() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	today := time.Now().Format("2006-01-02")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == w.prefix+".log" {
			w.archiveFile(filepath.Join(w.dir, name))
			continue
		}
		if strings.HasPrefix(name, w.prefix+"_") && strings.HasSuffix(name, ".log") && !strings.Contains(name, today) {
			w.archiveFile(filepath.Join(w.dir, name))
		}
	}
}

func (w *asyncWriter) cleanup() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	deadline := time.Now().Add(-time.Duration(w.maxAge) * 24 * time.Hour)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(deadline) {
			_ = os.Remove(filepath.Join(w.dir, entry.Name()))
		}
	}
}

func initLogFiles(cfg *config.Config) {
	logDir := cfg.Log.Path
	if logDir == "" {
		logDir = "logs"
	}
	maxAge := cfg.Log.MaxAge
	if maxAge <= 0 {
		maxAge = 7
	}

	infoWriter = newAsyncWriter(logDir, "info", maxAge, cfg.Log.BufferSize, cfg.Log.FlushInterval, cfg.Log.Zip, cfg.Log.EncryptKey, cfg.Log.DropPolicy, cfg.Log.ArchiveWorkers)
	errorWriter = newAsyncWriter(logDir, "error", maxAge, cfg.Log.BufferSize, cfg.Log.FlushInterval, cfg.Log.Zip, cfg.Log.EncryptKey, cfg.Log.DropPolicy, cfg.Log.ArchiveWorkers)
}

func FlushLogs() {
	if infoWriter != nil {
		infoWriter.Stop()
	}
	if errorWriter != nil {
		errorWriter.Stop()
	}
}

func LogStats() (infoDropped, errorDropped int64) {
	if infoWriter != nil {
		infoDropped, _ = infoWriter.Stats()
	}
	if errorWriter != nil {
		errorDropped, _ = errorWriter.Stats()
	}
	return
}

func compactBody(b []byte) []byte {
	j := 0
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			b[j] = c
			j++
		}
	}
	return b[:j]
}

type bodyCaptureReadCloser struct {
	rc    io.ReadCloser
	limit int
	buf   []byte
}

func newBodyCaptureReadCloser(rc io.ReadCloser, limit int) *bodyCaptureReadCloser {
	return &bodyCaptureReadCloser{
		rc:    rc,
		limit: limit,
		buf:   make([]byte, 0, limit),
	}
}

func (b *bodyCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	if n <= 0 || b.limit <= 0 || len(b.buf) >= b.limit {
		return n, err
	}

	remaining := b.limit - len(b.buf)
	if remaining > n {
		remaining = n
	}
	b.buf = append(b.buf, p[:remaining]...)
	return n, err
}

func (b *bodyCaptureReadCloser) Close() error {
	return b.rc.Close()
}

func (b *bodyCaptureReadCloser) Captured() []byte {
	return b.buf
}

func logWithLevel(level, message string) {
	switch strings.ToUpper(level) {
	case "ERROR":
		if errorWriter != nil {
			errorWriter.writeLine(level, message)
		}
	default:
		if infoWriter != nil {
			infoWriter.writeLine(level, message)
		}
	}

	// debug模式下同时输出到控制台
	if config.AppConfig.Log.Level == "debug" {
		log.Print(message)
	}
}

func Logger() gin.HandlerFunc {
	cfg := config.AppConfig
	logOnce.Do(func() {
		initLogFiles(cfg)
	})

	logLevel := strings.ToLower(cfg.Log.Level)
	needBody := logLevel != "none" && logLevel != "error"

	return func(c *gin.Context) {
		var bodyCapture *bodyCaptureReadCloser
		if needBody && (c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH") && c.Request.Body != nil {
			bodyCapture = newBodyCaptureReadCloser(c.Request.Body, maxBodyReadSize)
			c.Request.Body = bodyCapture
		}

		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := "INFO"
		switch {
		case status >= 500:
			level = "ERROR"
		case status != 200:
			level = "WARN"
		}

		if (logLevel == "warn" && level == "INFO") || (logLevel == "error" && level != "ERROR") || logLevel == "none" {
			return
		}

		var bodyStr string
		if bodyCapture != nil {
			if body := bodyCapture.Captured(); len(body) > 0 {
				bodyStr = string(compactBody(body))
				if cfg.Log.Zip {
					if compressed, err := utils.GzipCompressBase64([]byte(bodyStr)); err == nil {
						bodyStr = compressed
					}
				} else if len(bodyStr) > maxBodyLogLen {
					bodyStr = bodyStr[:maxBodyLogLen] + "...[truncated]"
				}
			}
		}

		latency := time.Since(start)

		var sb strings.Builder
		sb.Grow(256)
		sb.WriteString(c.Request.Method)
		sb.WriteByte(' ')

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		sb.WriteString(path)
		fmt.Fprintf(&sb, " status=%d latency=%v ip=%s", status, latency, c.ClientIP())
		sb.WriteString(" url=")
		sb.WriteString(c.Request.URL.Path)

		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			sb.WriteString(" query=")
			sb.WriteString(rawQuery)
		}
		if bodyStr != "" {
			sb.WriteString(" body=")
			sb.WriteString(bodyStr)
		}

		logWithLevel(level, sb.String())
	}
}
