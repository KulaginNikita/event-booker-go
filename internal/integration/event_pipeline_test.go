//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
	"github.com/KulaginNikita/event-booker/internal/migrator"
	pgrepo "github.com/KulaginNikita/event-booker/internal/repository/postgres"
	"github.com/KulaginNikita/event-booker/internal/service"
)

type recordingNotifier struct {
	created   []domain.Booking
	confirmed []domain.Booking
	cancelled []domain.Booking
}

func (n *recordingNotifier) Notify(_ context.Context, item domain.NotificationEvent) error {
	switch item.Type {
	case domain.NotificationBookingCreated:
		n.created = append(n.created, item.Booking)
	case domain.NotificationBookingConfirmed:
		n.confirmed = append(n.confirmed, item.Booking)
	case domain.NotificationBookingCancelled:
		n.cancelled = append(n.cancelled, item.Booking)
	}
	return nil
}

func TestEventBookingOwnershipAndOutboxWithPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("eventbooker"),
		tcpostgres.WithUsername("eventbooker"),
		tcpostgres.WithPassword("eventbooker"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	runMigrations(t, ctx, dsn)

	repo, err := pgrepo.NewEventRepository(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("create event repository: %v", err)
	}
	defer repo.Close()

	notifier := &recordingNotifier{}
	svc := service.NewEventService(repo, notifier, time.Minute, zap.NewNop().Sugar(), nil)

	event, err := svc.CreateEvent(ctx, service.CreateEventInput{
		Title:    "Go workshop",
		StartsAt: time.Now().UTC().Add(time.Hour),
		Capacity: 1,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	booking, err := svc.Book(ctx, service.BookInput{
		EventID:       event.ID,
		OwnerUsername: "alice",
		UserName:      "Alice",
		UserEmail:     "alice@example.com",
		UserTelegram:  "",
	})
	if err != nil {
		t.Fatalf("book event: %v", err)
	}
	if booking.OwnerUsername != "alice" {
		t.Fatalf("booking owner = %q, want alice", booking.OwnerUsername)
	}

	_, err = svc.Confirm(ctx, service.ConfirmInput{
		EventID:       event.ID,
		BookingID:     booking.ID,
		ActorUsername: "bob",
		ActorRole:     service.RoleUser,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("foreign confirm error = %v, want forbidden", err)
	}

	confirmed, err := svc.Confirm(ctx, service.ConfirmInput{
		EventID:       event.ID,
		BookingID:     booking.ID,
		ActorUsername: "alice",
		ActorRole:     service.RoleUser,
	})
	if err != nil {
		t.Fatalf("owner confirm: %v", err)
	}
	if confirmed.Status != domain.StatusConfirmed {
		t.Fatalf("booking status = %q, want confirmed", confirmed.Status)
	}

	sent, err := svc.DispatchNotifications(ctx)
	if err != nil {
		t.Fatalf("dispatch notifications: %v", err)
	}
	if sent != 3 {
		t.Fatalf("sent notifications = %d, want 3", sent)
	}
	if len(notifier.created) != 0 || len(notifier.confirmed) != 3 {
		t.Fatalf("notifier calls: created=%d confirmed=%d", len(notifier.created), len(notifier.confirmed))
	}
}

func TestConcurrentBookingsDoNotExceedCapacity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("eventbooker"),
		tcpostgres.WithUsername("eventbooker"),
		tcpostgres.WithPassword("eventbooker"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	runMigrations(t, ctx, dsn)

	repo, err := pgrepo.NewEventRepository(ctx, dsn, 10)
	if err != nil {
		t.Fatalf("create event repository: %v", err)
	}
	defer repo.Close()

	svc := service.NewEventService(repo, &recordingNotifier{}, time.Minute, zap.NewNop().Sugar(), nil)
	event, err := svc.CreateEvent(ctx, service.CreateEventInput{
		Title:    "Concurrent Go workshop",
		StartsAt: time.Now().UTC().Add(time.Hour),
		Capacity: 10,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Book(ctx, service.BookInput{
				EventID:       event.ID,
				OwnerUsername: fmt.Sprintf("user-%03d", i),
				UserName:      fmt.Sprintf("User %03d", i),
				UserEmail:     fmt.Sprintf("user-%03d@example.com", i),
			})
			if err == nil {
				successes.Add(1)
				return
			}
			if !errors.Is(err, domain.ErrNoSeats) {
				t.Errorf("unexpected booking error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if successes.Load() != 10 {
		t.Fatalf("successful bookings = %d, want 10", successes.Load())
	}

	loaded, err := svc.GetEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if loaded.FreeSeats != 0 || loaded.PendingBookings != 10 {
		t.Fatalf("event counters: free=%d pending=%d", loaded.FreeSeats, loaded.PendingBookings)
	}
}

func runMigrations(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pg config: %v", err)
	}

	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping migration db: %v", err)
	}
	if err := migrator.NewMigrator(db, "../../migrations").Up(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}
