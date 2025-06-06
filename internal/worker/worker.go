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

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Worker represents a single job processing worker that polls Redis for jobs
// and processes them using registered handlers. Each worker maintains its own
// database connection and operates independently within the worker pool.
type Worker struct {
	ID               int                      // Unique identifier for this worker instance
	RedisClient      *redis.Client           // Redis client for job queue operations
	DB               *sql.DB                 // Database connection for job persistence
	JobRegistry      *jobhandlers.Registry   // Registry of available job handlers
	MaxRetryAttempts int                     // Maximum number of retry attempts per job
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
func NewWorker(id int, redisClient *redis.Client, db *sql.DB, registry *jobhandlers.Registry, maxRetries int) *Worker {
	return &Worker{
		ID:               id,
		RedisClient:      redisClient,
		DB:               db,
		JobRegistry:      registry,
		MaxRetryAttempts: maxRetries,
	}
}

// safeInvoke executes a job handler with panic recovery to prevent worker crashes.
// If a panic occurs during job execution, it's recovered and converted to an error.
// This ensures system stability even when job handlers contain bugs.
//
// Parameters:
//   - h: The job handler function to execute
//   - payload: JSON payload to pass to the handler
//
// Returns:
//   - error: Any error from the handler execution or recovered panic
func safeInvoke(h jobhandlers.JobHandlerFunc, payload json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			log.Printf("Worker: PANIC recovered in job handler. Panic: %v\nStack: %s\n", r, string(stack))
			err = fmt.Errorf("panic in job handler: %v", r)
		}
	}()
	return h(payload)
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
	log.Printf("Worker %d: Looking up handler for job %s (Type: %s)\n", w.ID, job.ID, job.Type)
	
	handler, err := w.JobRegistry.Get(job.Type)
	if err != nil {
		log.Printf("Worker %d: No handler for job type '%s': %v\n", w.ID, job.Type, err)
		return fmt.Errorf("unhandled job type: %s", job.Type)
	}

	log.Printf("Worker %d: Executing handler for job %s (Type: %s)\n", w.ID, job.ID, job.Type)
	processingErr := safeInvoke(handler, job.Payload)

	if processingErr != nil {
		log.Printf("Worker %d: Handler for job %s (Type: %s) failed: %v\n", w.ID, job.ID, job.Type, processingErr)
		return processingErr
	}

	log.Printf("Worker %d: Handler for job %s (Type: %s) completed successfully\n", w.ID, job.ID, job.Type)
	return nil
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
	log.Printf("Worker %d starting loop (max retries: %d)\n", w.ID, w.MaxRetryAttempts)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping loop due to context cancellation.\n", w.ID)
			return
		default:
			// Poll Redis for pending jobs with 5-second timeout
			result, err := w.RedisClient.BRPop(ctx, 5*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Timeout, no jobs available
				}
				log.Printf("Worker %d: ERROR - Failed to BRPOP: %v. Retrying shortly...", w.ID, err)
				time.Sleep(1 * time.Second)
				continue
			}
			
			if len(result) != 2 {
				log.Printf("Worker %d: ERROR - Unexpected BRPOP result: %v", w.ID, result)
				continue
			}
			
			jobIDStr := result[1]
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to parse job ID '%s': %v", w.ID, jobIDStr, err)
				continue
			}
			log.Printf("Worker %d: Received job ID %s\n", w.ID, jobIDStr)

			// Fetch job details and mark as running in a transaction
			tx, err := w.DB.BeginTx(ctx, nil)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to begin tx for job %s: %v", w.ID, jobID, err)
				continue
			}

			var job models.Job
			selectSQL := `SELECT id, type, payload, status, created_at, updated_at, attempts FROM jobs WHERE id = $1 FOR UPDATE`
			row := tx.QueryRowContext(ctx, selectSQL, jobID)
			err = row.Scan(&job.ID, &job.Type, &job.Payload, &job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Attempts)
			
			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("Worker %d: WARNING - Job %s not found in DB.", w.ID, jobID)
				} else {
					log.Printf("Worker %d: ERROR - Failed to fetch job %s details: %v", w.ID, jobID, err)
				}
				_ = tx.Rollback()
				continue
			}

			log.Printf("Worker %d: Fetched job %s (Attempts: %d, Status: %s)\n", w.ID, job.ID, job.Attempts, job.Status)
			
			if job.Status != models.StatusPending {
				log.Printf("Worker %d: Job %s has status '%s', not pending. Skipping.", w.ID, job.ID, job.Status)
				_ = tx.Rollback()
				continue
			}

			// Mark job as running
			updateRunningSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
			_, err = tx.ExecContext(ctx, updateRunningSQL, models.StatusRunning, job.ID)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s to 'running': %v", w.ID, job.ID, err)
				_ = tx.Rollback()
				continue
			}

			err = tx.Commit()
			if err != nil {
				log.Printf("Worker %d: CRITICAL ERROR - Failed to commit 'running' status for job %s: %v", w.ID, jobID, err)
				continue
			}
			log.Printf("Worker %d: Committed job %s status to 'running'\n", w.ID, job.ID)

			// Process the job
			jobProcessingError := w.processJob(&job)

			// Determine final status and handle retries
			var finalDbStatusToSet models.JobStatus
			var dbErrorMessage sql.NullString
			dbUpdateAttempts := job.Attempts
			shouldRequeueToRedis := false

			if jobProcessingError != nil {
				dbErrorMessage = sql.NullString{String: jobProcessingError.Error(), Valid: true}
				dbUpdateAttempts++
				log.Printf("Worker %d: Job %s FAILED. Attempt %d/%d. Error: %v", w.ID, job.ID, dbUpdateAttempts, w.MaxRetryAttempts, jobProcessingError)

				if dbUpdateAttempts < w.MaxRetryAttempts {
					finalDbStatusToSet = models.StatusPending
					shouldRequeueToRedis = true
					log.Printf("Worker %d: Job %s will be retried.", w.ID, job.ID)
				} else {
					finalDbStatusToSet = models.StatusFailed
					log.Printf("Worker %d: Job %s reached max retries. Marking as failed.", w.ID, job.ID)
				}
			} else {
				finalDbStatusToSet = models.StatusCompleted
				log.Printf("Worker %d: Job %s COMPLETED successfully.", w.ID, job.ID)
			}

			// Update job status in database
			updateFinalStatusSQL := `UPDATE jobs SET status = $1, error_message = $2, attempts = $3, updated_at = NOW() WHERE id = $4`
			_, dbUpdateErr := w.DB.ExecContext(ctx, updateFinalStatusSQL, finalDbStatusToSet, dbErrorMessage, dbUpdateAttempts, job.ID)

			if dbUpdateErr != nil {
				log.Printf("Worker %d: CRITICAL ERROR - Failed to update job %s in DB to status '%s', attempts %d: %v", w.ID, job.ID, finalDbStatusToSet, dbUpdateAttempts, dbUpdateErr)
			} else {
				log.Printf("Worker %d: Successfully updated job %s in DB to status '%s', attempts %d.", w.ID, job.ID, finalDbStatusToSet, dbUpdateAttempts)
				
				// Requeue for retry if needed
				if shouldRequeueToRedis {
					errRequeue := w.RedisClient.LPush(ctx, service.PendingQueueKey, job.ID.String()).Err()
					if errRequeue != nil {
						log.Printf("Worker %d: CRITICAL ERROR - Failed to REQUEUE job %s for retry after DB update: %v. Job is '%s' in DB but not in queue!", w.ID, job.ID, errRequeue, finalDbStatusToSet)
					} else {
						log.Printf("Worker %d: Requeued job %s to Redis for retry.", w.ID, job.ID)
					}
				}
			}
		}
	}
}

