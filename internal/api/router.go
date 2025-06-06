package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouterWithDeps configures the API routes using a pre-built handler.
func SetupRouterWithDeps(apiHandler *ApiHandler) *gin.Engine {
	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.ExposeHeaders = []string{"Content-Length"}
	config.AllowCredentials = true
	config.MaxAge = 12 * time.Hour
	router.Use(cors.New(config))
	v1 := router.Group("/api/v1")
	{
		v1.POST("/jobs", apiHandler.SubmitJobHandler)
		v1.GET("/jobs/:job_id", apiHandler.GetJobStatusHandler)
		v1.GET("/jobs", apiHandler.ListJobsHandler)
	}
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	return router
}
