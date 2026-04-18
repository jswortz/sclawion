# Admin UI screenshots

Captured against `cmd/admin-api` running locally with seeded sample data
(3 tenants, 3 connectors including iMessage, 2 agent profiles, 2 swarms,
3 admin users, 7 audit entries). Caller is `owner@dev.local` via the
dev IAP bypass.

| Page | URL path | Screenshot |
|------|----------|------------|
| Tenants list | `/ui/tenants` | [`tenants.png`](tenants.png) |
| Tenant overview | `/ui/tenants/acme` | [`tenant-acme.png`](tenant-acme.png) |
| Connectors (incl. iMessage) | `/ui/tenants/acme/connectors` | [`connectors-acme.png`](connectors-acme.png) |
| Agent profiles | `/ui/tenants/acme/agents` | [`agents-acme.png`](agents-acme.png) |
| Swarms | `/ui/tenants/acme/swarms` | [`swarms-acme.png`](swarms-acme.png) |
| Admin users | `/ui/admin-users` | [`admin-users.png`](admin-users.png) |
| Audit log | `/ui/audit` | [`audit.png`](audit.png) |

To regenerate: `go run /tmp/seed_admin.go` (or any equivalent seeder
that writes to a `config.MemStore`), then drive playwright at
`http://localhost:8088`.
