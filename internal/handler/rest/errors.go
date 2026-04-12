package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

func errorStatus(err error) int {
	switch {
	case errors.Is(err, port.ErrNotFound),
		errors.Is(err, service.ErrPlayerNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrPlayerAlreadyRegistered):
		return http.StatusConflict
	case errors.Is(err, service.ErrInvalidFaction):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func respondError(c *gin.Context, err error) {
	c.JSON(errorStatus(err), gin.H{"error": err.Error()})
}
