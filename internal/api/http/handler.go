package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
	"github.com/KulaginNikita/event-booker/internal/service"
)

type EventService interface {
	CreateEvent(ctx context.Context, in service.CreateEventInput) (*domain.Event, error)
	ListEvents(ctx context.Context, in service.ListEventsInput) ([]domain.Event, error)
	GetEvent(ctx context.Context, id int64) (*domain.Event, error)
	ListMyBookings(ctx context.Context, username string) ([]domain.Booking, error)
	Book(ctx context.Context, in service.BookInput) (*domain.Booking, error)
	Confirm(ctx context.Context, in service.ConfirmInput) (*domain.Booking, error)
}

type AuthService interface {
	Login(username string, password string) (string, error)
	Parse(token string) (*service.Claims, error)
}

type HealthService interface {
	Ready(ctx context.Context) error
}

type Handler struct {
	service EventService
	auth    AuthService
	health  HealthService
	logger  *zap.SugaredLogger
}

func NewHandler(service EventService, auth AuthService, health HealthService, log *zap.SugaredLogger) *Handler {
	return &Handler{service: service, auth: auth, health: health, logger: log}
}

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.health.Ready(ctx); err != nil {
		h.logger.Errorw("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "service is not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	token, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		h.handleError(w, err)
		return
	}
	claims, err := h.auth.Parse(token)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, LoginResponse{Token: token, Username: claims.Subject, Role: claims.Role})
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	event, err := h.service.CreateEvent(r.Context(), service.CreateEventInput{
		Title:    req.Title,
		StartsAt: req.StartsAt,
		Capacity: req.Capacity,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEventResponse(event))
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseUintQuery(w, r, "limit", 20)
	if !ok {
		return
	}
	offset, ok := parseUintQuery(w, r, "offset", 0)
	if !ok {
		return
	}

	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "starts_at"
	}

	events, err := h.service.ListEvents(r.Context(), service.ListEventsInput{
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEventResponses(events))
}

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPublicEventResponse(event))
}

func (h *Handler) EventBookings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBookingResponses(event.Bookings))
}

func (h *Handler) MyBookings(w http.ResponseWriter, r *http.Request) {
	username, _, ok := currentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
		return
	}

	bookings, err := h.service.ListMyBookings(r.Context(), username)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBookingResponses(bookings))
}

func (h *Handler) Book(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	owner, _, ok := currentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
		return
	}

	var req BookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.service.Book(r.Context(), service.BookInput{
		EventID:       id,
		OwnerUsername: owner,
		UserName:      req.UserName,
		UserEmail:     req.UserEmail,
		UserTelegram:  req.UserTelegram,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBookingResponse(booking))
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	username, role, ok := currentUser(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
		return
	}

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.service.Confirm(r.Context(), service.ConfirmInput{
		EventID:       id,
		BookingID:     req.BookingID,
		ActorUsername: username,
		ActorRole:     role,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBookingResponse(booking))
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrEventNotFound),
		errors.Is(err, domain.ErrBookingNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrNoSeats):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrBookingExpired),
		errors.Is(err, domain.ErrBookingNotPending),
		errors.Is(err, domain.ErrAlreadyBooked),
		errors.Is(err, domain.ErrEventAlreadyStarted):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrInvalidDate),
		errors.Is(err, domain.ErrInvalidCapacity),
		errors.Is(err, domain.ErrInvalidUser),
		errors.Is(err, domain.ErrInvalidPagination):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: err.Error()})
	default:
		h.logger.Errorw("internal error", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return id, true
}

func parseUintQuery(w http.ResponseWriter, r *http.Request, name string, fallback uint64) (uint64, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid " + name})
		return 0, false
	}
	return value, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
