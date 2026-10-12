package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andretop97/UserApiV2/src/bootstrap"
	"github.com/andretop97/UserApiV2/src/middlewares"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

const ADDR = ":8080"

func main() {
	if err := run(); err != nil {
		slog.Error("aplicação encerrada com erro", "error", err)
		os.Exit(1)
	}
}

func run() error {
	godotenv.Load()
	utils.SetLoggerSettings()

	container, err := bootstrap.NewContainer()
	if err != nil {
		return fmt.Errorf("inicializando container: %w", err)
	}
	defer func() {
		if err := container.Close(); err != nil {
			slog.Error("erro ao fechar container", "error", err)
		}
	}()

	router := gin.New()
	router.Use(middlewares.SlogLogger())
	router.Use(middlewares.SlogRecovery())
	router = routes.Routes(router, container.Controllers)

	srv := &http.Server{
		Addr:              ADDR,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		slog.Info("servidor iniciado", "addr", ADDR)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("servidor: %w", err)

	case <-ctx.Done():
		slog.Info("sinal recebido, encerrando")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("erro ao encerrar servidor: %w", err)

	}
	slog.Info("servidor encerrado")
	return nil
}
