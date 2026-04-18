# `pkg/connectors/`

Each subpackage implements one chat platform. The contract is three
interfaces in [`connector.go`](connector.go):

- **`Verifier`** — authenticates an inbound webhook (HMAC, Ed25519, or
  OIDC JWT). Always uses `crypto/subtle.ConstantTimeCompare` and rejects
  timestamps outside `auth.MaxSkew` (±5 min).
- **`Decoder`** — turns a verified body into a normalized `event.Envelope`.
- **`Encoder`** — posts an outbound `Envelope` back to the platform.

Adding a new platform: see [`docs/CONNECTORS.md`](../../docs/CONNECTORS.md).
Onboarding an existing platform to a tenant (operator-side):
[`docs/CHAT-INTEGRATION.md`](../../docs/CHAT-INTEGRATION.md).

## Status — implementation matrix

| Platform | Verifier | Decoder | Encoder | Package |
|----------|----------|---------|---------|---------|
| Slack | ✓ HMAC-SHA256 | ✓ partial (Events API subset) | stub | [`slack/`](slack/README.md) |
| Discord | ✓ Ed25519 | stub | stub | [`discord/`](discord/README.md) |
| Google Chat | ✓ OIDC RS256 JWT | stub | stub | [`gchat/`](gchat/README.md) |
| WhatsApp | ✓ HMAC-SHA256 | stub | stub | [`whatsapp/`](whatsapp/README.md) |
| iMessage (Sendblue / BlueBubbles bridge) | ✓ HMAC-SHA256 | stub | stub | [`imessage/`](imessage/README.md) |

"Stub" means the file exists and returns `errors.New("...: not implemented")`
so the call site is wired but the body is a TODO. Verifiers are the only
parts that are security-critical; everything else is best-effort scaffolding
until the data plane (`cmd/ingress`, `cmd/router`, `cmd/emitter`) lands.

## Cross-cutting references

- Envelope spec: [`docs/EVENT_SCHEMA.md`](../../docs/EVENT_SCHEMA.md) — every
  Decoder must produce this shape.
- Security non-negotiables: [`docs/SECURITY.md`](../../docs/SECURITY.md) —
  constant-time HMAC, ±5-min skew, no JSON SA keys, CMEK in non-dev.
- Per-platform signature schemes: [`CLAUDE.md`](../../CLAUDE.md#per-platform-signature-schemes).
- Architecture: [`docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md).
