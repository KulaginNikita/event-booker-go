package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func TestAuthServiceLoginAndParse(t *testing.T) {
	svc := NewAuthService("secret", time.Hour, "event-booker", "event-booker-api", testUserStore{
		"admin": {Username: "admin", PasswordHash: "$2a$10$7pEsC5lYWcqJUe26x3v/sO47ZvgFnSpH/RQHW6frVW0MiwT1ZUxGO", Role: RoleAdmin},
	})

	token, err := svc.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	claims, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if claims.Subject != "admin" || claims.Role != RoleAdmin {
		t.Fatalf("unexpected claims: subject=%s role=%s", claims.Subject, claims.Role)
	}
}

func TestAuthServiceRejectsInvalidPassword(t *testing.T) {
	svc := NewAuthService("secret", time.Hour, "event-booker", "event-booker-api", testUserStore{
		"admin": {Username: "admin", PasswordHash: "$2a$10$7pEsC5lYWcqJUe26x3v/sO47ZvgFnSpH/RQHW6frVW0MiwT1ZUxGO", Role: RoleAdmin},
	})

	_, err := svc.Login("admin", "wrong")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestAuthServiceRejectsUnknownUser(t *testing.T) {
	svc := NewAuthService("secret", time.Hour, "event-booker", "event-booker-api", testUserStore{})

	_, err := svc.Login("guest", "password")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

type testUserStore map[string]domain.User

func (s testUserStore) GetUserByUsername(_ context.Context, username string) (*domain.User, error) {
	user, ok := s[username]
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return &user, nil
}
