package server

import (
	"context"
	"ontree-node/internal/database"
)

type contextKey string

const userContextKey contextKey = "user"

// setUserContext adds user to context
func setUserContext(ctx context.Context, user *database.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// getUserFromContext retrieves user from context
func getUserFromContext(ctx context.Context) *database.User {
	user, _ := ctx.Value(userContextKey).(*database.User)
	return user
}