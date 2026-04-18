# Integrating a chat workspace

This guide walks an **operator** — not a developer — through onboarding a chat
workspace (Slack team, Discord guild, Google Chat space, WhatsApp number,
iMessage handle) to a tenant on a running sclawion deployment. Adding a new
*platform* (code-level connector) is a separate doc:
[`CONNECTORS.md`](CONNECTORS.md).

The flow has three parties:

```
       chat platform                  sclawion admin-api          GCP
   ┌──────────────────┐         ┌──────────────────────┐   ┌──────────────┐
   │ Slack/Discord/…  │ webhook │ /v1/{tid}/connectors │   │ Secret Mgr   │
   │  signing secret  ├────────▶│   (Firestore writes) ├──▶│  (versions)  │
   │  bot/OAuth token │         │   /secrets:rotate    │   │              │
   └──────────────────┘         └──────────────────────┘   └──────────────┘
```

## Prerequisites

- You have an Owner role in the admin UI (`admin_users/{your-email}.role = "owner"`).
  If you don't, ask whoever ran `terraform apply` — they seeded the bootstrap owner.
- A tenant exists, or you're about to create one. A tenant is a logical
  workspace; one GCP project can host many.
- The Secret Manager **resource** for the connector's signing/OAuth slot
  already exists. Resource creation = IAM change = Terraform's job; admin-api
  only adds *versions*. If you get a `409: secret resource missing`,
  Terraform hasn't applied yet.

For visual reference, see [`docs/figures/admin-ui/`](figures/admin-ui/) —
the screenshots there were captured from a seeded admin-api with the
same sample tenant (`acme`) used throughout this guide.

## Step 1 — Create the tenant (once per workspace)

UI: `/ui/tenants` → **New tenant** → enter `id` (lowercase DNS-label,
`acme`-style) and a display name. ([screenshot](figures/admin-ui/tenants.png))

API:

```
POST /v1/tenants
Content-Type: application/json
{ "id": "acme", "display_name": "Acme Corp" }
```

The tenant ID flows into every secret name (`sclawion-acme-slack-signing`),
Firestore path (`config_tenants/acme/...`), and audit entry. Pick it once and
don't rename — there is no migration tool today.

## Step 2 — Provision Secret Manager resources (one-time, per platform)

Run from `deploy/terraform/`:

```hcl
# In a tenants.tf or per-tenant module:
resource "google_secret_manager_secret" "acme_slack_signing" {
  secret_id = "sclawion-acme-slack-signing"
  replication { auto {} }
  # CMEK in non-dev
  kms_key_name = google_kms_crypto_key.secrets.id
}
resource "google_secret_manager_secret" "acme_slack_oauth" {
  secret_id = "sclawion-acme-slack-oauth"
  replication { auto {} }
  kms_key_name = google_kms_crypto_key.secrets.id
}
```

`terraform apply`. This creates empty secret containers — no versions yet.

## Step 3 — Get the platform-side credentials

| Platform   | Signing secret                                         | OAuth / bot token                  |
|------------|--------------------------------------------------------|------------------------------------|
| Slack      | App config → **Basic Information** → "Signing Secret"  | App config → **OAuth & Permissions** → bot token (`xoxb-…`) |
| Discord    | App config → **General Information** → "Public Key"    | Bot config → "Token" (`MTk4Nj…`)   |
| Google Chat| _no shared secret — IAP/OIDC_                          | Workload Identity binding (no value to paste) |
| WhatsApp   | Meta App dashboard → **Webhooks** → "App Secret"       | System User access token           |
| iMessage   | Sendblue dashboard → **Webhooks** → "Signing Secret"   | Sendblue **API Key**               |

Copy the values to a private buffer (1Password, scratch tab — **not Slack/email**).
Plan to paste them only into the admin UI's password field.

## Step 4 — Create the connector config

UI: `/ui/tenants/acme/connectors` → **Upsert connector**
([screenshot](figures/admin-ui/connectors-acme.png)):

- Platform: `slack`
- Allowed channels (optional): comma-separated channel IDs the bot may post to
- Rate per conversation: `60` is a safe default
- Replay cache: leave on

API:

```
PUT /v1/tenants/acme/connectors/slack
{
  "webhook_path": "/v1/slack",
  "allowed_channels": ["C0123ABC"],
  "rate_limit_per_conv": 60,
  "replay_cache_enabled": true
}
```

