package worker

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/Devashish08/taskq/internal/jobhandlers"
	"github.com/Devashish08/taskq/internal/models"
	"github.com/Devashish08/taskq/internal/service"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Worker represents a processing unit that fetches jobs from Redis queue and executes them.
type Worker struct {
	ID          int
	RedisClient *redis.Client
	DB          *sql.DB
}

// NewWorker creates and returns a new Worker instance.
func NewWorker(id int, redisClient *redis.Client, db *sql.DB) *Worker {
	return &Worker{
		ID:          id,
		RedisClient: redisClient,
		DB:          db,
	}
}

// Start begins the worker's main processing loop.
// The worker continuously fetches jobs from Redis, updates their status in the database,
// and executes the appropriate job handler.
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker %d starting loop\n", w.ID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping loop due to context cancellation\n", w.ID)
			return
		default:
			result, err := w.RedisClient.BRPop(ctx, 5*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				log.Printf("Worker %d: ERROR - Failed to BRPOP from Redis: %v. Retrying shortly...\n", w.ID, err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) != 2 {
				log.Printf("Worker %d: ERROR - Unexpected result format from BRPOP: %v\n", w.ID, result)
				continue
			}
			jobIDStr := result[1]
			log.Printf("Worker %d: Received job ID %s from Redis list '%s'\n", w.ID, jobIDStr, service.PendingQueueKey)

			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to parse job ID '%s': %v\n", w.ID, jobIDStr, err)
				continue
			}

			// Start transaction to fetch job and mark as running
			tx, err := w.DB.BeginTx(ctx, nil)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to begin transaction for job %s: %v\n", w.ID, jobID, err)
				continue
			}

			var job models.Job
			selectSQL := `SELECT id, type, payload, status, created_at, updated_at, attempts FROM jobs WHERE id = $1 FOR UPDATE`
			row := tx.QueryRowContext(ctx, selectSQL, jobID)
			err = row.Scan(
				&job.ID, &job.Type, &job.Payload,
				&job.Status, &job.CreatedAt, &job.UpdatedAt, &job.Attempts,
			)

			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("Worker %d: WARNING - Job %s not found in DB. Possibly deleted or an orphan ID.\n", w.ID, jobID)
				} else {
					log.Printf("Worker %d: ERROR - Failed to fetch job %s details from DB: %v\n", w.ID, jobID, err)
				}
				_ = tx.Rollback()
				continue
			}
			log.Printf("Worker %d: Fetched job %s details (Type: %s, Status: %s)\n", w.ID, job.ID, job.Type, job.Status)

			if job.Status != models.StatusPending {
				log.Printf("Worker %d: WARNING - Job %s already has status '%s', skipping.\n", w.ID, job.ID, job.Status)
				_ = tx.Rollback()
				continue
			}

			updateRunningSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
			_, err = tx.ExecContext(ctx, updateRunningSQL, models.StatusRunning, job.ID)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s status to 'running': %v\n", w.ID, job.ID, err)
				_ = tx.Rollback()
				continue
			}
			log.Printf("Worker %d: Updated job %s status to 'running' in transaction\n", w.ID, job.ID)

			err = tx.Commit()
			if err != nil {
				log.Printf("Worker %d: CRITICAL ERROR - Failed to commit transaction for job %s after marking as running: %v\n", w.ID, jobID, err)
				continue
			}
			log.Printf("Worker %d: Committed transaction for job %s (status running)\n", w.ID, job.ID)

			// Process job using registry
			finalStatus := models.StatusCompleted
			var jobProcessingError error

			handler, err := jobhandlers.Get(job.Type)
			if err != nil {
				log.Printf("Worker %d: ERROR - No handler for job type '%s' (Job ID: %s): %v\n", w.ID, job.Type, job.ID, err)
				finalStatus = models.StatusFailed
				jobProcessingError = err
			} else {
				log.Printf("Worker %d: Executing handler for job %s (Type: %s)\n", w.ID, job.ID, job.Type)
				err = handler(job.Payload)
				if err != nil {
					log.Printf("Worker %d: ERROR - Job %s (Type: %s) handler failed: %v\n", w.ID, job.ID, job.Type, err)
					finalStatus = models.StatusFailed
					jobProcessingError = err
				} else {
					log.Printf("Worker %d: Job %s (Type: %s) handler completed successfully.\n", w.ID, job.ID, job.Type)
				}
			}

			// Update final status
			var updateFinalStatusSQL string
			var execArgs []interface{}
			var errMsgToStore sql.NullString

			if finalStatus == models.StatusFailed {
				updateFinalStatusSQL = `UPDATE jobs SET status = $1, error_message = $2, updated_at = NOW() WHERE id = $3`
				if jobProcessingError != nil {
					errMsgToStore = sql.NullString{String: jobProcessingError.Error(), Valid: true}
				} else {
					errMsgToStore = sql.NullString{String: "Unknown processing error", Valid: true}
				}
				execArgs = []interface{}{finalStatus, errMsgToStore, job.ID}
			} else {
				updateFinalStatusSQL = `UPDATE jobs SET status = $1, error_message = NULL, updated_at = NOW() WHERE id = $2`
				execArgs = []interface{}{finalStatus, job.ID}
			}

			_, err = w.DB.ExecContext(ctx, updateFinalStatusSQL, execArgs...)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s final status to '%s': %v\n", w.ID, job.ID, finalStatus, err)
			} else {
				log.Printf("Worker %d: Updated job %s final status to '%s'\n", w.ID, job.ID, finalStatus)
				if finalStatus == models.StatusFailed && errMsgToStore.Valid {
					log.Printf("Worker %d: Stored error for failed job %s: %s\n", w.ID, job.ID, errMsgToStore.String)
				}
			}
		}
	}
}

// WorkerPool manages a collection of Workers.
type WorkerPool struct {
	NumWorkers  int
	RedisClient *redis.Client
	DB          *sql.DB
	wg          sync.WaitGroup
	cancelCtx   context.Context
	cancelFunc  context.CancelFunc
}

// NewWorkerPool creates and returns a new WorkerPool instance.
func NewWorkerPool(numWorkers int, redisClient *redis.Client, db *sql.DB) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		NumWorkers:  numWorkers,
		RedisClient: redisClient,
		DB:          db,
		cancelCtx:   ctx,
		cancelFunc:  cancel,
	}
}

// Start initializes and starts all worker goroutines in the pool.
func (p *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers\n", p.NumWorkers)

	for i := 1; i <= p.NumWorkers; i++ {
		worker := NewWorker(i, p.RedisClient, p.DB)
		p.wg.Add(1)
		go worker.Start(p.cancelCtx, &p.wg)
	}

	log.Println("Worker pool started successfully")
}

// Stop gracefully shuts down the worker pool.
func (p *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	p.cancelFunc()
	p.wg.Wait()
	log.Println("Worker pool stopped successfully")
}