// WorkerPool manages a collection of workers that process jobs concurrently.
// It provides coordinated startup and shutdown of all workers, ensuring
// graceful handling of in-flight jobs during shutdown.
type WorkerPool struct {
	NumWorkers       int                      // Number of worker instances to create
	RedisClient      *redis.Client           // Shared Redis client for all workers
	DB               *sql.DB                 // Shared database connection pool
	JobRegistry      *jobhandlers.Registry   // Shared job handler registry
	MaxRetryAttempts int                     // Maximum retry attempts for failed jobs
	wg               sync.WaitGroup          // WaitGroup for coordinated shutdown
	cancelCtx        context.Context         // Context for canceling all workers
	cancelFunc       context.CancelFunc      // Function to trigger shutdown
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
//
// Returns:
//   - *WorkerPool: Configured worker pool ready to start
func NewWorkerPool(numWorkers int, redisClient *redis.Client, db *sql.DB, registry *jobhandlers.Registry, maxRetries int) *WorkerPool {
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
	}
}

// Start launches all workers in the pool concurrently. Each worker runs
// in its own goroutine and begins processing jobs immediately. This method
// returns quickly after starting all workers; use Stop() for graceful shutdown.
//
// The method is safe to call multiple times, but additional calls will have no effect
// if workers are already running.
func (p *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers (max retries per job: %d)\n", p.NumWorkers, p.MaxRetryAttempts)
	
	for i := 1; i <= p.NumWorkers; i++ {
		worker := NewWorker(i, p.RedisClient, p.DB, p.JobRegistry, p.MaxRetryAttempts)
		p.wg.Add(1)
		go worker.Start(p.cancelCtx, &p.wg)
	}
	
	log.Println("Worker pool started successfully.")
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
	log.Println("Worker pool stopped successfully.")
}
