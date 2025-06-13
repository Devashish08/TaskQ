package jobhandlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// JobHandlerFunc defines the signature for all job handler functions.
// It takes the raw JSON payload and returns an error if processing fails.
type JobHandlerFunc func(s *discordgo.Session, payload json.RawMessage) error

// Registry stores the mapping from job type strings to handler functions.
type Registry struct {
	handlers map[string]JobHandlerFunc
}

// NewRegistry creates and returns a new, empty Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]JobHandlerFunc),
	}
}

// Register associates a job type string with a handler function in this registry instance.
// If a handler for the given type is already registered, it will be overwritten,
// and a warning will be logged.
func (r *Registry) Register(jobType string, handler JobHandlerFunc) {
	if handler == nil {
		// It's unusual to register a nil handler, log a more prominent warning or error.
		log.Printf("ERROR: Attempted to register nil handler for job type '%s'. Handler not registered.", jobType)
		return
	}

	if _, exists := r.handlers[jobType]; exists {
		log.Printf("WARN: Overwriting existing handler for job type '%s'", jobType)
	}

	r.handlers[jobType] = handler
	log.Printf("Registered handler for job type '%s'", jobType)
}

// Get retrieves a handler function for a given job type from this registry instance.
// It returns the handler and an error if no handler is registered for the type.
func (r *Registry) Get(jobType string) (JobHandlerFunc, error) {
	handler, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for job type: %s", jobType)
	}
	return handler, nil
}
