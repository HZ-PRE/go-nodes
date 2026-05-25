package main

import (
	"github.com/gin-gonic/gin"

	"nodes/controllers"
	"nodes/middleware"
	"nodes/repositories"
	"nodes/routes"
	"nodes/services"
)

func InitApp() (*middleware.ServerTask, *gin.Engine) {
	repo := repositories.NewRepository()
	svc := services.NewService(repo)
	task := middleware.NewServerTask(svc)
	ctl := controllers.NewServerControllers(svc)
	return task, routes.SetupRouter(task, ctl)
}
