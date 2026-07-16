package http

import "github.com/KulaginNikita/event-booker/internal/domain"

func toEventResponse(event *domain.Event) EventResponse {
	return EventResponse{
		ID:                event.ID,
		Title:             event.Title,
		StartsAt:          event.StartsAt,
		Capacity:          event.Capacity,
		FreeSeats:         event.FreeSeats,
		PendingBookings:   event.PendingBookings,
		ConfirmedBookings: event.ConfirmedBookings,
		CreatedAt:         event.CreatedAt,
		UpdatedAt:         event.UpdatedAt,
		Bookings:          toBookingResponses(event.Bookings),
	}
}

func toEventResponses(events []domain.Event) []EventResponse {
	result := make([]EventResponse, 0, len(events))
	for _, event := range events {
		copy := event
		result = append(result, toEventResponse(&copy))
	}
	return result
}

func toBookingResponse(booking *domain.Booking) BookingResponse {
	return BookingResponse{
		ID:            booking.ID,
		EventID:       booking.EventID,
		OwnerUsername: booking.OwnerUsername,
		UserName:      booking.UserName,
		UserEmail:     booking.UserEmail,
		UserTelegram:  booking.UserTelegram,
		Status:        string(booking.Status),
		ExpiresAt:     booking.ExpiresAt,
		CreatedAt:     booking.CreatedAt,
		UpdatedAt:     booking.UpdatedAt,
	}
}

func toBookingResponses(bookings []domain.Booking) []BookingResponse {
	result := make([]BookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		copy := booking
		result = append(result, toBookingResponse(&copy))
	}
	return result
}
