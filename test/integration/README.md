# Integration tests

## Today: hermetic admin-api smoke

`admin_api_test.go` wires the real `cmd/admin-api/handlers` tree against
an in-memory `config.MemStore` and a stub `secrets.Writer`, then drives
the full HTTP surface end-to-end via `httptest`. No emulators required:

```bash
go test ./test/integration/...
```

This is what CI runs ([`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)).

## Future: data-plane round-trip (emulator-driven)

When `cmd/ingress`, `cmd/router`, `cmd/scion-bridge`, and `cmd/emitter`
land, the suite expands to round-trip tests behind the `integration`
build tag:

```bash
gcloud beta emulators pubsub    start --host-port=localhost:8085 &
gcloud beta emulators firestore start --host-port=localhost:8086 &
PUBSUB_EMULATOR_HOST=localhost:8085 \
FIRESTORE_EMULATOR_HOST=localhost:8086 \
  go test -tags=integration ./...
```

Each platform should have a test that:
1. Posts a signed fixture from `test/fixtures/<platform>-message.json` to the local ingress.
2. Asserts an `Envelope` lands on `sclawion.inbound`.
3. Asserts the router calls the fake Scion stub with the expected `DispatchRequest`.
4. Sends a fake `agent.reply` through `sclawion.outbound`.
5. Asserts the matching emitter would post back with the right thread metadata.

Negative paths to cover per platform:
- Tampered body → 401, no publish.
- Stale timestamp (>5 min) → 401.
- Replay (same hash within window) → 401.

The Verifier-level versions of those negative paths are already covered
in the per-connector unit tests (see e.g. `pkg/connectors/slack/verify_test.go`).
