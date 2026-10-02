package main

import (
	"log/slog"

	"github.com/andretop97/UserApiV2/src/bootstrap"
	"github.com/andretop97/UserApiV2/src/middlewares"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	utils.SetLoggerSettings()

	container, err := bootstrap.NewContainer()
	if err != nil {
		slog.Error("Erro ao inicializar container", "error", err)
		panic(err)
	}
	defer func() {
		if err := container.Close(); err != nil {
			slog.Error("Erro ao fechar container", "error", err)
		}
	}()

	router := gin.New()
	router.Use(middlewares.SlogLogger())
	router.Use(middlewares.SlogRecovery())

	router = routes.Routes(router, container.Controllers)
	router.Run(":8080")
}
