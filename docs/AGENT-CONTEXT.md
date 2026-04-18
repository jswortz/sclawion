# Wiring agent context: tools, skills, memory

This guide is for an operator (Owner role) who has a tenant onboarded per
[`CHAT-INTEGRATION.md`](CHAT-INTEGRATION.md) and now wants to define **what
the agent on the other end of that chat is actually allowed to do**.

The configuration surface is the `AgentProfile` document under
`config_tenants/{tid}/agents/{agent_id}`:

```go
type AgentProfile struct {
    TenantID, ID, Name, Model string
    MCPServers, Skills        []string
    MemoryScope               string  // "conversation" | "user"
    BudgetUSDPerHour          float64
}
```

Every field is operator-editable through the admin UI or `/v1/...` API.
Mutations are audited; reads are not.

## Mental model

```
chat platform ─► ingress ─► router ─► Scion Hub ─► <agent run>
                              │
                              ▼
                     config_tenants/{tid}/
                       agents/{id}     ← this doc binds {model, MCPs, skills, memory}
                       swarms/{id}     ← this binds rosters of agents into a topology
                                          (see SWARMS.md)
```

The router (cmd/router) reads the tenant's agent profile per inbound
`user.message` event and assembles the **runtime context** that gets handed to
Scion Hub:

| Profile field      | Becomes                                                     |
|--------------------|-------------------------------------------------------------|
| `Model`            | `runs.create.model` on the Scion Hub call                   |
| `MCPServers`       | `tools[].mcp` references in the run spec                    |
| `Skills`           | `skills[]` references — Scion fetches each skill's prompt   |
| `MemoryScope`      | which `memory_*` namespace the run reads/writes             |
| `BudgetUSDPerHour` | hard cap; Scion's billing pre-flight rejects runs over budget |

The connector + correlation layer never see this — the agent profile lives
strictly between the admin plane and the router.

## Step 1 — Create an agent profile

UI: `/ui/tenants/acme/agents` → **New agent profile**
([screenshot](figures/admin-ui/agents-acme.png)).

API:

```
POST /v1/tenants/acme/agents
{
  "id": "support-triage",
  "name": "Acme support triage agent",
  "model": "claude-opus-4-7",
  "mcp_servers": ["github", "linear", "internal-kb"],
  "skills": ["customer-empathy", "log-grep", "ticket-link"],
  "memory_scope": "conversation",
  "budget_usd_per_hour": 5.0
}
```

`memory_scope` is a closed set today: `conversation` (state lives only for
the lifetime of the chat thread) or `user` (state persists across that user's
threads, scoped per tenant). The two values map to different Memory Bank
collections in the data plane.

`budget_usd_per_hour: 0` means *unlimited* — fine for internal/dev tenants;
always set a positive cap in production.

## Step 2 — Register MCP servers (currently free-form)

Today the `mcp_servers` field is an opaque list of string identifiers. The
data plane resolves each name against:

1. A **tenant-scoped** override at `config_tenants/{tid}/mcp_servers/{name}`
   (Firestore doc: `endpoint`, `auth_secret_ref`, `tools_allowlist`).
2. The **global registry** at `config_mcp_servers/{name}` if no override.
3. Otherwise: 404 → router rejects the run with `mcp_unknown:<name>`.

Until the catalog UI lands, you populate the tenant-scoped collection with a
small Firestore script (one-off, by an Owner from `gcloud firestore`):

```bash
gcloud firestore documents create \
  --collection-path=config_tenants/acme/mcp_servers \
  --data='{
    "name": "github",
    "endpoint": "https://mcp.acme.example.com/github",
    "auth_secret_ref": {
      "name": "projects/.../secrets/sclawion-acme-mcp-github-token",
      "version": "1"
    },
    "tools_allowlist": ["pull_request.read", "issue.create", "search.code"]
  }'
```

The MCP secret resource (the bearer token used to call the MCP endpoint) is
created by Terraform exactly like a connector secret in
[`CHAT-INTEGRATION.md`](CHAT-INTEGRATION.md) Step 2. To rotate, use the same
`/secrets:rotate` endpoint family — once the MCP rotate route lands, until
then `gcloud secrets versions add` works.

`tools_allowlist` is the **must-have**: it constrains what the agent can call
on that MCP. Default to least-privilege; agents only need read-most/write-few.

## Step 3 — Register skills

A "skill" in sclawion vocabulary is a named, version-pinned piece of agent
context (a system prompt fragment, a few-shot pack, a structured rubric). It
exists so prompts don't sprawl across tenants and can be audited centrally.

