# CLAUDE.md

Read this before changing any code. It's the load-bearing context for the project.

## What this repo is

`sclawion` is an async bridge between chat platforms (Google Chat, Slack, Discord, WhatsApp) and [Scion](https://github.com/GoogleCloudPlatform/scion), GCP's multi-agent orchestration platform. The wire between them is GCP Pub/Sub.

## Architecture rules

1. **Pub/Sub is the only path between connectors and Scion.** No direct calls from `cmd/ingress` or `cmd/emitter` into `pkg/scion`. Only `cmd/router` and `cmd/scion-bridge` may import `pkg/scion`. The config plane (`cmd/admin-api`) never imports `pkg/scion` either — it only writes Firestore + Secret Manager state that the data plane reads.
2. **One normalized event schema** lives in `pkg/event/envelope.go` (CloudEvents-compliant, `spec_version: sclawion/v1`). Every connector encodes/decodes to this shape — never platform-specific structs across package boundaries.
3. **Each connector implements three things**: `Verifier` (HMAC/signature), `Decoder` (platform → Event), `Encoder` (Event → platform message). Adding a new platform means adding a `pkg/connectors/<name>/` package; nothing else changes.
4. **Stateless services.** All state lives in Firestore (correlation, nonce cache, idempotency) or Secret Manager (creds). Do not keep state in-process beyond a single request.
5. **Idempotency by event ID.** Router writes `processed_events/{event_id}` with `CreateIfMissing` before doing work. Pub/Sub's at-least-once is fine; double-delivery must be a no-op.

## Security non-negotiables

- HMAC comparisons use `crypto/subtle.ConstantTimeCompare`. Never `==`.
- Webhook timestamp window: ±5 minutes. Older = reject.
- All service-to-service auth uses Workload Identity / OIDC tokens. **No JSON service-account keys** anywhere — not in env vars, not in Secret Manager, not in code.
- Secrets accessed via `pkg/secrets`, never `os.Getenv` for credentials.
- Pub/Sub topics, Firestore, Artifact Registry: **CMEK required** in non-dev envs.

## Per-platform signature schemes

| Platform     | Header(s)                                            | Algorithm |
|--------------|------------------------------------------------------|-----------|
| Slack        | `X-Slack-Signature`, `X-Slack-Request-Timestamp`     | HMAC-SHA256 over `v0:{ts}:{body}` |
| Discord      | `X-Signature-Ed25519`, `X-Signature-Timestamp`       | Ed25519 over `{ts}{body}` |
| Google Chat  | `Authorization: Bearer <JWT>`                        | RS256 JWT, audience = project |
| WhatsApp     | `X-Hub-Signature-256`                                | HMAC-SHA256 over raw body |

## Layout

```
cmd/                 Cloud Run service entrypoints
  ingress/           webhook receiver (all platforms)
  router/            inbound → Scion dispatcher
  scion-bridge/      Scion → outbound topic
  emitter/           outbound → platform (--platform flag)
  admin-api/         IAP-fronted config plane (Firestore + Secret Manager writes, htmx UI)
pkg/
  event/             normalized envelope + schema
  connectors/<p>/    Verifier/Decoder/Encoder per platform
  scion/             typed Scion Hub REST client
  correlation/       Firestore conversation↔agent store
  config/            tenant/connector/agent/swarm types + Store + RBAC + audit (admin-api)
  secrets/           Secret Manager wrapper (Get + AddVersion), lazy + cached
  auth/              HMAC, OIDC, IAP claims, replay cache helpers
  pubsub/            publish/ack helpers, ordering keys
  obs/               OpenTelemetry init (matches Scion's OTEL setup)
deploy/
  terraform/         all GCP resources
  cloudrun/          service manifests
skills/openclaw/     Scion skill so agents can self-publish events
test/
  integration/       runs against emulators
  fixtures/          signed sample webhooks per platform
```

## When you change things

- Adding a platform → new `pkg/connectors/<name>/` package + register in `cmd/ingress/main.go`'s router and in `cmd/emitter/main.go`'s switch on `--platform`.
- Changing the event schema → bump `spec_version` in `pkg/event/envelope.go`. Old schema must remain readable for at least one release.
- Touching `pkg/scion` → it's the only abstraction over Scion's Hub API; if Scion's API moves, all changes belong here.

## Don't

- Don't add a fifth service to read directly from Pub/Sub when an emitter subscription with attribute filters would do.
- Don't reach into Firestore from a connector — go through `pkg/correlation`.
- Don't add platform-specific logic to `cmd/router`; it must stay platform-agnostic. If it can't, the connector hasn't normalized enough.
