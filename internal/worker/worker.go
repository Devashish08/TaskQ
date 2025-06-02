package worker

import (
	"context"
	"database/sql"
	"encoding/json" // Keep if safeInvoke uses it, though payload is already RawMessage
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

// Worker struct - (no changes from your last version)
type Worker struct {
	ID               int
	RedisClient      *redis.Client
	DB               *sql.DB
	JobRegistry      *jobhandlers.Registry
	MaxRetryAttempts int
}

// NewWorker constructor - (no changes from your last version)
func NewWorker(id int, redisClient *redis.Client, db *sql.DB, registry *jobhandlers.Registry, maxRetries int) *Worker {
	return &Worker{
		ID:               id,
		RedisClient:      redisClient,
		DB:               db,
		JobRegistry:      registry,
		MaxRetryAttempts: maxRetries,
	}
}

// safeInvoke - (no changes from your last version)
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

// processJob method - (no changes from your last version)
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

// Start method: The main loop with corrected retry logic
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker %d starting loop (max retries: %d)\n", w.ID, w.MaxRetryAttempts)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping loop due to context cancellation.\n", w.ID)
			return
		default:
			// --- BRPOP (No changes here from your last version) ---
			result, err := w.RedisClient.BRPop(ctx, 5*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue
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
			// --- End BRPOP ---

			// --- Transaction to Fetch and Mark as Running (No changes here from your last version) ---
			tx, err := w.DB.BeginTx(ctx, nil)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to begin tx for job %s: %v", w.ID, jobID, err)
				continue
			}
			var job models.Job
			selectSQL := `SELECT id, type, payload, status, created_at, updated_at, attempts FROM jobs WHERE id = $1 FOR UPDATE`
			row := tx.QueryRowContext(ctx, selectSQL, jobID)
			err = row.Scan(&job.ID, &job.Type, &job.Payload, &job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Attempts)
			if err != nil { /* ... handle ErrNoRows, other errors, rollback, continue ... */
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
			// --- End Transaction ---

			// --- Process Job ---
			jobProcessingError := w.processJob(&job)
			// --- End Process Job ---

			// --- **** REVISED FINAL STATUS UPDATE AND RETRY LOGIC **** ---
			var finalDbStatusToSet models.JobStatus
			var dbErrorMessage sql.NullString
			dbUpdateAttempts := job.Attempts // Start with fetched attempts
			shouldRequeueToRedis := false

			if jobProcessingError != nil {
				dbErrorMessage = sql.NullString{String: jobProcessingError.Error(), Valid: true}
				dbUpdateAttempts++ // Increment attempts because this run failed
				log.Printf("Worker %d: Job %s FAILED. Attempt %d/%d. Error: %v", w.ID, job.ID, dbUpdateAttempts, w.MaxRetryAttempts, jobProcessingError)

				if dbUpdateAttempts < w.MaxRetryAttempts {
					finalDbStatusToSet = models.StatusPending // Mark for retry
					shouldRequeueToRedis = true               // Will requeue if DB update is successful
					log.Printf("Worker %d: Job %s will be retried.", w.ID, job.ID)
				} else {
					finalDbStatusToSet = models.StatusFailed // Max retries reached
					log.Printf("Worker %d: Job %s reached max retries. Marking as failed.", w.ID, job.ID)
				}
			} else {
				finalDbStatusToSet = models.StatusCompleted
				// dbUpdateAttempts remains as fetched if successful on first true try,
				// or if it was a retry that succeeded.
				log.Printf("Worker %d: Job %s COMPLETED successfully.", w.ID, job.ID)
			}

			// Update the database with the determined final state for this attempt
			updateFinalStatusSQL := `UPDATE jobs SET status = $1, error_message = $2, attempts = $3, updated_at = NOW() WHERE id = $4`
			_, dbUpdateErr := w.DB.ExecContext(ctx, updateFinalStatusSQL, finalDbStatusToSet, dbErrorMessage, dbUpdateAttempts, job.ID)

			if dbUpdateErr != nil {
				log.Printf("Worker %d: CRITICAL ERROR - Failed to update job %s in DB to status '%s', attempts %d: %v", w.ID, job.ID, finalDbStatusToSet, dbUpdateAttempts, dbUpdateErr)
				// If DB update fails, we do NOT requeue, even if it was a retry attempt.
				// The job's state in DB might be 'running' or inconsistent. This needs monitoring.
			} else {
				log.Printf("Worker %d: Successfully updated job %s in DB to status '%s', attempts %d.", w.ID, job.ID, finalDbStatusToSet, dbUpdateAttempts)
				// Requeue to Redis ONLY if it was a retry attempt AND DB update for retry was successful
				if shouldRequeueToRedis {
					errRequeue := w.RedisClient.LPush(ctx, service.PendingQueueKey, job.ID.String()).Err()
					if errRequeue != nil {
						log.Printf("Worker %d: CRITICAL ERROR - Failed to REQUEUE job %s for retry after DB update: %v. Job is '%s' in DB but not in queue!", w.ID, job.ID, errRequeue, finalDbStatusToSet)
					} else {
						log.Printf("Worker %d: Requeued job %s to Redis for retry.", w.ID, job.ID)
					}
				}
			}
			// --- **** END REVISED LOGIC **** ---
		} // End default case for select
	} // End select
} // End for loop
// End Start method

// WorkerPool struct - (no changes from your last version)
type WorkerPool struct {
	NumWorkers       int
	RedisClient      *redis.Client
	DB               *sql.DB
	JobRegistry      *jobhandlers.Registry
	MaxRetryAttempts int
	wg               sync.WaitGroup
	cancelCtx        context.Context
	cancelFunc       context.CancelFunc
}

// NewWorkerPool constructor - (no changes from your last version)
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

// Start method for WorkerPool - (no changes from your last version)
func (p *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers (max retries per job: %d)\n", p.NumWorkers, p.MaxRetryAttempts)
	for i := 1; i <= p.NumWorkers; i++ {
		worker := NewWorker(i, p.RedisClient, p.DB, p.JobRegistry, p.MaxRetryAttempts)
		p.wg.Add(1)
		go worker.Start(p.cancelCtx, &p.wg)
	}
	log.Println("Worker pool started successfully.")
}

// Stop method for WorkerPool - (no changes from your last version)
func (p *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	p.cancelFunc()
	p.wg.Wait()
	log.Println("Worker pool stopped successfully.")
}
