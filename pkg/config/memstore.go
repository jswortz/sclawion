package config

import (
	"context"
	"sort"
	"sync"

	"github.com/jswortz/sclawion/pkg/event"
)

// MemStore is an in-memory Store for tests and the local dev loop. Safe for
// concurrent use. Production callers should use FirestoreStore.
type MemStore struct {
	mu       sync.RWMutex
	tenants  map[string]Tenant
	conns    map[string]map[event.Platform]ConnectorConfig
	agents   map[string]map[string]AgentProfile
	swarms   map[string]map[string]SwarmDef
	admins   map[string]AdminUser
	audit    []AuditEntry
}

// NewMemStore returns a fresh in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{
		tenants: map[string]Tenant{},
		conns:   map[string]map[event.Platform]ConnectorConfig{},
		agents:  map[string]map[string]AgentProfile{},
		swarms:  map[string]map[string]SwarmDef{},
		admins:  map[string]AdminUser{},
	}
}

func (s *MemStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemStore) GetTenant(ctx context.Context, tid string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[tid]
	if !ok {
		return nil, ErrNotFound
	}
	return &t, nil
}

func (s *MemStore) PutTenant(ctx context.Context, t *Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = *t
	return nil
}

func (s *MemStore) DeleteTenant(ctx context.Context, tid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenants[tid]
	if !ok {
		return ErrNotFound
	}
	t.Disabled = true
	s.tenants[tid] = t
	return nil
}

func (s *MemStore) ListConnectors(ctx context.Context, tid string) ([]ConnectorConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.conns[tid]
	out := make([]ConnectorConfig, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Platform < out[j].Platform })
	return out, nil
}

func (s *MemStore) GetConnector(ctx context.Context, tid string, p event.Platform) (*ConnectorConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.conns[tid]
	if !ok {
		return nil, ErrNotFound
	}
	c, ok := m[p]
	if !ok {
		return nil, ErrNotFound
	}
	return &c, nil
}

func (s *MemStore) PutConnector(ctx context.Context, c *ConnectorConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conns[c.TenantID]; !ok {
		s.conns[c.TenantID] = map[event.Platform]ConnectorConfig{}
	}
	s.conns[c.TenantID][c.Platform] = *c
	return nil
}

func (s *MemStore) ListAgents(ctx context.Context, tid string) ([]AgentProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.agents[tid]
	out := make([]AgentProfile, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemStore) GetAgent(ctx context.Context, tid, id string) (*AgentProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.agents[tid]
	if !ok {
		return nil, ErrNotFound
	}
	a, ok := m[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &a, nil
}

func (s *MemStore) PutAgent(ctx context.Context, a *AgentProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[a.TenantID]; !ok {
		s.agents[a.TenantID] = map[string]AgentProfile{}
	}
	s.agents[a.TenantID][a.ID] = *a
	return nil
}

func (s *MemStore) DeleteAgent(ctx context.Context, tid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.agents[tid]
	if !ok {
		return ErrNotFound
	}
	if _, ok := m[id]; !ok {
		return ErrNotFound
	}
	delete(m, id)
	return nil
}

func (s *MemStore) ListSwarms(ctx context.Context, tid string) ([]SwarmDef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.swarms[tid]
	out := make([]SwarmDef, 0, len(m))
	for _, sw := range m {
		out = append(out, sw)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemStore) GetSwarm(ctx context.Context, tid, id string) (*SwarmDef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.swarms[tid]
	if !ok {
		return nil, ErrNotFound
	}
	sw, ok := m[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &sw, nil
}

func (s *MemStore) PutSwarm(ctx context.Context, sw *SwarmDef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.swarms[sw.TenantID]; !ok {
		s.swarms[sw.TenantID] = map[string]SwarmDef{}
	}
	s.swarms[sw.TenantID][sw.ID] = *sw
	return nil
}

func (s *MemStore) DeleteSwarm(ctx context.Context, tid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.swarms[tid]
	if !ok {
		return ErrNotFound
	}
	if _, ok := m[id]; !ok {
		return ErrNotFound
	}
	delete(m, id)
	return nil
}

func (s *MemStore) ListAdminUsers(ctx context.Context) ([]AdminUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AdminUser, 0, len(s.admins))
	for _, u := range s.admins {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out, nil
}

func (s *MemStore) GetAdminUser(ctx context.Context, email string) (*AdminUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.admins[email]
	if !ok {
		return nil, ErrNotFound
	}
	return &u, nil
}

func (s *MemStore) PutAdminUser(ctx context.Context, u *AdminUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admins[u.Email] = *u
	return nil
}

func (s *MemStore) DeleteAdminUser(ctx context.Context, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.admins[email]; !ok {
		return ErrNotFound
	}
	delete(s.admins, email)
	return nil
}

func (s *MemStore) PutAudit(ctx context.Context, e *AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, *e)
	return nil
}

func (s *MemStore) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditEntry, 0, len(s.audit))
	for _, e := range s.audit {
		if f.TenantID != "" && e.TenantID != f.TenantID {
			continue
		}
		if f.Actor != "" && e.Actor != f.Actor {
			continue
		}
		if f.ResourceType != "" && e.ResourceType != f.ResourceType {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}
