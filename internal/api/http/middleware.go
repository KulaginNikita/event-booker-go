package http

import (
	"net/http"
	"strings"

	"github.com/wb-go/wbf/ginext"

	"github.com/KulaginNikita/event-booker/internal/domain"
)

func (h *Handler) RequireRole(roles ...string) ginext.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *ginext.Context) {
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
			return
		}

		claims, err := h.auth.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: domain.ErrUnauthorized.Error()})
			return
		}
		if _, ok := allowed[claims.Role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Error: domain.ErrForbidden.Error()})
			return
		}

		c.Set("user", claims.Subject)
		c.Set("role", claims.Role)
		c.Next()
	}
}
