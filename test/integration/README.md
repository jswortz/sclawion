# Integration tests

Run with the `integration` build tag against local emulators:

```bash
docker compose -f ../../deploy/compose.yaml up -d   # TODO
PUBSUB_EMULATOR_HOST=localhost:8085 \
FIRESTORE_EMULATOR_HOST=localhost:8086 \
go test -tags=integration ./...
```

Each platform should have a test that:
1. Posts a signed fixture from `test/fixtures/<platform>-message.json` to the local ingress.
2. Asserts an Envelope lands on `sclawion.inbound`.
3. Asserts the router calls the fake Scion stub with the expected `DispatchRequest`.
4. Sends a fake `agent.reply` through `sclawion.outbound`.
5. Asserts the matching emitter would post back with the right thread metadata.

Negative paths to cover per platform:
- Tampered body → 401, no publish.
- Stale timestamp (>5 min) → 401.
- Replay (same hash within window) → 401.
