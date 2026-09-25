package httpapi

import (
	"log"
	"net/http"

	"carservice/internal/http/routes"
	"github.com/gin-gonic/gin"
)

func NewRouter(clientService routes.ClientService, health routes.HealthChecker, logger *log.Logger) *gin.Engine {
	if logger == nil {
		logger = log.Default()
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.CustomRecoveryWithWriter(logger.Writer(), func(context *gin.Context, _ any) {
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "an unexpected internal error occurred"})
	}))
	router.HandleMethodNotAllowed = true

	routes.RegisterRoutes(router, clientService, health, logger)
	return router
}
