package config

import (
	"log"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port   int    `mapstructure:"port"`
	NodeId int64  `mapstructure:"node-id"`
	Active string `mapstructure:"active"`
	Email  string `mapstructure:"email"`
}
type LogConfig struct {
	Level          string `mapstructure:"Level"`
	Path           string `mapstructure:"Path"`
	MaxAge         int    `mapstructure:"MaxAge"`
	Zip            bool   `mapstructure:"Zip"`
	EncryptKey     string `mapstructure:"EncryptKey"`
	BufferSize     int    `mapstructure:"BufferSize"`
	FlushInterval  int    `mapstructure:"FlushInterval"`
	DropPolicy     string `mapstructure:"DropPolicy"`
	ArchiveWorkers int    `mapstructure:"ArchiveWorkers"`
}

type DBConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	DBName      string `mapstructure:"dbname"`
	SSLMode     string `mapstructure:"sslmode"`
	MaxOpen     int    `mapstructure:"max-open"`
	MaxIdle     int    `mapstructure:"max-idle"`
	MaxLifetime int    `mapstructure:"max-lifetime"`
	MaxIdleTime int    `mapstructure:"max-idle-time"`
}

type TelegramConfig struct {
	BotToken string `mapstructure:"bot-token"`
	ChatID   string `mapstructure:"chat-id"`
}
type NodeConfig struct {
	URL []string `mapstructure:"url"`
}
type TaskConfig struct {
	RetentionDays int `mapstructure:"retention-days"`
}

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	DB       DBConfig       `mapstructure:"database"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	Task     TaskConfig     `mapstructure:"task"`
	Node     NodeConfig     `mapstructure:"node"`
}

var AppConfig *Config

func LoadConfig(path string) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("error reading config file: %v", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		log.Fatalf("unable to decode into struct: %v", err)
	}

	AppConfig = cfg
}
