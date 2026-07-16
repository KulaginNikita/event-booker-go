package http

import "time"

type ErrorResponse struct {
	Error string `json:"error"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type CreateEventRequest struct {
	Title    string    `json:"title" binding:"required"`
	StartsAt time.Time `json:"starts_at" binding:"required"`
	Capacity int       `json:"capacity" binding:"required"`
}

type BookRequest struct {
	UserName     string `json:"user_name" binding:"required"`
	UserEmail    string `json:"user_email" binding:"required"`
	UserTelegram string `json:"user_telegram"`
}

type ConfirmRequest struct {
	BookingID int64 `json:"booking_id" binding:"required"`
}

type EventResponse struct {
	ID                int64             `json:"id"`
	Title             string            `json:"title"`
	StartsAt          time.Time         `json:"starts_at"`
	Capacity          int               `json:"capacity"`
	FreeSeats         int               `json:"free_seats"`
	PendingBookings   int               `json:"pending_bookings"`
	ConfirmedBookings int               `json:"confirmed_bookings"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Bookings          []BookingResponse `json:"bookings,omitempty"`
}

type BookingResponse struct {
	ID            int64     `json:"id"`
	EventID       int64     `json:"event_id"`
	OwnerUsername string    `json:"owner_username"`
	UserName      string    `json:"user_name"`
	UserEmail     string    `json:"user_email"`
	UserTelegram  string    `json:"user_telegram,omitempty"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
