# sclawion

Async messaging-platform integration for [Scion](https://github.com/GoogleCloudPlatform/scion) — Google Cloud's multi-agent orchestration platform.

`sclawion` lets users converse with Scion agents from **Google Chat, Slack, Discord, and WhatsApp**. The control plane is GCP **Pub/Sub**: every inbound chat event and every outbound agent event flows through normalized topics, decoupling connectors from Scion and enabling fan-out, retry, ordering, and audit.

> Name: a mashup of **Sc**ion + open**claw**ion.

## Why this exists

Scion's Hub exposes a REST API for dispatching agents and a WebSocket for log streaming, but **no native webhook, event-bus, or chat integration**. `sclawion` fills that gap with a stateless, GCP-native bridge.

## Architecture

```
chat platforms ──webhooks──▶ Cloud Armor + LB
                                    │
                                    ▼
                         Cloud Run: ingress (verify HMAC, normalize)
                                    │
                                    ▼
                       Pub/Sub: sclawion.inbound  (CMEK, DLQ)
                                    │  OIDC push
                                    ▼
                         Cloud Run: router  ─────▶ Scion Hub
                                                       │
                                    ┌──────────────────┘
                                    ▼ status callbacks
                         Cloud Run: scion-bridge
                                    │
                                    ▼
                       Pub/Sub: sclawion.outbound  (per-platform filters)
                                    │  OIDC push
                       ┌────────────┼────────────┬────────────┐
                       ▼            ▼            ▼            ▼
                    gchat        slack       discord     whatsapp
                                  emitters (Cloud Run, post replies back)
```

## Status

Pre-alpha scaffolding. See `CLAUDE.md` for the architecture detail every contributor (human or agent) should read first.

## Quickstart

```bash
go mod tidy
go build ./...
docker compose -f deploy/compose.yaml up   # local Pub/Sub + Firestore emulators (TODO)
```

## License

Apache-2.0 — matches Scion.
