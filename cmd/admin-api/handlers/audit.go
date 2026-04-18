package handlers

import (
	"net/http"
	"strconv"

	"github.com/jswortz/sclawion/pkg/config"
)

func listAudit(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := config.AuditFilter{
			TenantID:     q.Get("tenant"),
			Actor:        q.Get("actor"),
			ResourceType: q.Get("resource"),
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				f.Limit = n
			}
		}
		if f.Limit == 0 {
			f.Limit = 200
		}
		es, err := d.Store.ListAudit(r.Context(), f)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, es)
	}
}
