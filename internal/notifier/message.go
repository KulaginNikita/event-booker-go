package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func buildCreatedText(booking domain.Booking) string {
	return fmt.Sprintf(
		"Ваша бронь #%d на мероприятие %q создана. Подтвердите бронь до %s.",
		booking.ID,
		eventName(booking),
		formatTime(booking.ExpiresAt),
	)
}

func buildConfirmedText(booking domain.Booking) string {
	return fmt.Sprintf(
		"Ваша бронь #%d на мероприятие %q подтверждена. Ждем вас!",
		booking.ID,
		eventName(booking),
	)
}

func buildCancellationText(booking domain.Booking) string {
	return fmt.Sprintf(
		"Ваша бронь #%d на мероприятие %q отменена, потому что не была подтверждена до дедлайна.",
		booking.ID,
		eventName(booking),
	)
}

func eventName(booking domain.Booking) string {
	if title := strings.TrimSpace(booking.EventTitle); title != "" {
		return title
	}
	return fmt.Sprintf("#%d", booking.EventID)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "дедлайна"
	}
	return t.Local().Format("02.01.2006 15:04")
}
