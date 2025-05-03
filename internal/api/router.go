package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRouterWithDeps configures the API routes using a pre-built handler.
func SetupRouterWithDeps(apiHandler *ApiHandler) *gin.Engine {
	router := gin.Default()
	v1 := router.Group("/api/v1")
	{
		v1.POST("/jobs", apiHandler.SubmitJobHandler)
	}
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	return router
}
