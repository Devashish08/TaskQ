package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Devashish08/taskq/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type OutboxPoller struct {
	DB          *sql.DB
	RedisClient *redis.Client
	Interval    time.Duration
	stopChan    chan struct{}
}

func NewOutboxPoller(db *sql.DB, redisClient *redis.Client, interval time.Duration) *OutboxPoller {
	return &OutboxPoller{
		DB:          db,
		RedisClient: redisClient,
		Interval:    interval,
		stopChan:    make(chan struct{}),
	}
}

func (p *OutboxPoller) Start() {
	log.Println("OutboxPoller: Starting...")
	ticker := time.NewTicker(p.Interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := p.processOutboxEvents(); err != nil {
					log.Printf("OutboxPoller: Error processing events: %v\n", err)
				}

			case <-p.stopChan:
				ticker.Stop()
				log.Println("OutboxPoller: Stopped.")
				return
			}
		}
	}()
}
func (p *OutboxPoller) Stop() {
	log.Println("OutboxPoller: Stopping...")
	close(p.stopChan)
}
func (p *OutboxPoller) processOutboxEvents() error {
	ctx := context.Background()

	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	query := `
		SELECT event_id, job_id
		FROM job_queue_outbox
		WHERE processed_at IS NULL
		ORDER BY created_at
		LIMIT 100
		FOR UPDATE SKIP LOCKED`
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var eventIDsToUpdate []uuid.UUID
	redisPipe := p.RedisClient.Pipeline()
	var jobsProcessed int

	for rows.Next() {
		var eventID, jobID uuid.UUID
		if err := rows.Scan(&eventID, &jobID); err != nil {
			return err
		}

		redisPipe.LPush(ctx, service.PendingQueueKey, jobID.String())

		eventIDsToUpdate = append(eventIDsToUpdate, eventID)
		jobsProcessed++
	}
	rows.Close()

	if jobsProcessed > 0 {
		return nil
	}

	_, err = redisPipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline failed: %w", err)
	}

	updateQuery := `UPDATE job_queue_outbox SET processed_at = NOW() WHERE event_id = ANY($1)`
	_, err = tx.ExecContext(ctx, updateQuery, eventIDsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to mark events as processed: %w", err)
	}

	log.Printf("OutboxPoller: Relayed %d jobs to Redis.", jobsProcessed)
	return tx.Commit()
}
