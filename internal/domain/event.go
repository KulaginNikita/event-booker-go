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

type ListEventsFilter struct {
	Limit  uint64
	Offset uint64
	Sort   string
}

type Booking struct {
	ID            int64
	EventID       int64
	EventTitle    string
	OwnerUsername string
	UserName      string
	UserEmail     string
	UserTelegram  string
	Status        BookingStatus
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type NotificationEventType string

const (
	NotificationBookingCreated   NotificationEventType = "booking_created"
	NotificationBookingConfirmed NotificationEventType = "booking_confirmed"
	NotificationBookingCancelled NotificationEventType = "booking_cancelled"
)

type NotificationEvent struct {
	ID          int64
	Type        NotificationEventType
	Booking     Booking
	Attempts    int
	MaxAttempts int
}
