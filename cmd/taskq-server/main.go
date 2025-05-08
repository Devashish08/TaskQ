package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	router := api.SetupRouterWithDeps(apiHandler)

	// --- Setup and Start API Server ---
	numWorkers := 3
	pool := worker.NewWorkerPool(numWorkers, redisClient, dbPool)
	pool.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := "8080"
		log.Printf("API Server Listening on port %s\n", port)
		if err := router.Run(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	sig := <-quit
	log.Printf("Received signal: %s, signal down server and workers...\n", sig)

	pool.Stop()

	log.Println("TaskQ server stopped")
}
