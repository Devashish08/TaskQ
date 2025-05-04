package redisclient

import (
	"context"
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/config"
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	log.Println("Connecting to redis...", cfg.Addr)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Printf("Redis connection successful ping response: %s", pong)
	return rdb, nil

}
