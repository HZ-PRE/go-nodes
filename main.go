package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"nodes/config"
	"nodes/database"
	"nodes/middleware"
	"nodes/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var port int

func init() {
	config.LoadConfig("config.yaml")
	cfg := config.AppConfig
	if cfg.Server.Active == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	database.InitDB(cfg.DB)
	utils.InitSnowflake(cfg.Server.NodeId)
	port = cfg.Server.Port
}

func main() {
	os.Setenv(
		"LEGO_DNS_RESOLVERS",
		"119.29.29.29:53,114.114.114.114:53,223.5.5.5:53,1.1.1.1:53",
	)
	task, engine := InitApp()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: engine,
	}

	go func() {
		log.Printf("server starting on :%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	if task != nil {
		task.Stop()
	}
	middleware.FlushLogs()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server exited")
}
