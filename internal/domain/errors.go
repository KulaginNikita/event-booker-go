package domain

import "errors"

var (
	ErrEventNotFound        = errors.New("event not found")
	ErrBookingNotFound      = errors.New("booking not found")
	ErrNoSeats              = errors.New("no free seats")
	ErrAlreadyBooked        = errors.New("user already has active booking for this event")
	ErrEventAlreadyStarted  = errors.New("event already started")
	ErrBookingExpired       = errors.New("booking expired")
	ErrBookingNotPending    = errors.New("booking is not pending")
	ErrInvalidTitle         = errors.New("invalid event title")
	ErrInvalidDate          = errors.New("invalid event date")
	ErrInvalidCapacity      = errors.New("invalid event capacity")
	ErrInvalidUser          = errors.New("invalid user data")
	ErrInvalidPagination    = errors.New("invalid pagination")
	ErrInvalidState         = errors.New("invalid state")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrTelegramChatNotFound = errors.New("telegram chat not found")
)
