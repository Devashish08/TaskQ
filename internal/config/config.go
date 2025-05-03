package config

import (
	"fmt"
	"os"
	"strconv"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
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

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
