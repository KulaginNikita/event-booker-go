package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type EventRepository struct {
	pool *pgxpool.Pool
}

const notificationLockTTL = 5 * time.Minute

func NewEventRepository(ctx context.Context, dsn string, maxPoolSize int32) (*EventRepository, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if maxPoolSize > 0 {
		cfg.MaxConns = maxPoolSize
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &EventRepository{pool: pool}, nil
}

func (r *EventRepository) Close() {
	r.pool.Close()
}

func (r *EventRepository) Ping(ctx context.Context) error {
	var one int
	if err := r.pool.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func (r *EventRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUnauthorized
		}
		return nil, mapPostgresError(err, "get user by username")
	}
	return user, nil
}

func (r *EventRepository) CreateEvent(ctx context.Context, event *domain.Event) error {
	sql, args, err := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("events").
		Columns("title", "starts_at", "capacity").
		Values(event.Title, event.StartsAt, event.Capacity).
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build create event query: %w", err)
	}

	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt); err != nil {
		return mapPostgresError(err, "create event")
	}
	event.FreeSeats = event.Capacity
	return nil
}

func (r *EventRepository) ListEvents(ctx context.Context, filter domain.ListEventsFilter) ([]domain.Event, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	orderBy := "e.starts_at ASC, e.id ASC"
	switch filter.Sort {
	case "-starts_at":
		orderBy = "e.starts_at DESC, e.id DESC"
	case "created_at":
		orderBy = "e.created_at ASC, e.id ASC"
	case "-created_at":
		orderBy = "e.created_at DESC, e.id DESC"
	case "title":
		orderBy = "lower(e.title) ASC, e.id ASC"
	case "-title":
		orderBy = "lower(e.title) DESC, e.id DESC"
	}

	query := fmt.Sprintf(`
		SELECT e.id, e.title, e.starts_at, e.capacity, e.created_at, e.updated_at,
			COUNT(b.id) FILTER (WHERE b.status IN ('pending', 'confirmed')) AS busy,
			COUNT(b.id) FILTER (WHERE b.status = 'pending') AS pending,
			COUNT(b.id) FILTER (WHERE b.status = 'confirmed') AS confirmed
		FROM events e
		LEFT JOIN bookings b ON b.event_id = e.id
		GROUP BY e.id
		ORDER BY %s
		LIMIT $1 OFFSET $2
	`, orderBy)

	rows, err := r.pool.Query(ctx, query, filter.Limit, filter.Offset)
	if err != nil {
		return nil, mapPostgresError(err, "list events")
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
	event, err := r.getEventSummary(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}

	bookings, err := r.listBookings(ctx, r.pool, id)
	if err != nil {
		return nil, err
	}
	event.Bookings = bookings
	return event, nil
}

func (r *EventRepository) Book(ctx context.Context, eventID int64, ownerUsername string, userName string, userEmail string, userTelegram string, deadline time.Duration) (*domain.Booking, error) {
	var created *domain.Booking
	err := r.executeInTx(ctx, func(tx pgx.Tx) error {
		eventTitle, startsAt, err := r.lockEvent(ctx, tx, eventID)
		if err != nil {
			return err
		}
		if !startsAt.After(time.Now().UTC()) {
			return domain.ErrEventAlreadyStarted
		}
		cancelled, err := r.cancelExpired(ctx, tx, eventID)
		if err != nil {
			return err
		}
		for _, booking := range cancelled {
			booking.EventTitle = eventTitle
			if err := r.enqueueNotification(ctx, tx, booking.ID, domain.NotificationBookingCancelled); err != nil {
				return err
			}
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
			INSERT INTO bookings (event_id, owner_username, user_name, user_email, user_telegram, status, expires_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', $6)
			RETURNING id, event_id, owner_username, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		`, eventID, ownerUsername, userName, userEmail, userTelegram, expiresAt).Scan(
			&booking.ID,
			&booking.EventID,
			&booking.OwnerUsername,
			&booking.UserName,
			&booking.UserEmail,
			&booking.UserTelegram,
			&booking.Status,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return mapPostgresError(err, "insert booking")
		}
		booking.EventTitle = eventTitle
		if err := r.enqueueNotification(ctx, tx, booking.ID, domain.NotificationBookingCreated); err != nil {
			return err
		}
		created = booking
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *EventRepository) Confirm(ctx context.Context, eventID int64, bookingID int64, actorUsername string, isAdmin bool) (*domain.Booking, error) {
	var confirmed *domain.Booking
	var resultErr error
	err := r.executeInTx(ctx, func(tx pgx.Tx) error {
		eventTitle, startsAt, err := r.lockEvent(ctx, tx, eventID)
		if err != nil {
			return err
		}
		if !startsAt.After(time.Now().UTC()) {
			return domain.ErrEventAlreadyStarted
		}

		booking, err := r.getBookingForUpdate(ctx, tx, eventID, bookingID)
		if err != nil {
			return err
		}
		booking.EventTitle = eventTitle
		if !isAdmin && booking.OwnerUsername != actorUsername {
			return domain.ErrForbidden
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
				return mapPostgresError(err, "cancel expired booking")
			}
			booking.Status = domain.StatusCancelled
			if err := r.enqueueNotification(ctx, tx, booking.ID, domain.NotificationBookingCancelled); err != nil {
				return err
			}
			resultErr = domain.ErrBookingExpired
			return nil
		}

		err = tx.QueryRow(ctx, `
			UPDATE bookings
			SET status = 'confirmed', updated_at = now()
			WHERE id = $1
			RETURNING id, event_id, owner_username, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		`, booking.ID).Scan(
			&booking.ID,
			&booking.EventID,
			&booking.OwnerUsername,
			&booking.UserName,
			&booking.UserEmail,
			&booking.UserTelegram,
			&booking.Status,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return mapPostgresError(err, "confirm booking")
		}
		booking.EventTitle = eventTitle
		if err := r.enqueueNotification(ctx, tx, booking.ID, domain.NotificationBookingConfirmed); err != nil {
			return err
		}
		confirmed = booking
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return confirmed, nil
}

func (r *EventRepository) CancelExpired(ctx context.Context) ([]domain.Booking, error) {
	var cancelled []domain.Booking
	err := r.executeInTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH picked AS (
				SELECT b.id
				FROM bookings b
				WHERE b.status = 'pending' AND b.expires_at <= now()
				ORDER BY b.expires_at ASC, b.id ASC
				LIMIT 100
				FOR UPDATE SKIP LOCKED
			)
			UPDATE bookings b
			SET status = 'cancelled', updated_at = now()
			FROM events e
			WHERE b.event_id = e.id
			  AND b.id IN (SELECT id FROM picked)
			RETURNING b.id, b.event_id, e.title, b.owner_username, b.user_name, b.user_email, b.user_telegram, b.status, b.expires_at, b.created_at, b.updated_at
		`)
		if err != nil {
			return mapPostgresError(err, "cancel expired bookings")
		}
		defer rows.Close()

		items, err := scanBookingsWithEventTitle(rows)
		if err != nil {
			return err
		}
		for _, booking := range items {
			if err := r.enqueueNotification(ctx, tx, booking.ID, domain.NotificationBookingCancelled); err != nil {
				return err
			}
		}
		cancelled = items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cancelled, nil
}

func (r *EventRepository) ListBookingsByOwner(ctx context.Context, ownerUsername string) ([]domain.Booking, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT b.id, b.event_id, e.title, b.owner_username, b.user_name, b.user_email,
		       b.user_telegram, b.status, b.expires_at, b.created_at, b.updated_at
		FROM bookings b
		JOIN events e ON e.id = b.event_id
		WHERE b.owner_username = $1
		ORDER BY b.created_at DESC, b.id DESC
	`, ownerUsername)
	if err != nil {
		return nil, mapPostgresError(err, "list owner bookings")
	}
	defer rows.Close()
	return scanBookingsWithEventTitle(rows)
}

func (r *EventRepository) lockEvent(ctx context.Context, tx queryExecuter, id int64) (string, time.Time, error) {
	var title string
	var startsAt time.Time
	if err := tx.QueryRow(ctx, `SELECT title, starts_at FROM events WHERE id = $1 FOR UPDATE`, id).Scan(&title, &startsAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, domain.ErrEventNotFound
		}
		return "", time.Time{}, mapPostgresError(err, "lock event")
	}
	return title, startsAt, nil
}

func (r *EventRepository) cancelExpired(ctx context.Context, tx queryExecuter, eventID int64) ([]domain.Booking, error) {
	rows, err := tx.Query(ctx, `
		UPDATE bookings
		SET status = 'cancelled', updated_at = now()
		WHERE event_id = $1 AND status = 'pending' AND expires_at <= now()
		RETURNING id, event_id, owner_username, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
	`, eventID)
	if err != nil {
		return nil, mapPostgresError(err, "cancel expired event bookings")
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (r *EventRepository) freeSeats(ctx context.Context, tx queryExecuter, eventID int64) (int, error) {
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
		return 0, mapPostgresError(err, "count free seats")
	}
	return capacity - busy, nil
}

func (r *EventRepository) getBookingForUpdate(ctx context.Context, tx queryExecuter, eventID int64, bookingID int64) (*domain.Booking, error) {
	booking := &domain.Booking{}
	err := tx.QueryRow(ctx, `
		SELECT id, event_id, owner_username, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
		FROM bookings
		WHERE event_id = $1 AND id = $2
		FOR UPDATE
	`, eventID, bookingID).Scan(
		&booking.ID,
		&booking.EventID,
		&booking.OwnerUsername,
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
		return nil, mapPostgresError(err, "get booking for update")
	}
	return booking, nil
}

func (r *EventRepository) getEventSummary(ctx context.Context, q queryExecuter, id int64) (*domain.Event, error) {
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

func (r *EventRepository) listBookings(ctx context.Context, q queryExecuter, eventID int64) ([]domain.Booking, error) {
	rows, err := q.Query(ctx, `
		SELECT id, event_id, owner_username, user_name, user_email, user_telegram, status, expires_at, created_at, updated_at
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

func (r *EventRepository) FetchDueNotifications(ctx context.Context, limit int) ([]domain.NotificationEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	var result []domain.NotificationEvent
	err := r.executeInTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH picked AS (
				SELECT id
				FROM notification_outbox
				WHERE
					(status = 'pending' AND next_attempt_at <= now())
					OR (status = 'processing' AND locked_until <= now())
				ORDER BY next_attempt_at ASC, id ASC
				LIMIT $1
				FOR UPDATE SKIP LOCKED
			),
			updated AS (
				UPDATE notification_outbox o
				SET status = 'processing',
					attempts = attempts + 1,
					locked_until = now() + ($2::integer * interval '1 second'),
					updated_at = now()
				FROM picked
				WHERE o.id = picked.id
				RETURNING o.id, o.booking_id, o.event_type, o.channel, o.attempts, o.max_attempts
			)
			SELECT u.id, u.event_type, u.channel, u.attempts, u.max_attempts,
				b.id, b.event_id, e.title, b.owner_username, b.user_name, b.user_email,
				b.user_telegram, b.status, b.expires_at, b.created_at, b.updated_at
			FROM updated u
			JOIN bookings b ON b.id = u.booking_id
			JOIN events e ON e.id = b.event_id
			ORDER BY u.id ASC
		`, limit, int(notificationLockTTL.Seconds()))
		if err != nil {
			return mapPostgresError(err, "fetch due notifications")
		}
		defer rows.Close()

		items, err := scanNotificationEvents(rows)
		if err != nil {
			return err
		}
		result = items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *EventRepository) MarkNotificationSent(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'sent', locked_until = NULL, last_error = '', updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`, id)
	if err != nil {
		return mapPostgresError(err, "mark notification sent")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidState
	}
	return nil
}

func (r *EventRepository) RescheduleNotification(ctx context.Context, id int64, nextAttemptAt time.Time, reason string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'pending',
			next_attempt_at = $2,
			locked_until = NULL,
			last_error = $3,
			updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`, id, nextAttemptAt, trimReason(reason))
	if err != nil {
		return mapPostgresError(err, "reschedule notification")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidState
	}
	return nil
}

func (r *EventRepository) MarkNotificationFailed(ctx context.Context, id int64, reason string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'failed',
			locked_until = NULL,
			last_error = $2,
			updated_at = now()
		WHERE id = $1 AND status = 'processing'
	`, id, trimReason(reason))
	if err != nil {
		return mapPostgresError(err, "mark notification failed")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidState
	}
	return nil
}

func (r *EventRepository) enqueueNotification(ctx context.Context, tx queryExecuter, bookingID int64, eventType domain.NotificationEventType) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_outbox (booking_id, event_type, channel)
		VALUES
			($1, $2, $3),
			($1, $2, $4),
			($1, $2, $5)
		ON CONFLICT (booking_id, event_type, channel) DO NOTHING
	`, bookingID, eventType,
		domain.NotificationChannelLog,
		domain.NotificationChannelEmail,
		domain.NotificationChannelTelegram,
	)
	if err != nil {
		return mapPostgresError(err, "enqueue notification")
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

type queryExecuter interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *EventRepository) executeInTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
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
		&booking.OwnerUsername,
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

func scanBookingWithEventTitle(scanner scanner) (*domain.Booking, error) {
	var booking domain.Booking
	if err := scanner.Scan(
		&booking.ID,
		&booking.EventID,
		&booking.EventTitle,
		&booking.OwnerUsername,
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
		return nil, fmt.Errorf("scan booking with event title: %w", err)
	}
	return &booking, nil
}

func scanBookingsWithEventTitle(rows pgx.Rows) ([]domain.Booking, error) {
	var result []domain.Booking
	for rows.Next() {
		booking, err := scanBookingWithEventTitle(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *booking)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings with event title: %w", err)
	}
	return result, nil
}

func scanNotificationEvents(rows pgx.Rows) ([]domain.NotificationEvent, error) {
	var result []domain.NotificationEvent
	for rows.Next() {
		var item domain.NotificationEvent
		var eventType string
		var channel string
		if err := rows.Scan(
			&item.ID,
			&eventType,
			&channel,
			&item.Attempts,
			&item.MaxAttempts,
			&item.Booking.ID,
			&item.Booking.EventID,
			&item.Booking.EventTitle,
			&item.Booking.OwnerUsername,
			&item.Booking.UserName,
			&item.Booking.UserEmail,
			&item.Booking.UserTelegram,
			&item.Booking.Status,
			&item.Booking.ExpiresAt,
			&item.Booking.CreatedAt,
			&item.Booking.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification event: %w", err)
		}
		item.Type = domain.NotificationEventType(eventType)
		item.Channel = domain.NotificationChannel(channel)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification events: %w", err)
	}
	return result, nil
}

func trimReason(reason string) string {
	const maxReasonLen = 1000
	if len(reason) <= maxReasonLen {
		return reason
	}
	return reason[:maxReasonLen]
}

func mapPostgresError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "idx_bookings_one_active_per_owner" {
				return domain.ErrAlreadyBooked
			}
			return domain.ErrInvalidState
		case "23503":
			return domain.ErrEventNotFound
		case "23514":
			switch pgErr.ConstraintName {
			case "events_title_check":
				return domain.ErrInvalidTitle
			case "events_capacity_check":
				return domain.ErrInvalidCapacity
			case "bookings_user_name_check", "bookings_user_email_check":
				return domain.ErrInvalidUser
			}
			return domain.ErrInvalidUser
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
