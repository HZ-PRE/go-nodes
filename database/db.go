package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nodes/config"
)

var DB *gorm.DB

func InitDB(cfg config.DBConfig) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	// GORM 日志级别基于 cfg.Log.Level
	logLevel := logger.Silent
	switch strings.ToLower(config.AppConfig.Log.Level) {
	case "debug":
		logLevel = logger.Info
	case "warn":
		logLevel = logger.Warn
	case "error":
		logLevel = logger.Error
	default:
		logLevel = logger.Silent
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatal("failed to connect database: ", err)
	}
	DB = db

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("failed to get underlying sql.DB: ", err)
	}

	maxOpen := cfg.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxLifetime := time.Duration(cfg.MaxLifetime) * time.Second
	if maxLifetime <= 0 {
		maxLifetime = time.Hour
	}
	maxIdleTime := time.Duration(cfg.MaxIdleTime) * time.Second
	if maxIdleTime <= 0 {
		maxIdleTime = 5 * time.Minute
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(maxLifetime)
	sqlDB.SetConnMaxIdleTime(maxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		log.Fatal("failed to ping database: ", err)
	}

	log.Printf("database connected (pool: maxOpen=%d maxIdle=%d lifetime=%v idleTime=%v)",
		maxOpen, maxIdle, maxLifetime, maxIdleTime)
}
