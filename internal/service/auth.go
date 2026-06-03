package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type AuthService struct {
	secret []byte
	ttl    time.Duration
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(secret string, ttl time.Duration) *AuthService {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &AuthService{secret: []byte(secret), ttl: ttl}
}

func (s *AuthService) Login(username string, role string) (string, error) {
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	if username == "" || !validRole(role) {
		return "", domain.ErrUnauthorized
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
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
	})
	if err != nil || !token.Valid || !validRole(claims.Role) {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

func validRole(role string) bool {
	return role == RoleAdmin || role == RoleUser
}
