package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const healthCheckTimeout = 2 * time.Second

func (handler *routeHandler) healthCheck(ginContext *gin.Context) {

	healthContext, cancelHealthCheck := context.WithTimeout(ginContext.Request.Context(), healthCheckTimeout)

	defer cancelHealthCheck()

	if err := handler.health.Ping(healthContext); err != nil {
		handler.logger.Printf("database health check failed: %v", err)
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": "database is unavailable"})
		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "ok"})
}
