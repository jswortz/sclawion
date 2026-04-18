package handlers

import (
	"net/http"
	"time"

	"github.com/jswortz/sclawion/pkg/config"
)

func listSwarms(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ss, err := d.Store.ListSwarms(r.Context(), tid)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, ss)
	}
}

func getSwarm(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := d.Store.GetSwarm(r.Context(), tid, id)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func createSwarm(d Deps, tid string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s config.SwarmDef
		if err := readJSON(r, &s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.TenantID = tid
		if err := config.ValidateSwarm(&s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		s.UpdatedAt = time.Now().UTC()
		s.UpdatedBy = caller.Email
		if err := d.Store.PutSwarm(r.Context(), &s); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "swarm.create", ResourceType: "swarm",
			ResourceID: s.ID, TenantID: tid,
			After: config.Redact(s),
		})
		writeJSON(w, http.StatusCreated, s)
	}
}

func patchSwarm(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		existing, err := d.Store.GetSwarm(r.Context(), tid, id)
		if err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		before := *existing
		var patch config.SwarmDef
		if err := readJSON(r, &patch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if patch.Topology != "" {
			existing.Topology = patch.Topology
		}
		if patch.Roster != nil {
			existing.Roster = patch.Roster
		}
		if patch.BudgetEnvelope.MaxTokens != 0 {
			existing.BudgetEnvelope.MaxTokens = patch.BudgetEnvelope.MaxTokens
		}
		if patch.BudgetEnvelope.MaxWallClock != 0 {
			existing.BudgetEnvelope.MaxWallClock = patch.BudgetEnvelope.MaxWallClock
		}
		if patch.BudgetEnvelope.MaxDeploys != 0 {
			existing.BudgetEnvelope.MaxDeploys = patch.BudgetEnvelope.MaxDeploys
		}
		caller, _ := config.CallerFromContext(r.Context())
		existing.UpdatedAt = time.Now().UTC()
		existing.UpdatedBy = caller.Email
		if err := config.ValidateSwarm(existing); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := d.Store.PutSwarm(r.Context(), existing); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "swarm.patch", ResourceType: "swarm",
			ResourceID: id, TenantID: tid,
			Before: config.Redact(before), After: config.Redact(existing),
		})
		writeJSON(w, http.StatusOK, existing)
	}
}

func deleteSwarm(d Deps, tid, id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.Store.DeleteSwarm(r.Context(), tid, id); err != nil {
			http.Error(w, err.Error(), httpStatusFor(err))
			return
		}
		caller, _ := config.CallerFromContext(r.Context())
		d.Recorder.Record(r.Context(), &config.AuditEntry{
			Actor: caller.Email, ActorRole: caller.Role,
			Action: "swarm.delete", ResourceType: "swarm",
			ResourceID: id, TenantID: tid,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
