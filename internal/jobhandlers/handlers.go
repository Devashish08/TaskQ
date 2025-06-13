package jobhandlers // Only once at the top

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ReminderPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
}

// ConditionalPayload is an example payload struct for HandlePotentiallyFailingJob.
type ConditionalPayload struct {
	Message    string `json:"message"`
	ShouldFail bool   `json:"force_fail"` // JSON tag is correct: "should_fail"
}

// HandleSimpleLogJob logs the received payload and simulates quick processing.
func HandleSimpleLogJob(s *discordgo.Session, payload json.RawMessage) error {
	log.Printf("[JobHandler - HandleSimpleLogJob] Received payload: %s", string(payload))
	time.Sleep(10 * time.Millisecond)
	log.Printf("[JobHandler - HandleSimpleLogJob] Finished processing.")
	return nil
}

// HandlePotentiallyFailingJob can fail based on its payload or randomly.
// In internal/jobhandlers/handlers.go

func HandlePotentiallyFailingJob(s *discordgo.Session, payload json.RawMessage) error {
	log.Printf("[JobHandler - HandlePotentiallyFailingJob] RAW PAYLOAD RECEIVED: %s\n", string(payload)) // Log the exact raw string

	var p ConditionalPayload
	err := json.Unmarshal(payload, &p) // Store the error from Unmarshal

	if err != nil {
		log.Printf("[JobHandler - HandlePotentiallyFailingJob] ERROR DURING UNMARSHAL: %v. struct after attempt: %+v\n", err, p)
		return fmt.Errorf("HandlePotentiallyFailingJob: bad payload format: %w", err)
	}

	// Log the struct *after* unmarshalling, before any logic
	log.Printf("[JobHandler - HandlePotentiallyFailingJob] UNMARSHALLED STRUCT: Message='%s', ShouldFail=%t\n", p.Message, p.ShouldFail)
	time.Sleep(500 * time.Millisecond)

	if p.ShouldFail {
		log.Printf("[JobHandler - HandlePotentiallyFailingJob] Condition p.ShouldFail is TRUE. Failing as requested by payload.")
		return errors.New("job was instructed to fail via payload")
	} else {
		log.Printf("[JobHandler - HandlePotentiallyFailingJob] Condition p.ShouldFail is FALSE.")
	}

	// Random failure
	if rand.Intn(3) == 0 {
		log.Printf("[JobHandler - HandlePotentiallyFailingJob] Encountered a simulated random error.")
		return errors.New("simulated random processing error")
	}

	log.Printf("[JobHandler - HandlePotentiallyFailingJob] Succeeded (no forced fail, no random fail).")
	return nil
}

// HandleLongRunningJob simulates a job that takes more time to complete.
func HandleLongRunningJob(s *discordgo.Session, payload json.RawMessage) error {
	log.Printf("[JobHandler - HandleLongRunningJob] Starting long task with payload: %s", string(payload))
	time.Sleep(3 * time.Second)
	log.Printf("[JobHandler - HandleLongRunningJob] Finished long task.")
	return nil
}

func HandleDiscordReminder(s *discordgo.Session, payload json.RawMessage) error {
	var p ReminderPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("failed to unmarshal reminder payload: %w", err)
	}

	log.Printf("Sending reminder to channel %s for user %s", p.ChannelID, p.UserID)

	reminderText := fmt.Sprintf("Hey <@%s>, here is your reminder: %s", p.UserID, p.Message)

	_, err := s.ChannelMessageSend(p.ChannelID, reminderText)
	if err != nil {
		log.Printf("ERROR sending reminder message: %v", err)
		return err
	}

	log.Printf("Successfully sent reminder for user %s", p.UserID)
	return nil
}
