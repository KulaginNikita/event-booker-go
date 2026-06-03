package notifier

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/domain"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramChatStore interface {
	UpsertTelegramChat(ctx context.Context, username string, chatID int64) error
	GetTelegramChatIDByUsername(ctx context.Context, username string) (int64, error)
}

type TelegramNotifier struct {
	cfg   config.TelegramConfig
	bot   *tgbotapi.BotAPI
	store TelegramChatStore
	log   logger.Logger
}

func NewTelegram(ctx context.Context, cfg config.TelegramConfig, store TelegramChatStore, log logger.Logger) (*TelegramNotifier, error) {
	if !cfg.Enabled {
		return &TelegramNotifier{cfg: cfg, store: store, log: log}, nil
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("telegram token is empty")
	}
	if store == nil {
		return nil, errors.New("telegram chat store is nil")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	n := &TelegramNotifier{
		cfg:   cfg,
		bot:   bot,
		store: store,
		log:   log,
	}
	n.startUpdatesListener(ctx)
	return n, nil
}

func (n *TelegramNotifier) BookingCreated(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, buildCreatedText(booking))
}

func (n *TelegramNotifier) BookingConfirmed(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, buildConfirmedText(booking))
}

func (n *TelegramNotifier) BookingCancelled(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, buildCancellationText(booking))
}

func (n *TelegramNotifier) send(ctx context.Context, booking domain.Booking, text string) error {
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

	msg, err := n.buildTelegramMessage(ctx, recipient, text)
	if err != nil {
		return err
	}
	return sendTelegramMessage(ctx, n.bot, msg)
}

func (n *TelegramNotifier) buildTelegramMessage(ctx context.Context, recipient string, text string) (tgbotapi.MessageConfig, error) {
	if chatID, err := strconv.ParseInt(recipient, 10, 64); err == nil {
		return tgbotapi.NewMessage(chatID, text), nil
	}

	username := normalizeTelegramUsername(recipient)
	if username == "" {
		return tgbotapi.MessageConfig{}, errors.New("telegram recipient is empty")
	}

	chatID, err := n.store.GetTelegramChatIDByUsername(ctx, username)
	if err == nil {
		return tgbotapi.NewMessage(chatID, text), nil
	}
	if !errors.Is(err, domain.ErrTelegramChatNotFound) {
		return tgbotapi.MessageConfig{}, fmt.Errorf("resolve telegram username %q: %w", recipient, err)
	}

	if strings.HasPrefix(recipient, "@") {
		return tgbotapi.NewMessageToChannel(recipient, text), nil
	}

	return tgbotapi.MessageConfig{}, fmt.Errorf(
		"telegram username %q is unknown: user must send any message to the bot first",
		recipient,
	)
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

func (n *TelegramNotifier) startUpdatesListener(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := n.bot.GetUpdatesChan(u)
	go func() {
		defer n.bot.StopReceivingUpdates()

		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				n.rememberChat(ctx, update)
			}
		}
	}()
}

func (n *TelegramNotifier) rememberChat(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil || update.Message.Chat == nil || update.Message.From == nil {
		return
	}

	username := normalizeTelegramUsername(update.Message.From.UserName)
	if username == "" {
		return
	}

	chatID := update.Message.Chat.ID
	if chatID == 0 {
		return
	}

	if err := n.store.UpsertTelegramChat(ctx, username, chatID); err != nil {
		n.log.Warn("failed to remember telegram chat", "username", username, "chat_id", chatID, "error", err)
		return
	}

	n.log.Info("telegram chat remembered", "username", username, "chat_id", chatID)
}

func normalizeTelegramUsername(username string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
}
