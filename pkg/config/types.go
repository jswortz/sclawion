// Package config models the operator-facing configuration surface for sclawion:
// tenants (logical workspaces), per-tenant connector configs, agent profiles,
// swarm definitions, admin users, and an audit log of every change.
//
// All persistence lives in Firestore (config_*) and Secret Manager. The
// admin-api Cloud Run service is the only writer. Data-plane services
// (cmd/router, cmd/emitter, connectors) only read.
package config

import (
	"encoding/json"
	"time"

	"github.com/jswortz/sclawion/pkg/event"
	"github.com/jswortz/sclawion/pkg/secrets"
)

// Tenant is a logical workspace. Lives in Firestore at config_tenants/{ID}.
type Tenant struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Disabled    bool      `json:"disabled"`
}

// ConnectorConfig holds the per-tenant, per-platform configuration. The
// SigningSecretRef and OAuthTokenRef point at Secret Manager versions; the
// actual values are never returned by the admin API.
type ConnectorConfig struct {
	TenantID           string            `json:"tenant_id"`
	Platform           event.Platform    `json:"platform"`
	WebhookPath        string            `json:"webhook_path"` // /v1/{platform}
	AllowedChannels    []string          `json:"allowed_channels,omitempty"`
	SigningSecretRef   secrets.SecretRef `json:"signing_secret_ref"`
	OAuthTokenRef      secrets.SecretRef `json:"oauth_token_ref"`
	RateLimitPerConv   int               `json:"rate_limit_per_conv"`
	ReplayCacheEnabled bool              `json:"replay_cache_enabled"`
	UpdatedAt          time.Time         `json:"updated_at"`
	UpdatedBy          string            `json:"updated_by"`
}

// AgentProfile binds a name to a model + tools + memory scope + budget. The
// router uses these when dispatching to Scion.
type AgentProfile struct {
	TenantID         string    `json:"tenant_id"`
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Model            string    `json:"model"`
	MCPServers       []string  `json:"mcp_servers,omitempty"`
	Skills           []string  `json:"skills,omitempty"`
	MemoryScope      string    `json:"memory_scope"`        // conversation | user
	BudgetUSDPerHour float64   `json:"budget_usd_per_hour"` // 0 = unlimited
	UpdatedAt        time.Time `json:"updated_at"`
	UpdatedBy        string    `json:"updated_by"`
}

// SwarmDef is a multi-agent topology. Roles must match the registry in
// docs/clawpath/SWARMS.md.
type SwarmDef struct {
	TenantID       string         `json:"tenant_id"`
	ID             string         `json:"id"`
	Topology       string         `json:"topology"` // pipeline|fanout|mesh|hierarchical
	Roster         []string       `json:"roster"`
	BudgetEnvelope BudgetEnvelope `json:"budget_envelope"`
	UpdatedAt      time.Time      `json:"updated_at"`
	UpdatedBy      string         `json:"updated_by"`
}

// BudgetEnvelope mirrors the per-swarm budget shape from SWARMS.md.
type BudgetEnvelope struct {
	MaxTokens    int           `json:"max_tokens"`
	MaxWallClock time.Duration `json:"max_wallclock"`
	MaxDeploys   int           `json:"max_deploys"`
}

// Role gates writes. Owner > Operator > Viewer.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// AdminUser is an entry in the global admin_users/{email} collection.
type AdminUser struct {
	Email   string    `json:"email"`
	Role    Role      `json:"role"`
	AddedBy string    `json:"added_by"`
	AddedAt time.Time `json:"added_at"`
}

// AuditEntry is written for every mutating call (success or failure). The
// Firestore mirror at config_audit/{ULID} is for fast UI display; the
// long-term store is BigQuery sclawion_audit_${env}.entries via the log sink.
type AuditEntry struct {
	ID           string          `json:"id"` // ULID
	Actor        string          `json:"actor"`
	ActorRole    Role            `json:"actor_role"`
	Action       string          `json:"action"` // e.g. connector.rotate_secret
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	TenantID     string          `json:"tenant_id,omitempty"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
	At           time.Time       `json:"at"`
	TraceID      string          `json:"trace_id,omitempty"`
	SpanID       string          `json:"span_id,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	Result       string          `json:"result"` // success|failure
	Error        string          `json:"error,omitempty"`
}

// SecretKind identifies which secret slot on a connector is being rotated.
type SecretKind string

const (
	SecretKindSigning SecretKind = "signing_secret"
	SecretKindOAuth   SecretKind = "oauth_token"
)
