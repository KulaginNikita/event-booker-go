package notifier

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type BookingNotifier interface {
	BookingCreated(ctx context.Context, booking domain.Booking) error
	BookingConfirmed(ctx context.Context, booking domain.Booking) error
	BookingCancelled(ctx context.Context, booking domain.Booking) error
}

type ChannelNotifier struct {
	Channel  domain.NotificationChannel
	Notifier BookingNotifier
}

type MultiNotifier struct {
	items map[domain.NotificationChannel]BookingNotifier
	log   *zap.SugaredLogger
}

func NewChannel(channel domain.NotificationChannel, item BookingNotifier) ChannelNotifier {
	return ChannelNotifier{Channel: channel, Notifier: item}
}

func NewMulti(log *zap.SugaredLogger, items ...ChannelNotifier) *MultiNotifier {
	result := &MultiNotifier{items: make(map[domain.NotificationChannel]BookingNotifier), log: log}
	for _, item := range items {
		if item.Channel == "" || item.Notifier == nil {
			continue
		}
		result.items[item.Channel] = item.Notifier
	}
	return result
}

func (n *MultiNotifier) Notify(ctx context.Context, item domain.NotificationEvent) error {
	notifier, ok := n.items[item.Channel]
	if !ok {
		return fmt.Errorf("notification channel %q is not configured", item.Channel)
	}

	switch item.Type {
	case domain.NotificationBookingCreated:
		return notifier.BookingCreated(ctx, item.Booking)
	case domain.NotificationBookingConfirmed:
		return notifier.BookingConfirmed(ctx, item.Booking)
	case domain.NotificationBookingCancelled:
		return notifier.BookingCancelled(ctx, item.Booking)
	default:
		return fmt.Errorf("unknown notification event type %q", item.Type)
	}
}

func ParseChannel(raw string) domain.NotificationChannel {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(domain.NotificationChannelEmail):
		return domain.NotificationChannelEmail
	case string(domain.NotificationChannelTelegram):
		return domain.NotificationChannelTelegram
	default:
		return domain.NotificationChannelLog
	}
}
