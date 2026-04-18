# Google Chat connector

Inbound: Google Chat app webhooks (HTTPS). Outbound: Chat REST API
`spaces.messages.create`.

## Status

| Component | Status | File |
|-----------|--------|------|
| `Verifier` | ✓ OIDC RS256 JWT (`Authorization: Bearer …`), audience-checked | [`gchat.go`](gchat.go) |
| `Decoder` | stub | [`gchat.go`](gchat.go) |
| `Encoder` | stub | [`gchat.go`](gchat.go) |

## Signature scheme

Google signs an ID token (RS256) and sends it in the `Authorization`
header. The token's `aud` claim must equal the configured project audience
(typically the project number or the Cloud Run service URL). Verification
uses Google's published JWKS, which is cached and rotated automatically by
[`pkg/auth/oidc.go`](../../auth/oidc.go).

There is no shared secret. This is the **only** platform where the connector
auth is purely identity-federated — leaking the service URL is harmless.

## Provider citations

- **Authenticating Chat app requests** — <https://developers.google.com/workspace/chat/authenticate-authorize-chat-app>
- **Verifying bearer tokens** (general OIDC) — <https://developers.google.com/identity/protocols/oauth2/openid-connect#validatinganidtoken>
- **Receive and respond to events** — <https://developers.google.com/workspace/chat/receive-respond-interactions>
- **`spaces.messages.create`** (outbound) — <https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/create>
- **App publishing** — <https://developers.google.com/workspace/chat/publish-app>
- **Quotas and limits** — <https://developers.google.com/workspace/chat/limits>

## Operator setup

[`docs/CHAT-INTEGRATION.md`](../../../docs/CHAT-INTEGRATION.md) — for
Google Chat the credential field in Step 5 is empty: there is nothing to
paste. Instead the workspace admin pastes the webhook URL in Step 6 and
the bridge's service account is granted the `chat.bot` scope via
Workload Identity.
