package config

import (
	"context"

	"github.com/jswortz/sclawion/pkg/event"
)

// FirestoreStore is the production Store implementation.
//
// Layout:
//
//	config_tenants/{tid}
//	  connectors/{platform}
//	  agents/{id}
//	  swarms/{id}
//	admin_users/{email}
//	config_audit/{ulid}
//
// Implementation TODO: wrap cloud.google.com/go/firestore. Use single-doc
// writes (no distributed locks; Firestore is strongly consistent). Add a TTL
// policy of 90 days on config_audit (long-term store is BigQuery via the log
// sink).
type FirestoreStore struct {
	// ProjectID, *firestore.Client populated by NewFirestoreStore in real impl.
}

// NewFirestoreStore is the constructor; stub until the SDK wiring lands.
func NewFirestoreStore(ctx context.Context, projectID string) (*FirestoreStore, error) {
	_ = ctx
	_ = projectID
	return &FirestoreStore{}, nil
}

func (s *FirestoreStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) GetTenant(ctx context.Context, tid string) (*Tenant, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) PutTenant(ctx context.Context, t *Tenant) error    { return nil }
func (s *FirestoreStore) DeleteTenant(ctx context.Context, tid string) error { return nil }

func (s *FirestoreStore) ListConnectors(ctx context.Context, tid string) ([]ConnectorConfig, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) GetConnector(ctx context.Context, tid string, p event.Platform) (*ConnectorConfig, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) PutConnector(ctx context.Context, c *ConnectorConfig) error { return nil }

func (s *FirestoreStore) ListAgents(ctx context.Context, tid string) ([]AgentProfile, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) GetAgent(ctx context.Context, tid, id string) (*AgentProfile, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) PutAgent(ctx context.Context, a *AgentProfile) error    { return nil }
func (s *FirestoreStore) DeleteAgent(ctx context.Context, tid, id string) error { return nil }

func (s *FirestoreStore) ListSwarms(ctx context.Context, tid string) ([]SwarmDef, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) GetSwarm(ctx context.Context, tid, id string) (*SwarmDef, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) PutSwarm(ctx context.Context, sw *SwarmDef) error      { return nil }
func (s *FirestoreStore) DeleteSwarm(ctx context.Context, tid, id string) error { return nil }

func (s *FirestoreStore) ListAdminUsers(ctx context.Context) ([]AdminUser, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) GetAdminUser(ctx context.Context, email string) (*AdminUser, error) {
	return nil, ErrNotFound
}
func (s *FirestoreStore) PutAdminUser(ctx context.Context, u *AdminUser) error  { return nil }
func (s *FirestoreStore) DeleteAdminUser(ctx context.Context, email string) error { return nil }

func (s *FirestoreStore) PutAudit(ctx context.Context, e *AuditEntry) error { return nil }
func (s *FirestoreStore) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	return nil, nil
}
