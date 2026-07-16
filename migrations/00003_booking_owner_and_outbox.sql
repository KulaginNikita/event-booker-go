-- +goose Up
-- +goose StatementBegin
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS owner_username TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('booking_created', 'booking_confirmed', 'booking_cancelled')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_until TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (booking_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_outbox_due
    ON notification_outbox(status, next_attempt_at, locked_until);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_outbox;
ALTER TABLE bookings DROP COLUMN IF EXISTS owner_username;
-- +goose StatementEnd
