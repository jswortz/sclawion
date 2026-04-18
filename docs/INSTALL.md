# Install and run

End-to-end setup for `sclawion`, from clone to a tenant receiving its first
chat message. Read alongside:

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — what the components are.
- [`OPERATIONS.md`](OPERATIONS.md) — runbooks, SLOs, monitoring.
- [`SECURITY.md`](SECURITY.md) — controls and threat model.

This guide is GCP-only. The data plane uses Cloud Run, Pub/Sub, Firestore,
Secret Manager, KMS, and (in non-dev) Cloud Armor + IAP. There is no
Kubernetes path today.

## 1. Local development

### Prerequisites

```bash
go version            # 1.22 or newer
gcloud --version      # 460+ recommended
gcloud components install pubsub-emulator cloud-firestore-emulator beta
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

### Build everything

```bash
git clone https://github.com/jswortz/sclawion
cd sclawion
go mod download
go build ./...
go test ./...
```

The codebase has no third-party dependencies (verify with
`go list -m all` — only the module itself appears). Anything imported
from outside `pkg/` belongs to the Go standard library.

### Run admin-api locally with seeded sample data

The fastest way to see the control plane is the in-memory seeded mode
used to capture the screenshots in
[`docs/figures/admin-ui/`](figures/admin-ui/README.md):

```bash
# Bypass IAP locally; only honoured when ENV=dev.
ADMIN_API_DEV_IAP_BYPASS_EMAIL=owner@dev.local \
ENV=dev \
go run ./cmd/admin-api
```

Then visit <http://localhost:8088/ui/tenants>. With the in-memory store
the page is empty until you POST a tenant; for a populated demo, run the
sample seeder under `test/integration/`.

### Emulator-driven integration tests

```bash
gcloud beta emulators pubsub start    --host-port=localhost:8085 &
gcloud beta emulators firestore start --host-port=localhost:8086 &
export PUBSUB_EMULATOR_HOST=localhost:8085
export FIRESTORE_EMULATOR_HOST=localhost:8086
go test ./test/integration/...
```

## 2. GCP project bootstrap

Per environment (`dev`, `stage`, `prod`):

```bash
PROJECT=sclawion-dev-$USER
gcloud projects create "$PROJECT"
gcloud beta billing projects link "$PROJECT" --billing-account=XXXXXX-XXXXXX-XXXXXX

gcloud services enable --project="$PROJECT" \
  run.googleapis.com pubsub.googleapis.com firestore.googleapis.com \
  secretmanager.googleapis.com cloudkms.googleapis.com \
  artifactregistry.googleapis.com cloudbuild.googleapis.com \
  iap.googleapis.com bigquery.googleapis.com \
  binaryauthorization.googleapis.com
```

Apply infrastructure:

```bash
cd deploy/terraform
terraform init
terraform apply \
  -var="project_id=$PROJECT" \
  -var="env=dev" \
  -var="admin_owner_email=you@example.com" \
  -var="iap_support_email=you@example.com"
```

Terraform creates: KMS keyring + crypto keys, Pub/Sub topics + DLQs,
Firestore (Native), Artifact Registry, BigQuery audit dataset,
Cloud Run service accounts, IAP brand + OAuth client, the bootstrap
`admin_users/{owner}` Firestore document.

What Terraform does **not** create: Secret Manager *versions* (the
admin-api `/secrets:rotate` endpoint adds those — see
[`CHAT-INTEGRATION.md`](CHAT-INTEGRATION.md)).

## 3. Build and deploy services

```bash
SHA=$(git rev-parse --short HEAD)
REGION=us-central1
REPO=$REGION-docker.pkg.dev/$PROJECT/sclawion

for svc in admin-api ingress router scion-bridge emitter; do
  gcloud builds submit --tag "$REPO/$svc:$SHA" --file deploy/cloudrun/Dockerfile.$svc .
done

