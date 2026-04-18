// Package handlers wires HTTP routes for the admin-api Cloud Run service.
//
// Two router trees share the same dependencies:
//   New(...)  → /v1/...   JSON REST API
//   UI(...)   → /ui/...   templ+htmx pages
//
// Both go through the IAP middleware mounted in cmd/admin-api/main.go.
package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jswortz/sclawion/pkg/config"
	"github.com/jswortz/sclawion/pkg/event"
)

// Deps is the bag of dependencies every handler needs.
type Deps struct {
	Store     config.Store
	Recorder  *config.Recorder
	Rotator   *config.SecretRotator
	ProjectID string
	Env       string
}

// New returns the JSON API mux mounted at /v1/.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Tenants
	mux.HandleFunc("/v1/tenants", routeTenantsList(d))
	mux.HandleFunc("/v1/tenants/", routeTenantsItem(d))

	// Admin users
	mux.HandleFunc("/v1/admin-users", routeAdminUsers(d))
	mux.HandleFunc("/v1/admin-users/", routeAdminUserItem(d))

	// Audit
	mux.HandleFunc("/v1/audit", listAudit(d))

	return mux
}

// routeTenantsList handles GET (list) and POST (create) on /v1/tenants.
func routeTenantsList(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTenants(d)(w, r)
		case http.MethodPost:
			ownerOnly(createTenant(d))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// routeTenantsItem handles /v1/tenants/{id} and its nested resources.
func routeTenantsItem(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1/tenants/")
		segs := strings.Split(path, "/")
		tid := segs[0]
		if tid == "" {
			http.NotFound(w, r)
			return
		}
		// /v1/tenants/{tid}
		if len(segs) == 1 {
			switch r.Method {
			case http.MethodGet:
				getTenant(d, tid)(w, r)
			case http.MethodPatch:
				ownerOnly(patchTenant(d, tid))(w, r)
			case http.MethodDelete:
				ownerOnly(deleteTenant(d, tid))(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		switch segs[1] {
		case "connectors":
			handleConnectorPath(d, tid, segs[2:], w, r)
		case "agents":
			handleAgentPath(d, tid, segs[2:], w, r)
		case "swarms":
			handleSwarmPath(d, tid, segs[2:], w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func handleConnectorPath(d Deps, tid string, rest []string, w http.ResponseWriter, r *http.Request) {
	if len(rest) == 0 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		listConnectors(d, tid)(w, r)
		return
	}
	platform := event.Platform(rest[0])
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			getConnector(d, tid, platform)(w, r)
		case http.MethodPut:
			ownerOnly(putConnector(d, tid, platform))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if rest[1] == "secrets:rotate" && r.Method == http.MethodPost {
		ownerOnly(rotateSecret(d, tid, platform))(w, r)
		return
	}
	http.NotFound(w, r)
}

func handleAgentPath(d Deps, tid string, rest []string, w http.ResponseWriter, r *http.Request) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			listAgents(d, tid)(w, r)
		case http.MethodPost:
			ownerOnly(createAgent(d, tid))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	id := rest[0]
	switch r.Method {
	case http.MethodGet:
		getAgent(d, tid, id)(w, r)
	case http.MethodPatch:
		ownerOnly(patchAgent(d, tid, id))(w, r)
	case http.MethodDelete:
		ownerOnly(deleteAgent(d, tid, id))(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSwarmPath(d Deps, tid string, rest []string, w http.ResponseWriter, r *http.Request) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			listSwarms(d, tid)(w, r)
		case http.MethodPost:
			ownerOnly(createSwarm(d, tid))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	id := rest[0]
	switch r.Method {
	case http.MethodGet:
		getSwarm(d, tid, id)(w, r)
	case http.MethodPatch:
		ownerOnly(patchSwarm(d, tid, id))(w, r)
	case http.MethodDelete:
		ownerOnly(deleteSwarm(d, tid, id))(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ownerOnly is a per-route wrapper that requires Owner role.
func ownerOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := config.CallerFromContext(r.Context())
		if !ok {
			http.Error(w, "no caller", http.StatusUnauthorized)
			return
		}
		if c.Role != config.RoleOwner {
			http.Error(w, "owner required", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// writeJSON writes v as JSON with the given status code. Errors during encode
// land in the log; the response is already partially flushed by then.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("admin-api: encode response", "err", err)
	}
}

// readJSON decodes the request body into v.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// httpStatusFor maps domain errors to HTTP status codes.
func httpStatusFor(err error) int {
	switch {
	case errors.Is(err, config.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, config.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, config.ErrSecretMissing):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
