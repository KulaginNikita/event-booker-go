package service

import (
	"context"
	"strings"
	"time"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

const (
	maxTitleLen = 160
	maxUserLen  = 100
	maxEmailLen = 180
)

type EventRepository interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEvent(ctx context.Context, id int64) (*domain.Event, error)
	Book(ctx context.Context, eventID int64, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error)
	Confirm(ctx context.Context, eventID int64, bookingID int64) (*domain.Booking, error)
	CancelExpired(ctx context.Context) ([]domain.Booking, error)
}

type Notifier interface {
	BookingCancelled(ctx context.Context, booking domain.Booking) error
}

type CreateEventInput struct {
	Title    string
	StartsAt time.Time
	Capacity int
}

type BookInput struct {
	EventID      int64
	UserName     string
	UserEmail    string
	UserTelegram string
}

type ConfirmInput struct {
	EventID   int64
	BookingID int64
}

type EventService struct {
	repo            EventRepository
	notifier        Notifier
	paymentDeadline time.Duration
	log             logger.Logger
}

func NewEventService(repo EventRepository, notifier Notifier, paymentDeadline time.Duration, log logger.Logger) *EventService {
	return &EventService{
		repo:            repo,
		notifier:        notifier,
		paymentDeadline: paymentDeadline,
		log:             log,
	}
}

func (s *EventService) CreateEvent(ctx context.Context, in CreateEventInput) (*domain.Event, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || len([]rune(title)) > maxTitleLen {
		return nil, domain.ErrInvalidTitle
	}
	if in.StartsAt.IsZero() {
		return nil, domain.ErrInvalidDate
	}
	if in.Capacity <= 0 {
		return nil, domain.ErrInvalidCapacity
	}

	event := &domain.Event{
		Title:    title,
		StartsAt: in.StartsAt.UTC(),
		Capacity: in.Capacity,
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *EventService) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return s.repo.ListEvents(ctx)
}

func (s *EventService) GetEvent(ctx context.Context, id int64) (*domain.Event, error) {
	return s.repo.GetEvent(ctx, id)
}

func (s *EventService) Book(ctx context.Context, in BookInput) (*domain.Booking, error) {
	userName := strings.TrimSpace(in.UserName)
	userEmail := strings.TrimSpace(in.UserEmail)
	userTelegram := strings.TrimSpace(in.UserTelegram)
	if in.EventID <= 0 || userName == "" || userEmail == "" ||
		len([]rune(userName)) > maxUserLen || len([]rune(userEmail)) > maxEmailLen ||
		!strings.Contains(userEmail, "@") {
		return nil, domain.ErrInvalidUser
	}
	return s.repo.Book(ctx, in.EventID, userName, userEmail, userTelegram, s.paymentDeadline)
}

func (s *EventService) Confirm(ctx context.Context, in ConfirmInput) (*domain.Booking, error) {
	if in.EventID <= 0 || in.BookingID <= 0 {
		return nil, domain.ErrBookingNotFound
	}
	return s.repo.Confirm(ctx, in.EventID, in.BookingID)
}

func (s *EventService) CancelExpired(ctx context.Context) (int, error) {
	cancelled, err := s.repo.CancelExpired(ctx)
	if err != nil {
		return 0, err
	}
	for _, booking := range cancelled {
		if err := s.notifier.BookingCancelled(ctx, booking); err != nil {
			s.log.Error("notify cancelled booking", "booking_id", booking.ID, "error", err)
		}
	}
	return len(cancelled), nil
}
