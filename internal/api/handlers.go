package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/Devashish08/taskq/internal/models"
	"github.com/Devashish08/taskq/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type submitJobRequest struct {
	Type    string                 `json:"type" binding:"required"`
	Payload map[string]interface{} `json:"payload"`
}

// ApiHandler handles HTTP API requests for job management
type ApiHandler struct {
	JobSvc *service.JobService
}

// NewApiHandler creates a new API handler instance
// Parameters:
//   - jobService: Service for managing job operations
//
// Returns:
//   - *ApiHandler: Configured API handler
func NewApiHandler(jobService *service.JobService) *ApiHandler {
	return &ApiHandler{
		JobSvc: jobService,
	}
}

// SubmitJobHandler handles POST requests to submit new jobs for processing
// Request body should contain job type and optional payload
// Returns 202 Accepted with job ID on success
func (h *ApiHandler) SubmitJobHandler(c *gin.Context) {
	var req submitJobRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	submittedJob, err := h.JobSvc.SubmitJob(req.Type, req.Payload, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Job accepted for processing",
		"job_id":  submittedJob.ID,
	})
}

// GetJobStatusHandler retrieves the current status of a job by ID
// URL parameter: job_id (UUID format)
// Returns job details including status, attempts, and error messages
func (h *ApiHandler) GetJobStatusHandler(c *gin.Context) {
	jobIDStr := c.Param("job_id")

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	jobDetails, err := h.JobSvc.GetJobStatus(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job status"})
		return
	}

	c.JSON(http.StatusOK, jobDetails)
}

// ListJobsHandler returns a list of recent jobs ordered by creation time
// Default limit is 20 jobs
// Returns array of job objects with full details
func (h *ApiHandler) ListJobsHandler(c *gin.Context) {
	limit := 20

	jobs, err := h.JobSvc.ListRecentJobs(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve jobs"})
		return
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	c.JSON(http.StatusOK, jobs)
}
