package utils

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestHTTPProbeReusesConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var connections atomic.Int32
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "small probe response")
		}))
		server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
			if state == http.StateNew {
				connections.Add(1)
			}
		}
		old := httpClient
		httpClient = server.Client()
		defer func() { httpClient = old }()
		for range 3 {
			if ok, err := YCIpNelNet("http://probe.example.com"); !ok || err != nil {
				t.Fatalf("probe = %v, %v", ok, err)
			}
			synctest.Wait() // 等待 Go 1.27 Close 的有限响应排空完成。
		}
		if got := connections.Load(); got != 1 {
			t.Fatalf("opened %d connections, want 1", got)
		}
	})
}

func TestHTTPProbeTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		old := httpClient
		httpClient = server.Client()
		httpClient.Timeout = 10 * time.Second
		defer func() { httpClient = old }()
		start := time.Now()
		ok, err := YCIpNelNet("http://probe.example.com")
		if ok || err == nil {
			t.Fatalf("timeout probe = %v, %v", ok, err)
		}
		if elapsed := time.Since(start); elapsed != 10*time.Second {
			t.Fatalf("timeout after %v, want 10s", elapsed)
		}
	})
}

func TestHTTPProbeRejectsErrorStatus(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	old := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = old })
	ok, err := YCIpNelNet("http://probe.example.com")
	if ok || err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("error status probe = %v, %v", ok, err)
	}
}
