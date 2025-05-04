package worker

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/Devashish08/taskq/internal/models"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Worker struct definition... (no changes needed here)
type Worker struct {
	ID          int
	RedisClient *redis.Client
	DB          *sql.DB
}

// NewWorker constructor... (no changes needed here)
func NewWorker(id int, redisClient *redis.Client, db *sql.DB) *Worker {
	return &Worker{
		ID:          id,
		RedisClient: redisClient,
		DB:          db,
	}
}

// Start begins the worker's loop, using BRPOP.
func (w *Worker) Start() {
	log.Printf("Worker %d starting\n", w.ID) // Changed log message slightly for consistency

	ctx := context.Background()

	go func() {
		for {
			// --- Redis BRPOP ---
			result, err := w.RedisClient.BRPop(ctx, 0*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				log.Printf("Worker %d: Error on BRPOP: %v...", w.ID, err)
				time.Sleep(5 * time.Second)
				continue
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
			log.Printf("Worker %d: Committed transaction for job %s (status running)\n", w.ID, job.ID)
			updateCompletedSQL := `UPDATE jobs SET status = $1, updated_at = NOW() WHERE id = $2`
			_, err = w.DB.ExecContext(ctx, updateCompletedSQL, models.StatusCompleted, job.ID)
			if err != nil {
				log.Printf("Worker %d: ERROR - Failed to update job %s status to 'completed': %v", w.ID, job.ID, err)
			} else {
				log.Printf("Worker %d: Updated job %s status to 'completed'\n", w.ID, job.ID)
			}
			// --- End Update Status ---
		}
	}()
}

// processJob definition...
func (w *Worker) processJob(job *models.Job) {
	// --- FIX Printf Statement ---
	log.Printf("Worker %d: Processing job %s (Type: %s)\n", w.ID, job.ID, job.Type)
	// --- End FIX ---
	log.Printf("Worker %d: Working on job %s...", w.ID, job.ID)
	time.Sleep(2 * time.Second)
	log.Printf("Worker %d: Finished Processing job %s\n", w.ID, job.ID) // Changed message slightly
}
