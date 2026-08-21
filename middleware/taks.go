package middleware

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"nodes/config"
	"nodes/services"
)

type ServerTask struct {
	svc  services.Service
	cron *cron.Cron
}

func NewServerTask(svc services.Service) *ServerTask {
	return &ServerTask{
		svc: svc,
		cron: cron.New(
			cron.WithSeconds(),
		),
	}
}

func (t *ServerTask) Start() {
	if t == nil || t.cron == nil {
		return
	}

	retentionDays := config.AppConfig.Task.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 2
	}
	retention := time.Duration(retentionDays) * 24 * time.Hour

	// 每小时第8分钟清理过期数据
	_, _ = t.cron.AddJob("0 8 * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.PrintfLogger(log.Default()))).Then(cron.FuncJob(func() {
		before := time.Now().Add(-retention)
		if err := t.svc.TaskHour(before); err != nil {
			log.Printf("[task] delete old data error: %v", err)
		}
	})))
	// 每5分钟检查节点网络连通性
	_, _ = t.cron.AddJob("0 */5 * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.PrintfLogger(log.Default()))).Then(cron.FuncJob(func() {
		if err := t.svc.NodeNelNet(); err != nil {
			log.Printf("[task] node network check error: %v", err)
		}
	})))
	// 每7分钟检查API网络连通性
	_, _ = t.cron.AddJob("0 */7 * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.PrintfLogger(log.Default()))).Then(cron.FuncJob(func() {
		if err := t.svc.ApiNelNet(); err != nil {
			log.Printf("[task] API network check error: %v", err)
		}
	})))
	// 每天凌晨5点清空日级缓存
	_, _ = t.cron.AddJob("0 0 5 * * *", cron.NewChain(cron.SkipIfStillRunning(cron.PrintfLogger(log.Default()))).Then(cron.FuncJob(func() {
		t.svc.TaskDay()
		log.Println("[task] daily cache cleared")
	})))
	// 每天凌晨6点升级证书
	_, _ = t.cron.AddJob("0 30 5 * * *", cron.NewChain(cron.SkipIfStillRunning(cron.PrintfLogger(log.Default()))).Then(cron.FuncJob(func() {
		if err := t.svc.UploadCDNDomainCert(); err != nil {
			log.Printf("[task] daily Certificates upgraded error: %v", err)
		}
	})))
	t.cron.Start()
	log.Printf("[task] started (retention=%d days)", retentionDays)
}

func (t *ServerTask) Stop() {
	if t != nil && t.cron != nil {
		t.cron.Stop()
		log.Println("[task] stopped")
	}
}
