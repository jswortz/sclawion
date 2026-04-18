# End-to-end demo: one swarm, six chat surfaces, one customer network

This walkthrough deploys a CLAWPATH instance to GCP, registers all six
chat platforms, and runs a real conversation with a three-agent swarm
that reaches into a (toy) customer network over a SCION path.

Time budget: ~90 minutes if you have all credentials ready.

## Scenario

> **You** (in any of six chat apps): *"@release-bot ship the hotfix
> from PR-1234 to acme-corp's staging."*
>
> **Swarm** (over the next ~3 min): planner decomposes; coder reads PR;
> reviewer checks the change against acme-corp's CI rules; deployer
> calls acme-corp's GitLab API over SCION to merge and trigger their
> deploy; monitor posts updates back into the originating chat thread
> in real time.

## Prerequisites

| What | Why | Where |
|------|-----|-------|
| GCP project, billing enabled, owner role | All deployments | console.cloud.google.com |
| `gcloud` CLI logged in | terraform / scripts | `gcloud auth login` |
| `terraform` ≥ 1.6 | infra | terraform.io/downloads |
| `go` ≥ 1.22 | build images | go.dev/dl |
| Docker | container builds | docker.com |
| A SCIONLab account | demo SCION ISD | scionlab.org/users/registration |
| A "customer-side" GCP project or VM | SIG endpoint | second project recommended |
| Slack workspace (admin) | Slack app | api.slack.com/apps |
| Discord server (admin) | Discord app | discord.com/developers/applications |
| Google Workspace tenant | Chat app | console.cloud.google.com → Chat API |
| Meta Business Account + verified WhatsApp number | WhatsApp Cloud API | business.facebook.com |
| Microsoft 365 tenant + bot framework registration | Teams bot | dev.botframework.com |
| Signal-cli operator host | Signal daemon | github.com/AsamK/signal-cli |

> **Signal caveat.** Signal has no official bot API. The demo uses
> [signal-cli](https://github.com/AsamK/signal-cli) as a "linked
> device" — it logs into a real Signal account and exposes a local
> JSON-RPC. This works but means: (a) you need a real phone number,
> (b) it's a single point of failure, and (c) Signal can deauthorize
> the linked device unilaterally. Treat as preview-quality. Production
> users typically restrict Signal to internal staff who all trust the
> daemon operator.

## Step 1 — apply the Terraform

```bash
git clone https://github.com/jswortz/sclawion
cd sclawion/deploy/terraform/clawpath  # added in M5

terraform init
terraform apply \
  -var project_id="$GCP_PROJECT" \
  -var env=demo \
  -var enable_teams=true \
  -var enable_signal=true \
  -var scion_isd=64    # SCIONLab global testbed
```

Provisions:

- 4 Pub/Sub topics: `demo.inbound`, `demo.outbound`, `demo.swarm.tasks`,
  `demo.swarm.results`
- 11 Cloud Run services: `ingress`, `router`, `swarm-dispatcher`,
  `scion-bridge`, `sig`, `emitter-{slack,discord,gchat,whatsapp,teams,signal}`
- KMS keyring `clawpath-demo-kr`
- Firestore database
- Cloud Armor policy on the ingress LB
- Secret Manager placeholders for every platform's credentials

Output:

```
ingress_url  = "https://ingress-xxx-uc.a.run.app"
sig_endpoint = "10.20.0.5:30041"  # internal IP for SCION dispatcher
```

## Step 2 — push secrets

```bash
echo -n "$SLACK_SIGNING_SECRET"   | gcloud secrets versions add slack-signing-secret   --data-file=-
echo -n "$SLACK_BOT_TOKEN"        | gcloud secrets versions add slack-bot-token        --data-file=-
echo -n "$DISCORD_PUBLIC_KEY"     | gcloud secrets versions add discord-public-key     --data-file=-
echo -n "$DISCORD_BOT_TOKEN"      | gcloud secrets versions add discord-bot-token      --data-file=-
echo -n "$WHATSAPP_APP_SECRET"    | gcloud secrets versions add whatsapp-app-secret    --data-file=-
echo -n "$WHATSAPP_ACCESS_TOKEN"  | gcloud secrets versions add whatsapp-access-token  --data-file=-
echo -n "$TEAMS_APP_PASSWORD"     | gcloud secrets versions add teams-app-password     --data-file=-
echo -n "$TEAMS_APP_ID"           | gcloud secrets versions add teams-app-id           --data-file=-
echo -n "$SIGNAL_LINK_URI"        | gcloud secrets versions add signal-link-uri        --data-file=-
gcloud secrets versions add gchat-service-account --data-file=path/to/sa.json   # only one that's a JSON, by Google's design
```

## Step 3 — wire each chat platform

### Slack

1. Create a Slack app at api.slack.com/apps.
2. Bot scopes: `app_mentions:read`, `chat:write`, `channels:history`,
   `im:history`.
3. Event Subscriptions → Request URL: `${ingress_url}/v1/slack`.
4. Subscribe to: `app_mention`, `message.im`.
5. Install to workspace; copy bot token + signing secret into Secret
   Manager (step 2).

### Discord

1. discord.com/developers/applications → New Application → Bot.
2. Public Key (under General Information) → secret manager.
3. Interactions Endpoint URL: `${ingress_url}/v1/discord`.
4. OAuth2 → URL Generator → scopes `bot` + `applications.commands`,
   permissions `Send Messages`, `Read Message History`.
5. Invite URL → install in your server.

### Google Chat

1. console.cloud.google.com → APIs & Services → Enable "Google Chat API".
2. Configure → App Name = "release-bot" → Connection settings: HTTP
   endpoint URL = `${ingress_url}/v1/gchat`.
3. Functionality: Receive 1:1 messages, Join spaces and group conversations.
4. Permissions: Specific people and groups in your domain.
5. Install: the user `@release-bot` is now addressable in any space.

### WhatsApp (Meta Cloud API)

1. business.facebook.com → System User → generate access token, push to
   Secret Manager.
2. WhatsApp Manager → Phone Numbers → register a test number.
3. Configuration → Webhook URL = `${ingress_url}/v1/whatsapp`,
   Verify Token = a random string you also put in Secret Manager,
   App Secret = also pushed.
4. Subscribe to: `messages`.
5. Add the test number to your phone's contacts; send the bot a
   message to initiate a 24-hour conversation window.

### Microsoft Teams

1. dev.botframework.com → New Bot → Microsoft App ID + password to
   Secret Manager.
2. Messaging endpoint: `${ingress_url}/v1/teams`.
3. Channels → enable Microsoft Teams.
4. In Azure portal: configure the Bot Service with the same App ID.
5. Sideload the bot's manifest into your Teams tenant (or publish to
   org catalog).

