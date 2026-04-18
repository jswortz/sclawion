# iMessage connector

Inbound: HTTPS webhooks from a third-party iMessage **bridge** (Apple has
no public webhook API for iMessage). Outbound: REST call back through the
same bridge's send-message endpoint.

## Provider landscape

Apple's first-party "Messages for Business" is gated to approved
Messaging Service Providers and not generally available, so this
connector targets the two open bridges:

| Bridge | Hosting | Verifier `Header` value |
|--------|---------|-------------------------|
| **Sendblue** (default) | Managed SaaS | `sb-signature` |
| **BlueBubbles** | Self-hosted on a Mac mini | `X-BlueBubbles-Signature` |

Both sign webhooks with HMAC-SHA256 over the raw JSON body. The
`Verifier.Header` field selects which header to read; the algorithm is
identical.

## Status

| Component | Status | File |
|-----------|--------|------|
| `Verifier` | ✓ HMAC-SHA256 over raw body | [`imessage.go`](imessage.go) |
| `Decoder` | stub (Sendblue payload shape documented in package doc) | [`imessage.go`](imessage.go) |
| `Encoder` | stub | [`imessage.go`](imessage.go) |

## Signature scheme

```
expected = hex(HMAC-SHA256(signing_secret, raw_body))   // hex, no prefix
```

Compared against the configured header. Timestamp skew is *not* enforced
at the header level — neither bridge sends a signed timestamp; the
Decoder will apply `auth.MaxSkew` against the bridge's `date_sent`
field once that lands.

## Provider citations

- **Sendblue webhook security** — <https://docs.sendblue.com/docs/security>
- **Sendblue send-message API** — <https://docs.sendblue.com/reference/send-message>
- **BlueBubbles webhook integration** — <https://docs.bluebubbles.app/server/integrations/webhooks>
- **BlueBubbles server REST API** — <https://documenter.getpostman.com/view/765844/UV5RnfwM>
- **Apple Messages for Business** (gated MSP path, for reference) — <https://register.apple.com/resources/messages/messaging-documentation/>

## Operator setup

[`docs/CHAT-INTEGRATION.md`](../../../docs/CHAT-INTEGRATION.md) — Step 3's
iMessage row covers Sendblue's signing secret + API key fields; Step 6
covers webhook URL configuration via the Sendblue dashboard's "Send test
webhook" button.
