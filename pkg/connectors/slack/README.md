# Slack connector

Inbound: Slack Events API HTTPS webhooks. Outbound: `chat.postMessage`
via Slack Web API.

## Status

| Component | Status | File |
|-----------|--------|------|
| `Verifier` | ✓ HMAC-SHA256 over `v0:{ts}:{body}`, ±5-min skew enforced | [`verify.go`](verify.go) |
| `Decoder` | ✓ partial — Events API subset (channel, ts, thread_ts, text, user) | [`decode.go`](decode.go) |
| `Encoder` | stub | [`encode.go`](encode.go) |

## Signature scheme

```
basestring = "v0:" + X-Slack-Request-Timestamp + ":" + raw_body
expected   = "v0=" + hex(HMAC-SHA256(signing_secret, basestring))
```

Compared against header `X-Slack-Signature` with constant-time compare.

## Provider citations

- **Verifying requests** — <https://docs.slack.dev/authentication/verifying-requests-from-slack>
- **Events API** — <https://docs.slack.dev/apis/events-api/>
- **`chat.postMessage`** (outbound) — <https://docs.slack.dev/methods/chat.postMessage>
- **OAuth scopes for bot tokens** — <https://docs.slack.dev/authentication/oauth-v2>
- **Rate limits** — <https://docs.slack.dev/apis/rate-limits>

## Operator setup

To onboard a Slack workspace to a tenant:
[`docs/CHAT-INTEGRATION.md`](../../../docs/CHAT-INTEGRATION.md) — covers
signing-secret retrieval, `/secrets:rotate`, and **Event Subscriptions**
URL configuration. Required scopes: `chat:write`, `channels:history`,
`im:history`.

## Tests

Verifier + Decoder unit tests: [`verify_test.go`](verify_test.go) — covers
positive path, tampered body, stale timestamp, missing headers, and
malformed `v0=` prefix. Sample signed payloads live in
[`test/fixtures/slack-message.json`](../../../test/fixtures/slack-message.json).
