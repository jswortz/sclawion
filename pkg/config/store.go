package config

import (
	"context"
	"errors"

	"github.com/jswortz/sclawion/pkg/event"
)

// ErrNotFound is returned when a config doc does not exist.
var ErrNotFound = errors.New("config: not found")

// AuditFilter narrows a config_audit listing.
type AuditFilter struct {
	TenantID     string
	Actor        string
	ResourceType string
	Limit        int
}

// Store is the persistence interface for the admin plane. Implemented by
// firestore.go in production and by an in-memory map in tests.
type Store interface {
	// Tenants ----------------------------------------------------------------
	ListTenants(ctx context.Context) ([]Tenant, error)
	GetTenant(ctx context.Context, tid string) (*Tenant, error)
	PutTenant(ctx context.Context, t *Tenant) error
	DeleteTenant(ctx context.Context, tid string) error // soft

	// Connectors -------------------------------------------------------------
	ListConnectors(ctx context.Context, tid string) ([]ConnectorConfig, error)
	GetConnector(ctx context.Context, tid string, p event.Platform) (*ConnectorConfig, error)
	PutConnector(ctx context.Context, c *ConnectorConfig) error

	// Agent profiles ---------------------------------------------------------
	ListAgents(ctx context.Context, tid string) ([]AgentProfile, error)
	GetAgent(ctx context.Context, tid, id string) (*AgentProfile, error)
	PutAgent(ctx context.Context, a *AgentProfile) error
	DeleteAgent(ctx context.Context, tid, id string) error

	// Swarms -----------------------------------------------------------------
	ListSwarms(ctx context.Context, tid string) ([]SwarmDef, error)
	GetSwarm(ctx context.Context, tid, id string) (*SwarmDef, error)
	PutSwarm(ctx context.Context, s *SwarmDef) error
	DeleteSwarm(ctx context.Context, tid, id string) error

	// Admin users (global, not tenant-scoped) -------------------------------
	ListAdminUsers(ctx context.Context) ([]AdminUser, error)
	GetAdminUser(ctx context.Context, email string) (*AdminUser, error)
	PutAdminUser(ctx context.Context, u *AdminUser) error
	DeleteAdminUser(ctx context.Context, email string) error

	// Audit ------------------------------------------------------------------
	PutAudit(ctx context.Context, e *AuditEntry) error
	ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, error)
}
