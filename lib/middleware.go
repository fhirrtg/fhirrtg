package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type ctxLoggerKey struct{}

// ResponseWriter wrapper to capture status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func generateTraceID() (string, error) {
	bytes := make([]byte, 8) // 16 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Middleware that logs every request with timing, status, and query params
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		traceID := r.Header.Get("X-Request-ID")
		if traceID == "" {
			traceID, _ = generateTraceID()
		}

		logger := slog.Default().With(
			"ip", clientIP(r),
			"user_agent", r.UserAgent(),
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"trace_id", traceID,
		)
		ctx := context.WithValue(r.Context(), ctxLoggerKey{}, logger)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		duration := time.Since(start)

		// Log everything including query parameters
		logger.Info("request completed",
			"status", rec.status,
			"total_ms", duration.Milliseconds(),
		)
	})
}

// Retrieve logger from context
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxLoggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func LoggerFromRequest(r *http.Request) *slog.Logger {
	var logger *slog.Logger
	if r != nil {
		logger = LoggerFromContext(r.Context())
	} else {
		logger = slog.Default()
	}
	return logger
}
