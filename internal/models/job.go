package models

import (
	"database/sql" // Make sure this is imported
	"encoding/json"
	"fmt" // Good practice to import for errors
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusScheduled JobStatus = "scheduled" // The new status for jobs with a future execution time
)

// Job struct
type Job struct {
	ID           uuid.UUID       `json:"id"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
	Status       JobStatus       `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Attempts     int             `json:"attempts"`
	ErrorMessage sql.NullString  `json:"error_message,omitempty"`
	ExecuteAt    sql.NullTime    `json:"execute_at,omitempty"` // CORRECT: Use sql.NullTime for nullable timestamps
}

// NewJob constructor
func NewJob(jobType string, payload map[string]interface{}, executeAt *time.Time) (*Job, error) { // Parameter is *time.Time
	jobID := uuid.New()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job payload: %w", err)
	}

	now := time.Now().UTC()

	// --- CORRECT LOGIC for Status and ExecuteAt ---
	jobStatus := StatusPending
	var jobExecuteAt sql.NullTime // This is an sql.NullTime struct

	// Check if a future execution time was provided
	if executeAt != nil && executeAt.After(now) {
		jobStatus = StatusScheduled // If it's for the future, set status to scheduled
		jobExecuteAt = sql.NullTime{
			Time:  executeAt.UTC(), // Set the time
			Valid: true,            // Mark it as valid (will not be NULL in DB)
		}
	} else {
		// If no time is provided, or it's in the past, the job is pending immediate execution.
		// jobExecuteAt remains a zero-valued sql.NullTime, where `Valid` is false.
		// The database driver will correctly interpret this as NULL.
	}
	// --- END CORRECT LOGIC ---

	return &Job{
		ID:           jobID,
		Type:         jobType,
		Payload:      payloadBytes,
		Status:       jobStatus, // Use the determined status
		CreatedAt:    now,
		UpdatedAt:    now,
		ExecuteAt:    jobExecuteAt, // Assign the sql.NullTime struct
		Attempts:     0,
		ErrorMessage: sql.NullString{}, // Initialize as not valid
	}, nil
}
