package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"to-do-list/internal/config"
	"to-do-list/internal/database"
	"to-do-list/internal/handlers"
	"to-do-list/internal/repositories"
	"to-do-list/internal/server"
	"to-do-list/internal/services"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to initialize MongoDB: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer disconnectCancel()

		if err := client.Disconnect(disconnectCtx); err != nil {
			log.Printf("failed to disconnect MongoDB: %v", err)
		}
	}()

	todoRepository := repositories.NewMongoTodoRepository(client.Database(cfg.MongoDatabase), cfg.MongoCollection)
	todoService := services.NewTodoService(todoRepository)
	todoHandler := handlers.NewTodoHandler(todoService, 5*time.Second)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddress(),
		Handler:      server.NewRouter(todoHandler),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("HTTP server listening on %s", cfg.HTTPAddress())

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped with error: %v", err)
	}
}
