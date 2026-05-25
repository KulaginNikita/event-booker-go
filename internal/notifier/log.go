package notifier

import (
	"context"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type LogNotifier struct {
	log logger.Logger
}

func NewLogNotifier(log logger.Logger) *LogNotifier {
	return &LogNotifier{log: log}
}

func (n *LogNotifier) BookingCancelled(_ context.Context, booking domain.Booking) error {
	n.log.Info("booking cancelled notification",
		"booking_id", booking.ID,
		"event_id", booking.EventID,
		"user_email", booking.UserEmail,
	)
	return nil
}
