# Contributing

Welcome. `sclawion` is open to contributions; this document is the rules of
engagement.

## Quick orientation

- Read [`CLAUDE.md`](../CLAUDE.md) and [`ARCHITECTURE.md`](ARCHITECTURE.md).
  Together they're ~400 lines and they save you hours.
- The five "architecture rules" in `CLAUDE.md` are non-negotiable for PR
  approval.
- For any change touching auth, crypto, or secret handling, read
  [`SECURITY.md`](SECURITY.md).

## Dev environment

```bash
# Go 1.22+
go version

# install dev tools
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

# install gcloud CLI + emulators
gcloud components install pubsub-emulator cloud-firestore-emulator beta

# bootstrap
git clone https://github.com/jswortz/sclawion
cd sclawion
go mod tidy
```

Local emulators are wired into a `docker-compose` (TODO) for one-shot startup.
For now:

```bash
gcloud beta emulators pubsub start --host-port=localhost:8085 &
gcloud beta emulators firestore start --host-port=localhost:8086 &
export PUBSUB_EMULATOR_HOST=localhost:8085
export FIRESTORE_EMULATOR_HOST=localhost:8086
```

## Branching and PRs

- Trunk-based. Branches off `main`, short-lived (target <3 days).
- PR titles in [Conventional Commits](https://www.conventionalcommits.org/)
  style: `feat(slack): handle thread replies`, `fix(router): dedupe by event.id`.
- One logical change per PR. If a refactor enables a feature, that's two PRs.
- Required for merge:
  - All CI checks green ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml)
    runs `go vet`, `staticcheck`, `go test -race -cover`, `govulncheck`,
    `go build` per service, plus a stdlib-only guard on `go.mod`).
  - One reviewer approval; two for changes touching `pkg/auth`, `pkg/scion`,
    or `deploy/terraform/`.
  - Docs updated when behavior changes.

### Repository secrets

CI exposes the following secrets to test jobs:

| Secret | Aliased as | Used by |
|--------|------------|---------|
| `GEMINI_API_KEY` | `GOOGLE_API_KEY` (matches `google-genai` SDK convention) | Future LLM-touching tests; safely empty for forks |

Secrets are elided on PRs from external forks, so the build still passes
on community contributions; tests that *require* the key must skip gracefully
with `t.Skip("GEMINI_API_KEY not set")` rather than fail.

## Coding standards

### Go

- `gofmt -s` (CI enforces).
- Package-scoped errors as `var ErrX = errors.New(...)`; wrap with `%w`.
- No package-level mutable state outside `main`.
- Context first, error last:
  `func F(ctx context.Context, x string) (Y, error)`.
- Avoid third-party dependencies for things the standard library does:
  HTTP, JSON, HMAC, hex, time. Adding a dep needs justification in the PR.

### Python (skill helpers, tooling)

The repo is Go-only today, but if you add a `.py` file (e.g., a skill
helper or a one-off script), CI will run `ruff` against it with rules
`E,F,B,S,I,UP,SIM,N` and a 100-char line length — see the `ruff` job in
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml). No `pyproject.toml`
is required; add one only if you want to override the inline defaults.

### Comments

- Default to none. Comment the *why*, not the *what*.
- Document every exported symbol with a doc comment that starts with the
  symbol name (`// Foo does X.`).
- TODOs include a username or issue link: `// TODO(jswortz): wire up CMEK`.

### Tests

- Unit tests live next to the code (`foo.go` ⇄ `foo_test.go`).
- Integration tests live under `test/integration/` with build tag `integration`.
- Fixtures (signed sample webhooks) live under `test/fixtures/`.
- Every connector PR adds at least: positive path + tampered body + stale
  timestamp + replay.

### Logging

- Structured JSON via `log/slog`.
- Always include `event.id`, `conversation_id`, `platform`, `kind` when
  available.
- Never log secrets, tokens, or full message bodies. Hash if you need a
  reference.

## Commit messages

Body of the commit should answer "why this change" — the *what* is the diff.
Bullet points are fine. Keep the first line ≤72 chars.

Co-authored-by lines for pair work; AI-assisted commits should include the
`Co-Authored-By: Claude ...` line per project convention.

## Reviewing a PR

When reviewing, look for:

1. Does it respect the architecture rules? (Pub/Sub firewall, normalized
   events, stateless services, idempotency, no platform leakage.)
2. Does it use the right primitives? (`pkg/auth`, `pkg/secrets`,
   `pkg/correlation` — not direct Firestore/Secret Manager calls from
   feature code.)
3. Tests cover positive + at least one negative path.
4. Docs updated: `CLAUDE.md` if the rule set changes;
   `ARCHITECTURE.md` if components or seams change; `EVENT_SCHEMA.md` if the
   envelope changes; `SECURITY.md` if controls change.
5. No new third-party dependencies without justification.

## Releasing

`main` is always deployable. There are no release branches.

- Tags follow `vMAJOR.MINOR.PATCH` semver.
- Patch: bug fix only, no schema or API change.
- Minor: additive change to envelope, new connector, new feature.
- Major: breaking change to envelope (`spec_version` bump) or removal of a
  public surface.

Releases are cut from `main` via `gh release create`; CI builds and signs
images, and the `prod` deploy is gated by a GitHub deployment approval.

## Security disclosure

See [`SECURITY.md`](SECURITY.md) §"Disclosure". **Do not** open a public
issue for a security finding.

## Code of conduct

Be kind. Assume good faith. Disagree with ideas, not people. Maintainers
reserve the right to remove comments and lock threads that don't meet that
bar.
