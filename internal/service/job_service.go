package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	PendingQueueKey    = `taskq:pending_jobs`
	jobQueueBufferSize = 100
)

type JobService struct {
	redisClient *redis.Client
	db          *sql.DB
}

func NewJobService(db *sql.DB, redisClient *redis.Client) *JobService {
	return &JobService{
		redisClient: redisClient,
		db:          db,
	}
}

func (s *JobService) SubmitJob(jobType string, payload map[string]interface{}) (*models.Job, error) {
	job, err := models.NewJob(jobType, payload)
	if err != nil {
		log.Printf("error creating job: %v\n", err)
		return nil, fmt.Errorf("failed to create job structure: %w", err)
	}

	insertSQL := `
        INSERT INTO jobs (id, type, payload, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = s.db.Exec(insertSQL,
		job.ID,
		job.Type,
		job.Payload,
		job.Status,
		job.CreatedAt,
		job.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error inserting job %s into database: %v\n", job.ID, err)
		return nil, fmt.Errorf("database insert failed: %w", err)
	}
	log.Printf("JobService: Inserted job %s into database\n", job.ID)

	ctx := context.Background()
	jobIDStr := job.ID.String()

	err = s.redisClient.LPush(ctx, PendingQueueKey, jobIDStr).Err()
	if err != nil {
		log.Printf("CRITICAL: Failed to push job %s to Redis queue after DB insert: %v\n", jobIDStr, err)
		return job, fmt.Errorf("failed to enqueue job after saving: %w", err)
	}

	log.Printf("JobService: Enqueued job %s to Redis list '%s'\n", jobIDStr, PendingQueueKey)

	return job, nil
}

// In internal/service/job_service.go
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
		log.Printf("ERROR fetching job %s: %v\n", jobID, err)
		return nil, fmt.Errorf("failed to query or scan job %s: %w", jobID, err)
	}

	return &job, nil
}
