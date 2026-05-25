package controllers

import (
	"net/http"
	"nodes/services"

	"github.com/gin-gonic/gin"
)

type ServerControllers struct {
	svc services.Service
}

func NewServerControllers(svc services.Service) *ServerControllers {
	return &ServerControllers{svc: svc}
}
func (c *ServerControllers) respond(ctx *gin.Context, data any, err error) {
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, data)
}
