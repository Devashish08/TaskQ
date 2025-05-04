package worker

import (
	"context"
	"log"
	"time"

	"github.com/Devashish08/taskq/internal/service"
	"github.com/redis/go-redis/v9"
)

// Worker struct might hold dependencies later (like DB connection)
type Worker struct {
	ID          int
	RedisClient *redis.Client
}

// NewWorker creates a new worker instance.
func NewWorker(id int, redisClient *redis.Client /*, dbPool *sql.DB */) *Worker {
	return &Worker{
		ID:          id,
		RedisClient: redisClient,
	}
}

// Start begins the worker's loop, using BRPOP.
func (w *Worker) Start() {
	log.Printf("Worker %d: Starting...\n", w.ID)

	ctx := context.Background()

	go func() {
		for {
			result, err := w.RedisClient.BRPop(ctx, 0*time.Second, service.PendingQueueKey).Result()
			if err != nil {
				log.Printf("Worker %d: Error on BRPOP from Redis: %v. Retrying shortly...\n", w.ID, err)
				time.Sleep(5 * time.Second)
				continue
			}

			if len(result) != 2 {
				log.Printf("Worker %d: Unexpected result from BRPOP: %v\n", w.ID, result)
				continue // Skip this iteration
			}
			jobIDStr := result[1]
			log.Printf("Worker %d: Received job ID %s from Redis list '%s'\n", w.ID, jobIDStr, service.PendingQueueKey)

			w.simulateProcessByID(jobIDStr)
		}
	}()
}

func (w *Worker) simulateProcessByID(jobID string) {
	log.Printf("Worker %d: Simulating processing for job ID %s...\n", w.ID, jobID)
	time.Sleep(2 * time.Second) // Simulate work
	log.Printf("Worker %d: Finished simulated processing for job ID %s\n", w.ID, jobID)
}
