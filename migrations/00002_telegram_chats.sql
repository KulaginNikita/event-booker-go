-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS telegram_chats (
    username   TEXT PRIMARY KEY,
    chat_id    BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS telegram_chats;
-- +goose StatementEnd
