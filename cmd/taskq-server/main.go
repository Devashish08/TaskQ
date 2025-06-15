// Package main implements the TaskQ server application.
// TaskQ is a distributed task queue system that provides job scheduling,
// execution, and management capabilities using Redis as a message broker
// and PostgreSQL for persistence.
package main

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Devashish08/taskq/internal/api"
	"github.com/Devashish08/taskq/internal/bot"
	"github.com/Devashish08/taskq/internal/config"
	"github.com/Devashish08/taskq/internal/database"
	"github.com/Devashish08/taskq/internal/jobhandlers"
	redisclient "github.com/Devashish08/taskq/internal/redisClient"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/Devashish08/taskq/internal/worker"
)

// main is the entry point for the TaskQ server application.
// It initializes all required components including database connections,
// Redis client, job handlers, worker pool, and HTTP server.
// The function implements graceful shutdown handling for SIGINT and SIGTERM signals.
//
// Initialization sequence:
// 1. Seed random number generator for job handlers
// 2. Load application configuration
// 3. Initialize database connection and run migrations
// 4. Initialize Redis connection
// 5. Setup job handler registry with supported job types
// 6. Initialize job service and API handlers
// 7. Start worker pool for job processing
// 8. Start HTTP server with graceful shutdown support
//
// The application will terminate with a fatal error if any critical
// component fails to initialize properly.
func main() {
	// Seed the random number generator for job handlers
	rand.Seed(time.Now().UnixNano())

	// Load application configuration
	appCfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load application config: %v", err)
	}

	// Connect to the database
	dbPool, err := database.ConnectDB(appCfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	database.CheckConnectionInfo(dbPool, "After ConnectDB in main")

	if err := database.MigrateDB(dbPool); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	err = database.VerifyTables(dbPool)
	if err != nil {
		log.Fatalf("FATAL: Table verification failed after migration: %v", err)
	}
	log.Println("Table verification successful after migration.")

	// Connect to Redis
	redisClient, err := redisclient.ConnectRedis(appCfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Create and populate job handler registry
	jobHandlerRegistry := jobhandlers.NewRegistry()
	jobHandlerRegistry.Register("send_discord_reminder", jobhandlers.HandleDiscordReminder)
	jobHandlerRegistry.Register("log_payload", jobhandlers.HandleSimpleLogJob)
	jobHandlerRegistry.Register("failing_job", jobhandlers.HandlePotentiallyFailingJob)
	jobHandlerRegistry.Register("long_job", jobhandlers.HandleLongRunningJob)

	// Initialize services
	jobService := service.NewJobService(dbPool, redisClient)

	chronosBot, err := bot.NewBot(appCfg.Bot.Token, jobService)
	if err != nil {
		log.Fatalf("Failed to create chronos bot: %v", err)
	}

	bot.BotInstance = chronosBot
	apiHandler := api.NewApiHandler(jobService)
	router := api.SetupRouterWithDeps(apiHandler)

	// Initialize worker pool and outbox poller
	numWorkers := 3 // TODO: Make this configurable from appCfg.Worker
	pool := worker.NewWorkerPool(
		numWorkers,
		redisClient,
		dbPool,
		jobHandlerRegistry,
		appCfg.Worker.MaxRetryAttempts,
		chronosBot.Session,
	)
	pool.Start()

	outboxPoller := worker.NewOutboxPoller(dbPool, redisClient, 2*time.Second)
	outboxPoller.Start()

	if err = chronosBot.Start(); err != nil {
		log.Fatalf("Failed to start chronos bot: %v", err)
	}

	// Start HTTP server with graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := "8080" // TODO: Make this configurable
		log.Printf("API Server listening on port %s", port)
		if err := router.Run(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-quit
	log.Printf("Shutting down server (signal: %s)", sig)

	// Graceful shutdown
	outboxPoller.Stop()
	chronosBot.Stop()
	pool.Stop()
}