`skills/openclaw/` (in this repo) is the skill the data plane uses to let
agents *publish back* to Pub/Sub. That's a built-in. Tenant-scoped skills go
into `config_tenants/{tid}/skills/{name}`:

```bash
gcloud firestore documents create \
  --collection-path=config_tenants/acme/skills \
  --data='{
    "name": "customer-empathy",
    "version": 3,
    "prompt": "You are responding to a paying customer of Acme. Lead with acknowledgment...",
    "updated_by": "alice@acme.com",
    "updated_at": "2026-04-18T12:00:00Z"
  }'
```

Pin **version**, not "latest". An agent profile that says
`"skills": ["customer-empathy@3"]` is reproducible six months later; one that
says `"customer-empathy"` is not. Today the version pin is parsed by the
router; the admin UI's free-text field accepts the `name@N` shape.

## Step 4 — Choose memory scope

Two values, two Memory Bank tables:

| Scope          | Lifetime                           | What lives there                            |
|----------------|------------------------------------|---------------------------------------------|
| `conversation` | Single chat thread                 | Working memory: scratchpad, latest tool results, plan state |
| `user`         | All threads from the same identity | Long-term: user preferences, prior tickets resolved, established context |

The router emits `MemoryScope` as a field on the run spec. The Memory Bank
service (separate from sclawion; Scion subsystem) keys writes per scope. There
is no *deeper* per-agent isolation — two agents on the same swarm in the same
conversation share the conversation memory by design (that's how a planner
hands off to a coder).

If you need stronger isolation (e.g., a third-party agent that should not see
the human's prior conversation context), use a **separate tenant** today.
Per-agent memory namespaces are on the roadmap; not v1.

## Step 5 — Bind the agent to a swarm (optional)

A solo agent answers chats directly. A *swarm* is multiple agents working on
one task — see [`docs/clawpath/SWARMS.md`](clawpath/SWARMS.md) for the
topology semantics (pipeline, fanout, mesh, hierarchical).

UI: `/ui/tenants/acme/swarms` → **New swarm**, pick topology, check the role
boxes, set the budget envelope. ([screenshot](figures/admin-ui/swarms-acme.png))

API:

```
POST /v1/tenants/acme/swarms
{
  "id": "support-pipeline",
  "topology": "pipeline",
  "roster": ["planner", "coder", "reviewer", "deployer"],
  "budget_envelope": {
    "max_tokens":    500000,
    "max_wallclock": "PT30M",
    "max_deploys":   3
  }
}
```

The swarm's roster references **role names**, not agent IDs. The mapping
"planner role → which agent_id in this tenant" is currently a router-side
constant (`planner` → first agent profile whose `name` contains "planner").
Explicit role-to-agent binding is a follow-up; document any non-obvious
choices in the swarm doc's `notes` field.

## Step 6 — Smoke-test the agent

Send a message in the chat workspace bound to this tenant. In order, you
should observe:

1. **Audit log** for `agent.create` and `swarm.create` you just made (UI: `/ui/audit`).
2. **Cloud Run** `cmd/router` log line: `dispatched event=… agent=support-triage model=claude-opus-4-7 mcps=[github,linear,internal-kb] skills=[customer-empathy@3,...]`.
3. **Scion Hub** run id; if you have access to its console, the run shows the
   tools the MCPs registered.
4. **Reply** in chat from the bot.

If the agent replies with `mcp_unknown:foo`, you forgot to register `foo` in
Step 2. If you get `skill_version_unpinned`, fix the profile to use `name@N`.
If you get `budget_exceeded` immediately, your `budget_usd_per_hour` is too
low for the model + MCP fan-out — bump it.

## Patching live without disruption

Agent profile updates are read per inbound event by the router; there's no
caching layer. So:

- Adding/removing an MCP server: takes effect on the next message. No restart.
- Changing the model: same. The currently-running agent keeps its model; the
  next one runs with the new value.
- Changing memory scope: this is the one that hurts. A scope change orphans
  the conversation memory (the new scope reads from a different namespace).
  For active conversations, change scope only at conversation boundaries or
  with a one-off migration.
- Lowering the budget below current consumption: the in-flight run finishes;
  the next one rejects.

## What this guide does *not* cover

- Catalog UIs for MCP servers and skills (planned; today everything is
  per-tenant Firestore docs).
- Per-agent memory isolation within a conversation (roadmap; v1 punt).
- Skill *content* authoring guidance — that's a prompt-engineering concern,
  not a sclawion concern.
- The Scion Hub side of things — see [`docs/clawpath/SCION.md`](clawpath/SCION.md)
  and [`docs/clawpath/SWARMS.md`](clawpath/SWARMS.md).
