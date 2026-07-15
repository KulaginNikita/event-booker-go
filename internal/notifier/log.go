package notifier

import (
	"context"

	"go.uber.org/zap"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type LogNotifier struct {
	log *zap.SugaredLogger
}

func NewLogNotifier(log *zap.SugaredLogger) *LogNotifier {
	return &LogNotifier{log: log}
}

func (n *LogNotifier) BookingCreated(_ context.Context, booking domain.Booking) error {
	n.log.Infow("booking created notification",
		"booking_id", booking.ID,
		"event_id", booking.EventID,
		"user_email", booking.UserEmail,
	)
	return nil
}

func (n *LogNotifier) BookingConfirmed(_ context.Context, booking domain.Booking) error {
	n.log.Infow("booking confirmed notification",
		"booking_id", booking.ID,
		"event_id", booking.EventID,
		"user_email", booking.UserEmail,
	)
	return nil
}

func (n *LogNotifier) BookingCancelled(_ context.Context, booking domain.Booking) error {
	n.log.Infow("booking cancelled notification",
		"booking_id", booking.ID,
		"event_id", booking.EventID,
		"user_email", booking.UserEmail,
	)
	return nil
}
