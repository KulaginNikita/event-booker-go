package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

type contextKey string

const (
	userContextKey contextKey = "user"
	roleContextKey contextKey = "role"
)

func (h *Handler) RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(header, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))

			claims, err := h.auth.Parse(token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				writeJSON(w, http.StatusForbidden, ErrorResponse{Error: domain.ErrForbidden.Error()})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims.Subject)
			ctx = context.WithValue(ctx, roleContextKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
