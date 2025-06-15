package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Devashish08/taskq/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// PendingQueueKey is the Redis key for the pending jobs queue
	PendingQueueKey    = `taskq:pending_jobs`
	jobQueueBufferSize = 100
)

// JobService handles job persistence and queue management
type JobService struct {
	redisClient *redis.Client
	db          *sql.DB
}

// NewJobService creates a new job service instance
// Parameters:
//   - db: Database connection for job persistence
//   - redisClient: Redis client for queue operations
//
// Returns:
//   - *JobService: Configured job service
func NewJobService(db *sql.DB, redisClient *redis.Client) *JobService {
	return &JobService{
		redisClient: redisClient,
		db:          db,
	}
}

// SubmitJob creates a new job and adds it to the processing queue
// Uses database transactions to ensure atomicity between job creation and queue insertion
// Parameters:
//   - jobType: Type identifier for job handler selection
//   - payload: Job-specific data as key-value pairs
//   - executeAt: Optional scheduled execution time (nil for immediate execution)
//
// Returns:
//   - *models.Job: Created job with assigned ID
//   - error: Any error during job creation or queuing
func (s *JobService) SubmitJob(jobType string, payload map[string]interface{}, executeAt *time.Time) (*models.Job, error) {
	job, err := models.NewJob(jobType, payload, executeAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create job structure: %w", err)
	}

	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not begin transaction: %w", err)
	}

	defer tx.Rollback()

	// Insert job into database
	jobInsertSQL := `
        INSERT INTO jobs (id, type, payload, status, created_at, updated_at, execute_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = tx.ExecContext(ctx, jobInsertSQL,
		job.ID, job.Type, job.Payload, job.Status,
		job.CreatedAt, job.UpdatedAt, job.ExecuteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("job insert failed: %w", err)
	}

	// Insert outbox event for transactional outbox pattern
	outboxInsertSQL := `INSERT INTO job_queue_outbox (job_id, created_at) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, outboxInsertSQL, job.ID, job.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("outbox insert failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("db transaction commit failed: %w", err)
	}

	return job, nil
}

// GetJobStatus retrieves the current status and details of a job
// Parameters:
//   - jobID: UUID of the job to query
//
// Returns:
//   - *models.Job: Job details including status, attempts, and error messages
//   - error: sql.ErrNoRows if job not found, or other database errors
func (s *JobService) GetJobStatus(jobID uuid.UUID) (*models.Job, error) {
	var job models.Job
	ctx := context.Background()

	selectSQL := `
		SELECT id, type, payload, status, created_at, updated_at, attempts, error_message
		FROM jobs
		WHERE id = $1`

	row := s.db.QueryRowContext(ctx, selectSQL, jobID)
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Payload,
		&job.Status,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.Attempts,
		&job.ErrorMessage,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query or scan job %s: %w", jobID, err)
	}

	return &job, nil
}

// ListRecentJobs retrieves the most recent jobs ordered by creation time
// Parameters:
//   - limit: Maximum number of jobs to return (capped at 100)
//
// Returns:
//   - []models.Job: List of recent jobs with full details
//   - error: Any database query errors
func (s *JobService) ListRecentJobs(limit int) ([]models.Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	ctx := context.Background()
	query := `
		SELECT id, type, payload, status, created_at, updated_at, attempts, error_message
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Payload,
			&job.Status,
			&job.CreatedAt,
			&job.UpdatedAt,
			&job.Attempts,
			&job.ErrorMessage,
		); err != nil {
			return jobs, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return jobs, fmt.Errorf("error iterating job rows: %w", err)
	}

	return jobs, nil
}
