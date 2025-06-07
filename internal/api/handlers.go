package api

import (
	"database/sql"
	"errors"
	"log"
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

type ApiHandler struct {
	JobSvc *service.JobService
}

func NewApiHandler(jobService *service.JobService) *ApiHandler {
	return &ApiHandler{
		JobSvc: jobService,
	}
}

func (h *ApiHandler) SubmitJobHandler(c *gin.Context) {
	log.Println("--- NEW SUBMIT JOB REQUEST ---")
	var req submitJobRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	submittedJob, err := h.JobSvc.SubmitJob(req.Type, req.Payload)
	if err != nil {
		log.Printf("API: Error from JobService on submission: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Job accepted for processing",
		"job_id":  submittedJob.ID,
	})
}

func (h *ApiHandler) GetJobStatusHandler(c *gin.Context) {
	jobIDStr := c.Param("job_id")

	jobID, err := uuid.Parse(jobIDStr)

	if err != nil {
		log.Printf("Invalid job ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	jobDetails, err := h.JobSvc.GetJobStatus(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("API: Job not found for ID: %s\n", jobID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		log.Printf("API: Error fetching job status for ID %s: %v\n", jobID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job status"})
		return
	}

	log.Printf("API: Successfully retrieved job status for ID: %s\n", jobID)
	c.JSON(http.StatusOK, jobDetails)

}

func (h *ApiHandler) ListJobsHandler(c *gin.Context) {

	limit := 20

	jobs, err := h.JobSvc.ListRecentJobs(limit)
	if err != nil {
		log.Printf("API: Error listing jobs: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve jobs"})
		return
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	log.Printf("API: Successfully retrieved %d recent jobs\n", len(jobs))
	c.JSON(http.StatusOK, jobs)
}
