package service

import (
	"context"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

const (
	maxTitleLen       = 160
	maxUserLen        = 100
	maxOwnerLen       = 100
	maxEmailLen       = 180
	maxTelegramLen    = 64
	maxCapacity       = 100000
	maxEventFuture    = 2 * 365 * 24 * time.Hour
	defaultListLimit  = 20
	maxListLimit      = 100
	outboxBatchSize   = 50
	outboxBaseBackoff = 30 * time.Second
	outboxMaxBackoff  = 10 * time.Minute
)

var telegramRe = regexp.MustCompile(`^(@[A-Za-z0-9_]{5,32}|-?[0-9]{5,20})$`)

type EventRepository interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	ListEvents(ctx context.Context, filter domain.ListEventsFilter) ([]domain.Event, error)
	GetEvent(ctx context.Context, id int64) (*domain.Event, error)
	Book(ctx context.Context, eventID int64, ownerUsername string, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error)
	Confirm(ctx context.Context, eventID int64, bookingID int64, actorUsername string, isAdmin bool) (*domain.Booking, error)
	CancelExpired(ctx context.Context) ([]domain.Booking, error)
	FetchDueNotifications(ctx context.Context, limit int) ([]domain.NotificationEvent, error)
	MarkNotificationSent(ctx context.Context, id int64) error
	RescheduleNotification(ctx context.Context, id int64, nextAttemptAt time.Time, reason string) error
	MarkNotificationFailed(ctx context.Context, id int64, reason string) error
}

type Notifier interface {
	BookingCreated(ctx context.Context, booking domain.Booking) error
	BookingConfirmed(ctx context.Context, booking domain.Booking) error
	BookingCancelled(ctx context.Context, booking domain.Booking) error
}

type CreateEventInput struct {
	Title    string
	StartsAt time.Time
	Capacity int
}

type ListEventsInput struct {
	Limit  uint64
	Offset uint64
	Sort   string
}

type BookInput struct {
	EventID       int64
	OwnerUsername string
	UserName      string
	UserEmail     string
	UserTelegram  string
}

type ConfirmInput struct {
	EventID       int64
	BookingID     int64
	ActorUsername string
	ActorRole     string
}

type EventService struct {
	repo            EventRepository
	notifier        Notifier
	paymentDeadline time.Duration
	log             *zap.SugaredLogger
}

func NewEventService(repo EventRepository, notifier Notifier, paymentDeadline time.Duration, log *zap.SugaredLogger) *EventService {
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
	startsAt := in.StartsAt.UTC()
	now := time.Now().UTC()
	if startsAt.Before(now) || startsAt.After(now.Add(maxEventFuture)) {
		return nil, domain.ErrInvalidDate
	}
	if in.Capacity <= 0 || in.Capacity > maxCapacity {
		return nil, domain.ErrInvalidCapacity
	}

	event := &domain.Event{
		Title:    title,
		StartsAt: startsAt,
		Capacity: in.Capacity,
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *EventService) ListEvents(ctx context.Context, in ListEventsInput) ([]domain.Event, error) {
	limit := in.Limit
	if limit == 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		return nil, domain.ErrInvalidPagination
	}

	sort := strings.TrimSpace(in.Sort)
	switch sort {
	case "", "starts_at", "-starts_at", "created_at", "-created_at", "title", "-title":
	default:
		return nil, domain.ErrInvalidPagination
	}

	return s.repo.ListEvents(ctx, domain.ListEventsFilter{
		Limit:  limit,
		Offset: in.Offset,
		Sort:   sort,
	})
}

func (s *EventService) GetEvent(ctx context.Context, id int64) (*domain.Event, error) {
	return s.repo.GetEvent(ctx, id)
}

func (s *EventService) Book(ctx context.Context, in BookInput) (*domain.Booking, error) {
	ownerUsername := strings.TrimSpace(in.OwnerUsername)
	userName := strings.TrimSpace(in.UserName)
	userEmail := strings.TrimSpace(in.UserEmail)
	userTelegram := strings.TrimSpace(in.UserTelegram)
	if in.EventID <= 0 || ownerUsername == "" || userName == "" || userEmail == "" ||
		len([]rune(ownerUsername)) > maxOwnerLen ||
		len([]rune(userName)) > maxUserLen || len([]rune(userEmail)) > maxEmailLen ||
		len([]rune(userTelegram)) > maxTelegramLen {
		return nil, domain.ErrInvalidUser
	}
	if _, err := mail.ParseAddress(userEmail); err != nil {
		return nil, domain.ErrInvalidUser
	}
	if userTelegram != "" && !telegramRe.MatchString(userTelegram) {
		return nil, domain.ErrInvalidUser
	}
	booking, err := s.repo.Book(ctx, in.EventID, ownerUsername, userName, userEmail, userTelegram, s.paymentDeadline)
	if err != nil {
		return nil, err
	}
	return booking, nil
}

func (s *EventService) Confirm(ctx context.Context, in ConfirmInput) (*domain.Booking, error) {
	actorUsername := strings.TrimSpace(in.ActorUsername)
	if in.EventID <= 0 || in.BookingID <= 0 || actorUsername == "" {
		return nil, domain.ErrBookingNotFound
	}
	booking, err := s.repo.Confirm(ctx, in.EventID, in.BookingID, actorUsername, in.ActorRole == RoleAdmin)
	if err != nil {
		return nil, err
	}
	return booking, nil
}

func (s *EventService) CancelExpired(ctx context.Context) (int, error) {
	cancelled, err := s.repo.CancelExpired(ctx)
	if err != nil {
		return 0, err
	}
	return len(cancelled), nil
}

func (s *EventService) DispatchNotifications(ctx context.Context) (int, error) {
	items, err := s.repo.FetchDueNotifications(ctx, outboxBatchSize)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, item := range items {
		if err := s.dispatchNotification(ctx, item); err != nil {
			s.log.Warnw("booking notification failed",
				"outbox_id", item.ID,
				"booking_id", item.Booking.ID,
				"type", item.Type,
				"attempts", item.Attempts,
				"error", err,
			)
			if item.Attempts >= item.MaxAttempts {
				if markErr := s.repo.MarkNotificationFailed(ctx, item.ID, err.Error()); markErr != nil {
					return sent, markErr
				}
				continue
			}
			if rescheduleErr := s.repo.RescheduleNotification(ctx, item.ID, nextNotificationAttempt(item.Attempts), err.Error()); rescheduleErr != nil {
				return sent, rescheduleErr
			}
			continue
		}

		if err := s.repo.MarkNotificationSent(ctx, item.ID); err != nil {
			return sent, err
		}
		sent++
	}

	return sent, nil
}

func (s *EventService) dispatchNotification(ctx context.Context, item domain.NotificationEvent) error {
	switch item.Type {
	case domain.NotificationBookingCreated:
		return s.notifier.BookingCreated(ctx, item.Booking)
	case domain.NotificationBookingConfirmed:
		return s.notifier.BookingConfirmed(ctx, item.Booking)
	case domain.NotificationBookingCancelled:
		return s.notifier.BookingCancelled(ctx, item.Booking)
	default:
		return domain.ErrInvalidState
	}
}

func nextNotificationAttempt(attempts int) time.Time {
	delay := time.Duration(attempts) * outboxBaseBackoff
	if delay <= 0 {
		delay = outboxBaseBackoff
	}
	if delay > outboxMaxBackoff {
		delay = outboxMaxBackoff
	}
	return time.Now().UTC().Add(delay)
}
