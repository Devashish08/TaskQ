package api

import (
	"github.com/Devashish08/taskq/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	jobService := service.NewJobService()
	apiHandler := NewApiHandler(jobService)

	router := gin.Default()

	v1 := router.Group("api/v1")
	{
		v1.POST("/jobs", apiHandler.SubmitJobHandler)
		// TODO: Add GET /jobs/{job_id} endpoint later
	}
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	return router
}
