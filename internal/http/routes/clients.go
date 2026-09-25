package routes

import (
	"errors"
	"net/http"
	"strconv"

	"carservice/internal/service"

	"github.com/gin-gonic/gin"
)

func (handler *routeHandler) listClients(context *gin.Context) {
	page, ok := positiveQueryInt(context, "page", 1)
	if !ok {
		context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		return
	}

	perPage, ok := positiveQueryInt(context, "per_page", 50)
	if !ok {
		context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		return
	}

	result, err := handler.service.ListClients(context.Request.Context(), page, perPage)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
			return
		}
		handler.logger.Printf("list clients failed: %v", err)
		context.JSON(http.StatusInternalServerError, gin.H{"message": "an unexpected internal error occurred"})
		return
	}

	context.JSON(http.StatusOK, result)
}

func (handler *routeHandler) searchClient(context *gin.Context) {

	name, hasName := context.GetQuery("name")

	personalID, hasPersonalID := context.GetQuery("personal_id")

	input := service.SearchInput{}

	if hasName {
		input.Name = &name
	}

	if hasPersonalID {
		input.PersonalID = &personalID
	}

	result, err := handler.service.SearchClient(context.Request.Context(), input)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "request input is invalid"})
		case errors.Is(err, service.ErrAmbiguousSearch):
			context.JSON(http.StatusUnprocessableEntity, gin.H{"message": "name search returned multiple clients"})
		case errors.Is(err, service.ErrNotFound):
			context.JSON(http.StatusNotFound, gin.H{"message": "requested resource was not found"})
		default:
			handler.logger.Printf("search client failed: %v", err)
			context.JSON(http.StatusInternalServerError, gin.H{"message": "an unexpected internal error occurred"})
		}
		return
	}

	context.JSON(http.StatusOK, result)
}

func positiveQueryInt(context *gin.Context, key string, fallback int) (int, bool) {

	raw, exists := context.GetQuery(key)

	if !exists {
		return fallback, true
	}

	value, err := strconv.Atoi(raw)

	if err != nil || value < 1 {
		return 0, false
	}

	return value, true
}
