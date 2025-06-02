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
	"github.com/Devashish08/taskq/internal/config"
	"github.com/Devashish08/taskq/internal/database"
	"github.com/Devashish08/taskq/internal/jobhandlers" // Import jobhandlers
	redisclient "github.com/Devashish08/taskq/internal/redisClient"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/Devashish08/taskq/internal/worker"
)

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
	jobHandlerRegistry.Register("log_payload", jobhandlers.HandleSimpleLogJob)
	jobHandlerRegistry.Register("failing_job", jobhandlers.HandlePotentiallyFailingJob)
	jobHandlerRegistry.Register("long_job", jobhandlers.HandleLongRunningJob)
	// --- End Registry Setup ---

	// Initialize Job Service with DB and Redis clients
	jobService := service.NewJobService(dbPool, redisClient)

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
	)
	pool.Start() // Starts worker goroutines

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
	pool.Stop()

	log.Println("TaskQ server shutdown sequence complete. Deferred cleanups (DB, Redis close) will now run.")
}
