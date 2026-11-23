package authbootstrap

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrUnauthenticated = errors.New("access session is not valid")

// Identity contains only the live authorization fields needed by request
// middleware. It intentionally excludes email, password hashes, and token data.
type Identity struct {
	UserID   uuid.UUID
	Role     string
	SchoolID *uuid.UUID
}

// Store validates one access-token session and returns the user's current
// authorization identity. Implementations must fail closed on invalid sessions.
type Store interface {
	AuthenticateAccessSession(ctx context.Context, sessionID, userID uuid.UUID) (Identity, error)
}