# Render Cloud Run manifest with project + sha + audience and apply.
sed -e "s|PROJECT_ID|$PROJECT|g" \
    -e "s|REGION|$REGION|g" \
    -e "s|GIT_SHA|$SHA|g" \
    -e "s|ENV_NAME|dev|g" \
    -e "s|IAP_AUDIENCE|$(terraform -chdir=deploy/terraform output -raw iap_audience)|g" \
    deploy/cloudrun/admin-api.yaml | gcloud run services replace - --region="$REGION"
```

The four data-plane services (`ingress`, `router`, `scion-bridge`,
`emitter-*`) follow the same pattern. The `emitter` binary is one image
deployed four times with different `--platform` flags
([`cmd/emitter/main.go`](../cmd/emitter/main.go)).

> **Status note.** As of `98cea09`, only `cmd/admin-api` is fully
> implemented. The other four services build and respond on `/healthz`
> but return `501 not implemented` for everything else
> ([`pkg/connectors/README.md`](../pkg/connectors/README.md) tracks the
> connector matrix).

## 4. Onboard the first tenant

Once admin-api is reachable through the IAP-fronted LB, follow:

- [`CHAT-INTEGRATION.md`](CHAT-INTEGRATION.md) — connect a chat workspace.
- [`AGENT-CONTEXT.md`](AGENT-CONTEXT.md) — define the agent that responds.

Both guides are operator-facing (no code changes required) and use the
same `acme` sample tenant shown in
[`docs/figures/admin-ui/`](figures/admin-ui/README.md).

## 5. Monitor

Production observability is detailed in
[`OPERATIONS.md`](OPERATIONS.md). The fast path:

| Signal | Where |
|--------|-------|
| Service health | Cloud Run console → per-service revisions + request metrics |
| Auth-failure rate | Cloud Logging filter on `jsonPayload.action="connector.verify"` AND `severity="ERROR"` |
| DLQ depth | Pub/Sub console → `<env>.inbound.dead`, `<env>.outbound.dead` |
| Audit history (UI) | `/ui/audit` — last 200 entries from Firestore mirror |
| Audit history (long-term) | BigQuery `sclawion_audit_${env}.entries` (400-day retention per [`SECURITY.md`](SECURITY.md)) |
| End-to-end traces | Cloud Trace, filtered by `service.name=sclawion-*` |

Default SLOs and burn-rate alerts are listed in
[`OPERATIONS.md` → Service-level objectives](OPERATIONS.md#service-level-objectives).

## 6. Secure

Read [`SECURITY.md`](SECURITY.md) end-to-end before any non-dev
deployment. The five non-negotiables:

1. **CMEK on Pub/Sub, Firestore, Artifact Registry** in `stage` and
   `prod`. The Terraform module enforces this; do not pass
   `kms_key_name = ""`.
2. **No JSON service-account keys.** Service-to-service auth is
   Workload Identity (Cloud Run → GCP APIs) or OIDC ID tokens
   (Pub/Sub push → Cloud Run).
3. **IAP on admin-api** — Cloud Run ingress is set to
   `internal-and-cloud-load-balancing`; the only external entry is
   through the LB with IAP enabled. Bypassing IAP requires `roles/owner`
   on the project.
4. **Quarterly secret rotation.** Use
   `POST /v1/tenants/{tid}/connectors/{platform}/secrets:rotate`; the
   audit trail captures the version bump but never the value.
5. **Binary Authorization** in `prod`. Only images attested by the
   build attestor can deploy. CI signs on every successful merge to
   `main`.

## 7. Upgrade

Trunk-based, see [`OPERATIONS.md` → Releases](OPERATIONS.md#releases).
Schema migrations: bump `spec_version` in
[`pkg/event/envelope.go`](../pkg/event/envelope.go); old envelopes must
remain readable for at least one release per
[`CLAUDE.md` "When you change things"](../CLAUDE.md#when-you-change-things).
