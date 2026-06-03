package service

import (
	"errors"
	"testing"
	"time"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func TestAuthServiceLoginAndParse(t *testing.T) {
	svc := NewAuthService("secret", time.Hour)

	token, err := svc.Login("admin", RoleAdmin)
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

func TestAuthServiceRejectsInvalidRole(t *testing.T) {
	svc := NewAuthService("secret", time.Hour)

	_, err := svc.Login("guest", "guest")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
