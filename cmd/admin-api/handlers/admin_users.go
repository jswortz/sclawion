package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/jswortz/sclawion/pkg/config"
)

func routeAdminUsers(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ownerOnly(listAdminUsers(d))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func routeAdminUserItem(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := strings.TrimPrefix(r.URL.Path, "/v1/admin-users/")
		if email == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			ownerOnly(putAdminUser(d, email))(w, r)
		case http.MethodDelete:
			ownerOnly(deleteAdminUser(d, email))(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func listAdminUsers(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		us, err := d.Store.ListAdminUsers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, us)
	}
}

func putAdminUser(d Deps, email string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u config.AdminUser
		if err := readJSON(r, &u); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u.Email = email
		if u.AddedAt.IsZero() {
			u.AddedAt = time.Now().UTC()
		}
		if err := config.ValidateAdminUser(&u); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		u.AddedBy = caller.Email
		if err := d.Store.PutAdminUser(r.Context(), &u); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "admin_user.put", ResourceType: "admin_user", ResourceID: email,
			After: config.Redact(u),
		})
		writeJSON(w, http.StatusOK, u)
	}
}

func deleteAdminUser(d Deps, email string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.Store.DeleteAdminUser(r.Context(), email); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "admin_user.delete", ResourceType: "admin_user", ResourceID: email,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
