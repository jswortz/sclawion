# WhatsApp connector

Inbound: Meta WhatsApp Cloud API webhooks. Outbound: Graph API
`POST /{phone-number-id}/messages`.

## Status

| Component | Status | File |
|-----------|--------|------|
| `Verifier` | ✓ HMAC-SHA256 over raw body, header `X-Hub-Signature-256` | [`whatsapp.go`](whatsapp.go) |
| `Decoder` | stub | [`whatsapp.go`](whatsapp.go) |
| `Encoder` | stub | [`whatsapp.go`](whatsapp.go) |

## Signature scheme

```
expected = "sha256=" + hex(HMAC-SHA256(app_secret, raw_body))
```

Compared against `X-Hub-Signature-256`. The secret is the **App Secret**
(Meta App dashboard → Basic Settings), *not* the system-user access token.

Webhook URL verification (`hub.verify_token` query handshake) happens
once at subscription time and is also the operator's responsibility to
match — see operator setup below.

## Provider citations

- **Webhooks payload validation** — <https://developers.facebook.com/docs/messenger-platform/webhooks#validate-payloads>
- **Cloud API webhooks reference** — <https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks>
- **Sending messages** (outbound) — <https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages>
- **System user access tokens** — <https://developers.facebook.com/docs/whatsapp/business-management-api/get-started>
- **Rate limits** — <https://developers.facebook.com/docs/whatsapp/cloud-api/overview#rate-limits>
- **Pricing tiers (template vs session)** — <https://developers.facebook.com/docs/whatsapp/pricing>

## Operator setup

[`docs/CHAT-INTEGRATION.md`](../../../docs/CHAT-INTEGRATION.md) — Step 6's
WhatsApp section covers the `hub.verify_token` callback handshake and
the Meta App → Webhooks subscription panel.
