# GCP-native patterns

Why the architecture leans on specific GCP primitives instead of generic
cloud building blocks. If you're considering porting `sclawion` to AWS or
Azure, this document is the loss column.

## TL;DR

| Pattern                                          | What it gives us                                                |
|--------------------------------------------------|-----------------------------------------------------------------|
| Pub/Sub push + OIDC to Cloud Run                 | No client SDK, no workers, scale-to-zero, free retry/DLQ        |
| Firestore strong consistency on single docs      | Race-free idempotency without distributed locks                 |
| Workload Identity (Federation) end-to-end        | Zero JSON service-account keys to manage                        |
| Cloud KMS CMEK on Pub/Sub + Firestore            | Per-env crypto isolation, separate from app IAM                 |
| Cloud Armor adaptive protection at the edge      | OWASP + L7 rate limit on the only public surface                |
| Binary Authorization on Cloud Run                | Only attested images run in prod                                |
| Cloud Trace auto-correlation                     | Single span tree from chat → ingress → router → Scion → emitter |
| BigQuery audit log sink                          | SQL-native security analytics with multi-year retention         |
| Eventarc as a future ingress alternative         | Architectural exit ramp built in                                |

## Pub/Sub push + OIDC to Cloud Run

The receivers (`router`, emitters) do **not** import the Pub/Sub client
library. They're plain HTTP servers that:

1. Validate an Authorization header (OIDC ID token, RS256, audience = service URL).
2. Read the JSON message body.
3. Return 2xx to ack, 5xx to nack-with-retry.

This means:

- No client SDK in the hot path; the binary stays small.
- Cloud Run scales to zero between events; first request after idle pays cold
  start (typically <500 ms in Gen2), but Pub/Sub's slow-start algorithm
  ramps up gracefully.
- Per-subscription retry policy, exponential backoff, and DLQ are
  configuration in Terraform — no error-handling code in the receivers.

The cost: a small per-message overhead vs. pull-based subscriptions. At our
scale (10k–100k msg/day) it's invisible.

## Firestore for idempotency without locks

The router's idempotency check is one Firestore call:

```go
ref := fs.Collection("processed_events").Doc(event.ID)
_, err := ref.Create(ctx, map[string]any{"at": time.Now()})
if status.Code(err) == codes.AlreadyExists {
    return // duplicate; ack and return 200
}
```

Firestore guarantees strong consistency on single-document writes. No
distributed lock, no consensus protocol, no Redis. TTL on the document
(30 days) cleans up automatically.

The same pattern handles the replay-protection nonce cache (`nonces/{hash}`,
10-min TTL) and conversation correlation (`correlation/{conv_id}`).

## Workload Identity end-to-end

Three dimensions of identity, no JSON keys at any layer:

1. **CI → GCP**: GitHub Actions uses Workload Identity Federation; the OIDC
   token from `id-token: write` is exchanged for a short-lived GCP access
   token to push images and run `terraform apply`.
2. **Cloud Run → GCP services**: each Cloud Run service has a dedicated GSA;
   the metadata server provides short-lived tokens. No `GOOGLE_APPLICATION_CREDENTIALS`.
3. **Pub/Sub → Cloud Run**: push subscriptions sign an OIDC ID token; the
   receiver validates it (audience = its own URL).

Net result: a `git grep -i 'service.*account.*key'` over the repo returns
zero hits, and the `iam.serviceAccountKey.create` permission can be denied
at the org level.

## CMEK with separate keyrings per env

`projects/<proj>/locations/us-central1/keyRings/sclawion-prod-kr/cryptoKeys/pubsub`

Per-env keyring means:

- Key admin role can be held by a different team than the application
  developers; revoking key access disables the env without touching IAM
  roles on the data resources.
- Key rotation (90-day default) happens transparently without re-encrypting
  data.
- Key destruction is the kill switch for an environment's data.

We attach CMEK to: Pub/Sub topics, Firestore database, Artifact Registry
images. Secret Manager and Cloud Logging support CMEK too; enable per
compliance need.

