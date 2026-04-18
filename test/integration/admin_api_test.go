// Integration smoke for the admin-api config plane.
//
// This test wires the real handler tree against an in-memory Store and a stub
// Secret Manager writer, then drives the full HTTP surface end-to-end through
// httptest. The IAP middleware runs in dev-bypass mode (Env=dev, BypassEmail
// set per request via context swap) so we can assert RBAC behavior across
// owner/operator callers without building real IAP JWTs.
//
// When the Firestore + Secret Manager SDK wiring lands, swap NewMemStore for a
// FirestoreStore pointed at the emulator and replace stubWriter with the real
// secrets.Writer impl. The test cases stay the same.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jswortz/sclawion/cmd/admin-api/handlers"
	"github.com/jswortz/sclawion/pkg/config"
	"github.com/jswortz/sclawion/pkg/secrets"
)

// stubWriter satisfies secrets.Writer. It records each AddVersion call and
// hands back monotonically increasing version numbers so the rotate handler
// has something to write back into the connector doc.
type stubWriter struct {
	mu       sync.Mutex
	versions map[string]int
	calls    []stubCall
}

type stubCall struct {
	Name  string
	Value []byte
}

func newStubWriter() *stubWriter {
	return &stubWriter{versions: map[string]int{}}
}

func (s *stubWriter) AddVersion(_ context.Context, name string, value []byte) (secrets.SecretRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versions[name]++
	s.calls = append(s.calls, stubCall{Name: name, Value: append([]byte(nil), value...)})
	return secrets.SecretRef{Name: name, Version: itoa(s.versions[name])}, nil
}

func itoa(i int) string {
	// Avoid pulling in strconv; this is small.
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// fixture holds the full server stack plus pointers to the underlying store
// and stub writer so test bodies can assert side effects directly.
type fixture struct {
	t        *testing.T
	store    *config.MemStore
	writer   *stubWriter
	api      http.Handler
	caller   string // mutated per-request via withCaller
	callerMu sync.Mutex
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := config.NewMemStore()
	if err := store.PutAdminUser(context.Background(), &config.AdminUser{
		Email: "owner@dev.local", Role: config.RoleOwner, AddedBy: "test",
	}); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := store.PutAdminUser(context.Background(), &config.AdminUser{
		Email: "operator@dev.local", Role: config.RoleOperator, AddedBy: "test",
	}); err != nil {
		t.Fatalf("seed operator: %v", err)
	}

	writer := newStubWriter()
	deps := handlers.Deps{
		Store:     store,
		Recorder:  &config.Recorder{Store: store},
		Rotator:   &config.SecretRotator{ProjectID: "test-project", Writer: writer, Store: store},
		ProjectID: "test-project",
		Env:       "dev",
	}

	f := &fixture{t: t, store: store, writer: writer, caller: "owner@dev.local"}

	// IAPAuth in dev mode honors BypassEmail; we swap it per request via
	// the IAPAuthConfig.BypassEmail closure on each call.
	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.callerMu.Lock()
			email := f.caller
			f.callerMu.Unlock()
			cfg := config.IAPAuthConfig{
				Store: store, Env: "dev", BypassEmail: email,
			}
			config.IAPAuth(cfg)(next).ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", authMW(handlers.New(deps)))
	f.api = mux
	return f
}

func (f *fixture) withCaller(email string) {
	f.callerMu.Lock()
	defer f.callerMu.Unlock()
	f.caller = email
}

func (f *fixture) do(method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatalf("marshal request: %v", err)
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) decode(rec *httptest.ResponseRecorder, into any) {
	f.t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		f.t.Fatalf("decode response %s: %v\nbody=%s", rec.Result().Status, err, rec.Body.String())
	}
}

