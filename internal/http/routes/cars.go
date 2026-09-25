package routes

import (
	"errors"
	"net/http"
	"strconv"

	"carservice/internal/service"

	"github.com/gin-gonic/gin"
)

func (handler *routeHandler) listClientCars(context *gin.Context) {

	result, err := handler.service.ListClientCars(context.Request.Context(), context.Param("clientId"))

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		case errors.Is(err, service.ErrNotFound):
			context.JSON(http.StatusNotFound, gin.H{"message": "requested resource was not found"})
		default:
			handler.logger.Printf("list client cars failed: %v", err)
			context.JSON(http.StatusInternalServerError, gin.H{"message": "an unexpected internal error occurred"})
		}
		return
	}

	context.JSON(http.StatusOK, result)
}

func (handler *routeHandler) getEventHistory(context *gin.Context) {

	carID, err := strconv.Atoi(context.Param("carId"))

	if err != nil || carID < 1 {
		context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		return
	}

	result, err := handler.service.GetEventHistory(context.Request.Context(), context.Param("clientId"), carID)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		case errors.Is(err, service.ErrNotFound):
			context.JSON(http.StatusNotFound, gin.H{"message": "requested resource was not found"})
		default:
			handler.logger.Printf("get event history failed: %v", err)
			context.JSON(http.StatusInternalServerError, gin.H{"message": "an unexpected internal error occurred"})
		}
		return
	}

	context.JSON(http.StatusOK, result)
}
