# sclawion

> Async chat ↔ agent bridge for [Scion](https://github.com/GoogleCloudPlatform/scion).
> Talk to a GCP multi-agent fleet from Slack, Discord, Google Chat, WhatsApp, or iMessage.

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Status: pre-alpha](https://img.shields.io/badge/status-pre--alpha-orange.svg)](docs/ROADMAP.md)
[![Built for: GCP](https://img.shields.io/badge/cloud-GCP-4285F4.svg)](docs/GCP_PATTERNS.md)
[![Language: Go](https://img.shields.io/badge/lang-Go%201.22-00ADD8.svg)](go.mod)

`sclawion` is the missing async surface for [Scion](https://github.com/GoogleCloudPlatform/scion):
a stateless, GCP-native bridge that puts every chat platform on the same wire
— **GCP Pub/Sub** — so a Scion agent fleet can be addressed as if it were a
teammate sitting in your channel.

![End-to-end flow](docs/figures/sclawion-flow.png)

## At a glance

The bridge sits between three layers — chat surfaces on top, the agent swarm in
the middle, and (in the enterprise [CLAWPATH](docs/CLAWPATH.md) tier) a
SCION-routed network underlay reaching into customer infrastructure on the
bottom. The Pub/Sub envelope is the single contract between them.

![Three-layer view](docs/figures/clawpath-layers.png)

End-to-end, the same diagram with concrete GCP services and topic flow:

![End-to-end architecture](docs/figures/clawpath-arch.png)

> Both figures show the **CLAWPATH** enterprise tier (chat + swarm + SCION
> network underlay). The base `sclawion` project is just the top two layers —
> chat → ingress → Pub/Sub → router → Scion Hub → bridge → emitters → chat.
> The SCION underlay is opt-in for customers who need agents to reach inside
> private networks without BGP / VPN sprawl. See [`CLAWPATH.md`](docs/CLAWPATH.md)
> and [`clawpath/SCION.md`](docs/clawpath/SCION.md) for the full story.

## Why this exists

Scion's Hub gives you a REST API to dispatch agents and a WebSocket to tail
logs — and that's it. There's no event bus, no webhook spec, no chat
integration. Every team that wants ChatOps over Scion has to write the same
glue. `sclawion` is that glue, done once, with a security and reliability
posture that holds up under enterprise review.

## What you get

- **One bus, five chat platforms.** Slack, Discord, Google Chat, WhatsApp,
  iMessage (via Sendblue / BlueBubbles bridge) — all normalize to a single
  CloudEvents-shaped envelope ([`pkg/event`](pkg/event/envelope.go)).
  Per-connector status: [`pkg/connectors/README.md`](pkg/connectors/README.md).
- **IAP-fronted control plane.** [`cmd/admin-api`](cmd/admin-api/) gives
  operators a Firestore-backed UI + REST API to onboard tenants, rotate
  secrets, define agent profiles and swarms, and audit every change.
- **Stateless services.** Cloud Run, scale-to-zero, no VMs to patch. State
  lives in Firestore (correlation, idempotency) and Secret Manager (creds).
- **Bidirectional.** Users mention the bot → agent spawned. Agent emits
  updates → posted back into the originating thread.
- **Enterprise security baseline.** CMEK on Pub/Sub + Firestore, Workload
  Identity (no JSON keys), constant-time HMAC, Cloud Armor at the edge,
  VPC-SC perimeter, Binary Authorization on container images. See
  [`docs/SECURITY.md`](docs/SECURITY.md).
- **Reliability primitives.** OIDC push, dead-letter topics, ordering keys,
  Firestore-backed idempotency. See [`docs/OPERATIONS.md`](docs/OPERATIONS.md).

## Documentation

Index by audience: [`docs/README.md`](docs/README.md).

**Get started**

| Doc | For |
|-----|-----|
| [`docs/INSTALL.md`](docs/INSTALL.md) | Install and run end-to-end (local + GCP). |
| [`docs/CHAT-INTEGRATION.md`](docs/CHAT-INTEGRATION.md) | **Operator** guide — onboard a chat workspace to a tenant. |
| [`docs/AGENT-CONTEXT.md`](docs/AGENT-CONTEXT.md) | **Operator** guide — bind an agent to MCPs, skills, memory, swarms. |
| [`docs/figures/admin-ui/`](docs/figures/admin-ui/README.md) | Screenshots of the seeded admin UI. |
| [`examples/`](examples/) | Per-platform raw-shell walkthroughs (Slack, WhatsApp). |

**Deep dive**

| Doc | What's in it |
|-----|--------------|
| [`CLAUDE.md`](CLAUDE.md) | Load-bearing context for any contributor (human or agent). Read first. |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Components, sequence diagrams, design tradeoffs, exit ramps. |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Threat model, controls, compliance posture. |
| [`docs/OPERATIONS.md`](docs/OPERATIONS.md) | Deployment topology, SLOs, runbooks, incident response. |
| [`docs/EVENT_SCHEMA.md`](docs/EVENT_SCHEMA.md) | The normalized `Envelope` reference + versioning rules. |
| [`docs/CONNECTORS.md`](docs/CONNECTORS.md) | How to add a new chat platform end-to-end (developer-facing). |
| [`pkg/connectors/README.md`](pkg/connectors/README.md) | Per-connector status matrix + provider citations. |
| [`docs/GCP_PATTERNS.md`](docs/GCP_PATTERNS.md) | Why GCP-native primitives carry their weight here. |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | Dev setup, coding standards, PR + CI process. |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | What's shipped, what's next. |

## Quickstart

```bash
git clone https://github.com/jswortz/sclawion
cd sclawion
go build ./...
go test -race ./...

# Run the admin UI locally with the dev IAP bypass
ADMIN_API_DEV_IAP_BYPASS_EMAIL=owner@dev.local ENV=dev \
  go run ./cmd/admin-api
# → http://localhost:8088/ui/tenants
```

Full install + run + monitor + secure walkthrough:
[`docs/INSTALL.md`](docs/INSTALL.md). Onboarding a chat workspace once
the platform is deployed: [`docs/CHAT-INTEGRATION.md`](docs/CHAT-INTEGRATION.md).

## Repo layout

```
cmd/                   five Cloud Run service entrypoints
  ingress/             webhook receiver (all platforms; data-plane stub)
  router/              inbound → Scion dispatcher (data-plane stub)
  scion-bridge/        Scion → outbound topic (data-plane stub)
  emitter/             outbound → platform (one binary, --platform flag; stub)
  admin-api/           IAP-fronted config plane (Firestore + Secret Manager
                       writes, htmx UI) — implemented
pkg/
  event/               normalized CloudEvent envelope (sclawion/v1)
  connectors/<p>/      Verifier + Decoder + Encoder per platform
                       — see pkg/connectors/README.md for status matrix
  scion/               typed Scion Hub REST client (skeleton)
  correlation/         Firestore conversation ↔ agent store (interface only)
  config/              tenant/connector/agent/swarm types + Store + RBAC + audit
  secrets/             Secret Manager wrapper (Get + AddVersion)
  auth/                HMAC, OIDC, IAP claims, replay-cache helpers
  pubsub/              publish/ack helpers
  obs/                 OpenTelemetry init
deploy/
  terraform/           all GCP infra (main.tf + admin.tf)
  cloudrun/            service manifests
skills/openclaw/       Scion skill so agents can self-publish events
test/
  integration/         emulator + httptest end-to-end tests
  fixtures/            signed sample webhooks
docs/                  deep-dive documentation (index in docs/README.md)
examples/              per-platform raw-shell onboarding walkthroughs
.github/workflows/     CI: vet, staticcheck, race-tested coverage, govulncheck
```

## Status

Pre-alpha scaffolding. APIs and schemas may break.
Roadmap and milestones in [`docs/ROADMAP.md`](docs/ROADMAP.md).

## License

[Apache 2.0](LICENSE) — matches Scion. Contributions welcome; see
[`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md).

## Acknowledgements

- [Scion](https://github.com/GoogleCloudPlatform/scion) — the orchestration
  platform `sclawion` makes conversational.
- [CloudEvents](https://cloudevents.io/) — the envelope shape we borrow.
