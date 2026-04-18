package handlers

import (
	"net/http"
	"time"

	"github.com/jswortz/sclawion/pkg/config"
)

func listAgents(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		as, err := d.Store.ListAgents(r.Context(), tid)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, as)
	}
}

func getAgent(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := d.Store.GetAgent(r.Context(), tid, id)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func createAgent(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var a config.AgentProfile
		if err := readJSON(r, &a); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.TenantID = tid
		if err := config.ValidateAgent(&a); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		a.UpdatedAt = time.Now().UTC()
		a.UpdatedBy = caller.Email
		if err := d.Store.PutAgent(r.Context(), &a); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "agent.create", ResourceType: "agent",
			ResourceID: a.ID, TenantID: tid,
			After: config.Redact(a),
		})
		writeJSON(w, http.StatusCreated, a)
	}
}

func patchAgent(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		existing, err := d.Store.GetAgent(r.Context(), tid, id)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		before := *existing
		var patch config.AgentProfile
		if err := readJSON(r, &patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mergeAgent(existing, &patch)
		caller, _ := config.CallerFromContext(r.Context())
		existing.UpdatedAt = time.Now().UTC()
		existing.UpdatedBy = caller.Email
		if err := config.ValidateAgent(existing); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := d.Store.PutAgent(r.Context(), existing); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "agent.patch", ResourceType: "agent",
			ResourceID: id, TenantID: tid,
			Before: config.Redact(before), After: config.Redact(existing),
		})
		writeJSON(w, http.StatusOK, existing)
	}
}

func deleteAgent(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.Store.DeleteAgent(r.Context(), tid, id); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "agent.delete", ResourceType: "agent",
			ResourceID: id, TenantID: tid,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

func mergeAgent(dst, src *config.AgentProfile) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.MCPServers != nil {
		dst.MCPServers = src.MCPServers
	}
	if src.Skills != nil {
		dst.Skills = src.Skills
	}
	if src.MemoryScope != "" {
		dst.MemoryScope = src.MemoryScope
	}
	if src.BudgetUSDPerHour != 0 {
		dst.BudgetUSDPerHour = src.BudgetUSDPerHour
	}
}
