# Roadmap

Living document. What's shipped, what's next, what's parked. Reorganize
freely; treat as conversation, not contract.

## Status legend

- ✅ Done
- 🚧 In progress
- 🎯 Next
- 🧊 Parked / future
- ❌ Dropped

## Milestones

### M0 — Scaffold (✅)

- ✅ Repository structure and module layout
- ✅ Apache 2.0 license, .gitignore, CLAUDE.md
- ✅ Normalized event schema (`sclawion/v1`)
- ✅ Connector interfaces + four package skeletons
- ✅ Real signature verifiers: Slack (HMAC-SHA256), Discord (Ed25519),
  WhatsApp (HMAC-SHA256), Google Chat (RS256 JWT)
- ✅ Four `cmd/` services compiling
- ✅ Terraform skeleton (KMS, CMEK Pub/Sub topics, DLQs)
- ✅ Cloud Run manifest for ingress
- ✅ Scion skill scaffold
- ✅ Slack + WhatsApp onboarding examples
- ✅ Architecture, security, ops, contributing, event-schema, connectors,
  GCP-patterns docs

### M1 — Slack end-to-end (🎯 next)

Goal: real round-trip from Slack mention to Scion agent reply, in stage.

- 🎯 Slack `Decoder` covers `app_mention`, `message.im`, `message.channels`
- 🎯 Slack `Encoder` posts via `chat.postMessage` with `thread_ts`
- 🎯 `pkg/scion/client.go` real implementation against a local Scion Hub
- 🎯 `pkg/correlation` Firestore implementation (interface backed by emulator
  in tests)
- 🎯 `pkg/secrets` GSM client implementation with cache + invalidation
- 🎯 `cmd/ingress` wires verifier → decoder → publisher
- 🎯 `cmd/router` wires push receiver → idempotency → dispatch
- 🎯 `cmd/scion-bridge` v1: WebSocket log tailer that recognizes
  `<<sclawion-event>>` markers
- 🎯 `cmd/emitter` Slack path
- 🎯 Terraform: push subscriptions, Cloud Run services, IAM, Cloud Armor,
  Secret Manager
- 🎯 Local `docker-compose.yaml` with Pub/Sub + Firestore emulators + fake
  Scion Hub
- 🎯 Integration test passes against emulators
- 🎯 Stage smoke test: real Slack channel → real Scion agent → real reply

### M2 — Three more platforms

- 🎯 Discord end-to-end (Interactions API + bot)
- 🎯 Google Chat end-to-end (Workspace API)
- 🎯 WhatsApp end-to-end (Meta Cloud API + verification flow)

### M3 — Hardening

- 🎯 OTEL tracing fully wired (spans propagated end-to-end)
- 🎯 SLO definitions + Cloud Monitoring alerts
- 🎯 Auto-rollback on Cloud Run revision SLO breach
- 🎯 Replay-cache fallback mode
- 🎯 BigQuery audit log sink + Looker Studio dashboard template
- 🎯 Binary Authorization attestor and Cloud Build integration
- 🎯 `govulncheck` and `staticcheck` in CI

### M4 — Polish

- 🧊 Web dashboard (Next.js on Cloud Run): live conversation list, agent
  status, replay controls
- 🧊 Scion plugin replacing the WebSocket-tailing bridge fallback
- 🧊 Per-tenant project isolation pattern (Terraform module)
- 🧊 OpenAPI spec for the Hub-facing contract (in case Scion's API drifts)
- 🧊 Multi-region active/active (vs current active/passive)

### Future / parked

- 🧊 Mattermost connector (the example in `CONNECTORS.md`)
- 🧊 Microsoft Teams connector
- 🧊 SMS via Twilio Programmable Messaging
- 🧊 Voice (transcribe → user.message; agent.reply → TTS)
- 🧊 Eventarc-only ingress (no `cmd/ingress` service) once GCP-side support
  matures
- 🧊 ADK / Vertex Agent Builder backend behind the same envelope
- 🧊 Confidential Computing on Cloud Run
- 🧊 Per-tenant CMEK with customer-controlled keys
- 🧊 E2EE for chat content (envelope-encryption with per-conversation key)

### Won't do

- ❌ Per-platform topic proliferation (use attribute filters instead)
- ❌ Long-lived stateful workers (kills scale-to-zero)
- ❌ JSON service-account keys (use Workload Identity)
- ❌ Bypassing signature verification "for testing" (tests sign their fixtures)

## Open design questions

These are real decisions the project hasn't made yet. PRs that take a
position are welcome.

1. **Bridge plugin vs WebSocket tail.** v1 will tail logs for simplicity.
   When does the cost of keeping a WebSocket per active agent exceed the
   cost of building/maintaining a Scion plugin?
2. **Per-tenant isolation model.** Per-project gives strong blast-radius
   guarantees but quadruples ops surface. Per-tenant resource prefixing in a
   shared project is cheaper but couples tenants. First customer's
   requirements decide.
3. **Outbound rate limiting model.** Per-conversation default of 30/min may
   be too restrictive for some use cases (e.g., long agent monologues).
   Should the limit be per `conversation_id`, per `scion_agent_id`, or both?
4. **Schema validator location.** Each receiver currently does ad-hoc
   validation. Worth a `pkg/event.Validate(*Envelope) error`? Probably yes.
5. **Multi-tenant audit isolation.** One BigQuery dataset per tenant or one
   shared with row-level security?

## How to propose changes to this roadmap

Open a PR that:
- Adds the item under the right milestone or in "future."
- States the user need it serves (one sentence).
- States what success looks like (one sentence).

Maintainers can promote items to 🎯 / 🚧 in subsequent commits.
