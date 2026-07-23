-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_one_active_per_owner
    ON bookings(event_id, owner_username)
    WHERE status IN ('pending', 'confirmed');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_bookings_one_active_per_owner;
-- +goose StatementEnd
