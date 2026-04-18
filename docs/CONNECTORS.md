# Adding a new connector

A connector is the only place platform-specific code lives. To add a new chat
platform end-to-end, you implement three interfaces, register the platform in
two `cmd/` services, and add the deploy plumbing. That's it.

This guide walks through adding a hypothetical "Mattermost" connector.

## Step 0: prerequisites

- Read [`EVENT_SCHEMA.md`](EVENT_SCHEMA.md). Connectors are the boundary
  between platform shapes and the normalized envelope.
- Read [`SECURITY.md`](SECURITY.md) §"Webhook authentication". You're
  responsible for the verifier; the rest of the system trusts you got it right.

## Step 1: implement the three interfaces

Create `pkg/connectors/mattermost/` with three files. The interfaces are
defined in `pkg/connectors/connector.go`:

```go
type Verifier interface {
    Verify(ctx context.Context, r *http.Request, body []byte) error
}
type Decoder interface {
    Decode(ctx context.Context, body []byte) (*event.Envelope, error)
}
type Encoder interface {
    Encode(ctx context.Context, e *event.Envelope) error
}
```

### `verify.go`

Constant-time signature comparison and ±5-minute timestamp skew are
non-negotiable. Use `pkg/auth` helpers — don't roll your own HMAC.

```go
package mattermost

import (
    "context"
    "fmt"
    "net/http"
    "github.com/jswortz/sclawion/pkg/auth"
)

type Verifier struct{ Token []byte }

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
    sig := r.Header.Get("X-Mattermost-Signature")
    ts  := r.Header.Get("X-Mattermost-Timestamp")
    if sig == "" || ts == "" {
        return fmt.Errorf("mattermost: missing signature headers")
    }
    // ...validate timestamp window...
    msg := append([]byte(ts+":"), body...)
    return auth.VerifyHMACSHA256(v.Token, msg, sig)
}
```

### `decode.go`

The decoder's job is to produce a valid `Envelope`. The router downstream
should not need to know anything about Mattermost.

Critical fields:

- `ID` — derive from the platform's native event ID. **Idempotency depends
  on this being stable across redeliveries.**
- `ConversationID` — `mattermost:<channel_id>:<thread_id_or_root>`.
- `Platform` — add a new `event.PlatformMattermost` constant.
- `Kind` — almost always `event.KindUserMessage` for inbound.

### `encode.go`

The encoder posts an outbound `Envelope` back to the platform. It needs:

- The platform's API client (built around its REST / WS API).
- The OAuth/access token, fetched from `pkg/secrets` (add a new
  `secrets.NameMattermostBotToken` constant).
- Channel/thread metadata, fetched from `pkg/correlation`.

The encoder is the place where platform-specific niceties (rich text,
buttons, attachments) get materialized from the kind-specific payload.

## Step 2: extend `pkg/event`

In `pkg/event/envelope.go`:

```go
const (
    PlatformSlack      Platform = "slack"
    PlatformDiscord    Platform = "discord"
    PlatformGChat      Platform = "gchat"
    PlatformWhatsApp   Platform = "whatsapp"
    PlatformMattermost Platform = "mattermost" // new
)
```

## Step 3: extend `pkg/secrets`

```go
const (
    // ...
    NameMattermostBotToken = "mattermost-bot-token"
    NameMattermostSigningSecret = "mattermost-signing-secret"
)
```

Mirror these in `deploy/terraform/secrets.tf` (placeholder values; actual
values pushed via `gcloud secrets versions add` per the example doc you'll
write in step 6).

## Step 4: register in `cmd/ingress`

Add a route:

```go
mux.HandleFunc("/v1/mattermost", handler(mattermost.Verifier{...}, mattermost.Decoder{}))
```

The `handler` factory should be the same generic shape used for the other
four platforms — verify, decode, dedupe via replay cache, publish to inbound.
If you find yourself writing platform-specific code in `cmd/ingress` outside
this registration, push it down into the connector.

## Step 5: register in `cmd/emitter`

```go
switch *platform {
case "slack":      enc = &slack.Encoder{...}
case "discord":    enc = &discord.Encoder{...}
case "gchat":      enc = &gchat.Encoder{...}
case "whatsapp":   enc = &whatsapp.Encoder{...}
case "mattermost": enc = &mattermost.Encoder{...} // new
}
```

And add a Cloud Run service in `deploy/terraform/cloudrun.tf` (or wherever
the emitter resources live) launched with `--platform=mattermost` and an
attribute-filtered Pub/Sub subscription:

```hcl
resource "google_pubsub_subscription" "outbound_mattermost" {
  name  = "${local.name_prefix}.outbound.mattermost"
  topic = google_pubsub_topic.outbound.name
  filter = "attributes.platform = \"mattermost\""
  push_config {
    push_endpoint = google_cloud_run_service.emitter_mattermost.status[0].url
    oidc_token { service_account_email = google_service_account.emitter_mattermost.email }
  }
  dead_letter_policy { dead_letter_topic = google_pubsub_topic.outbound_dead.id, max_delivery_attempts = 5 }
}
```

## Step 6: write the onboarding example

Add `examples/mattermost-quickstart.md` walking through:

1. Creating the integration in the platform's admin console.
2. Required scopes / permissions.
3. Webhook URL and signature secret retrieval.
4. Pushing secrets to Secret Manager.
5. Granting the right IAM bindings.
6. Sending a test message.

Use the existing Slack and WhatsApp examples as templates.

## Step 7: write tests

In `test/fixtures/`:

- `mattermost-message.json` — a valid signed sample (use a fixed test secret).
- Optionally, `mattermost-tampered.json` — same payload, broken signature.

In `test/integration/mattermost_test.go` (build tag `integration`):

1. Post the signed fixture to local `ingress`; assert 200.
2. Read from emulator `<env>.inbound`; assert envelope shape matches.
3. Drive a fake `agent.reply` through `<env>.outbound`; assert the
   Mattermost emitter calls the expected API endpoint with the right thread
   metadata.
4. Negative paths: tampered body → 401; stale timestamp → 401; replay → 401.

## Step 8: wire it up in the architecture doc

Add Mattermost to the diagram in [`ARCHITECTURE.md`](ARCHITECTURE.md) and to
the signature-scheme table in [`CLAUDE.md`](../CLAUDE.md). Update
[`README.md`](../README.md)'s feature list.

## Anti-patterns (don't do this)

- **Don't reach into Firestore from the connector.** Use `pkg/correlation`.
- **Don't read secrets via `os.Getenv`.** Always `secrets.Manager.Get()`.
- **Don't emit platform-specific event kinds.** Add to the payload instead.
- **Don't skip the verifier in test code paths.** Tests sign their fixtures.
- **Don't add platform-specific knobs to `cmd/router`.** The router is
  platform-agnostic; if it can't be, your connector hasn't normalized enough.
- **Don't use `==` to compare signatures.** Constant-time, every time.

## Checklist before opening the PR

- [ ] `Verifier`, `Decoder`, `Encoder` implemented and unit-tested.
- [ ] `event.Platform` constant added.
- [ ] `secrets.Name*` constants added; Terraform `secrets.tf` mirrors them.
- [ ] `cmd/ingress` and `cmd/emitter` register the platform.
- [ ] Cloud Run service and Pub/Sub filtered subscription in Terraform.
- [ ] `examples/<platform>-quickstart.md` exists.
- [ ] Integration tests pass against emulators (positive + negative paths).
- [ ] `ARCHITECTURE.md`, `CLAUDE.md`, `README.md` updated.
- [ ] No `os.Getenv` for credentials; no `==` for signatures; no Firestore in
  the connector package.
