package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type contextKey string

const userContextKey contextKey = "authenticated-user"

func RequireAuth(service authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				respondUnauthorized(w, "missing bearer token")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			user, err := service.Authenticate(r.Context(), token)
			if err != nil {
				respondUnauthorized(w, "unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(userContextKey).(domain.User)
	return user, ok
}

func respondUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
