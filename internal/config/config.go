package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	DefaultMaxJobAttempts = 3
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type WorkerConfig struct {
	MaxRetryAttempts int
}

type AppConfig struct {
	Database *DBConfig
	Redis    *RedisConfig
	Worker   *WorkerConfig
}

func LoadConfig() (*AppConfig, error) {
	dbcfg, err := LoadDBConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load database config: %w", err)
	}

	rediscfg, err := loadRedisConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load redis config: %w", err)
	}

	workerCfg, err := loadWorkerConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load worker config: %w", err)
	}
	return &AppConfig{
		Database: dbcfg,
		Redis:    rediscfg,
		Worker:   workerCfg,
	}, nil
}

func LoadDBConfig() (*DBConfig, error) {
	portStr := getEnv("DB_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	cfg := &DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     port,
		User:     getEnv("DB_USER", "taskq_user"),
		Password: getEnv("DB_PASSWORD", "taskq_password"),
		DBName:   getEnv("DB_NAME", "taskq_db"),
	}
	if cfg.User == "" || cfg.Password == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("DB_USER, DB_PASSWORD, and DB_NAME environment variables must be set")
	}
	return cfg, nil
}

func loadRedisConfig() (*RedisConfig, error) {
	redisDBStr := getEnv("REDIS_DB", "0")
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB value: %w", err)
	}

	cfg := &RedisConfig{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       redisDB,
	}

	return cfg, nil
}

func loadWorkerConfig() (*WorkerConfig, error) {
	return &WorkerConfig{
		MaxRetryAttempts: DefaultMaxJobAttempts,
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
