package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jswortz/sclawion/pkg/config"
)

// TestUIRenders walks every page template against an empty store. The template
// set wires header/footer partials in _layout.html with each page template's
// body, so a missing partial reference would 500 here even though it would
// pass package-level parsing.
func TestUIRenders(t *testing.T) {
	store := config.NewMemStore()
	if err := store.PutAdminUser(context.Background(), &config.AdminUser{
		Email: "owner@dev.local", Role: config.RoleOwner,
	}); err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	deps := Deps{Store: store, Env: "dev"}
	ui := UI(deps)

	authMW := config.IAPAuth(config.IAPAuthConfig{
		Store: store, Env: "dev", BypassEmail: "owner@dev.local",
	})
	mux := http.NewServeMux()
	mux.Handle("/ui/", authMW(ui))

	cases := []string{
		"/ui/",
		"/ui/tenants",
		"/ui/tenants/acme",
		"/ui/tenants/acme/connectors",
		"/ui/tenants/acme/agents",
		"/ui/tenants/acme/swarms",
		"/ui/admin-users",
		"/ui/audit",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: got %d, body=%s", path, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "</html>") {
				t.Fatalf("GET %s: response missing html shell\n%s", path, body)
			}
		})
	}
}
