# Operations

How to deploy, watch, and recover `sclawion`. Intended audience: SREs and
on-call engineers. Assumes the architecture in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Environments

| Env   | Purpose                              | Project                 | Region(s)             | Data sensitivity |
|-------|--------------------------------------|-------------------------|-----------------------|------------------|
| dev   | Engineer sandbox                     | `sclawion-dev-<user>`   | us-central1           | Synthetic only   |
| stage | Pre-prod, shadow real traffic        | `sclawion-stage`        | us-central1           | Anonymized       |
| prod  | Live customer/internal traffic       | `sclawion-prod`         | us-central1, us-east4 | Real             |

Two regions in prod for active/passive failover; only one is hot at a time.
Cutover via DNS weight change in Cloud DNS.

## Deployment topology

Per environment:

- 1× External HTTPS Load Balancer + Cloud Armor policy
- 7× Cloud Run services: `ingress`, `router`, `scion-bridge`, and
  `emitter-{slack,discord,gchat,whatsapp}`
- 4× Pub/Sub topics: `<env>.inbound`, `<env>.outbound`, `<env>.inbound.dead`,
  `<env>.outbound.dead`
- 5× Pub/Sub push subscriptions (one to router, four to emitters with
  per-platform attribute filters)
- 1× Firestore database (Native mode, multi-region in prod)
- 1× KMS keyring with rotating crypto keys for Pub/Sub, Firestore, Artifact Registry
- 1× Artifact Registry repo with vulnerability scanning + Binary Auth attestor
- 1× BigQuery dataset for audit log sink

All managed via Terraform in [`deploy/terraform/`](../deploy/terraform/).

## Releases

Trunk-based. `main` is always deployable. Each PR triggers:

1. `go vet`, `go test`, `govulncheck`, `staticcheck`
2. Build per-service images, tag with `git-sha`, push to Artifact Registry
3. Sign images with Binary Auth attestor
4. `terraform plan` against `dev`

Merges to `main`:

1. Promote images to `latest-stage` tag
2. `terraform apply` against `stage`
3. Run smoke suite: synthetic webhook to each platform endpoint → assert
   round-trip in <10 s
4. Wait 15 minutes; if SLO holds, manually trigger `prod` promotion

`prod` deploys are manual (`gh workflow run promote-prod.yml`) and require
two reviewers on the corresponding GitHub deployment approval.

## Service-level objectives

| Objective                                     | Target                  | Measurement                              |
|-----------------------------------------------|-------------------------|------------------------------------------|
| Webhook ack latency (ingress p99)             | <500 ms                 | Cloud Monitoring on `run.googleapis.com/request_latencies` |
| End-to-end inbound→agent dispatch (p99)       | <2 s                    | OTEL span: `ingress.handle` → `scion.dispatch` |
| Outbound delivery (agent.reply → channel post)| <3 s p99                | OTEL span: `bridge.publish` → `emitter.post` |
| Availability (ingress 2xx/5xx)                | ≥99.5%                  | Cloud Monitoring SLO; 30-day window      |
| DLQ depth                                     | =0 sustained            | Pub/Sub `subscription/dead_letter_message_count` |

Error budget burn alerts (1h, 6h windows) page on-call via Pub/Sub → PagerDuty.

## Runbooks

### Inbound DLQ depth > 0

**Symptom:** Pub/Sub alert "inbound DLQ has messages."

1. Read messages from `<env>.inbound.dead` (do not ack):
   ```bash
   gcloud pubsub subscriptions pull <env>.inbound.dead.inspect --limit=10 --format=json
   ```
2. Look at `event.id`, `platform`, error message in attributes.
3. Most common causes:
   - Router 5xx from Scion Hub down → check Scion status, replay messages
     once Hub recovers (`gcloud pubsub topics publish` from inspector).
   - Schema mismatch from a connector PR → roll back the connector image.
   - Firestore quota exceeded → check Firestore quota dashboard, request increase.
4. After fix, replay DLQ:
   ```bash
   ./scripts/replay-dlq.sh <env>.inbound.dead <env>.inbound
   ```

### Outbound replies not appearing in chat

**Symptom:** User reports "the bot isn't responding."

1. Get `event.id` of the user's message — Cloud Logging:
   ```
   resource.type="cloud_run_revision"
   resource.labels.service_name="sclawion-ingress"
   jsonPayload.platform="slack"
   jsonPayload.channel_id="<channel>"
   ```
