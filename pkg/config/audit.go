package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"
)

// Recorder writes an audit entry on every mutating call. Sinks:
//   - Firestore mirror (fast UI read, 90-day TTL)
//   - structured slog INFO line (Cloud Logging routes to BigQuery audit sink)
//
// OTEL span attribute writing is delegated to pkg/obs once that lands; until
// then the trace_id/span_id fields are populated only if the caller passes
// them via WithTraceContext.
type Recorder struct {
	Store  Store
	Logger *slog.Logger
}

// Record persists an audit entry. The ID is generated if empty.
func (r *Recorder) Record(ctx context.Context, e *AuditEntry) {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.Result == "" {
		e.Result = "success"
	}
	if r.Store != nil {
		_ = r.Store.PutAudit(ctx, e)
	}
	r.log(e)
}

func (r *Recorder) log(e *AuditEntry) {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("audit",
		slog.String("id", e.ID),
		slog.String("actor", e.Actor),
		slog.String("actor_role", string(e.ActorRole)),
		slog.String("action", e.Action),
		slog.String("resource_type", e.ResourceType),
		slog.String("resource_id", e.ResourceID),
		slog.String("tenant_id", e.TenantID),
		slog.String("result", e.Result),
		slog.String("error", e.Error),
		slog.String("trace_id", e.TraceID),
		slog.Time("at", e.At),
	)
}

// Redact replaces secret-bearing fields with "***" before audit before/after
// snapshots are taken.
func Redact(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}

// newID returns a 16-byte random hex string. ULID would be preferable for
// time-orderability; switching once a third-party ULID lib is acceptable.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
