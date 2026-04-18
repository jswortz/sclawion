// Package connectors defines the interfaces every chat-platform implementation
// must satisfy. Each platform lives in its own subpackage.
package connectors

import (
	"context"
	"net/http"

	"github.com/jswortz/sclawion/pkg/event"
)

// Verifier authenticates an inbound webhook request. Implementations must use
// constant-time comparison and enforce auth.MaxSkew on any timestamp claim.
type Verifier interface {
	Verify(ctx context.Context, r *http.Request, body []byte) error
}

// Decoder converts a verified inbound HTTP request body into a normalized Envelope.
type Decoder interface {
	Decode(ctx context.Context, body []byte) (*event.Envelope, error)
}

// Encoder posts an outbound Envelope back to the originating platform.
// Implementations look up channel/thread metadata via correlation and
// fetch credentials via secrets.
type Encoder interface {
	Encode(ctx context.Context, e *event.Envelope) error
}
