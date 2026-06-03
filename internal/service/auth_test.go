package service

import (
	"errors"
	"testing"
	"time"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func TestAuthServiceLoginAndParse(t *testing.T) {
	svc := NewAuthService("secret", time.Hour, "admin:password:admin,user:password:user")

	token, err := svc.Login("admin", "password")
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
	svc := NewAuthService("secret", time.Hour, "admin:password:admin")

	_, err := svc.Login("admin", "wrong")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestAuthServiceRejectsUnknownUser(t *testing.T) {
	svc := NewAuthService("secret", time.Hour, "admin:password:admin")

	_, err := svc.Login("guest", "password")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
