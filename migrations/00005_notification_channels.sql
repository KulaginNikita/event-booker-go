-- +goose Up
-- +goose StatementBegin
ALTER TABLE notification_outbox
    ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'log'
        CHECK (channel IN ('log', 'email', 'telegram'));

ALTER TABLE notification_outbox
    DROP CONSTRAINT IF EXISTS notification_outbox_booking_id_event_type_key;

ALTER TABLE notification_outbox
    ADD CONSTRAINT notification_outbox_booking_event_channel_key
        UNIQUE (booking_id, event_type, channel);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE notification_outbox
    DROP CONSTRAINT IF EXISTS notification_outbox_booking_event_channel_key;

ALTER TABLE notification_outbox
    ADD CONSTRAINT notification_outbox_booking_id_event_type_key
        UNIQUE (booking_id, event_type);

ALTER TABLE notification_outbox
    DROP COLUMN IF EXISTS channel;
-- +goose StatementEnd
