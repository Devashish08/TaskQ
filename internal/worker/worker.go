package worker

import (
	"log"
	"time"

	"github.com/Devashish08/taskq/internal/models"
)

// Worker struct might hold dependencies later (like DB connection)
type Worker struct {
	ID       int
	JobQueue <-chan models.Job
}

// NewWorker creates a new worker instance.
func NewWorker(id int, jobQueue <-chan models.Job) *Worker {
	return &Worker{
		ID:       id,
		JobQueue: jobQueue,
	}
}

// Start begins the worker's loop in a new goroutine.
func (w *Worker) Start() {
	log.Printf("Worker %d starting\n", w.ID)
	go func() {
		for job := range w.JobQueue {
			w.processJob(job)
		}

		log.Printf("Worker %d shutting down\n", w.ID)
	}()
}

func (w *Worker) processJob(job models.Job) {
	log.Printf("Worker %d: Received job %s (Type: %s)\n", w.ID, job.ID, job.Type)

	// TODO: Decode job.Payload based on job.Type
	// TODO: perform actual work based on job.Type

	log.Printf("Worker %d: Processing job %s...\n", w.ID, job.ID)
	time.Sleep(2 * time.Second)

	log.Printf("Worker %d: Finished processing job %s\n", w.ID, job.ID)
}
