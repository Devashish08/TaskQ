package service

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/models"
)

const (
	jobQueueBufferSize = 100
)

type JobService struct {
	jobQueue chan models.Job
	db       *sql.DB
}

func NewJobService(db *sql.DB) *JobService {
	jobQueue := make(chan models.Job, jobQueueBufferSize)

	return &JobService{
		jobQueue: jobQueue,
		db:       db,
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

	s.jobQueue <- *job
	fmt.Printf("JobService: Enqueued job %s (type: %s) to in-memory channel\n", job.ID, job.Type)

	return job, nil
}

func (s *JobService) GetJobQueue() <-chan models.Job {
	return s.jobQueue
}
