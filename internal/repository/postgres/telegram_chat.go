package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func (r *EventRepository) UpsertTelegramChat(ctx context.Context, username string, chatID int64) error {
	const q = `
		INSERT INTO telegram_chats (username, chat_id, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (username) DO UPDATE
		SET chat_id = EXCLUDED.chat_id,
			updated_at = now()
	`
	if _, err := r.pool.Exec(ctx, q, username, chatID); err != nil {
		return fmt.Errorf("upsert telegram chat: %w", err)
	}
	return nil
}

func (r *EventRepository) GetTelegramChatIDByUsername(ctx context.Context, username string) (int64, error) {
	const q = `
		SELECT chat_id
		FROM telegram_chats
		WHERE username = $1
	`

	var chatID int64
	if err := r.pool.QueryRow(ctx, q, username).Scan(&chatID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrTelegramChatNotFound
		}
		return 0, fmt.Errorf("get telegram chat by username: %w", err)
	}
	return chatID, nil
}
