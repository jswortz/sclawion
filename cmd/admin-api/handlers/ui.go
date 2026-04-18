package handlers

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/jswortz/sclawion/pkg/config"
	"github.com/jswortz/sclawion/pkg/event"
)

//go:embed templates/*.html static/*
var assets embed.FS

var uiTpl = template.Must(template.New("ui").
	Funcs(template.FuncMap{"uiPath": uiPath}).
	ParseFS(assets, "templates/*.html"))

// UI returns the htmx-driven web UI mux mounted at /ui/. Templates live in
// cmd/admin-api/handlers/templates/ and are embedded into the binary.
//
// The plan calls for templ; we use stdlib html/template here to avoid adding
// a code-generation step. The template surface is small enough that the
// difference is presentational, not architectural — moving to templ later is
// a per-page port, not a redesign.
func UI(d Deps) http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(assets, "static")
	mux.Handle("/ui/static/", http.StripPrefix("/ui/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		// Default landing page.
		renderUITenants(d, w, r)
	})
	mux.HandleFunc("/ui/tenants", func(w http.ResponseWriter, r *http.Request) { renderUITenants(d, w, r) })
	mux.HandleFunc("/ui/tenants/", func(w http.ResponseWriter, r *http.Request) {
		// /ui/tenants/{tid}[/connectors|agents|swarms]
		rest := strings.TrimPrefix(r.URL.Path, "/ui/tenants/")
		segs := strings.Split(rest, "/")
		tid := segs[0]
		if tid == "" {
			renderUITenants(d, w, r)
			return
		}
		section := "overview"
		if len(segs) >= 2 && segs[1] != "" {
			section = segs[1]
		}
		switch section {
		case "connectors":
			renderUIConnectors(d, w, r, tid)
		case "agents":
			renderUIAgents(d, w, r, tid)
		case "swarms":
			renderUISwarms(d, w, r, tid)
		default:
			renderUITenant(d, w, r, tid)
		}
	})
	mux.HandleFunc("/ui/admin-users", func(w http.ResponseWriter, r *http.Request) { renderUIAdminUsers(d, w, r) })
	mux.HandleFunc("/ui/audit", func(w http.ResponseWriter, r *http.Request) { renderUIAudit(d, w, r) })

	return mux
}

type pageData struct {
	Caller     config.Caller
	Env        string
	Title      string
	ActiveNav  string
	TenantID   string
	Tenants    []config.Tenant
	Tenant     *config.Tenant
	Connectors []config.ConnectorConfig
	Agents     []config.AgentProfile
	Swarms     []config.SwarmDef
	Admins     []config.AdminUser
	Audit      []config.AuditEntry
	Platforms  []event.Platform
	Topologies []string
	Roles      []string
	Memories   []string
}

func basePage(d Deps, r *http.Request, title, active string) pageData {
	c, _ := config.CallerFromContext(r.Context())
	return pageData{
		Caller:     c,
		Env:        d.Env,
		Title:      title,
		ActiveNav:  active,
		Platforms:  []event.Platform{event.PlatformSlack, event.PlatformDiscord, event.PlatformGChat, event.PlatformWhatsApp, event.PlatformIMessage},
		Topologies: []string{"pipeline", "fanout", "mesh", "hierarchical"},
		Roles:      []string{"planner", "coder", "reviewer", "deployer", "monitor"},
		Memories:   []string{"conversation", "user"},
	}
}

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := uiTpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderUITenants(d Deps, w http.ResponseWriter, r *http.Request) {
	p := basePage(d, r, "Tenants", "tenants")
	if ts, err := d.Store.ListTenants(r.Context()); err == nil {
		p.Tenants = ts
	}
	render(w, "tenants.html", p)
}

func renderUITenant(d Deps, w http.ResponseWriter, r *http.Request, tid string) {
	p := basePage(d, r, "Tenant: "+tid, "tenants")
	p.TenantID = tid
	if t, err := d.Store.GetTenant(r.Context(), tid); err == nil {
		p.Tenant = t
	}
	if cs, err := d.Store.ListConnectors(r.Context(), tid); err == nil {
		p.Connectors = cs
	}
	if as, err := d.Store.ListAgents(r.Context(), tid); err == nil {
		p.Agents = as
	}
	if ss, err := d.Store.ListSwarms(r.Context(), tid); err == nil {
		p.Swarms = ss
	}
	render(w, "tenant.html", p)
}

func renderUIConnectors(d Deps, w http.ResponseWriter, r *http.Request, tid string) {
	p := basePage(d, r, "Connectors: "+tid, "tenants")
	p.TenantID = tid
	if t, err := d.Store.GetTenant(r.Context(), tid); err == nil {
		p.Tenant = t
	}
	if cs, err := d.Store.ListConnectors(r.Context(), tid); err == nil {
		p.Connectors = cs
	}
	render(w, "connectors.html", p)
}

func renderUIAgents(d Deps, w http.ResponseWriter, r *http.Request, tid string) {
	p := basePage(d, r, "Agents: "+tid, "tenants")
	p.TenantID = tid
	if t, err := d.Store.GetTenant(r.Context(), tid); err == nil {
		p.Tenant = t
	}
	if as, err := d.Store.ListAgents(r.Context(), tid); err == nil {
		p.Agents = as
	}
	render(w, "agents.html", p)
}

func renderUISwarms(d Deps, w http.ResponseWriter, r *http.Request, tid string) {
	p := basePage(d, r, "Swarms: "+tid, "tenants")
	p.TenantID = tid
	if t, err := d.Store.GetTenant(r.Context(), tid); err == nil {
		p.Tenant = t
	}
	if ss, err := d.Store.ListSwarms(r.Context(), tid); err == nil {
		p.Swarms = ss
	}
	render(w, "swarms.html", p)
}

func renderUIAdminUsers(d Deps, w http.ResponseWriter, r *http.Request) {
	p := basePage(d, r, "Admin Users", "admin-users")
	if us, err := d.Store.ListAdminUsers(r.Context()); err == nil {
		p.Admins = us
	}
	render(w, "admin_users.html", p)
}

func renderUIAudit(d Deps, w http.ResponseWriter, r *http.Request) {
	p := basePage(d, r, "Audit", "audit")
	if es, err := d.Store.ListAudit(r.Context(), config.AuditFilter{Limit: 200}); err == nil {
		p.Audit = es
	}
	render(w, "audit.html", p)
}

// uiPath is a small helper used inside templates (e.g., {{uiPath "tenants" .ID "connectors"}}).
func uiPath(parts ...string) string {
	return "/ui/" + path.Join(parts...)
}
