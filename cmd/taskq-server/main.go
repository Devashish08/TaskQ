package main

import (
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/api"
	"github.com/Devashish08/taskq/internal/config"
	"github.com/Devashish08/taskq/internal/database"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/Devashish08/taskq/internal/worker"
)

func main() {
	fmt.Println("Starting TaskQ server...")

	dbCfg, err := config.LoadDBConfig()
	if err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}

	dbPool, err := database.ConnectDB(dbCfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		log.Println("Closing database connection pool...")
		if cerr := dbPool.Close(); cerr != nil {
			log.Printf("Error closing database connection: %v\n", cerr)
		}
	}()

	// --- Initialize Dependencies ---
	// TODO: Initialize config loading
	jobService := service.NewJobService(dbPool)
	apiHandler := api.NewApiHandler(jobService)

	// --- Start Worker(s) ---
	// Get the job queue channel from the service
	jobQueueChannel := jobService.GetJobQueue()

	// --- Setup and Start API Server ---
	workerInstance := worker.NewWorker(1, jobQueueChannel)
	workerInstance.Start()

	router := api.SetupRouterWithDeps(apiHandler)

	part := "8080"
	fmt.Printf("Server listening on port %s\n", part)
	if err := router.Run(":" + part); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
