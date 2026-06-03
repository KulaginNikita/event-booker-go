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
		StartsAt: time.Now().Add(time.Hour),
		Capacity: 10,
	})
	if !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatalf("expected invalid title, got %v", err)
	}

	_, err = svc.CreateEvent(context.Background(), CreateEventInput{
		Title:    "Lecture",
		StartsAt: time.Now().Add(time.Hour),
		Capacity: 0,
	})
	if !errors.Is(err, domain.ErrInvalidCapacity) {
		t.Fatalf("expected invalid capacity, got %v", err)
	}

	_, err = svc.CreateEvent(context.Background(), CreateEventInput{
		Title:    "Past lecture",
		StartsAt: time.Now().Add(-time.Hour),
		Capacity: 10,
	})
	if !errors.Is(err, domain.ErrInvalidDate) {
		t.Fatalf("expected invalid date, got %v", err)
	}
}

func TestEventServiceListEventsValidatesPagination(t *testing.T) {
	repo := &mockRepo{}
	svc := newTestService(repo)

	_, err := svc.ListEvents(context.Background(), ListEventsInput{Limit: 101})
	if !errors.Is(err, domain.ErrInvalidPagination) {
		t.Fatalf("expected invalid pagination, got %v", err)
	}

	_, err = svc.ListEvents(context.Background(), ListEventsInput{Sort: "drop_table"})
	if !errors.Is(err, domain.ErrInvalidPagination) {
		t.Fatalf("expected invalid pagination, got %v", err)
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
	if !eventually(func() bool { return svc.notifier.(*mockNotifier).createdCalls == 1 }) {
		t.Fatalf("expected created notifier call")
	}
}

func TestEventServiceConfirmNotifiesUsers(t *testing.T) {
	repo := &mockRepo{
		confirmed: &domain.Booking{
			ID:         9,
			EventID:    42,
			EventTitle: "Go workshop",
			UserEmail:  "ann@example.com",
			Status:     domain.StatusConfirmed,
		},
	}
	notifier := &mockNotifier{}
	log, _ := logger.InitLogger(logger.ZapEngine, "test", "test", logger.WithLevel(logger.ErrorLevel))
	svc := NewEventService(repo, notifier, time.Minute, log)

	booking, err := svc.Confirm(context.Background(), ConfirmInput{EventID: 42, BookingID: 9})
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if booking.ID != 9 {
		t.Fatalf("unexpected booking id: %d", booking.ID)
	}
	if !eventually(func() bool { return notifier.confirmedCalls == 1 }) {
		t.Fatalf("expected confirmed notifier call, got %d", notifier.confirmedCalls)
	}
}

func TestEventServiceCancelExpiredNotifiesUsers(t *testing.T) {
	repo := &mockRepo{
		cancelled: []domain.Booking{
			{ID: 7, EventID: 1, EventTitle: "Go workshop", UserEmail: "user@example.com", Status: domain.StatusCancelled},
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
	if !eventually(func() bool { return notifier.cancelledCalls == 1 }) {
		t.Fatalf("expected notifier call, got %d", notifier.cancelledCalls)
	}
}

func newTestService(repo *mockRepo) *EventService {
	log, _ := logger.InitLogger(logger.ZapEngine, "test", "test", logger.WithLevel(logger.ErrorLevel))
	return NewEventService(repo, &mockNotifier{}, 2*time.Minute, log)
}

type mockRepo struct {
	lastDeadline time.Duration
	cancelled    []domain.Booking
	confirmed    *domain.Booking
}

func (m *mockRepo) CreateEvent(_ context.Context, event *domain.Event) error {
	event.ID = 1
	event.FreeSeats = event.Capacity
	return nil
}

func (m *mockRepo) ListEvents(_ context.Context, _ domain.ListEventsFilter) ([]domain.Event, error) {
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
		EventTitle:   "Go workshop",
		UserName:     userName,
		UserEmail:    userEmail,
		UserTelegram: userTelegram,
		Status:       domain.StatusPending,
		ExpiresAt:    time.Now().Add(deadline),
	}, nil
}

func (m *mockRepo) Confirm(_ context.Context, _ int64, _ int64) (*domain.Booking, error) {
	if m.confirmed != nil {
		return m.confirmed, nil
	}
	return &domain.Booking{ID: 1, EventID: 1, EventTitle: "Go workshop", Status: domain.StatusConfirmed}, nil
}

func (m *mockRepo) CancelExpired(_ context.Context) ([]domain.Booking, error) {
	return m.cancelled, nil
}

type mockNotifier struct {
	createdCalls   int
	confirmedCalls int
	cancelledCalls int
}

func (m *mockNotifier) BookingCreated(_ context.Context, _ domain.Booking) error {
	m.createdCalls++
	return nil
}

func (m *mockNotifier) BookingConfirmed(_ context.Context, _ domain.Booking) error {
	m.confirmedCalls++
	return nil
}

func (m *mockNotifier) BookingCancelled(_ context.Context, _ domain.Booking) error {
	m.cancelledCalls++
	return nil
}

func eventually(fn func() bool) bool {
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}
