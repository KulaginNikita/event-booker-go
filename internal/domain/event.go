package domain

import "time"

type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
)

type Event struct {
	ID                int64
	Title             string
	StartsAt          time.Time
	Capacity          int
	FreeSeats         int
	PendingBookings   int
	ConfirmedBookings int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Bookings          []Booking
}

type Booking struct {
	ID           int64
	EventID      int64
	UserName     string
	UserEmail    string
	UserTelegram string
	Status       BookingStatus
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
