// Package worker provides a robust job processing system with Redis-backed queuing
// and PostgreSQL persistence. It implements a worker pool pattern with automatic
// retry logic, graceful shutdown, and comprehensive error handling.
package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/Devashish08/taskq/internal/jobhandlers"
	"github.com/Devashish08/taskq/internal/models"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/bwmarrin/discordgo"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Worker represents a single job processing worker that polls Redis for jobs
// and processes them using registered handlers. Each worker maintains its own
// database connection and operates independently within the worker pool.
type Worker struct {
	ID               int                   // Unique identifier for this worker instance
	RedisClient      *redis.Client         // Redis client for job queue operations
	DB               *sql.DB               // Database connection for job persistence
	JobRegistry      *jobhandlers.Registry // Registry of available job handlers
	MaxRetryAttempts int                   // Maximum number of retry attempts per job
	DiscordSession   *discordgo.Session    // Discord session for bot operations
}

// NewWorker creates a new worker instance with the specified configuration.
// The worker will process jobs from Redis using handlers from the provided registry.
//
// Parameters:
//   - id: Unique identifier for the worker (used in logging)
//   - redisClient: Redis client for queue operations
//   - db: Database connection for job state persistence
//   - registry: Job handler registry containing all available job processors
//   - maxRetries: Maximum retry attempts before marking a job as failed
//   - dg: Discord session for bot operations
func NewWorker(id int, redisClient *redis.Client, db *sql.DB, registry *jobhandlers.Registry, maxRetries int, dg *discordgo.Session) *Worker {
	return &Worker{
		ID:               id,
		RedisClient:      redisClient,
		DB:               db,
		JobRegistry:      registry,
		MaxRetryAttempts: maxRetries,
		DiscordSession:   dg,
	}
}

// processJob handles the execution of a single job by looking up the appropriate
// handler and invoking it with the job's payload. The execution is wrapped in
// panic recovery to ensure worker stability.
//
// Parameters:
//   - job: The job to process, containing type, payload, and metadata
//
// Returns:
//   - error: Any error that occurred during job processing
func (w *Worker) processJob(job *models.Job) error {
	handler, err := w.JobRegistry.Get(job.Type)
	if err != nil {
		return fmt.Errorf("unhandled job type: %s", job.Type)
	}

	processingErr := safeInvoke(handler, w.DiscordSession, job.Payload)
	if processingErr != nil {
		return processingErr
	}

	return nil
}

// safeInvoke executes a job handler with panic recovery to prevent worker crashes.
// If a panic occurs during job execution, it's recovered and converted to an error.
// This ensures system stability even when job handlers contain bugs.
//
// Parameters:
//   - h: The job handler function to execute
//   - s: Discord session for bot operations
//   - payload: JSON payload to pass to the handler
//
// Returns:
//   - error: Any error from the handler execution or recovered panic
func safeInvoke(h jobhandlers.JobHandlerFunc, s *discordgo.Session, payload json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			log.Printf("PANIC recovered in job handler: %v\nStack: %s", r, string(stack))
			err = fmt.Errorf("panic in job handler: %v", r)
		}
	}()
	return h(s, payload)
}

// Start begins the worker's main processing loop. The worker continuously polls
// Redis for pending jobs, processes them, and updates their status in the database.
// The loop handles retries, failures, and graceful shutdown via context cancellation.
//
// The processing flow:
// 1. Poll Redis for pending jobs using BRPOP
// 2. Fetch job details from database and mark as running
// 3. Execute the job handler
// 4. Update job status based on execution result
// 5. Requeue failed jobs that haven't exceeded retry limit
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - wg: WaitGroup for coordinated shutdown with other workers
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Poll Redis for pending jobs with 5-second timeout
			result, err := w.RedisClient.BRPop(ctx, 5*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Timeout, no jobs available
				}
				log.Printf("Worker %d: Failed to poll queue: %v", w.ID, err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) != 2 {
				continue
			}

			jobIDStr := result[1]
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				log.Printf("Worker %d: Invalid job ID: %v", w.ID, err)
				continue
			}

			// Process the job
			if err := w.handleJob(ctx, jobID); err != nil {
				log.Printf("Worker %d: Job processing failed: %v", w.ID, err)
			}
		}
	}
}

