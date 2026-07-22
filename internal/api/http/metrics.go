package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type HTTPMetrics interface {
	ObserveHTTPRequest(method string, path string, status int, duration time.Duration)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func MetricsMiddleware(metrics HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if metrics == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			path := r.URL.Path
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if pattern := routeCtx.RoutePattern(); pattern != "" {
					path = pattern
				}
			}
			metrics.ObserveHTTPRequest(r.Method, path, rec.status, time.Since(start))
		})
	}
}
