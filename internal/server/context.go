package server

import (
	"context"
	"github.com/ontree-co/treeos/internal/database"
)

type contextKey string

const userContextKey contextKey = "user"

// setUserContext adds user to context
func setUserContext(ctx context.Context, user *database.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// getUserFromContext retrieves user from context
func getUserFromContext(ctx context.Context) *database.User {
	value := ctx.Value(userContextKey)
	if value == nil {
		return nil
	}
	user, ok := value.(*database.User)
	if !ok {
		return nil
	}
	return user
}
