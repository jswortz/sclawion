package handlers

import (
	"net/http"
	"time"

	"github.com/jswortz/sclawion/pkg/config"
	"github.com/jswortz/sclawion/pkg/event"
)

func listConnectors(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := d.Store.ListConnectors(r.Context(), tid)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, cs)
	}
}

func getConnector(d Deps, tid string, p event.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := d.Store.GetConnector(r.Context(), tid, p)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

// putConnector upserts non-secret fields on a connector. Secret refs are
// preserved if present on the existing doc; rotation goes through
// /secrets:rotate.
func putConnector(d Deps, tid string, p event.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in config.ConnectorConfig
		if err := readJSON(r, &in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		in.TenantID = tid
		in.Platform = p
		if in.WebhookPath == "" {
			in.WebhookPath = "/v1/" + string(p)
		}
		if err := config.ValidateConnector(&in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Preserve existing secret refs across non-secret PUTs.
		if existing, err := d.Store.GetConnector(r.Context(), tid, p); err == nil {
			in.SigningSecretRef = existing.SigningSecretRef
			in.OAuthTokenRef = existing.OAuthTokenRef
		}
		caller, _ := config.CallerFromContext(r.Context())
		in.UpdatedAt = time.Now().UTC()
		in.UpdatedBy = caller.Email
		if err := d.Store.PutConnector(r.Context(), &in); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "connector.put", ResourceType: "connector",
			ResourceID: string(p), TenantID: tid,
			After: config.Redact(in),
		})
		writeJSON(w, http.StatusOK, in)
	}
}

type rotateRequest struct {
	Kind   config.SecretKind `json:"kind"`
	Value  string            `json:"value"` // raw plaintext; never echoed
	Reason string            `json:"reason"`
}

type rotateResponse struct {
	SecretRef interface{} `json:"secret_ref"`
	RotatedAt time.Time   `json:"rotated_at"`
}

func rotateSecret(d Deps, tid string, p event.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req rotateRequest
		if err := readJSON(r, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Kind != config.SecretKindSigning && req.Kind != config.SecretKindOAuth {
			http.Error(w, "kind must be signing_secret|oauth_token", http.StatusBadRequest)
			return
		}
		if req.Value == "" {
			http.Error(w, "value required", http.StatusBadRequest)
			return
		}
		if d.Rotator == nil || d.Rotator.Writer == nil {
			http.Error(w, "secret writer not configured", http.StatusServiceUnavailable)
			return
		}
		ref, err := d.Rotator.Rotate(r.Context(), tid, p, req.Kind, []byte(req.Value))
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "connector.rotate_secret", ResourceType: "connector",
			ResourceID: string(p), TenantID: tid,
			// reason recorded via After; value is intentionally absent
			After: config.Redact(map[string]any{
				"kind":    req.Kind,
				"reason":  req.Reason,
				"version": ref.Version,
			}),
		})
		writeJSON(w, http.StatusOK, rotateResponse{
			SecretRef: ref,
			RotatedAt: time.Now().UTC(),
		})
	}
}
