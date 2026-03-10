package main

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a type for request context keys to avoid collisions
type contextKey string

const userIDKey contextKey = "userID"

// setUserID stores user ID in context
func setUserID(ctx context.Context, id int) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// GetUserID retrieves user ID from request context (call after RequireAuth)
func GetUserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

// RequireAuth wraps a handler and ensures the request has a valid JWT in the Authorization header.
// Format: "Bearer <token>"
// On success, the user_id from the token is stored in the request context for the next handler.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeJSONError(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeJSONError(w, "invalid authorization format, use: Bearer <token>", http.StatusUnauthorized)
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			writeJSONError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Store user ID in context for downstream handlers
		ctx := r.Context()
		ctx = setUserID(ctx, claims.UserID)
		r = r.WithContext(ctx)

		next(w, r)
	}
}
