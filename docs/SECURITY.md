# Security

This document is the threat model and control inventory for `sclawion`.
It targets a reviewer who's been asked "is it safe to install this in our
GCP org and connect it to corporate Slack?" and needs a defensible answer.

## Threat model

Assets we're protecting:

- **Conversation contents.** Chat messages and agent outputs may contain
  source code, customer data, credentials, or strategic plans.
- **Bot credentials.** OAuth tokens, signing secrets, app secrets — gateway to
  every conversation in connected workspaces.
- **Scion control.** Ability to dispatch arbitrary agents = ability to run
  arbitrary code in your project's container runtime.
- **Audit integrity.** Tamper-proof record of who asked what, when, and what
  the agent did.

Adversaries we account for:

| # | Adversary                          | Capability                                              |
|---|------------------------------------|---------------------------------------------------------|
| 1 | Public internet attacker           | Can hit the ingress LB; no creds                        |
| 2 | Compromised webhook secret         | Can forge signed requests for one platform              |
| 3 | Insider with project read access   | Can read logs, list resources, but not act              |
| 4 | Compromised Cloud Run revision     | Has the GSA's IAM; can reach what GSA can reach         |
| 5 | Compromised CI/CD                  | Can push images; Binary Authorization gates production  |
| 6 | Malicious agent output             | Agent inside Scion emits crafted events to confuse bridge |

We do **not** defend against an adversary with `roles/owner` on the project
or KMS key admin. Those are operational concerns, not application controls.

## Controls (mapped to threats)

### Edge & ingress (threat 1)

- **External HTTPS Load Balancer + Cloud Armor** in front of `ingress`. Cloud
  Armor policy includes:
  - OWASP CRS preset (SQL injection, XSS, LFI heuristics)
  - Per-source-IP rate limit (default 600 req/min, tunable per platform)
  - Geo-block list (configurable; off by default)
  - Body-size cap (1 MiB; chat messages are tiny)
- **TLS 1.3 only**, modern cipher suite, HSTS headers.
- **Cloud Armor adaptive protection** enabled in stage/prod.

### Webhook authentication (threat 2)

- Per-platform signature verification (see [`docs/CONNECTORS.md`](CONNECTORS.md)).
- All HMAC comparisons use `crypto/subtle.ConstantTimeCompare`.
- Discord uses Ed25519 — no shared secret to leak from a chat platform side.
- Google Chat verifies an RS256 JWT against Google's JWKS (rotated by Google).
- ±5-minute timestamp window enforced (`auth.MaxSkew`).
- Firestore-backed nonce cache catches in-window replays for 10 minutes.
- A failed verify produces an audit log entry and a `401`; **no Pub/Sub publish**.

### Identity & access (threats 3, 4)

- One Google Service Account per Cloud Run service:
  - `sclawion-ingress@…` — Pub/Sub publish on inbound, Secret accessor on
    signing secrets, Firestore RW on `nonces/`.
  - `sclawion-router@…` — Pub/Sub subscribe on inbound, Firestore RW on
    `correlation/` and `processed_events/`, HTTP egress to Scion Hub URL.
  - `sclawion-scion-bridge@…` — Pub/Sub publish on outbound, HTTP/WS to
    Scion Hub.
  - `sclawion-emitter-<platform>@…` — Pub/Sub subscribe on outbound (filtered),
    Secret accessor on that platform's OAuth token, Firestore R on `correlation/`.
- **No JSON service-account keys.** Anywhere. Service-to-service auth is
  Workload Identity (Cloud Run → Pub/Sub) or OIDC ID tokens (Pub/Sub push →
  Cloud Run).
- Pub/Sub push subscriptions configured with `oidcToken.serviceAccountEmail`;
  receivers verify the token's `aud` claim equals the receiver's URL.
- Firestore access uses the GSA's identity, not a service account key.

### Secrets management

- Google Secret Manager for all credentials. Inventory is centralized in
  `pkg/secrets/manager.go` — every name is a `const`.
