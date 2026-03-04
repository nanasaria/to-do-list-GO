package server

import (
	"log/slog"
	"net/http"
	"time"

	"to-do-list/internal/utils"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	size, err := r.ResponseWriter.Write(data)
	r.bytes += size

	return size, err
}

func LoggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = utils.NewUUID()
		}

		w.Header().Set("X-Request-ID", requestID)

		recorder := &responseRecorder{ResponseWriter: w}
		startedAt := time.Now()

		next.ServeHTTP(recorder, r)

		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		logAttrs := []any{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", recorder.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"bytes", recorder.bytes,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		}

		switch {
		case recorder.status >= http.StatusInternalServerError:
			logger.Error("http request completed", logAttrs...)
		case recorder.status >= http.StatusBadRequest:
			logger.Warn("http request completed", logAttrs...)
		default:
			logger.Info("http request completed", logAttrs...)
		}
	})
}