// handleJob manages the complete lifecycle of a single job from fetching
// to final status update. It uses database transactions for consistency.
func (w *Worker) handleJob(ctx context.Context, jobID uuid.UUID) error {
	// Fetch job details and mark as running in a transaction
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	var job models.Job
	selectSQL := `SELECT id, type, payload, status, created_at, updated_at, attempts FROM jobs WHERE id = $1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, selectSQL, jobID)
	err = row.Scan(&job.ID, &job.Type, &job.Payload, &job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Attempts)

	if err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			return fmt.Errorf("job %s not found", jobID)
		}
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	if job.Status != models.StatusPending {
		_ = tx.Rollback()
		return nil // Job already processed
	}

	// Mark job as running
	updateRunningSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err = tx.ExecContext(ctx, updateRunningSQL, models.StatusRunning, job.ID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to update job status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Process the job
	jobProcessingError := w.processJob(&job)

	// Update final status
	return w.updateJobStatus(ctx, &job, jobProcessingError)
}

// updateJobStatus updates the job's final status after processing and handles retries
func (w *Worker) updateJobStatus(ctx context.Context, job *models.Job, processingErr error) error {
	var finalStatus models.JobStatus
	var errorMessage sql.NullString
	attempts := job.Attempts
	shouldRequeue := false

	if processingErr != nil {
		errorMessage = sql.NullString{String: processingErr.Error(), Valid: true}
		attempts++

		if attempts < w.MaxRetryAttempts {
			finalStatus = models.StatusPending
			shouldRequeue = true
		} else {
			finalStatus = models.StatusFailed
			log.Printf("Job %s failed after %d attempts", job.ID, attempts)
		}
	} else {
		finalStatus = models.StatusCompleted
	}

	// Update job in database
	updateSQL := `UPDATE jobs SET status = $1, error_message = $2, attempts = $3, updated_at = NOW() WHERE id = $4`
	_, err := w.DB.ExecContext(ctx, updateSQL, finalStatus, errorMessage, attempts, job.ID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Requeue for retry if needed
	if shouldRequeue {
		if err := w.RedisClient.LPush(ctx, service.PendingQueueKey, job.ID.String()).Err(); err != nil {
			log.Printf("Failed to requeue job %s: %v", job.ID, err)
		}
	}

	return nil
}

// WorkerPool manages a collection of workers that process jobs concurrently.
// It provides coordinated startup and shutdown of all workers, ensuring
// graceful handling of in-flight jobs during shutdown.
type WorkerPool struct {
	NumWorkers       int                   // Number of worker instances to create
	RedisClient      *redis.Client         // Shared Redis client for all workers
	DB               *sql.DB               // Shared database connection pool
	JobRegistry      *jobhandlers.Registry // Shared job handler registry
	MaxRetryAttempts int                   // Maximum retry attempts for failed jobs
	wg               sync.WaitGroup        // WaitGroup for coordinated shutdown
	cancelCtx        context.Context       // Context for canceling all workers
	cancelFunc       context.CancelFunc    // Function to trigger shutdown
	DiscordSession   *discordgo.Session    // Discord session for bot operations
}

// NewWorkerPool creates a new worker pool with the specified configuration.
// The pool will manage the lifecycle of multiple worker instances, providing
// concurrent job processing with shared resources.
//
// Parameters:
//   - numWorkers: Number of worker instances to create
//   - redisClient: Redis client for job queue operations
//   - db: Database connection pool for job persistence
//   - registry: Job handler registry containing all available processors
//   - maxRetries: Maximum retry attempts before marking jobs as failed
//   - dg: Discord session for bot operations
//
// Returns:
//   - *WorkerPool: Configured worker pool ready to start
func NewWorkerPool(numWorkers int, redisClient *redis.Client, db *sql.DB, registry *jobhandlers.Registry, maxRetries int, dg *discordgo.Session) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		NumWorkers:       numWorkers,
		RedisClient:      redisClient,
		DB:               db,
		JobRegistry:      registry,
		MaxRetryAttempts: maxRetries,
		cancelCtx:        ctx,
		cancelFunc:       cancel,
		wg:               sync.WaitGroup{},
		DiscordSession:   dg,
	}
}

// Start launches all workers in the pool concurrently. Each worker runs
// in its own goroutine and begins processing jobs immediately. This method
// returns quickly after starting all workers; use Stop() for graceful shutdown.
func (p *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers", p.NumWorkers)

	for i := 1; i <= p.NumWorkers; i++ {
		worker := NewWorker(i, p.RedisClient, p.DB, p.JobRegistry, p.MaxRetryAttempts, p.DiscordSession)
		p.wg.Add(1)
		go worker.Start(p.cancelCtx, &p.wg)
	}
}

// Stop initiates graceful shutdown of all workers in the pool. It cancels the
// shared context to signal all workers to stop, then waits for them to complete
// their current jobs and exit cleanly.
//
// This method blocks until all workers have stopped. In-flight jobs will be
// completed before workers exit, ensuring no work is lost during shutdown.
func (p *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	p.cancelFunc()
	p.wg.Wait()
	log.Println("Worker pool stopped")
}
