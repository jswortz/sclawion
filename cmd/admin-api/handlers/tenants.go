package handlers

import (
	"net/http"
	"time"

	"github.com/jswortz/sclawion/pkg/config"
)

func listTenants(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ts, err := d.Store.ListTenants(r.Context())
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, ts)
	}
}

func getTenant(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := d.Store.GetTenant(r.Context(), tid)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func createTenant(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var t config.Tenant
		if err := readJSON(r, &t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateTenant(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		now := time.Now().UTC()
		t.CreatedAt = now
		t.UpdatedAt = now
		if err := d.Store.PutTenant(r.Context(), &t); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "tenant.create", ResourceType: "tenant", ResourceID: t.ID,
			TenantID: t.ID, After: config.Redact(t),
		})
		writeJSON(w, http.StatusCreated, t)
	}
}

func patchTenant(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		existing, err := d.Store.GetTenant(r.Context(), tid)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		before := *existing
		var patch config.Tenant
		if err := readJSON(r, &patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if patch.DisplayName != "" {
			existing.DisplayName = patch.DisplayName
		}
		existing.Disabled = patch.Disabled
		existing.UpdatedAt = time.Now().UTC()
		if err := config.ValidateTenant(existing); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := d.Store.PutTenant(r.Context(), existing); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "tenant.patch", ResourceType: "tenant", ResourceID: tid, TenantID: tid,
			Before: config.Redact(before), After: config.Redact(existing),
		})
		writeJSON(w, http.StatusOK, existing)
	}
}

func deleteTenant(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.Store.DeleteTenant(r.Context(), tid); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "tenant.delete", ResourceType: "tenant", ResourceID: tid, TenantID: tid,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
