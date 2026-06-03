package service

import (
	"crypto/subtle"
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
	secret   []byte
	ttl      time.Duration
	accounts map[string]Account
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Account struct {
	Username string
	Password string
	Role     string
}

func NewAuthService(secret string, ttl time.Duration, users string) *AuthService {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &AuthService{
		secret:   []byte(secret),
		ttl:      ttl,
		accounts: parseAccounts(users),
	}
}

func (s *AuthService) Login(username string, password string) (string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	account, ok := s.accounts[username]
	if username == "" || password == "" || !ok || !sameSecret(account.Password, password) {
		return "", domain.ErrUnauthorized
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: account.Role,
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

func parseAccounts(raw string) map[string]Account {
	accounts := make(map[string]Account)
	for _, part := range strings.Split(raw, ",") {
		fields := strings.Split(part, ":")
		if len(fields) != 3 {
			continue
		}
		account := Account{
			Username: strings.TrimSpace(fields[0]),
			Password: strings.TrimSpace(fields[1]),
			Role:     strings.TrimSpace(fields[2]),
		}
		if account.Username == "" || account.Password == "" || !validRole(account.Role) {
			continue
		}
		accounts[account.Username] = account
	}
	return accounts
}

func sameSecret(expected string, actual string) bool {
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}
