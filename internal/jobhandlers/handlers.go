package jobhandlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

// Example payload struct for the conditional fail job
type ConditionalPayload struct {
	Message    string `json:"message"`
	ShouldFail bool   `json:"should_fail"`
}

// HandleSimpleLogJob logs the received payload.
func HandleSimpleLogJob(payload json.RawMessage) error {
	log.Printf("[JobHandler - SimpleLogJob] Received payload: %s\n", string(payload))
	time.Sleep(1 * time.Millisecond)
	log.Printf("[JobHandler - SimpleLogJob] Finished processing.\n")
	return nil
}

// HandleConditionalFailJob can fail based on its payload.
func HandleConditionalFailJob(payload json.RawMessage) error {
	var p ConditionalPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("[JobHandler - ConditionalFailJob] failed to unmarshal payload: %w", err)
	}

	log.Printf("[JobHandler - ConditionalFailJob] Message: '%s', ShouldFail: %t\n", p.Message, p.ShouldFail)
	time.Sleep(500 * time.Millisecond)

	if p.ShouldFail {
		log.Printf("[JobHandler - ConditionalFailJob] Failing as requested.\n")
		return errors.New("job was instructed to fail to payload")
	}

	log.Println("[JobHandler - ConditionalFailJob] Succeeded.")
	return nil
}

// HandleLongRunningJob simulates a job that takes more time.
func HandleLongRunningJob(payload json.RawMessage) error {
	log.Printf("[JobHandler - LongRunningJob] Starting long task with payload: %s\n", string(payload))
	time.Sleep(3 * time.Second)
	log.Printf("[JobHandler - LongRunningJob] Finished long task.\n")
	return nil
}
