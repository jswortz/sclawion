# Event schema

Every message on Pub/Sub — inbound or outbound — is a JSON-encoded
`event.Envelope`. This is the contract between connectors, the router, the
bridge, and emitters.

## Why a single envelope

If every connector emitted its own shape, the router and downstream services
would have to know about every chat platform. Instead, connectors are the
*only* place platform-specific types live. Past `cmd/ingress`, everything
speaks `Envelope`. Adding a fifth platform doesn't touch the router.

## Wire format

```json
{
  "spec_version": "sclawion/v1",
  "id": "01HQXYZ123ABC456DEF789GHI",
  "conversation_id": "slack:C0001:1700000000.000100",
  "platform": "slack",
  "kind": "user.message",
  "occurred_at": "2026-04-18T12:34:56.789Z",
  "scion_agent_id": "agent-7f3a",
  "payload": { "...": "platform- or kind-specific" },
  "trace": {
    "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
    "span_id":  "00f067aa0ba902b7"
  }
}
```

JSON encoding lives in `pkg/event/envelope.go`. The corresponding Pub/Sub
message attributes (set by `pubsub.EnvelopeBytes` + the publisher) are:

```
platform     = "slack"
kind         = "user.message"
spec_version = "sclawion/v1"
```

Subscriptions filter on attributes (e.g., the Slack emitter subscribes with
`attributes.platform = "slack"`); receivers can short-circuit-reject messages
with the wrong `spec_version`.

## Field reference

| Field             | Type            | Required | Description                                                                 |
|-------------------|-----------------|----------|-----------------------------------------------------------------------------|
| `spec_version`    | string          | yes      | `"sclawion/v1"`. Bump on breaking change. Old versions must remain readable for one release. |
| `id`              | string (ULID)   | yes      | Globally unique. **Idempotency key.** For inbound, derived from the platform's event ID; for outbound, generated. |
| `conversation_id` | string          | yes      | `"<platform>:<channel>:<thread>"`. Stable for the lifetime of a conversation; used as Pub/Sub ordering key. |
| `platform`        | enum string     | yes      | `slack` \| `discord` \| `gchat` \| `whatsapp`                              |
| `kind`            | enum string     | yes      | See [Event kinds](#event-kinds) below                                       |
| `occurred_at`     | RFC 3339 string | yes      | When the event happened at the source. Set by the connector; emitters use this for relative-time formatting. |
| `scion_agent_id`  | string          | no       | Set on outbound events; set on inbound only after the router resolves correlation. |
| `payload`         | object (JSON)   | yes      | Kind-specific. See [Kind payloads](#kind-payloads) below.                  |
| `trace`           | object          | yes      | OTEL trace context propagated end-to-end.                                   |

## Event kinds

Inbound (chat → Scion):

- `user.message` — A human posted a message that should reach an agent.

Outbound (Scion → chat):

- `agent.started` — Agent was spawned. Emitter posts a "thinking…" indicator.
- `agent.reply` — Agent produced a message for the user.
- `agent.completed` — Agent finished its task. Emitter may post a completion
  marker or simply emit nothing.
- `agent.failed` — Agent crashed or exceeded its budget. Emitter posts an
  error message.

The set is deliberately small. Resist adding kinds for application-specific
behaviors — push that into the payload.

## Kind payloads

### `user.message`

```json
{
  "user_id": "U0001",
  "user_display_name": "Alex Doe",
  "text": "@sclawion build me a status dashboard for foo",
  "attachments": [
    { "type": "image/png", "url": "https://...", "name": "screenshot.png" }
  ]
}
```

### `agent.reply`

```json
{
  "text": "I checked the dashboard config — looks like the data source URL is wrong. Want me to open a PR?",
  "is_final": false,
  "buttons": [
    { "id": "yes", "label": "Yes, open PR" },
    { "id": "no",  "label": "No thanks" }
  ]
}
```

### `agent.started` / `agent.completed`

```json
{ "task": "build me a status dashboard for foo" }
```

### `agent.failed`

```json
{
  "task": "build me a status dashboard for foo",
  "error": "agent exceeded 10-minute wall-clock budget",
  "retryable": false
}
```

## Versioning

- `spec_version` follows `sclawion/v<MAJOR>`. Bump major on any breaking
  change (field removed, type changed, semantic change).
- Additive changes (new optional field, new event kind) do **not** bump major.
  Older consumers ignore unknown fields.
- After bumping major, the previous major must remain readable by all
  services for **one full release cycle** so we can roll back without losing
  in-flight messages.
- Receivers should reject `spec_version` they cannot parse with a 4xx so
  Pub/Sub sends to DLQ instead of retrying forever.

## Conversation ID format

```
<platform>:<channel_id>:<thread_id>
```

| Platform   | channel_id              | thread_id                                                |
|------------|-------------------------|----------------------------------------------------------|
| slack      | `C0001` (channel ID)    | `thread_ts` if reply-in-thread, else parent `ts`         |
| discord    | channel snowflake       | thread snowflake if a thread, else `_root`               |
| gchat      | space resource name     | thread resource name (`spaces/AAA/threads/BBB`)          |
| whatsapp   | E.164 phone (`+14155551234`) | always `_root` (WhatsApp has no threads)            |

Stable for the life of a conversation. Used as the Pub/Sub ordering key so
two messages in the same thread are guaranteed to arrive at the router in
order, while unrelated conversations are not blocked.

## Identifiers and idempotency

`id` is the idempotency key. Receivers should:

1. Derive `id` from the platform's native event ID (Slack `event_id`, Meta
   `entry.id`, Discord interaction `id`, GChat `event.eventTime+space`).
2. Pass it to `correlation.MarkProcessed(ctx, id)` *before* doing any work.
3. If `MarkProcessed` returns `firstSeen=false`, ack the message and return
   200 — this is a duplicate Pub/Sub delivery, not a logic error.

`processed_events/{id}` documents in Firestore have a 30-day TTL.

## Validation

```go
import "github.com/jswortz/sclawion/pkg/event"

func validate(e *event.Envelope) error {
    if e.SpecVersion != event.SpecVersion {
        return fmt.Errorf("unsupported spec_version %q", e.SpecVersion)
    }
    if e.ID == "" || e.ConversationID == "" {
        return errors.New("missing id or conversation_id")
    }
    // ...
}
```

A reusable `Validate()` helper is on the roadmap; for now each receiver does
its own checks. Keep them strict — silently accepting malformed events is the
fastest path to weird production behavior.
