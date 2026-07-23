package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type AuthService struct {
	secret   []byte
	ttl      time.Duration
	issuer   string
	audience []string
	store    UserStore
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type UserStore interface {
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
}

func NewAuthService(secret string, ttl time.Duration, issuer string, audience string, store UserStore) *AuthService {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	if issuer == "" {
		issuer = "event-booker"
	}
	if audience == "" {
		audience = "event-booker-api"
	}
	return &AuthService{
		secret:   []byte(secret),
		ttl:      ttl,
		issuer:   issuer,
		audience: []string{audience},
		store:    store,
	}
}

func (s *AuthService) Login(username string, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" || s.store == nil || len(s.secret) == 0 {
		return "", domain.ErrUnauthorized
	}
	user, err := s.store.GetUserByUsername(context.Background(), username)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	if !validRole(user.Role) {
		return "", domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrUnauthorized
	}

	now := time.Now().UTC()
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			Issuer:    s.issuer,
			Audience:  s.audience,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	})

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func (s *AuthService) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthorized
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithAudience(s.audience[0]))
	if err != nil || !token.Valid || !validRole(claims.Role) {
		return nil, domain.ErrUnauthorized
	}
	if claims.ID == "" {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

func validRole(role string) bool {
	return role == RoleAdmin || role == RoleUser
}

func newJTI() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate jwt id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
