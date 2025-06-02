package models

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

type Job struct {
	ID           uuid.UUID       `json:"id"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
	Status       JobStatus       `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Attempts     int             `json:"attempts"`
	ErrorMessage sql.NullString  `json:"error_message,omitempty"`
}

func NewJob(jobType string, payload map[string]interface{}) (*Job, error) {
	jobID := uuid.New()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	return &Job{
		ID:        jobID,
		Type:      jobType,
		Payload:   payloadBytes,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Attempts:  0,
	}, nil
}
