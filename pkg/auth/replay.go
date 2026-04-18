package auth

import (
	"context"
	"errors"
	"time"
)

// ErrReplay indicates the request hash was seen recently.
var ErrReplay = errors.New("auth: replayed request")

// ReplayCache rejects duplicate webhook deliveries within a TTL window.
// Backed by Firestore in production; in-memory map in tests.
type ReplayCache interface {
	// Remember stores the hash with the given TTL. Returns ErrReplay if it
	// was already present and unexpired.
	Remember(ctx context.Context, hash string, ttl time.Duration) error
}
