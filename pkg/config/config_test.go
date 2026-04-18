package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jswortz/sclawion/pkg/event"
	"github.com/jswortz/sclawion/pkg/secrets"
)

func TestValidateTenant(t *testing.T) {
	cases := []struct {
		name string
		in   Tenant
		ok   bool
	}{
		{"ok", Tenant{ID: "acme", DisplayName: "Acme Co"}, true},
		{"bad id uppercase", Tenant{ID: "Acme", DisplayName: "Acme"}, false},
		{"bad id leading hyphen", Tenant{ID: "-acme", DisplayName: "Acme"}, false},
		{"missing display name", Tenant{ID: "acme"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateTenant(&c.in)
			if c.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestValidateConnector(t *testing.T) {
	good := ConnectorConfig{
		TenantID:         "acme",
		Platform:         event.PlatformSlack,
		WebhookPath:      "/v1/slack",
		RateLimitPerConv: 60,
	}
	if err := ValidateConnector(&good); err != nil {
		t.Fatalf("good config rejected: %v", err)
	}
	bad := good
	bad.WebhookPath = "/v1/discord"
	if err := ValidateConnector(&bad); err == nil {
		t.Fatal("mismatched webhook_path should fail")
	}
	bad = good
	bad.RateLimitPerConv = 9999
	if err := ValidateConnector(&bad); err == nil {
		t.Fatal("over-limit rate should fail")
	}
	bad = good
	bad.Platform = "myspace"
	if err := ValidateConnector(&bad); err == nil {
		t.Fatal("unknown platform should fail")
	}
}

func TestValidateSwarm(t *testing.T) {
	good := SwarmDef{
		TenantID: "acme",
		ID:       "release-train",
		Topology: "pipeline",
		Roster:   []string{"planner", "coder", "reviewer", "deployer"},
	}
	if err := ValidateSwarm(&good); err != nil {
		t.Fatalf("good swarm rejected: %v", err)
	}
	bad := good
	bad.Topology = "blob"
	if err := ValidateSwarm(&bad); err == nil {
		t.Fatal("unknown topology should fail")
	}
	bad = good
	bad.Roster = []string{"planner", "wizard"}
	if err := ValidateSwarm(&bad); err == nil {
		t.Fatal("unknown role should fail")
	}
	bad = good
	bad.Roster = nil
	if err := ValidateSwarm(&bad); err == nil {
		t.Fatal("empty roster should fail")
	}
}

func TestValidateAgent(t *testing.T) {
	good := AgentProfile{
		TenantID:    "acme",
		ID:          "release-coder",
		Name:        "Release Coder",
		Model:       "claude-opus-4-7",
		MemoryScope: "conversation",
	}
	if err := ValidateAgent(&good); err != nil {
		t.Fatalf("good agent rejected: %v", err)
	}
	bad := good
	bad.MemoryScope = "global"
	if err := ValidateAgent(&bad); err == nil {
		t.Fatal("unknown memory_scope should fail")
	}
	bad = good
	bad.BudgetUSDPerHour = -1
	if err := ValidateAgent(&bad); err == nil {
		t.Fatal("negative budget should fail")
	}
}

func TestSecretNamer(t *testing.T) {
	got := SecretNamer("my-proj", "acme", event.PlatformSlack, SecretKindSigning)
	want := "projects/my-proj/secrets/sclawion-acme-slack-signing"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRBACDevBypass(t *testing.T) {
	store := NewMemStore()
	if err := store.PutAdminUser(context.Background(), &AdminUser{Email: "owner@dev.local", Role: RoleOwner}); err != nil {
		t.Fatal(err)
	}
	mw := IAPAuth(IAPAuthConfig{
		Store:       store,
		Env:         "dev",
		BypassEmail: "owner@dev.local",
	})
	var caller Caller
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := CallerFromContext(r.Context())
		if !ok {
			t.Fatal("no caller in context")
		}
		caller = c
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if caller.Email != "owner@dev.local" || caller.Role != RoleOwner {
		t.Fatalf("unexpected caller %+v", caller)
	}
}

func TestRBACRoleGate(t *testing.T) {
	store := NewMemStore()
	_ = store.PutAdminUser(context.Background(), &AdminUser{Email: "viewer@dev.local", Role: RoleViewer})
	mw := IAPAuth(IAPAuthConfig{Store: store, Env: "dev", BypassEmail: "viewer@dev.local"})
	gate := RequireRole(RoleOwner)
	h := mw(gate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for viewer when owner required")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/tenants", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// stubWriter satisfies secrets.Writer for testing the rotator.
type stubWriter struct {
	addCalls []string
	version  string
}

func (s *stubWriter) AddVersion(ctx context.Context, name string, value []byte) (secrets.SecretRef, error) {
	_ = ctx
	_ = value
	s.addCalls = append(s.addCalls, name)
	return secrets.SecretRef{Name: name, Version: s.version}, nil
}

func TestSecretRotator(t *testing.T) {
	store := NewMemStore()
	ctx := context.Background()
	_ = store.PutConnector(ctx, &ConnectorConfig{TenantID: "acme", Platform: event.PlatformSlack})
	w := &stubWriter{version: "7"}
	rot := SecretRotator{ProjectID: "p1", Writer: w, Store: store}
	ref, err := rot.Rotate(ctx, "acme", event.PlatformSlack, SecretKindSigning, []byte("v"))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Version != "7" {
		t.Fatalf("got version %q", ref.Version)
	}
	c, err := store.GetConnector(ctx, "acme", event.PlatformSlack)
	if err != nil {
		t.Fatal(err)
	}
	if c.SigningSecretRef.Version != "7" {
		t.Fatalf("connector not updated: %+v", c.SigningSecretRef)
	}
	if len(w.addCalls) != 1 || w.addCalls[0] != "projects/p1/secrets/sclawion-acme-slack-signing" {
		t.Fatalf("unexpected secret name: %v", w.addCalls)
	}
	// Missing connector path: rotator wraps ErrNotFound in a fmt.Errorf.
	if _, err := rot.Rotate(ctx, "ghost", event.PlatformSlack, SecretKindSigning, []byte("v")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}
