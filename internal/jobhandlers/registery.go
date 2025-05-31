package jobhandlers

import (
	"encoding/json"
	"fmt"
)

// JobHandlerFunc defines the signature for all job handler functions.
// It takes the raw JSON payload and returns an error if processing fails.
type JobHandlerFunc func(payload json.RawMessage) error

// jobRegistry stores the mapping from job type strings to handler functions.
var jobRegistry = make(map[string]JobHandlerFunc)

// init is automatically called when the package is loaded,
// allowing us to register our handlers.
func init() {
	Register("log_payload_job", HandleSimpleLogJob)
	Register("conditional_fail_job", HandleConditionalFailJob)
	Register("long_running_job", HandleLongRunningJob)
}

// Register associates a job type string with a handler function.
// If a handler for the given type is already registered, it will be overwritten.
func Register(jobType string, handler JobHandlerFunc) {
	if handler == nil {
		fmt.Printf("Warning: Attempted to register nil handler for job type '%s'\n", jobType)
		return
	}

	jobRegistry[jobType] = handler
}

// Get retrieves a handler function for a given job type.
// It returns the handler and an error if no handler is registered for the type.
func Get(jobType string) (JobHandlerFunc, error) {
	handler, ok := jobRegistry[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for job type: %s", jobType)
	}
	return handler, nil
}
