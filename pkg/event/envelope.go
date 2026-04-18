// Package event defines the normalized envelope that flows through Pub/Sub.
//
// Every connector (inbound and outbound) speaks this shape. Platform-specific
// types live inside pkg/connectors/<platform>/ and never cross package boundaries.
package event

import (
	"encoding/json"
	"time"
)

// SpecVersion is the schema identifier on every event. Bump on breaking change.
const SpecVersion = "sclawion/v1"

// Kind is the event taxonomy. Keep small and stable.
type Kind string

const (
	KindUserMessage    Kind = "user.message"     // inbound: a human said something
	KindAgentReply     Kind = "agent.reply"      // outbound: agent produced text
	KindAgentStarted   Kind = "agent.started"    // outbound: lifecycle
	KindAgentCompleted Kind = "agent.completed"  // outbound: lifecycle
	KindAgentFailed    Kind = "agent.failed"     // outbound: lifecycle
)

// Platform identifies the chat surface an event came from or is destined for.
type Platform string

const (
	PlatformSlack    Platform = "slack"
	PlatformDiscord  Platform = "discord"
	PlatformGChat    Platform = "gchat"
	PlatformWhatsApp Platform = "whatsapp"
	PlatformIMessage Platform = "imessage" // via a webhook bridge (Sendblue, BlueBubbles, Loop)
)

// Envelope is the wire format on Pub/Sub. JSON-encoded as the message data;
// platform/kind also duplicated as Pub/Sub message attributes for filtering.
type Envelope struct {
	SpecVersion    string          `json:"spec_version"`
	ID             string          `json:"id"`              // ULID; idempotency key
	ConversationID string          `json:"conversation_id"` // platform:channel:thread
	Platform       Platform        `json:"platform"`
	Kind           Kind            `json:"kind"`
	OccurredAt     time.Time       `json:"occurred_at"`
	ScionAgentID   string          `json:"scion_agent_id,omitempty"`
	Payload        json.RawMessage `json:"payload"` // platform- or kind-specific body
	Trace          TraceContext    `json:"trace"`
}

// TraceContext carries OTEL trace propagation across the bus.
type TraceContext struct {
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
}

// Attributes returns the Pub/Sub message attributes. Use for subscription
// filtering (e.g., per-emitter subs filter on platform).
func (e *Envelope) Attributes() map[string]string {
	return map[string]string{
		"platform":     string(e.Platform),
		"kind":         string(e.Kind),
		"spec_version": e.SpecVersion,
	}
}
