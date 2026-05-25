package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/domain"
	"gopkg.in/gomail.v2"
)

const emailSendTimeout = 10 * time.Second

type EmailNotifier struct {
	cfg    config.EmailConfig
	dialer *gomail.Dialer
}

func NewEmail(cfg config.EmailConfig) *EmailNotifier {
	dialer := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.Username, cfg.Password)
	return &EmailNotifier{cfg: cfg, dialer: dialer}
}

func (n *EmailNotifier) BookingCancelled(ctx context.Context, booking domain.Booking) error {
	if !n.cfg.Enabled {
		return nil
	}
	if booking.UserEmail == "" {
		return errors.New("booking email is empty")
	}
	if n.cfg.From == "" || n.cfg.Username == "" || n.cfg.Password == "" {
		return errors.New("email notifier is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	message := gomail.NewMessage()
	message.SetHeader("From", n.cfg.From)
	message.SetHeader("To", booking.UserEmail)
	message.SetHeader("Subject", "Бронь отменена")
	message.SetBody("text/plain", buildCancellationText(booking))

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- n.dialer.DialAndSend(message)
	}()

	timer := time.NewTimer(emailSendTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-resultCh:
		if err != nil {
			return fmt.Errorf("send email: %w", err)
		}
		return nil
	case <-timer.C:
		return errors.New("send email timeout")
	}
}