func TestAdminAPI_Smoke(t *testing.T) {
	f := newFixture(t)

	// 1. POST /v1/tenants creates a tenant doc + audit entry.
	rec := f.do(http.MethodPost, "/v1/tenants", map[string]any{
		"id": "acme", "display_name": "Acme Corp",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant: got %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := f.store.GetTenant(context.Background(), "acme"); err != nil {
		t.Fatalf("tenant not persisted: %v", err)
	}

	// 2. PUT a connector with non-secret fields.
	rec = f.do(http.MethodPut, "/v1/tenants/acme/connectors/slack", map[string]any{
		"webhook_path":          "/v1/slack",
		"allowed_channels":      []string{"C123"},
		"rate_limit_per_conv":   60,
		"replay_cache_enabled":  true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put connector: got %d, body=%s", rec.Code, rec.Body.String())
	}

	// 3. Rotate the signing secret. Version should be "1" after the first
	//    call and the value must NOT appear in any audit body.
	const secretVal = "topsecret-do-not-leak"
	rec = f.do(http.MethodPost, "/v1/tenants/acme/connectors/slack/secrets:rotate", map[string]any{
		"kind": "signing_secret", "value": secretVal, "reason": "smoke",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate: got %d, body=%s", rec.Code, rec.Body.String())
	}
	conn, err := f.store.GetConnector(context.Background(), "acme", "slack")
	if err != nil {
		t.Fatalf("get connector after rotate: %v", err)
	}
	if conn.SigningSecretRef.Version != "1" {
		t.Fatalf("connector signing version = %q, want %q", conn.SigningSecretRef.Version, "1")
	}
	audit, err := f.store.ListAudit(context.Background(), config.AuditFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, e := range audit {
		if bytes.Contains(e.After, []byte(secretVal)) || bytes.Contains(e.Before, []byte(secretVal)) {
			t.Fatalf("audit entry %s leaked secret value: action=%s body=%s", e.ID, e.Action, string(e.After))
		}
	}

	// 4. Round-trip an agent profile.
	rec = f.do(http.MethodPost, "/v1/tenants/acme/agents", map[string]any{
		"id": "planner-1", "name": "Planner", "model": "claude-opus-4-7",
		"memory_scope": "conversation", "budget_usd_per_hour": 1.5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create agent: got %d, body=%s", rec.Code, rec.Body.String())
	}
	rec = f.do(http.MethodGet, "/v1/tenants/acme/agents/planner-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get agent: got %d", rec.Code)
	}
	var got config.AgentProfile
	f.decode(rec, &got)
	if got.Model != "claude-opus-4-7" || got.MemoryScope != "conversation" {
		t.Fatalf("agent round-trip mismatch: %+v", got)
	}

	// 5. Swarm validation — valid topology accepts, invalid rejects.
	rec = f.do(http.MethodPost, "/v1/tenants/acme/swarms", map[string]any{
		"id": "build-train", "topology": "pipeline",
		"roster": []string{"planner", "coder", "reviewer", "deployer"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create valid swarm: got %d, body=%s", rec.Code, rec.Body.String())
	}
	rec = f.do(http.MethodPost, "/v1/tenants/acme/swarms", map[string]any{
		"id": "bogus", "topology": "invalid-topology",
		"roster": []string{"planner"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid swarm topology should 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 6. RBAC — operator can read connectors but not write admin users.
	f.withCaller("operator@dev.local")
	rec = f.do(http.MethodPut, "/v1/admin-users/newbie@dev.local", map[string]any{
		"role": "viewer",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("operator writing admin_users should 403, got %d", rec.Code)
	}
	rec = f.do(http.MethodPut, "/v1/tenants/acme/connectors/slack", map[string]any{
		"webhook_path": "/v1/slack", "rate_limit_per_conv": 60, "replay_cache_enabled": true,
	})
	if rec.Code != http.StatusForbidden {
		// PUT connector is owner-only per ownerOnly() in handlers.go.
		t.Fatalf("operator writing connector should 403, got %d", rec.Code)
	}
	f.withCaller("owner@dev.local")

	// 7. Soft-delete tenant. DeleteTenant on MemStore (and the Firestore impl
	//    once it lands) flips Disabled=true rather than removing the doc, so
	//    audit history and inbound traffic still resolve to a known tenant.
	rec = f.do(http.MethodDelete, "/v1/tenants/acme", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete tenant: got %d", rec.Code)
	}
	tn, err := f.store.GetTenant(context.Background(), "acme")
	if err != nil {
		t.Fatalf("tenant should still exist after soft delete: %v", err)
	}
	if !tn.Disabled {
		t.Fatalf("tenant should be Disabled=true after delete, got %+v", tn)
	}
}

// TestAdminAPI_UnknownCallerRejected asserts that a caller who passes IAP but
// is not in admin_users gets a 403, not a 200 — closing a gap where a freshly
// signed-in employee with no role assignment could read tenant data.
func TestAdminAPI_UnknownCallerRejected(t *testing.T) {
	f := newFixture(t)
	f.withCaller("nobody@dev.local")
	rec := f.do(http.MethodGet, "/v1/tenants", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unknown caller should 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAdminAPI_AuditOrdering — the audit handler returns entries newest-first.
// We rely on ULID sort order from Recorder.Record.
func TestAdminAPI_AuditOrdering(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"alpha", "bravo", "charlie"} {
		rec := f.do(http.MethodPost, "/v1/tenants", map[string]any{"id": id, "display_name": id + " corp"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed tenant %s: %d", id, rec.Code)
		}
	}
	rec := f.do(http.MethodGet, "/v1/audit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list audit: %d", rec.Code)
	}
	var entries []config.AuditEntry
	f.decode(rec, &entries)
	if len(entries) < 3 {
		t.Fatalf("want >=3 audit entries, got %d", len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].At.After(entries[i-1].At) {
			t.Fatalf("audit not in newest-first order at %d: %v vs %v",
				i, entries[i-1].At, entries[i].At)
		}
	}
}
