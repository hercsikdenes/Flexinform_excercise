package routes

import (
	"context"
	"log"
	"net/http"

	"carservice/internal/service"

	"github.com/gin-gonic/gin"
)

type ClientService interface {
	ListClients(ctx context.Context, page, perPage int) (service.ClientPage, error)
	SearchClient(ctx context.Context, input service.SearchInput) (service.Client, error)
	ListClientCars(ctx context.Context, clientID string) ([]service.ClientCar, error)
	GetEventHistory(ctx context.Context, clientID string, carID int) (service.EventHistory, error)
}

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type routeHandler struct {
	service ClientService
	health  HealthChecker
	logger  *log.Logger
}

func RegisterRoutes(router *gin.Engine, clientService ClientService, health HealthChecker, logger *log.Logger) {
	handler := &routeHandler{service: clientService, health: health, logger: logger}

	router.GET("/health", handler.healthCheck)

	api := router.Group("/api")
	api.GET("/clients", handler.listClients)
	api.GET("/clients/search", handler.searchClient)
	api.GET("/clients/:clientId/cars", handler.listClientCars)
	api.GET("/clients/:clientId/cars/:carId/services", handler.getEventHistory)

	router.NoRoute(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, gin.H{"message": "route not found"})
	})

	router.NoMethod(func(context *gin.Context) {
		context.JSON(http.StatusMethodNotAllowed, gin.H{"message": "method not allowed"})
	})
}
