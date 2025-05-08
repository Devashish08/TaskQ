package worker

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/Devashish08/taskq/internal/models"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Worker represents a processing unit that fetches jobs from a Redis queue,
// updates their status in a database, and executes the job logic.
// Each worker runs in its own goroutine.
type Worker struct {
	ID          int           // ID is a unique identifier for the worker.
	RedisClient *redis.Client // RedisClient is the client used to communicate with Redis for job queuing.
	DB          *sql.DB       // DB is the database connection used for job persistence and status updates.
}

// NewWorker creates and returns a new Worker instance.
// It initializes a worker with a unique ID, a Redis client, and a database connection.
func NewWorker(id int, redisClient *redis.Client, db *sql.DB) *Worker {
	return &Worker{
		ID:          id,
		RedisClient: redisClient,
		DB:          db,
	}
}

// Start begins the worker's main processing loop.
// It continuously attempts to fetch jobs from the configured Redis pending queue
// using a blocking pop (BRPOP). Once a job is retrieved, it's processed,
// and its status is updated in the database.
// The loop continues until the provided context (ctx) is canceled.
// The wg (WaitGroup) is decremented when the worker stops.
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker %d starting\n", w.ID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping loop due to context cancellation\n", w.ID)
			return
		default:
			// Fetching Job from Redis (w.RedisClient.BRPop)
			result, err := w.RedisClient.BRPop(ctx, 5*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				log.Printf("Worker %d: ERROR - Failed to BRPOP from Redis: %v", w.ID, err)
				time.Sleep(1 * time.Second)
			} // Condensed error
			if len(result) != 2 {
				log.Printf("Worker %d: Unexpected BRPOP result: %v", w.ID, result)
				continue
			}
			jobIDStr := result[1]
			log.Printf("Worker %d: Received job ID %s from Redis list '%s'\n", w.ID, jobIDStr, service.PendingQueueKey)
			// --- End Redis BRPOP ---

			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to parse job ID '%s': %v", w.ID, jobIDStr, err)
				continue
			}

			// --- Transaction to Fetch and Mark as Running ---
			tx, err := w.DB.BeginTx(ctx, nil)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to begin transaction for job %s: %v", w.ID, jobID, err)
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
				_ = tx.Rollback() // Use underscore to ignore rollback error here
				continue
			}
			log.Printf("Worker %d: Fetched job %s details (Status: %s)\n", w.ID, job.ID, job.Status)

			if job.Status != models.StatusPending {
				log.Printf("Worker %d: WARNING - Job %s already has status '%s', skipping.", w.ID, job.ID, job.Status)
				_ = tx.Rollback()
				continue
			}

			updateRunningSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
			_, err = tx.ExecContext(ctx, updateRunningSQL, models.StatusRunning, job.ID)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s status to 'running': %v", w.ID, job.ID, err)
				_ = tx.Rollback()
				continue
			}
			log.Printf("Worker %d: Updated job %s status to 'running' in transaction\n", w.ID, job.ID)

			err = tx.Commit()
			if err != nil {
				log.Printf("Worker %d: CRITICAL ERROR - Failed to commit transaction for job %s after marking as running: %v", w.ID, jobID, err)
				continue
			}

			var jobToProcess models.Job

			w.DB.QueryRowContext(ctx, `SELECT id, type, payload FROM jobs WHERE id = $1`, jobID).Scan(&jobToProcess.ID, &jobToProcess.Type, &jobToProcess.Payload)
			w.processJob(&jobToProcess)
			updateCompletedSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
			_, err = w.DB.ExecContext(ctx, updateCompletedSQL, models.StatusCompleted, job.ID)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s status to 'completed': %v", w.ID, job.ID, err)
			} else {
				log.Printf("Worker %d: Updated job %s status to 'completed'\n", w.ID, job.ID)
			}
			// --- End Update Status ---
		}
	}
}

// processJob simulates the actual execution of a job.
// It logs the beginning and end of processing for the given job
// and includes a simulated work delay.
// In a real application, this method would contain the specific logic
// for handling different job types.
func (w *Worker) processJob(job *models.Job) {
	// --- FIX Printf Statement ---
	log.Printf("Worker %d: Processing job %s (Type: %s)\n", w.ID, job.ID, job.Type)
	// --- End FIX ---
	log.Printf("Worker %d: Working on job %s...", w.ID, job.ID)
	time.Sleep(2 * time.Second)
	log.Printf("Worker %d: Finished Processing job %s\n", w.ID, job.ID) // Changed message slightly
}

// WorkerPool manages a collection of Workers.
// It is responsible for starting, stopping, and coordinating multiple worker goroutines.
type WorkerPool struct {
	NumWorkers  int                // NumWorkers is the desired number of worker goroutines in the pool.
	RedisClient *redis.Client      // RedisClient is the Redis client shared by all workers in the pool.
	DB          *sql.DB            // DB is the database connection shared by all workers in the pool.
	wg          sync.WaitGroup     // wg is used to wait for all worker goroutines to finish upon stopping.
	cancelCtx   context.Context    // cancelCtx is the context used to signal workers to stop.
	cancelFunc  context.CancelFunc // cancelFunc is the function to call to cancel cancelCtx.
}

// NewWorkerPool creates and returns a new WorkerPool instance.
// It initializes the pool with the specified number of workers, a Redis client,
// a database connection, and sets up a cancellable context for managing worker lifecycle.
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
// Each worker is created and launched in its own goroutine.
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
// It cancels the context shared with the workers, signaling them to stop,
// and then waits for all worker goroutines to complete their current tasks and exit.
func (p *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	p.cancelFunc()
	p.wg.Wait()
	log.Println("Worker pool stopped successfully")
}
