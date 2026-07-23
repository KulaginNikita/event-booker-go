package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler, metrics HTTPMetrics, metricsHandler http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(MetricsMiddleware(metrics))

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	router.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	router.Get("/healthz", handler.Live)
	router.Get("/livez", handler.Live)
	router.Get("/readyz", handler.Ready)
	if metricsHandler != nil {
		router.Handle("/metrics", metricsHandler)
	}
	router.Post("/auth/login", handler.Login)

	router.Get("/events", handler.ListEvents)
	router.Get("/events/{id}", handler.GetEvent)
	router.With(handler.RequireRole("admin")).Get("/events/{id}/bookings", handler.EventBookings)
	router.With(handler.RequireRole("user", "admin")).Get("/bookings/me", handler.MyBookings)
	router.With(handler.RequireRole("admin")).Post("/events", handler.CreateEvent)
	router.With(handler.RequireRole("user", "admin")).Post("/events/{id}/book", handler.Book)
	router.With(handler.RequireRole("user", "admin")).Post("/events/{id}/confirm", handler.Confirm)

	return router
}
