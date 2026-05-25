package notifier

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/domain"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	cfg config.TelegramConfig
	bot *tgbotapi.BotAPI
}

func NewTelegram(cfg config.TelegramConfig) (*TelegramNotifier, error) {
	if !cfg.Enabled {
		return &TelegramNotifier{cfg: cfg}, nil
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("telegram token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &TelegramNotifier{cfg: cfg, bot: bot}, nil
}

func (n *TelegramNotifier) BookingCancelled(ctx context.Context, booking domain.Booking) error {
	if !n.cfg.Enabled {
		return nil
	}
	if n.bot == nil {
		return errors.New("telegram bot is not initialized")
	}

	recipient := strings.TrimSpace(booking.UserTelegram)
	if recipient == "" {
		return nil
	}

	msg, err := buildTelegramMessage(recipient, buildCancellationText(booking))
	if err != nil {
		return err
	}
	return sendTelegramMessage(ctx, n.bot, msg)
}

func buildTelegramMessage(recipient string, text string) (tgbotapi.MessageConfig, error) {
	if chatID, err := strconv.ParseInt(recipient, 10, 64); err == nil {
		return tgbotapi.NewMessage(chatID, text), nil
	}
	if strings.HasPrefix(recipient, "@") {
		return tgbotapi.NewMessageToChannel(recipient, text), nil
	}
	return tgbotapi.MessageConfig{}, errors.New("telegram recipient must be chat id or @channel")
}

func sendTelegramMessage(ctx context.Context, bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}

	resultCh := make(chan error, 1)
	go func() {
		_, err := bot.Send(msg)
		resultCh <- err
	}()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-resultCh:
		if err != nil {
			return fmt.Errorf("send telegram: %w", err)
		}
		return nil
	case <-timer.C:
		return errors.New("send telegram timeout")
	}
}