2. Trace the event in Cloud Trace by `trace_id` — see where it stops.
3. Common stopping points:
   - **Stuck in inbound topic** → router cold-start or scaled to zero;
     check `unacked_message_count`. If sustained, increase `min_instances=1`
     on router temporarily.
   - **Router 200 but no Scion call** → idempotency hit (event already
     processed); check Firestore `processed_events/<event.id>`. May indicate
     duplicate webhook delivery, which is correct behavior.
   - **Scion dispatched but no outbound event** → check agent logs
     (`scion logs <agent-id>`); agent may have crashed before emitting.
   - **Outbound published but emitter rejected** → check emitter Cloud Run
     logs for `401`/`403` from chat platform → likely OAuth token expired,
     trigger rotation.

### OAuth token rotation

**Symptom:** Emitter 401s from chat platform.

1. Generate a new token in the platform's admin console.
2. Push to Secret Manager as a new version:
   ```bash
   echo -n "$NEW_TOKEN" | gcloud secrets versions add slack-bot-token --data-file=-
   ```
3. The rotation Cloud Function publishes `secrets.rotated`.
4. Emitters refresh on next cold start, or you can force:
   ```bash
   gcloud run services update sclawion-emitter-slack --update-env-vars=ROTATION_NONCE=$(date +%s)
   ```
5. Disable the old version after 24 hours:
   ```bash
   gcloud secrets versions disable <old-version> --secret=slack-bot-token
   ```

### Cloud Run revision rollback

**Symptom:** A new revision is producing 5xx > 1%.

```bash
# list recent revisions
gcloud run revisions list --service=sclawion-router --region=us-central1

# route 100% to previous revision
gcloud run services update-traffic sclawion-router --to-revisions=<prev-rev>=100 --region=us-central1
```

Cloud Run auto-rollback (when SLO breach is detected) is enabled in stage and
prod, so this is usually a confirmation step rather than a manual one.

### Region failover (prod)

**Symptom:** Primary region (us-central1) cannot serve.

1. Verify Pub/Sub, Cloud Run, and Firestore status in us-central1 (Cloud Status).
2. Update Cloud DNS weighted record to send 100% to us-east4:
   ```bash
   gcloud dns record-sets update sclawion-prod.example.com. \
     --zone=prod --type=CNAME --ttl=60 \
     --rrdatas=ingress-us-east4.run.app.
   ```
3. Promote us-east4 Firestore replica if needed (Native mode multi-region
   handles this automatically; Datastore mode requires manual failover).
4. Notify chat platform admins — webhook URLs do not change because they
   point at the LB, not at a region.

### Replay-protection cache outage

**Symptom:** Firestore latency spike → ingress p99 > 500 ms.

The replay cache is best-effort defense in depth (signature + timestamp window
already make replay hard). If Firestore is slow, you can temporarily skip the
nonce write:

```bash
gcloud run services update sclawion-ingress --update-env-vars=SKIP_REPLAY_CACHE=true
```

This is degraded mode — set a timer to revert within 4 hours and document in
the incident channel.

## Cost notes

Cloud Run scale-to-zero plus Pub/Sub's per-message pricing means baseline
cost in an idle environment is dominated by Firestore (a few dollars/month
per env) and Cloud Logging retention. A rough back-of-envelope at typical
ChatOps volume (10k messages/day):

- Pub/Sub: ~$0.40/month per topic
- Cloud Run: ~$3/month per service (assuming most requests <100 ms)
- Firestore: ~$5/month
- KMS: $0.06 per key per month + $0.03 per 10k operations
- Artifact Registry + Cloud Build: ~$5/month
- BigQuery audit sink: ~$5/month at default retention

Total ≈ $50/month per environment at low volume; scales linearly with
message rate.

## Capacity planning

| Component         | Bottleneck                                | Headroom signal                         |
|-------------------|-------------------------------------------|-----------------------------------------|
| Cloud Run         | Concurrency × max-instances               | Adjust `containerConcurrency` first     |
| Pub/Sub           | Message rate (1M msg/sec hard limit)      | Per-topic rate dashboard                |
| Firestore         | Writes/sec per document (~1)              | Sharded `processed_events/` if needed   |
| Scion Hub         | Agents per node                           | Scion's own dashboards                  |

If you're scaling past 1k webhooks/sec, the first thing to revisit is
Firestore's write hot-spot on `processed_events/`; distribute the key
prefix or move to Spanner.
