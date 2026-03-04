package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"to-do-list/internal/docs"
	"to-do-list/internal/handlers"
)

func NewRouter(taskHandler *handlers.TaskHandler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /swagger", docs.SwaggerUIHandler)
	mux.HandleFunc("GET /swagger/", docs.SwaggerUIHandler)
	mux.HandleFunc("GET /swagger/openapi.json", docs.OpenAPIHandler)
	taskHandler.RegisterRoutes(mux)

	return LoggingMiddleware(logger, mux)
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
