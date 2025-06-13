// Package main implements the TaskQ server application.
// TaskQ is a distributed task queue system that provides job scheduling,
// execution, and management capabilities using Redis as a message broker
// and PostgreSQL for persistence.
package main

import (
	"log"
	"math/rand" // For jobhandlers
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time" // For jobhandlers

	"github.com/Devashish08/taskq/internal/api"
	"github.com/Devashish08/taskq/internal/bot"
	"github.com/Devashish08/taskq/internal/config"
	"github.com/Devashish08/taskq/internal/database"
	"github.com/Devashish08/taskq/internal/jobhandlers" // Import jobhandlers
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
	// Seed the random number generator once at application startup.
	// This is important for the HandlePotentiallyFailingJob to have varied behavior.
	rand.Seed(time.Now().UnixNano())
	log.Println("Starting TaskQ server...") // Moved log to after seeding

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
	defer func() {
		log.Println("Closing database connection pool...")
		if cerr := dbPool.Close(); cerr != nil {
			log.Printf("ERROR closing database connection: %v\n", cerr)
		}
	}()

	if err := database.MigrateDB(dbPool); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	// Connect to Redis
	redisClient, err := redisclient.ConnectRedis(appCfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		log.Println("Closing Redis connection...")
		if cerr := redisClient.Close(); cerr != nil {
			log.Printf("ERROR closing Redis connection: %v\n", cerr)
		}
	}()

	// --- Create and Populate Job Handler Registry ---
	jobHandlerRegistry := jobhandlers.NewRegistry()
	// Register the handlers onto the specific registry instance.
	// Use consistent job type strings that you'll use in API requests.
	jobHandlerRegistry.Register("send_discord_reminder", jobhandlers.HandleDiscordReminder)
	jobHandlerRegistry.Register("log_payload", jobhandlers.HandleSimpleLogJob)
	jobHandlerRegistry.Register("failing_job", jobhandlers.HandlePotentiallyFailingJob)
	jobHandlerRegistry.Register("long_job", jobhandlers.HandleLongRunningJob)
	// --- End Registry Setup ---

	// Initialize Job Service with DB and Redis clients
	jobService := service.NewJobService(dbPool, redisClient)

	chronosBot, err := bot.NewBot(appCfg.Bot.Token, jobService)
	if err != nil {
		log.Fatalf("Failed to create chronos bot: %v", err)
	}

	bot.BotInstance = chronosBot
	// Initialize API Handler with Job Service
	apiHandler := api.NewApiHandler(jobService)

	// Setup API Router
	router := api.SetupRouterWithDeps(apiHandler)

	// Initialize and Start Worker Pool
	// Pass all dependencies including the populated jobHandlerRegistry.
	numWorkers := 3 // TODO: Make this configurable from appCfg.Worker
	pool := worker.NewWorkerPool(
		numWorkers,
		redisClient,
		dbPool,
		jobHandlerRegistry, // Pass the registry instance
		appCfg.Worker.MaxRetryAttempts,
		chronosBot.Session,
	)
	pool.Start() // Starts worker goroutines

	err = chronosBot.Start()
	if err != nil {
		log.Fatalf("Failed to start chronos bot: %v", err)
	}

	// --- HTTP Server Start and Graceful Shutdown Handling ---
	// Channel to listen for OS shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in a separate goroutine
	go func() {
		port := "8080" // TODO: Make this configurable
		log.Printf("API Server Listening on port %s", port)
		if err := router.Run(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Block main goroutine until a shutdown signal is received
	sig := <-quit
	log.Printf("Received signal: %s. Initiating graceful shutdown...", sig)

	// Stop the worker pool. This will signal workers and wait for them to finish.
	chronosBot.Stop()
	pool.Stop()

	log.Println("TaskQ server shutdown sequence complete. Deferred cleanups (DB, Redis close) will now run.")
}
