package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
	"github.com/KulaginNikita/event-booker/internal/service"
)

type EventService interface {
	CreateEvent(ctx context.Context, in service.CreateEventInput) (*domain.Event, error)
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEvent(ctx context.Context, id int64) (*domain.Event, error)
	Book(ctx context.Context, in service.BookInput) (*domain.Booking, error)
	Confirm(ctx context.Context, in service.ConfirmInput) (*domain.Booking, error)
}

type Handler struct {
	service EventService
	logger  logger.Logger
}

func NewHandler(service EventService, log logger.Logger) *Handler {
	return &Handler{service: service, logger: log}
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
	events, err := h.service.ListEvents(c.Request.Context())
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
		errors.Is(err, domain.ErrInvalidUser):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
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
