# Discord connector

Inbound: Discord **Interactions Endpoint** HTTPS webhooks (interaction
events + Gateway forwarders). Outbound: REST `POST /channels/{id}/messages`.

## Status

| Component | Status | File |
|-----------|--------|------|
| `Verifier` | ✓ Ed25519 signature over `timestamp ‖ body` | [`discord.go`](discord.go) |
| `Decoder` | stub | [`discord.go`](discord.go) |
| `Encoder` | stub | [`discord.go`](discord.go) |

## Signature scheme

```
sig = ed25519_verify(public_key, X-Signature-Timestamp || raw_body,
                     hex_decode(X-Signature-Ed25519))
```

`PublicKey` for the application is in App Config → **General Information**.
There is no shared secret to leak from the platform side — Ed25519 keys
are asymmetric.

## Provider citations

- **Receiving and responding to interactions** — <https://discord.com/developers/docs/interactions/receiving-and-responding>
- **Setting up an interactions endpoint (security)** — <https://discord.com/developers/docs/interactions/overview#setting-up-an-endpoint>
- **Create Message** (outbound) — <https://discord.com/developers/docs/resources/channel#create-message>
- **Bot authentication** — <https://discord.com/developers/docs/topics/oauth2#bot-authorization-flow>
- **Gateway intents** (for non-interaction events) — <https://discord.com/developers/docs/topics/gateway#gateway-intents>
- **Rate limits** — <https://discord.com/developers/docs/topics/rate-limits>

## Operator setup

[`docs/CHAT-INTEGRATION.md`](../../../docs/CHAT-INTEGRATION.md) — Step 6
walks through pasting the Interactions Endpoint URL; ingress responds
to Discord's `PING` automatically once the verifier is wired.
