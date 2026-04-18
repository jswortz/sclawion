// Package correlation maps a chat conversation (platform+channel+thread) to
// the Scion agent currently servicing it. Backed by Firestore in production.
package correlation

import (
	"context"
	"errors"

	"github.com/jswortz/sclawion/pkg/event"
)

// ErrNotFound is returned when no mapping exists for a conversation.
var ErrNotFound = errors.New("correlation: not found")

type Mapping struct {
	ConversationID string
	Platform       event.Platform
	ChannelID      string
	ThreadID       string
	ScionAgentID   string
}

type Store interface {
	Get(ctx context.Context, conversationID string) (*Mapping, error)
	Put(ctx context.Context, m *Mapping) error
	// MarkProcessed records eventID for idempotency. Returns true if newly
	// inserted, false if already present (i.e., this delivery is a duplicate).
	MarkProcessed(ctx context.Context, eventID string) (firstSeen bool, err error)
}
