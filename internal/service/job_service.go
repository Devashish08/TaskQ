package service

import (
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/models"
)

const (
	jobQueueBufferSize = 100
)

type JobService struct {
	jobQueue chan models.Job
}

func NewJobService() *JobService {
	jobQueue := make(chan models.Job, jobQueueBufferSize)

	return &JobService{
		jobQueue: jobQueue,
	}
}

func (s *JobService) SubmitJob(jobType string, payload map[string]interface{}) (*models.Job, error) {
	job, err := models.NewJob(jobType, payload)
	if err != nil {
		log.Printf("error creating job: %v\n", err)
		return nil, fmt.Errorf("failed to create job structure: %w", err)
	}

	s.jobQueue <- *job
	fmt.Printf("JobService: Enqueued job %s of type %s\n", job.ID, job.Type)

	return job, nil
}

func (s *JobService) GetJobQueue() <-chan models.Job {
	return s.jobQueue
}
