package config

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/jswortz/sclawion/pkg/auth"
)

// ErrUnauthenticated and ErrForbidden are returned by IAPAuth / RequireRole
// to signal HTTP 401 and 403 respectively.
var (
	ErrUnauthenticated = errors.New("rbac: unauthenticated")
	ErrForbidden       = errors.New("rbac: forbidden")
)

type ctxKey int

const callerKey ctxKey = 1

// Caller is the authenticated principal attached to every request after
// IAPAuth runs.
type Caller struct {
	Email string
	Role  Role
}

// CallerFromContext returns the Caller injected by IAPAuth. Handlers should
// use this rather than re-parsing the IAP header.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey).(Caller)
	return c, ok
}

// IAPAuthConfig wires the middleware to its dependencies.
type IAPAuthConfig struct {
	Verifier  auth.IAPVerifier
	Audience  string // /projects/<n>/global/backendServices/<id>
	Store     Store
	Env       string // "dev" enables BypassEmail
	BypassEmail string // ADMIN_API_DEV_IAP_BYPASS_EMAIL; ignored unless Env == "dev"
}

// IAPAuth is HTTP middleware that validates the IAP JWT (or honors the dev
// bypass), looks up the caller's role from the admin_users collection, and
// injects a Caller into the request context.
func IAPAuth(cfg IAPAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, err := resolveEmail(r, cfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			user, err := cfg.Store.GetAdminUser(r.Context(), email)
			if err != nil {
				// Unknown email = forbidden, not unauthenticated. The user
				// passed IAP but isn't in admin_users.
				http.Error(w, "rbac: caller not in admin_users", http.StatusForbidden)
				return
			}
			caller := Caller{Email: email, Role: user.Role}
			ctx := context.WithValue(r.Context(), callerKey, caller)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveEmail(r *http.Request, cfg IAPAuthConfig) (string, error) {
	if cfg.Env == "dev" && cfg.BypassEmail != "" {
		return cfg.BypassEmail, nil
	}
	jwt := r.Header.Get("X-Goog-IAP-JWT-Assertion")
	if jwt == "" {
		return "", ErrUnauthenticated
	}
	if cfg.Verifier == nil {
		return "", errors.New("rbac: no IAP verifier configured")
	}
	claims, err := cfg.Verifier.Verify(r.Context(), jwt, cfg.Audience)
	if err != nil {
		return "", err
	}
	return claims.Email, nil
}

// RequireRole returns a middleware that allows the request only if the caller
// has at least the given role. Owner > Operator > Viewer.
func RequireRole(min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := CallerFromContext(r.Context())
			if !ok {
				http.Error(w, "rbac: no caller in context", http.StatusUnauthorized)
				return
			}
			if !roleAtLeast(c.Role, min) {
				http.Error(w, "rbac: forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func roleAtLeast(have, want Role) bool {
	rank := map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleOwner: 3}
	return rank[have] >= rank[want]
}

// DevBypassEmail reads ADMIN_API_DEV_IAP_BYPASS_EMAIL. Only honored when
// env=dev (asserted by main.go at startup).
func DevBypassEmail() string {
	return os.Getenv("ADMIN_API_DEV_IAP_BYPASS_EMAIL")
}
