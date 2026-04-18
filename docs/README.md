# Documentation

Deep-dive guides for `sclawion`. Read order depends on your role.

## I want to use sclawion

1. [`../README.md`](../README.md) — what it is, quickstart
2. [`../examples/`](../examples/) — per-platform onboarding
3. [`OPERATIONS.md`](OPERATIONS.md) — deployment and runbooks

## I want to contribute code

1. [`../CLAUDE.md`](../CLAUDE.md) — load-bearing context, read first
2. [`ARCHITECTURE.md`](ARCHITECTURE.md) — components, sequences, design tradeoffs
3. [`EVENT_SCHEMA.md`](EVENT_SCHEMA.md) — the envelope every service speaks
4. [`CONTRIBUTING.md`](CONTRIBUTING.md) — workflow, standards
5. [`CONNECTORS.md`](CONNECTORS.md) — adding a new chat platform

## I want to review for security or compliance

1. [`SECURITY.md`](SECURITY.md) — threat model, controls, compliance posture
2. [`ARCHITECTURE.md`](ARCHITECTURE.md) §"Failure modes"
3. [`OPERATIONS.md`](OPERATIONS.md) §"SLOs", §"Runbooks"

## I want to know why GCP

[`GCP_PATTERNS.md`](GCP_PATTERNS.md) — what each GCP primitive earns its place
doing, and what'd you'd lose porting elsewhere.

## I want to know where it's headed

[`ROADMAP.md`](ROADMAP.md) — shipped, in-flight, parked, won't-do.
