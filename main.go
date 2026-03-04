package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"to-do-list/internal/config"
	"to-do-list/internal/controllers"
	"to-do-list/internal/database"
	"to-do-list/internal/handlers"
	"to-do-list/internal/logger"
	"to-do-list/internal/repositories"
	"to-do-list/internal/server"
	"to-do-list/internal/services"
)

func main() {
	applicationConfig := config.Load()
	appLogger := logger.New(applicationConfig.LogLevel, applicationConfig.LogFormat)
	slog.SetDefault(appLogger)

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	appLogger.Info("connecting to MongoDB",
		"uri", applicationConfig.MongoURI,
		"database", applicationConfig.MongoDatabase,
		"collection", applicationConfig.MongoCollection,
	)

	mongoClient, err := database.Connect(startupContext, applicationConfig.MongoURI)
	if err != nil {
		appLogger.Error("failed to initialize MongoDB", "error", err)
		os.Exit(1)
	}
	appLogger.Info("MongoDB connection established",
		"database", applicationConfig.MongoDatabase,
		"collection", applicationConfig.MongoCollection,
	)
	defer func() {
		disconnectContext, cancelDisconnect := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelDisconnect()

		if err := mongoClient.Disconnect(disconnectContext); err != nil {
			appLogger.Error("failed to disconnect MongoDB", "error", err)
		}
	}()

	taskRepository := repositories.NewMongoTaskRepository(mongoClient.Database(applicationConfig.MongoDatabase), applicationConfig.MongoCollection)
	taskService := services.NewTaskService(taskRepository)
	taskController := controllers.NewTaskController(taskService, appLogger, 5*time.Second)
	taskHandler := handlers.NewTaskHandler(taskController)

	httpServer := &http.Server{
		Addr:         applicationConfig.HTTPAddress(),
		Handler:      server.NewRouter(taskHandler, appLogger),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	appLogger.Info("HTTP server listening",
		"address", applicationConfig.HTTPAddress(),
		"log_level", applicationConfig.LogLevel,
		"log_format", applicationConfig.LogFormat,
	)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		appLogger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}
