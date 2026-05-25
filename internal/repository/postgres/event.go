package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type EventRepository struct {
	pg *pgxdriver.Postgres
	tm transaction.Manager
}

func NewEventRepository(pg *pgxdriver.Postgres, log logger.Logger) (*EventRepository, error) {
	tm, err := transaction.NewManager(pg, log)
	if err != nil {
		return nil, err
	}
	return &EventRepository{pg: pg, tm: tm}, nil
}

func (r *EventRepository) CreateEvent(ctx context.Context, event *domain.Event) error {
	sql, args, err := r.pg.Insert("events").
		Columns("title", "starts_at", "capacity").
		Values(event.Title, event.StartsAt, event.Capacity).
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build create event query: %w", err)
	}

	if err := r.pg.QueryRow(ctx, sql, args...).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt); err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	event.FreeSeats = event.Capacity
	return nil
}

func (r *EventRepository) ListEvents(ctx context.Context) ([]domain.Event, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT e.id, e.title, e.starts_at, e.capacity, e.created_at, e.updated_at,
			COUNT(b.id) FILTER (WHERE b.status IN ('pending', 'confirmed')) AS busy,
			COUNT(b.id) FILTER (WHERE b.status = 'pending') AS pending,
			COUNT(b.id) FILTER (WHERE b.status = 'confirmed') AS confirmed
		FROM events e
		LEFT JOIN bookings b ON b.event_id = e.id
		GROUP BY e.id
		ORDER BY e.starts_at ASC, e.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var result []domain.Event
	for rows.Next() {
		event, err := scanEventSummary(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return result, nil
}

func (r *EventRepository) GetEvent(ctx context.Context, id int64) (*domain.Event, error) {
	event, err := r.getEventSummary(ctx, r.pg, id)
	if err != nil {
		return nil, err
	}

	bookings, err := r.listBookings(ctx, r.pg, id)
	if err != nil {
		return nil, err
	}
	event.Bookings = bookings
	return event, nil
}

func (r *EventRepository) Book(ctx context.Context, eventID int64, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error) {
	var created *domain.Booking
	err := r.tm.ExecuteInTransaction(ctx, "book_event", func(tx pgxdriver.QueryExecuter) error {
		if err := r.lockEvent(ctx, tx, eventID); err != nil {
			return err
		}
		if _, err := r.cancelExpired(ctx, tx, eventID); err != nil {
			return err
		}

		freeSeats, err := r.freeSeats(ctx, tx, eventID)
		if err != nil {
			return err
		}
		if freeSeats <= 0 {
			return domain.ErrNoSeats
		}

		booking := &domain.Booking{}
		expiresAt := time.Now().UTC().Add(deadline)
		err = tx.QueryRow(ctx, `
			INSERT INTO bookings (event_id, user_name, user_email, user_telegram, status, expires_at)
			VALUES ($1, $2, $3, $4, 'pending', $5)
			RETURNING id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		`, eventID, userName, userEmail, userTelegram, expiresAt).Scan(
			&booking.ID,
			&booking.EventID,
			&booking.UserName,
			&booking.UserEmail,
			&booking.UserTelegram,
			&booking.Status,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert booking: %w", err)
		}
		created = booking
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *EventRepository) Confirm(ctx context.Context, eventID int64, bookingID int64) (*domain.Booking, error) {
	var confirmed *domain.Booking
	err := r.tm.ExecuteInTransaction(ctx, "confirm_booking", func(tx pgxdriver.QueryExecuter) error {
		if err := r.lockEvent(ctx, tx, eventID); err != nil {
			return err
		}

		booking, err := r.getBookingForUpdate(ctx, tx, eventID, bookingID)
		if err != nil {
			return err
		}
		if booking.Status == domain.StatusConfirmed {
			confirmed = booking
			return nil
		}
		if booking.Status != domain.StatusPending {
			return domain.ErrBookingNotPending
		}
		if time.Now().UTC().After(booking.ExpiresAt) {
			if _, err := tx.Exec(ctx, `
				UPDATE bookings
				SET status = 'cancelled', updated_at = now()
				WHERE id = $1
			`, booking.ID); err != nil {
				return fmt.Errorf("cancel expired booking: %w", err)
			}
			return domain.ErrBookingExpired
		}

		err = tx.QueryRow(ctx, `
			UPDATE bookings
			SET status = 'confirmed', updated_at = now()
			WHERE id = $1
			RETURNING id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		`, booking.ID).Scan(
			&booking.ID,
			&booking.EventID,
			&booking.UserName,
			&booking.UserEmail,
			&booking.UserTelegram,
			&booking.Status,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("confirm booking: %w", err)
		}
		confirmed = booking
		return nil
	})
	if err != nil {
		return nil, err
	}
	return confirmed, nil
}

func (r *EventRepository) CancelExpired(ctx context.Context) ([]domain.Booking, error) {
	var cancelled []domain.Booking
	err := r.tm.ExecuteInTransaction(ctx, "cancel_expired_bookings", func(tx pgxdriver.QueryExecuter) error {
		rows, err := tx.Query(ctx, `
			UPDATE bookings
			SET status = 'cancelled', updated_at = now()
			WHERE status = 'pending' AND expires_at <= now()
			RETURNING id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		`)
		if err != nil {
			return fmt.Errorf("cancel expired bookings: %w", err)
		}
		defer rows.Close()

		items, err := scanBookings(rows)
		if err != nil {
			return err
		}
		cancelled = items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cancelled, nil
}

func (r *EventRepository) lockEvent(ctx context.Context, tx pgxdriver.QueryExecuter, id int64) error {
	var exists int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM events WHERE id = $1 FOR UPDATE`, id).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrEventNotFound
		}
		return fmt.Errorf("lock event: %w", err)
	}
	return nil
}

func (r *EventRepository) cancelExpired(ctx context.Context, tx pgxdriver.QueryExecuter, eventID int64) ([]domain.Booking, error) {
	rows, err := tx.Query(ctx, `
		UPDATE bookings
		SET status = 'cancelled', updated_at = now()
		WHERE event_id = $1 AND status = 'pending' AND expires_at <= now()
		RETURNING id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("cancel expired event bookings: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *EventRepository) freeSeats(ctx context.Context, tx pgxdriver.QueryExecuter, eventID int64) (int, error) {
	var capacity int
	var busy int
	if err := tx.QueryRow(ctx, `
		SELECT e.capacity, COUNT(b.id) FILTER (WHERE b.status IN ('pending', 'confirmed')) AS busy
		FROM events e
		LEFT JOIN bookings b ON b.event_id = e.id
		WHERE e.id = $1
		GROUP BY e.id
	`, eventID).Scan(&capacity, &busy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrEventNotFound
		}
		return 0, fmt.Errorf("count free seats: %w", err)
	}
	return capacity - busy, nil
}

func (r *EventRepository) getBookingForUpdate(ctx context.Context, tx pgxdriver.QueryExecuter, eventID int64, bookingID int64) (*domain.Booking, error) {
	booking := &domain.Booking{}
	err := tx.QueryRow(ctx, `
		SELECT id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		FROM bookings
		WHERE event_id = $1 AND id = $2
		FOR UPDATE
	`, eventID, bookingID).Scan(
		&booking.ID,
		&booking.EventID,
		&booking.UserName,
		&booking.UserEmail,
		&booking.UserTelegram,
		&booking.Status,
		&booking.ExpiresAt,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("get booking for update: %w", err)
	}
	return booking, nil
}

func (r *EventRepository) getEventSummary(ctx context.Context, q pgxdriver.QueryExecuter, id int64) (*domain.Event, error) {
	sql, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select(
			"e.id", "e.title", "e.starts_at", "e.capacity", "e.created_at", "e.updated_at",
			"COUNT(b.id) FILTER (WHERE b.status IN ('pending', 'confirmed')) AS busy",
			"COUNT(b.id) FILTER (WHERE b.status = 'pending') AS pending",
			"COUNT(b.id) FILTER (WHERE b.status = 'confirmed') AS confirmed",
		).
		From("events e").
		LeftJoin("bookings b ON b.event_id = e.id").
		Where(squirrel.Eq{"e.id": id}).
		GroupBy("e.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get event query: %w", err)
	}

	event, err := scanEventSummary(q.QueryRow(ctx, sql, args...))
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (r *EventRepository) listBookings(ctx context.Context, q pgxdriver.QueryExecuter, eventID int64) ([]domain.Booking, error) {
	rows, err := q.Query(ctx, `
		SELECT id, event_id, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		FROM bookings
		WHERE event_id = $1
		ORDER BY created_at DESC, id DESC
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEventSummary(scanner scanner) (*domain.Event, error) {
	var event domain.Event
	var busy int
	if err := scanner.Scan(
		&event.ID,
		&event.Title,
		&event.StartsAt,
		&event.Capacity,
		&event.CreatedAt,
		&event.UpdatedAt,
		&busy,
		&event.PendingBookings,
		&event.ConfirmedBookings,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEventNotFound
		}
		return nil, fmt.Errorf("scan event: %w", err)
	}
	event.FreeSeats = event.Capacity - busy
	return &event, nil
}

func scanBooking(scanner scanner) (*domain.Booking, error) {
	var booking domain.Booking
	if err := scanner.Scan(
		&booking.ID,
		&booking.EventID,
		&booking.UserName,
		&booking.UserEmail,
		&booking.UserTelegram,
		&booking.Status,
		&booking.ExpiresAt,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("scan booking: %w", err)
	}
	return &booking, nil
}

func scanBookings(rows pgx.Rows) ([]domain.Booking, error) {
	var result []domain.Booking
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *booking)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}
	return result, nil
}