- Rotation: a scheduled Cloud Function rotates keys (per platform's policy)
  and publishes a `secrets.rotated` event. Emitters subscribe and refresh
  cached values.
- Secret Manager access is per-secret IAM; no service can read a secret it
  doesn't own.
- **Never** read credentials from `os.Getenv`. Reviewers should grep for
  `os.Getenv(.*TOKEN|SECRET|KEY)` in PRs.

### Encryption

| Resource          | At rest                          | In transit                  |
|-------------------|----------------------------------|-----------------------------|
| Pub/Sub topics    | CMEK (Cloud KMS, 90-day rotation)| TLS 1.3                     |
| Firestore         | CMEK                             | TLS 1.3                     |
| Secret Manager    | Google-managed (CMEK optional)   | TLS 1.3                     |
| Artifact Registry | CMEK on images                   | TLS 1.3                     |
| Inter-service     | n/a                              | TLS 1.3 + OIDC / mTLS option|

KMS keys are in a separate keyring per environment (`sclawion-{dev,stage,prod}-kr`).
Key admins do not have application access; application service accounts have
`roles/cloudkms.cryptoKeyEncrypterDecrypter` only on the keys they need.

### Network controls

- **Private Cloud Run** for `router`, `scion-bridge`, and emitters
  (`run.googleapis.com/ingress: internal`). Only `ingress` is reachable from
  the public LB.
- **VPC Service Controls perimeter** around Pub/Sub, Firestore, Secret Manager,
  Artifact Registry, Cloud KMS. Egress to Scion Hub goes through a Private
  Service Connect endpoint when the Hub is hosted in another project.
- **Serverless VPC connector** if reaching on-prem Scion Hub via VPN/Interconnect.

### Supply chain (threat 5)

- Builds run in Cloud Build with Workload Identity Federation from GitHub
  Actions; no long-lived service-account keys in CI.
- Images pushed to Artifact Registry with vulnerability scanning enabled.
- **Binary Authorization** on Cloud Run requires images to carry an
  attestation from the build attestor. Unsigned images cannot deploy to prod.
- SLSA provenance attached to every build; image labels record git SHA, build
  ID, and commit author.
- `go.sum` and `go.mod` enforced; CI runs `govulncheck` on every PR.

### Agent output handling (threat 6)

- The bridge treats every line of agent stdout as untrusted input.
- Only lines beginning with `<<sclawion-event>>` are parsed; everything else
  is logged and dropped.
- Parsed event JSON is validated against the `Envelope` schema; oversized,
  malformed, or wrong-`spec_version` events are dropped with an audit log.
- Agent text is **never** rendered as markdown by emitters that interpret
  links automatically without user intent — Slack `mrkdwn` is escaped,
  Discord embeds are constructed server-side, WhatsApp uses plain text.
- Outbound rate-limit per `conversation_id` (default 30 messages/min) prevents
  a runaway agent from spamming a channel.

### Audit & detection

- Cloud Audit Logs (Admin Activity, Data Read, Data Write) sinked to a BigQuery
  dataset `sclawion_audit_${env}` with 400-day retention.
- Application logs include:
  - `event.id`, `conversation_id`, `platform`, `kind` on every entry.
  - OTEL `trace_id` propagated end-to-end.
- Looker Studio dashboard surfaces:
  - Auth-failure rate per platform (alerting threshold: >1% over 5 min)
  - DLQ depth on each topic (alerting threshold: >0 for 5 min)
  - Outbound rate per conversation (anomaly detection)
- Cloud Logging retention: 30 days hot in `_Default`, 400 days in BigQuery sink.

## Compliance posture

`sclawion` does not itself certify against any specific framework, but the
defaults map to common controls:

| Framework       | Coverage |
|-----------------|----------|
| SOC 2 Type II   | Logical access (IAM), encryption (CMEK), monitoring (Audit Logs sink), change management (Binary Auth) |
| ISO 27001       | A.9 access control, A.10 cryptography, A.12 ops security, A.14 secure dev |
| GDPR / CCPA     | No PII stored beyond conversation correlation; chat content lives in chat platform; per-region deployment supported via Cloud Run regional services |
| HIPAA           | Possible with a BAA covering Cloud Run, Pub/Sub, Firestore, Secret Manager, KMS; do not enable WhatsApp until you've verified Meta's BAA stance for your use case |
| FedRAMP         | Not currently in scope; deploy in Assured Workloads if needed |

## Disclosure

Security issues should be reported privately. Open a draft GitHub Security
Advisory on the repo, or email the maintainer listed in `CODEOWNERS`. Please
include a reproducer; we aim to acknowledge within 72 hours and fix critical
issues within 14 days.

## Threat-model deltas to track

These are open security questions to revisit before promoting any environment
to handle production traffic:

- [ ] Per-tenant key isolation (CMEK per tenant vs per env).
- [ ] Customer-controlled key wrap for chat content (E2EE option).
- [ ] Workload Identity Federation for inbound webhooks (replaces shared
  signing secrets where the platform supports it; Slack and WhatsApp do not).
- [ ] Confidential Computing on Cloud Run for sensitive workloads.
