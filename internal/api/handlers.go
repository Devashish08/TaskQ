package api

import (
	"net/http"

	"github.com/Devashish08/taskq/internal/service"
	"github.com/gin-gonic/gin"
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
	var req submitJobRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.JobSvc.SubmitJob(req.Type, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Job submitted to service successfully",
		"job_type": req.Type,
	})
}
