package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

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
		EventID:       42,
		OwnerUsername: "ann",
		UserName:      "Ann",
		UserEmail:     "ann@example.com",
		UserTelegram:  "12345",
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
	if repo.lastOwner != "ann" {
		t.Fatalf("expected owner username to be propagated, got %q", repo.lastOwner)
	}
}

func TestEventServiceConfirmReturnsBooking(t *testing.T) {
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
	log := zap.NewNop().Sugar()
	svc := NewEventService(repo, notifier, time.Minute, log, nil)

	booking, err := svc.Confirm(context.Background(), ConfirmInput{
		EventID:       42,
		BookingID:     9,
		ActorUsername: "ann",
		ActorRole:     RoleUser,
	})
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if booking.ID != 9 {
		t.Fatalf("unexpected booking id: %d", booking.ID)
	}
}

func TestEventServiceConfirmRejectsForeignBooking(t *testing.T) {
	repo := &mockRepo{forbiddenConfirm: true}
	svc := newTestService(repo)

	_, err := svc.Confirm(context.Background(), ConfirmInput{
		EventID:       42,
		BookingID:     9,
		ActorUsername: "kate",
		ActorRole:     RoleUser,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestEventServiceCancelExpiredReturnsCancelledCount(t *testing.T) {
	repo := &mockRepo{
		cancelled: []domain.Booking{
			{ID: 7, EventID: 1, EventTitle: "Go workshop", UserEmail: "user@example.com", Status: domain.StatusCancelled},
		},
	}
	notifier := &mockNotifier{}
	log := zap.NewNop().Sugar()
	svc := NewEventService(repo, notifier, time.Minute, log, nil)

	count, err := svc.CancelExpired(context.Background())
	if err != nil {
		t.Fatalf("cancel expired failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 cancelled booking, got %d", count)
	}
}

func TestEventServiceDispatchNotificationsMarksSent(t *testing.T) {
	repo := &mockRepo{
		notifications: []domain.NotificationEvent{
			{
				ID:          10,
				Type:        domain.NotificationBookingCreated,
				Channel:     domain.NotificationChannelEmail,
				Attempts:    1,
				MaxAttempts: 5,
				Booking:     domain.Booking{ID: 7, EventTitle: "Go workshop", UserEmail: "ann@example.com", Status: domain.StatusPending},
			},
		},
	}
	notifier := &mockNotifier{}
	svc := NewEventService(repo, notifier, time.Minute, zap.NewNop().Sugar(), nil)

	count, err := svc.DispatchNotifications(context.Background())
	if err != nil {
		t.Fatalf("dispatch notifications failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one sent notification, got %d", count)
	}
	if notifier.createdCalls != 1 {
		t.Fatalf("expected created notifier call, got %d", notifier.createdCalls)
	}
	if len(repo.sentNotifications) != 1 || repo.sentNotifications[0] != 10 {
		t.Fatalf("expected notification 10 to be marked sent, got %#v", repo.sentNotifications)
	}
}

func TestEventServiceDispatchNotificationsReschedulesFailedSend(t *testing.T) {
	repo := &mockRepo{
		notifications: []domain.NotificationEvent{
			{
				ID:          11,
				Type:        domain.NotificationBookingCreated,
				Channel:     domain.NotificationChannelEmail,
				Attempts:    1,
				MaxAttempts: 5,
				Booking:     domain.Booking{ID: 8, EventTitle: "Go workshop", UserEmail: "ann@example.com", Status: domain.StatusPending},
			},
		},
	}
	notifier := &mockNotifier{failCreated: true}
	svc := NewEventService(repo, notifier, time.Minute, zap.NewNop().Sugar(), nil)

	count, err := svc.DispatchNotifications(context.Background())
	if err != nil {
		t.Fatalf("dispatch notifications failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero sent notifications, got %d", count)
	}
	if len(repo.rescheduledNotifications) != 1 || repo.rescheduledNotifications[0] != 11 {
		t.Fatalf("expected notification 11 to be rescheduled, got %#v", repo.rescheduledNotifications)
	}
}

func newTestService(repo *mockRepo) *EventService {
	log := zap.NewNop().Sugar()
	return NewEventService(repo, &mockNotifier{}, 2*time.Minute, log, nil)
}

type mockRepo struct {
	lastDeadline             time.Duration
	lastOwner                string
	cancelled                []domain.Booking
	confirmed                *domain.Booking
	forbiddenConfirm         bool
	notifications            []domain.NotificationEvent
	sentNotifications        []int64
	rescheduledNotifications []int64
	failedNotifications      []int64
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

func (m *mockRepo) ListBookingsByOwner(_ context.Context, ownerUsername string) ([]domain.Booking, error) {
	return []domain.Booking{
		{ID: 1, OwnerUsername: ownerUsername, EventTitle: "Go workshop", Status: domain.StatusPending},
	}, nil
}

func (m *mockRepo) Book(_ context.Context, eventID int64, ownerUsername string, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error) {
	m.lastDeadline = deadline
	m.lastOwner = ownerUsername
	return &domain.Booking{
		ID:            1,
		EventID:       eventID,
		EventTitle:    "Go workshop",
		OwnerUsername: ownerUsername,
		UserName:      userName,
		UserEmail:     userEmail,
		UserTelegram:  userTelegram,
		Status:        domain.StatusPending,
		ExpiresAt:     time.Now().Add(deadline),
	}, nil
}

func (m *mockRepo) Confirm(_ context.Context, _ int64, _ int64, _ string, _ bool) (*domain.Booking, error) {
	if m.forbiddenConfirm {
		return nil, domain.ErrForbidden
	}
	if m.confirmed != nil {
		return m.confirmed, nil
	}
	return &domain.Booking{ID: 1, EventID: 1, EventTitle: "Go workshop", Status: domain.StatusConfirmed}, nil
}

func (m *mockRepo) CancelExpired(_ context.Context) ([]domain.Booking, error) {
	return m.cancelled, nil
}

func (m *mockRepo) FetchDueNotifications(_ context.Context, _ int) ([]domain.NotificationEvent, error) {
	return m.notifications, nil
}

func (m *mockRepo) MarkNotificationSent(_ context.Context, id int64) error {
	m.sentNotifications = append(m.sentNotifications, id)
	return nil
}

func (m *mockRepo) RescheduleNotification(_ context.Context, id int64, _ time.Time, _ string) error {
	m.rescheduledNotifications = append(m.rescheduledNotifications, id)
	return nil
}

func (m *mockRepo) MarkNotificationFailed(_ context.Context, id int64, _ string) error {
	m.failedNotifications = append(m.failedNotifications, id)
	return nil
}

type mockNotifier struct {
	createdCalls   int
	confirmedCalls int
	cancelledCalls int
	failCreated    bool
}

func (m *mockNotifier) Notify(_ context.Context, item domain.NotificationEvent) error {
	if m.failCreated && item.Type == domain.NotificationBookingCreated {
		return errors.New("send failed")
	}
	switch item.Type {
	case domain.NotificationBookingCreated:
		m.createdCalls++
	case domain.NotificationBookingConfirmed:
		m.confirmedCalls++
	case domain.NotificationBookingCancelled:
		m.cancelledCalls++
	}
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
