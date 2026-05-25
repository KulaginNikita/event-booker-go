package notifier

import (
	"context"
	"errors"
	"fmt"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type BookingNotifier interface {
	BookingCancelled(ctx context.Context, booking domain.Booking) error
}

type MultiNotifier struct {
	items []BookingNotifier
	log   logger.Logger
}

func NewMulti(log logger.Logger, items ...BookingNotifier) *MultiNotifier {
	return &MultiNotifier{items: items, log: log}
}

func (n *MultiNotifier) BookingCancelled(ctx context.Context, booking domain.Booking) error {
	var result error
	for _, item := range n.items {
		if item == nil {
			continue
		}
		if err := item.BookingCancelled(ctx, booking); err != nil {
			n.log.Warn("booking notifier failed", "booking_id", booking.ID, "error", err)
			result = errors.Join(result, err)
		}
	}
	if result != nil {
		return fmt.Errorf("send cancellation notifications: %w", result)
	}
	return nil
}
