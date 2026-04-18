// Command admin-api is the operator-facing config plane for sclawion.
//
// It serves both a JSON REST API under /v1 and a templ+htmx UI under /ui from
// the same binary. State is owned in Firestore (config_*) and Secret Manager;
// Terraform stays authoritative for infra (KMS, topics, IAM, Cloud Armor rule
// structure, BigQuery sinks, Binary Auth).
//
// Auth is IAP in front. The IAP JWT (X-Goog-IAP-JWT-Assertion) is validated
// per request; the caller's email is mapped to a role via admin_users/{email}.
// Owner > Operator > Viewer.
//
// admin-api never imports pkg/scion or any connector verifier value.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/jswortz/sclawion/cmd/admin-api/handlers"
	"github.com/jswortz/sclawion/pkg/config"
)

func main() {
	ctx := context.Background()
	env := envOr("ENV", "dev")
	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" && env != "dev" {
		log.Fatal("admin-api: GCP_PROJECT required in non-dev env")
	}

	store, err := newStore(ctx, env, projectID)
	if err != nil {
		log.Fatalf("admin-api: store init: %v", err)
	}
	if err := requireOwnerSeed(ctx, store, env); err != nil {
		log.Fatalf("admin-api: %v", err)
	}

	deps := handlers.Deps{
		Store:     store,
		ProjectID: projectID,
		Env:       env,
		Recorder:  &config.Recorder{Store: store, Logger: slog.Default()},
		Rotator: &config.SecretRotator{
			ProjectID: projectID,
			// Writer is nil here; rotate handler returns 503 until the SDK
			// wiring lands. See pkg/secrets/writer.go for the interface.
			Store: store,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authCfg := config.IAPAuthConfig{
		Store:       store,
		Env:         env,
		BypassEmail: config.DevBypassEmail(),
	}
	if env != "dev" {
		// In non-dev, IAP audience must be set and a verifier wired.
		// Implementation TODO: build an IAPVerifier impl wrapping idtoken.
		authCfg.Audience = os.Getenv("IAP_AUDIENCE")
	}
	authMW := config.IAPAuth(authCfg)

	api := handlers.New(deps)
	mux.Handle("/v1/", authMW(api))
	mux.Handle("/ui/", authMW(handlers.UI(deps)))

	addr := ":" + envOr("PORT", "8080")
	log.Printf("sclawion-admin-api listening on %s (env=%s)", addr, env)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// newStore returns the production FirestoreStore in non-dev and an in-memory
// MemStore in dev. Tests construct MemStore directly.
func newStore(ctx context.Context, env, projectID string) (config.Store, error) {
	if env == "dev" {
		return config.NewMemStore(), nil
	}
	return config.NewFirestoreStore(ctx, projectID)
}

// requireOwnerSeed refuses to start if admin_users is empty in a non-dev env.
// Terraform seeds the first owner from a TF variable; this guard closes the
// boot-loop where a fresh deploy has no one with permission to add users.
func requireOwnerSeed(ctx context.Context, s config.Store, env string) error {
	if env == "dev" {
		// Auto-seed dev owner so local UI works without setup.
		bypass := config.DevBypassEmail()
		if bypass == "" {
			bypass = "owner@dev.local"
		}
		return s.PutAdminUser(ctx, &config.AdminUser{
			Email: bypass, Role: config.RoleOwner, AddedBy: "dev-bootstrap",
		})
	}
	users, err := s.ListAdminUsers(ctx)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return errors.New("admin_users is empty; Terraform must seed at least one Owner")
	}
	return nil
}
