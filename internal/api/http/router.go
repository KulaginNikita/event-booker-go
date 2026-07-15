package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	router.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
	router.Get("/healthz", handler.Live)
	router.Get("/livez", handler.Live)
	router.Get("/readyz", handler.Ready)
	router.Post("/auth/login", handler.Login)

	router.Get("/events", handler.ListEvents)
	router.Get("/events/{id}", handler.GetEvent)
	router.With(handler.RequireRole("admin")).Post("/events", handler.CreateEvent)
	router.With(handler.RequireRole("user", "admin")).Post("/events/{id}/book", handler.Book)
	router.With(handler.RequireRole("user", "admin")).Post("/events/{id}/confirm", handler.Confirm)

	return router
}