This writes the Firestore doc but does **not** touch any secret. The two
SecretRef fields stay zeroed until you rotate.

## Step 5 — Push the signing secret

UI: `/ui/tenants/acme/connectors` → **Rotate secret** → kind = `signing_secret`,
paste the value, reason = "initial onboarding".

API:

```
POST /v1/tenants/acme/connectors/slack/secrets:rotate
{ "kind": "signing_secret", "value": "8f3c…", "reason": "initial onboarding" }
```

Response:

```
200 OK
{ "secret_ref": {"name":"projects/.../sclawion-acme-slack-signing","version":"1"}, "rotated_at":"…" }
```

Repeat with `kind: "oauth_token"` for the bot/OAuth credential. The audit log
shows `connector.rotate_secret` with the new version number; **the value is
never echoed back, never written to logs, never persisted anywhere outside
Secret Manager**. (You can verify in the audit table: search `acme` and grep
the `after` column for the secret — it won't be there.)

## Step 6 — Wire the webhook on the platform side

The webhook URL is the public LB hostname for `cmd/ingress` plus the
connector's `webhook_path`. For example: `https://chat.sclawion.example.com/v1/slack`.

Per platform:

- **Slack**: App config → **Event Subscriptions** → enable, paste URL, subscribe to `message.channels` / `message.im`. Slack will challenge with a `url_verification` event; ingress responds automatically.
- **Discord**: App config → **Interactions Endpoint URL** → paste, save. Discord sends a PING; ingress replies PONG.
- **Google Chat**: Workspace admin → **Apps & Integrations** → add app, paste URL. No shared secret; auth is the IAP-issued OIDC token verified against the project audience.
- **WhatsApp**: Meta App → **Webhooks** → Callback URL + Verify Token. The token is a one-time challenge (Meta-specific); ingress checks the `hub.verify_token` query param.
- **iMessage** (Sendblue): Dashboard → **Webhooks** → URL + signing secret matches Step 5. Test with the dashboard's "Send test webhook" button.

## Step 7 — Smoke-test

Send a message from a real account in the connected workspace. You should see:

1. `cmd/ingress` log: `verified, published id=...` for that platform.
2. Audit log entry (eventually): not directly, since reads aren't audited; but
   any agent reply triggers the outbound emitter and appears in chat.
3. Pub/Sub metric: `sclawion.inbound` publish count ticks up.

If nothing arrives, in order:

1. Check the platform's webhook delivery log (Slack: **Event Subscriptions** → "Recent events"; Discord: dev portal → **Endpoint** errors).
2. Ingress logs for `signature mismatch` → wrong signing secret. Re-check Step 5; rotate again if needed (each rotate increments version).
3. Ingress logs for `timestamp skew` → server clock drift > 5 min.
4. Cloud Armor / IAP block: webhook URLs must bypass IAP (only the admin LB has IAP enabled; the ingress LB does not).

## Rotations

Quarterly or on suspicion of leak:

```
POST /v1/tenants/acme/connectors/slack/secrets:rotate
{ "kind": "signing_secret", "value": "<new value from platform>", "reason": "Q2 rotation" }
```

The data plane reads `SigningSecretRef.Version` on every request, so rotation
takes effect on the next inbound webhook with no service restart. The previous
version is *not* deleted automatically — Secret Manager keeps history; disable
the old version after the platform is confirmed using the new one.

## Disabling a connector / tenant

- Pause inbound: PATCH `connectors/{platform}` with `rate_limit_per_conv: 0` —
  ingress will 429 every request.
- Remove from rotation: PUT `admin-users/{email}` with `role: "viewer"` to
  prevent further mutations during incident response.
- Soft-delete the tenant: `DELETE /v1/tenants/{tid}` flips `Disabled=true`;
  ingress refuses to publish, audit history stays intact.

## What this guide does *not* cover

- Adding a *new platform* (code path): see [`CONNECTORS.md`](CONNECTORS.md).
- Defining the agent that responds: see [`AGENT-CONTEXT.md`](AGENT-CONTEXT.md).
- Setting up the LB / IAP / Cloud Armor: that's a one-time Terraform apply,
  not per-tenant.