The Teams connector uses Bot Framework JWT validation (Microsoft's
JWKS, audience = bot's App ID). HMAC equivalent of Slack's signing
secret = the App password.

### Signal (preview)

1. On your signal-cli host:
   ```bash
   apt install signal-cli
   signal-cli link -n "release-bot"
   # scan the QR with the Signal app on the operator's phone
   signal-cli daemon --http=127.0.0.1:8080
   ```
2. Configure the `emitter-signal` Cloud Run service to call
   `https://signal.your-domain.example/v1/send` (a small reverse-proxy
   in front of signal-cli). This proxy is the *only* component of the
   demo that isn't Cloud Run native — Signal's protocol requires a
   stateful process.
3. Inbound: signal-cli posts incoming messages to a webhook you point
   at `${ingress_url}/v1/signal`.

This is the gnarliest of the six. It works; it's just operationally
unlike the others.

## Step 4 — set up the customer side (SCION)

### Customer joins a SCION ISD

For the demo, we're using **SCIONLab ISD 64** (the global testbed; free).
In production, the customer joins their preferred ISD (Anapaya
commercial, a national ISD, or stands up their own).

```bash
# On the customer-side VM (Linux, public IP):
curl -sSL https://docs.scionlab.org/install.sh | bash
sudo systemctl enable --now scion-router scion-control scion-dispatcher
# Register the AS in SCIONLab dashboard
# Receive your ISD-AS, e.g. 64-2:0:abcd
```

### Customer runs a SIG

```bash
sudo apt install scion-ip-gateway
cat > /etc/scion/sig.toml <<EOF
[gateway]
local = "64-2:0:abcd,10.0.0.5"
[remotes]
clawpath = "64-2:0:1234"   # our AS
EOF
sudo systemctl restart scion-ip-gateway
```

### CLAWPATH side

The `sig` Cloud Run service in step 1's Terraform already has the
SCION dispatcher and SIG configured. Add the customer's AS to the
allowed peers:

```bash
gcloud run services update sig \
  --update-env-vars=SCION_ALLOWED_PEERS="64-2:0:abcd"
```

Verify path discovery:

```bash
# Run from a Cloud Run debug container in the same VPC:
scion showpaths 64-2:0:abcd
# Expected: ≥2 paths via different ISDs/transit ASes
```

If you see paths, you're done. Agent egress to `10.0.0.0/8` will now
go via SCION.

## Step 5 — register the swarm

```bash
gcloud run jobs execute swarm-register --args="
  --swarm=release-bot
  --roles=planner,coder,reviewer,deployer,monitor
  --customer-isd-as=64-2:0:abcd
  --budget-tokens=500000
  --budget-wallclock=PT30M
  --budget-deploys=3
"
```

This writes a `swarms/release-bot` document to Firestore. The
dispatcher consults it whenever it sees a chat message routed to
`release-bot`.

## Step 6 — talk to it

Open Slack. In any channel where `@release-bot` is invited:

```
@release-bot ship the hotfix from PR-1234 to acme-corp's staging
```

Within ~3 seconds you should see:

> **release-bot**: I'm on it. Spawning a 5-agent swarm for
> acme-corp/staging.

Within ~30 seconds:

> **release-bot**: Planner: PR-1234 changes 3 files in
> `services/payments`. Plan: pull PR, run acme-corp's pre-merge tests,
> if green merge into `staging`, kick acme's deploy pipeline.

Within ~2 minutes:

> **release-bot**: Coder: pulled. Reviewer: pre-merge checks pass
> (lint, unit, integration). Deployer: merging via SCION path
> `64-2:0:1234 → 64-1:0:01 → 64-2:0:abcd`. Pipeline triggered:
> https://gitlab.acme-corp.internal/payments/-/pipelines/9871.

Within ~3 minutes:

> **release-bot**: ✅ Deploy succeeded. Pipeline 9871 green. 1 agent
> wallclock budget remaining: 27m. Done.

Now repeat the same `@release-bot ship the hotfix...` from Discord,
Google Chat, WhatsApp, Teams, and Signal. The swarm doesn't care which
chat you used; the conversation context is preserved per `(platform,
channel, thread)`.

## Step 7 — observe

### Cloud Trace

Open Cloud Trace and filter on `service:ingress`. The trace for one
chat message becomes a span tree like:

```
ingress.handle [slack]                  20ms
└── publish.inbound                      4ms
    └── router.dispatch                  8ms
        └── swarm-dispatcher.spawn      12ms
            └── planner.plan          8400ms
                ├── coder.refactor    9200ms
                │   └── sig.egress    340ms  isd_as=64-2:0:abcd path=p1
                ├── reviewer.review   3100ms
                ├── deployer.merge    2800ms
                │   └── sig.egress    280ms  isd_as=64-2:0:abcd path=p2
                └── monitor.report     90ms
                    └── publish.outbound 3ms
                        └── emitter-slack.post 80ms
```

Each `sig.egress` span carries the SCION path used as a span attribute
— that's the auditable record of "where did that traffic actually go."

### BigQuery

```sql
SELECT
  timestamp,
  jsonPayload.swarm_id,
  jsonPayload.role,
  jsonPayload.scion.isd_as,
  jsonPayload.scion.path_id,
  jsonPayload.action
FROM `clawpath-demo.sclawion_audit_demo.cloudaudit_googleapis_com_data_access`
WHERE
  timestamp > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 1 HOUR)
  AND jsonPayload.scion.isd_as IS NOT NULL
ORDER BY timestamp;
```

### Cloud Monitoring

Default dashboard `clawpath-demo` shows:

- Swarm tasks/min
- p99 latency per role (planner, coder, …)
- SCION path failover events
- Per-customer egress volume (bytes per ISD-AS)

## Cleanup

```bash
cd deploy/terraform/clawpath
terraform destroy -var project_id="$GCP_PROJECT" -var env=demo
# On the customer-side VM:
sudo systemctl stop scion-ip-gateway scion-router scion-control scion-dispatcher
```

Remove the platform integrations through each platform's admin UI.

## What success looks like

- All six platforms route to the same swarm.
- Customer-side calls (deployer's GitLab API call) carry SCION path
  metadata in BigQuery audit.
- A simulated single-ISP outage (block one of the SCION paths in
  iptables on the customer SIG host) does *not* cause a user-visible
  failure — the SIG fails over to the other path within ~1 second.
- DLQ depth on every Pub/Sub topic stays at 0.
- p99 chat-to-first-reply latency under 3 seconds.

## Known rough edges in M5

- Signal connector is brittle; production users should restrict to
  internal channels.
- Teams adaptive-cards rendering uses a simplified subset; rich-card
  payloads fall back to plain text.
- SIG-on-Cloud-Run is the more experimental of the two SIG deployment
  modes (vs SIG-on-GKE-Autopilot). If you hit networking limits on
  Cloud Run, the GKE path is the recommended fallback.
- Path policy is per-tenant only in M5; per-task and per-conversation
  granularity in M6.
