package notifier

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type BookingNotifier interface {
	BookingCreated(ctx context.Context, booking domain.Booking) error
	BookingConfirmed(ctx context.Context, booking domain.Booking) error
	BookingCancelled(ctx context.Context, booking domain.Booking) error
}

type MultiNotifier struct {
	items []BookingNotifier
	log   *zap.SugaredLogger
}

func NewMulti(log *zap.SugaredLogger, items ...BookingNotifier) *MultiNotifier {
	return &MultiNotifier{items: items, log: log}
}

func (n *MultiNotifier) BookingCreated(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, "created", func(item BookingNotifier) error {
		return item.BookingCreated(ctx, booking)
	})
}

func (n *MultiNotifier) BookingConfirmed(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, "confirmed", func(item BookingNotifier) error {
		return item.BookingConfirmed(ctx, booking)
	})
}

func (n *MultiNotifier) BookingCancelled(ctx context.Context, booking domain.Booking) error {
	return n.send(ctx, booking, "cancelled", func(item BookingNotifier) error {
		return item.BookingCancelled(ctx, booking)
	})
}

func (n *MultiNotifier) send(ctx context.Context, booking domain.Booking, event string, sendFn func(item BookingNotifier) error) error {
	var result error
	for _, item := range n.items {
		if item == nil {
			continue
		}
		if err := sendFn(item); err != nil {
			n.log.Warnw("booking notifier failed", "booking_id", booking.ID, "event", event, "error", err)
			result = errors.Join(result, err)
		}
	}
	if result != nil {
		return fmt.Errorf("send %s notifications: %w", event, result)
	}
	return nil
}
