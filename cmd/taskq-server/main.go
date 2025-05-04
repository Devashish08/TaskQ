package main

import (
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/api"
	"github.com/Devashish08/taskq/internal/config"
	"github.com/Devashish08/taskq/internal/database"
	redisclient "github.com/Devashish08/taskq/internal/redisClient"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/Devashish08/taskq/internal/worker"
)

func main() {
	fmt.Println("Starting TaskQ server...")

	appCfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbPool, err := database.ConnectDB(appCfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer func() {
		log.Println("Closing database connection pool...")
		if cerr := dbPool.Close(); cerr != nil {
			log.Printf("Error closing database connection: %v\n", cerr)
		}
	}()

	redisClient, err := redisclient.ConnectRedis(appCfg.Redis) // Use appCfg.Redis
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		log.Println("Closing Redis connection...")
		if cerr := redisClient.Close(); cerr != nil {
			log.Printf("Error closing Redis connection: %v\n", cerr)
		}
	}()

	// --- Initialize Dependencies ---
	// TODO: Initialize config loading
	jobService := service.NewJobService(dbPool, redisClient)
	apiHandler := api.NewApiHandler(jobService)

	// --- Setup and Start API Server ---
	workerInstance := worker.NewWorker(1, redisClient, dbPool)
	workerInstance.Start()

	router := api.SetupRouterWithDeps(apiHandler)

	part := "8080"
	fmt.Printf("Server listening on port %s\n", part)
	if err := router.Run(":" + part); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