## Cloud Armor at the edge

The only public surface is the External HTTPS LB in front of `ingress`.
Cloud Armor sits inline:

- **OWASP Core Rule Set** preset blocks the SQLi/XSS/LFI baseline.
- **Per-source-IP rate limit** (default 600 rpm) bounds spray attacks.
- **Adaptive Protection** (ML-based anomaly detection) on by default in
  stage/prod.
- **Body-size cap** (1 MiB) — chat messages are tiny; no reason to accept
  large bodies on this surface.
- **Geo-block** off by default but available as one Terraform line.

We don't use Cloud Armor for authentication — that's the connector's job —
but it absorbs the volumetric and pattern-match noise so signature
verification only sees plausibly-real traffic.

## Binary Authorization on Cloud Run

Only images signed by the `sclawion-builder` attestor run in stage/prod.
The signature is produced by Cloud Build after:

1. `go vet`, `go test`, `staticcheck`, `govulncheck` all pass.
2. Container image vulnerability scan reports no Critical CVEs.
3. SLSA provenance is generated.

A developer can't push a hand-built image to prod even with full IAM access;
the deploy will be denied at admission. Combined with mandatory PR review,
this prevents "I just need to ssh in and patch this real quick" outages.

## Cloud Trace as the cross-system trace plane

Every `Envelope` carries `trace.trace_id` and `trace.span_id`. Every service
exports OTEL spans to Cloud Trace via the GCP exporter. The result: a single
span tree that begins with the inbound webhook and ends with the outbound
chat post, including the agent's runtime in Scion as a child span.

When a user says "the bot didn't respond to my 14:32 message," on-call runs:

```
trace_id matches Cloud Logging entry for that ingress request
```

and gets the entire pipeline in one view. No ELK correlation, no jq across
five log streams.

## BigQuery audit log sink

Cloud Audit Logs (Admin Activity, Data Read, Data Write) are sinked to
`<project>:sclawion_audit_<env>` with 400-day retention. This becomes:

- Looker Studio dashboards (auth failure rate, DLQ depth, per-conversation
  outbound volume).
- Ad-hoc SQL for incident investigations ("show me every Secret Manager
  read by the slack-emitter SA in the last 24 hours").
- Compliance evidence for SOC 2 / ISO 27001 reviewers.

The same dataset can ingest application logs via a separate sink, giving you
SQL over both audit and app logs in one place.

## Eventarc as an exit ramp

Eventarc can route events from over 130 GCP sources directly to Cloud Run.
If/when it adds first-class support for Slack / Discord webhooks, we could
replace the `ingress` service entirely:

```
chat platform → Eventarc → Pub/Sub.inbound → router (unchanged)
```

The router doesn't care where messages come from as long as they're
`Envelope`-shaped. Designing the schema as the bus contract (rather than
HTTP) keeps this option open.

## What you'd lose porting to AWS

- **Pub/Sub push + OIDC to Cloud Run** → SNS → SQS → Lambda. You get a
  compatible shape but at the cost of two services and SQS visibility-timeout
  tuning instead of native push backoff.
- **Firestore strong-consistency single-doc writes** → DynamoDB conditional
  writes work, but you give up multi-region active/active without DAX.
- **Cloud Trace auto-correlation** → X-Ray is fine but doesn't have the same
  cross-product propagation guarantees out of the box.
- **Workload Identity Federation from GitHub** → AWS supports OIDC from
  GitHub too, but role assumption is per-account vs per-org.
- **Binary Authorization** → ECR signing + admission-controller Lambda or
  Kyverno on EKS; doable but more glue.

## What we'd add if we picked GKE

- mTLS service mesh (Anthos Service Mesh / Istio).
- Pod Security Standards admission.
- Per-namespace network policies.
- Better fit for very high throughput (>10k msg/sec sustained).

Currently Cloud Run handles the load and the operational simplicity wins.
