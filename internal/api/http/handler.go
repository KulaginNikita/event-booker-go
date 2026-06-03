package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
	"github.com/KulaginNikita/event-booker/internal/service"
)

type EventService interface {
	CreateEvent(ctx context.Context, in service.CreateEventInput) (*domain.Event, error)
	ListEvents(ctx context.Context, in service.ListEventsInput) ([]domain.Event, error)
	GetEvent(ctx context.Context, id int64) (*domain.Event, error)
	Book(ctx context.Context, in service.BookInput) (*domain.Booking, error)
	Confirm(ctx context.Context, in service.ConfirmInput) (*domain.Booking, error)
}

type AuthService interface {
	Login(username string, role string) (string, error)
	Parse(token string) (*service.Claims, error)
}

type HealthService interface {
	Ready(ctx context.Context) error
}

type Handler struct {
	service EventService
	auth    AuthService
	health  HealthService
	logger  logger.Logger
}

func NewHandler(service EventService, auth AuthService, health HealthService, log logger.Logger) *Handler {
	return &Handler{service: service, auth: auth, health: health, logger: log}
}

func (h *Handler) Live(c *ginext.Context) {
	c.JSON(http.StatusOK, ginext.H{"status": "ok"})
}

func (h *Handler) Ready(c *ginext.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.health.Ready(ctx); err != nil {
		h.logger.Error("readiness check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "service is not ready"})
		return
	}
	c.JSON(http.StatusOK, ginext.H{"status": "ok"})
}

func (h *Handler) Login(c *ginext.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	token, err := h.auth.Login(req.Username, req.Role)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, LoginResponse{Token: token, Role: req.Role})
}

func (h *Handler) CreateEvent(c *ginext.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	event, err := h.service.CreateEvent(c.Request.Context(), service.CreateEventInput{
		Title:    req.Title,
		StartsAt: req.StartsAt,
		Capacity: req.Capacity,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toEventResponse(event))
}

func (h *Handler) ListEvents(c *ginext.Context) {
	limit, ok := parseUintQuery(c, "limit", 20)
	if !ok {
		return
	}
	offset, ok := parseUintQuery(c, "offset", 0)
	if !ok {
		return
	}

	events, err := h.service.ListEvents(c.Request.Context(), service.ListEventsInput{
		Limit:  limit,
		Offset: offset,
		Sort:   c.DefaultQuery("sort", "starts_at"),
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEventResponses(events))
}

func (h *Handler) GetEvent(c *ginext.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	event, err := h.service.GetEvent(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEventResponse(event))
}

func (h *Handler) Book(c *ginext.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req BookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.service.Book(c.Request.Context(), service.BookInput{
		EventID:      id,
		UserName:     req.UserName,
		UserEmail:    req.UserEmail,
		UserTelegram: req.UserTelegram,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toBookingResponse(booking))
}

func (h *Handler) Confirm(c *ginext.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req ConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.service.Confirm(c.Request.Context(), service.ConfirmInput{
		EventID:   id,
		BookingID: req.BookingID,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, toBookingResponse(booking))
}

func (h *Handler) handleError(c *ginext.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrEventNotFound),
		errors.Is(err, domain.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrNoSeats):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrBookingExpired),
		errors.Is(err, domain.ErrBookingNotPending):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrInvalidDate),
		errors.Is(err, domain.ErrInvalidCapacity),
		errors.Is(err, domain.ErrInvalidUser),
		errors.Is(err, domain.ErrInvalidPagination):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
	default:
		h.logger.Error("internal error", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}

func parseID(c *ginext.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return id, true
}

func parseUintQuery(c *ginext.Context, name string, fallback uint64) (uint64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid " + name})
		return 0, false
	}
	return value, true
}
