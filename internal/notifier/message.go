package notifier

import (
	"fmt"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func buildCancellationText(booking domain.Booking) string {
	return fmt.Sprintf(
		"Ваша бронь #%d на мероприятие #%d отменена, потому что не была подтверждена до дедлайна.",
		booking.ID,
		booking.EventID,
	)
}
