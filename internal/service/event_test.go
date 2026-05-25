package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func TestEventServiceCreateEventValidatesInput(t *testing.T) {
	repo := &mockRepo{}
	svc := newTestService(repo)

	_, err := svc.CreateEvent(context.Background(), CreateEventInput{
		Title:    "",
		StartsAt: time.Now(),
		Capacity: 10,
	})
	if !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatalf("expected invalid title, got %v", err)
	}

	_, err = svc.CreateEvent(context.Background(), CreateEventInput{
		Title:    "Lecture",
		StartsAt: time.Now(),
		Capacity: 0,
	})
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Fatalf("expected invalid capacity, got %v", err)
	}
}

func TestEventServiceBookUsesConfiguredDeadline(t *testing.T) {
	repo := &mockRepo{}
	svc := newTestService(repo)

	booking, err := svc.Book(context.Background(), BookInput{
		EventID:      42,
		UserName:     "Ann",
		UserEmail:    "ann@example.com",
		UserTelegram: "12345",
	})
	if err != nil {
		t.Fatalf("book failed: %v", err)
	}

	if booking.EventID != 42 {
		t.Fatalf("unexpected event id: %d", booking.EventID)
	}
	if repo.lastDeadline != 2*time.Minute {
		t.Fatalf("expected configured deadline, got %s", repo.lastDeadline)
	}
}

func TestEventServiceCancelExpiredNotifiesUsers(t *testing.T) {
	repo := &mockRepo{
		cancelled: []domain.Booking{
			{ID: 7, EventID: 1, UserEmail: "user@example.com", Status: domain.StatusCancelled},
		},
	}
	notifier := &mockNotifier{}
	log, _ := logger.InitLogger(logger.ZapEngine, "test", "test", logger.WithLevel(logger.ErrorLevel))
	svc := NewEventService(repo, notifier, time.Minute, log)

	count, err := svc.CancelExpired(context.Background())
	if err != nil {
		t.Fatalf("cancel expired failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 cancelled booking, got %d", count)
	}
	if notifier.calls != 1 {
		t.Fatalf("expected notifier call, got %d", notifier.calls)
	}
}

func newTestService(repo *mockRepo) *EventService {
	log, _ := logger.InitLogger(logger.ZapEngine, "test", "test", logger.WithLevel(logger.ErrorLevel))
	return NewEventService(repo, &mockNotifier{}, 2*time.Minute, log)
}

type mockRepo struct {
	lastDeadline time.Duration
	cancelled    []domain.Booking
}

func (m *mockRepo) CreateEvent(_ context.Context, event *domain.Event) error {
	event.ID = 1
	event.FreeSeats = event.Capacity
	return nil
}

func (m *mockRepo) ListEvents(_ context.Context) ([]domain.Event, error) {
	return nil, nil
}

func (m *mockRepo) GetEvent(_ context.Context, _ int64) (*domain.Event, error) {
	return nil, domain.ErrEventNotFound
}

func (m *mockRepo) Book(_ context.Context, eventID int64, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error) {
	m.lastDeadline = deadline
	return &domain.Booking{
		ID:           1,
		EventID:      eventID,
		UserName:     userName,
		UserEmail:    userEmail,
		UserTelegram: userTelegram,
		Status:       domain.StatusPending,
		ExpiresAt:    time.Now().Add(deadline),
	}, nil
}

func (m *mockRepo) Confirm(_ context.Context, _ int64, _ int64) (*domain.Booking, error) {
	return nil, nil
}

func (m *mockRepo) CancelExpired(_ context.Context) ([]domain.Booking, error) {
	return m.cancelled, nil
}

type mockNotifier struct {
	calls int
}

func (m *mockNotifier) BookingCancelled(_ context.Context, _ domain.Booking) error {
	m.calls++
	return nil
}
