# CLAWPATH security — customer-network integration

This document is a delta on top of [`../SECURITY.md`](../SECURITY.md).
That document covers the chat-bridge layer; this one covers everything
new that CLAWPATH introduces: agent swarms, the SCION network underlay,
and customer-network integration patterns.

Read both. They are additive, not alternatives.

## What changes when you add SCION

The base sclawion threat model assumed agent egress was either internal
to GCP (Pub/Sub, Firestore, Secret Manager) or to public APIs (Slack,
Discord, etc.). CLAWPATH adds a fundamentally new category: **agent
egress into a customer's private network**, traveling over a network
the customer partly operates.

This adds three new trust boundaries:

1. **The SIG (SCION-IP Gateway) on our side.** Translates IP traffic
   into SCION; chooses paths. Compromise = ability to route traffic
   wherever the SIG's path-policy allows.
2. **The SCION network itself.** Multiple ISDs, each with its own
   trust root. Compromise of an ISD's TRC is a major event but
   contained to that ISD.
3. **The SIG on the customer side.** Their host, their security
   posture. Compromise = ability to inject traffic *as if* it came
   over our SCION path.

And it removes (or shrinks) two old ones:

- **The public Internet between us and the customer.** No longer used
  for production agent traffic; can be relegated to OOB.
- **BGP and the global routing table.** Not in the trust path.

## Threat model additions

| # | Adversary | Capability | Mitigation |
|---|-----------|------------|------------|
| 7 | On-path attacker on a transit ISP | Inject / modify traffic between GCP and customer | SCION per-hop MAC fails verification; packets dropped |
| 8 | BGP hijacker | Reroute traffic through hostile AS | Not applicable — no BGP in path |
| 9 | Compromised SIG (our side) | Route customer-bound traffic to wrong destination | Path-policy service signs allowed peers; SIG must produce valid SCION paths to customer's TRC |
| 10 | Compromised SIG (customer side) | Forge inbound responses, inject traffic | Customer's responsibility; we authenticate at TLS layer regardless |
| 11 | Hostile transit ISD | Selectively drop / delay our traffic | Multi-path; failover to alternate path within ~1 s |
| 12 | Compromised swarm agent | Issue arbitrary actions in customer net | Per-role IAM (planner can't deploy), budget caps, customer-side API permissions are the final gate |

## SCION trust model in 60 seconds

- Trust is rooted in the **TRC** (Trust Root Configuration) of each
  ISD. The TRC enumerates the ISD's core CAs.
- Trust is **sovereign per ISD**. No single global authority.
- Cross-ISD trust requires explicit core-AS peering relationships.
- TRC rotation requires a quorum of existing core ASes — single-CA
  compromise does not unilaterally compromise an ISD.
- Per-AS keys derive from the AS's certificate, which chains to the
  TRC. Border routers verify hop-field MACs using AS keys.

What this gives the enterprise reviewer:

> "Our traffic to acme-corp goes via ISD 64. ISD 64's TRC is signed by
> {ETH Zürich, Swisscom, ...}. We can audit the TRC. We control which
> ISDs we accept paths from. If ETH's signing key is rotated, we see
> the new TRC and our path service updates within seconds."

This is a story BGP cannot tell.

## Comparison: SCION vs alternatives for customer network reach

| Property | Public Internet + bearer | IPsec VPN | Cloud Interconnect | SCION + SIG |
|---|---|---|---|---|
| Provisioning time | minutes | hours–days | weeks | hours per peer |
| Path control | none | none | one carrier | sender-selected, multi-path |
| BGP attack exposure | yes | yes | partial | none |
| Multi-path failover | TCP retry | manual second tunnel | redundant circuits ($$$) | native, sub-second |
| Per-packet auth | TLS only | IPsec ESP | none | TLS + SCION hop MAC |
| Sovereignty / transit auditing | best-effort | none | one carrier guarantee | first-class, per-request |
| Key rotation pain | n/a | high | n/a | low (DRKey hierarchical) |
| Compatible with unmodified apps | yes | yes | yes | yes (via SIG) |
| Operational complexity | low | medium | high | medium (one-time per peer) |

For one-customer engagements, IPsec is fine. For a multi-tenant
agentic platform serving N customers in different regulated sectors,
SCION's **sovereign trust** and **multi-path** properties scale where
IPsec doesn't.

## Customer isolation models

Pick one per tenant; document the choice in the contract.

### Option A — per-customer ISD (highest isolation)

- Customer brings their own ISD or joins their preferred one.
- We expose a separate AS in that ISD per customer.
- Pros: blast radius is one customer; aligns with regulated-customer
  expectations (e.g., financial sector).
- Cons: most ops surface; requires us to be a member of N ISDs.

### Option B — shared ISD, per-customer AS

- We're a member of one or two operational ISDs (e.g., Anapaya
  commercial + a national ISD).
- Each customer is a peer AS in one of those ISDs.
- Pros: simpler ops; same ISD trust root for all customers in that ISD.
- Cons: customers in the same ISD trust the same TRC.

### Option C — shared AS, per-customer path policy

- Single AS for all customers; path policy at the SIG enforces "this
  swarm's traffic to customer A may only use these ISDs."
- Pros: lowest ops cost.
- Cons: weakest isolation; one path-policy bug exposes one customer's
  traffic to another's path. **Not recommended for regulated tenants.**

Default for new customers: **Option A** unless they explicitly opt into
B for cost reasons.

## Compliance implications

### Swiss FINMA (financial supervision)

FINMA Circular 2018/3 ("Outsourcing — banks and insurers") requires:

- Demonstrable control over data location and transit.
- Auditable record of where data flowed.
- Risk assessment of cloud providers.

CLAWPATH's per-request SCION path-id logging into BigQuery, combined
with sovereign Swiss ISDs, is a strong story for this regime. SSFN —
the Swiss interbank network running on SCION — is the existence proof.

### EU GDPR (Article 44 transfers)

GDPR cares about data leaving the EU/EEA. With CLAWPATH:

- Path policy: agent egress to EU customers may only use ISDs whose
  member ASes are EU-incorporated.
- Logging: every egress request records `isd_as` chain; BigQuery
  retains 400 days; auditor can answer "did any of our traffic
  transit a non-EU AS in Q3?" with a SQL query.
- Data Processing Agreements: SCION ISD membership and path policy
  belong in the technical schedule.

### US FedRAMP

FedRAMP doesn't recognize SCION as a control by name. The compensating
controls map: SCION SIG = network device under SC-7 (boundary
protection), per-hop MAC = SC-8 (transmission confidentiality and
integrity), TRC = SC-12 (cryptographic key establishment).

Deploy in Assured Workloads if your customer is federal. SCION is
additive to FedRAMP boundary controls, not a replacement.

### HIPAA

Same posture as base sclawion: BAA covering Cloud Run, Pub/Sub,
Firestore, Secret Manager, KMS. Add: SCION traffic is in scope for
the BAA — confirm with Anapaya / your ISD operator that they can sign.

## Key management

### SCION AS keys

- AS keys live on the SCION control service; rotated per the ISD's
  policy (typical: monthly).
- Rotation is automated via SCION's certificate machinery; no human
  in the loop.

### DRKey

- Symmetric keys derived hierarchically from AS keys.
- Allows any two SCION hosts to agree on a shared key in microseconds
  with no online handshake.
- Used for: per-packet path MACs (EPIC), source authentication
  (PISKES).
- DRKey roots rotate with AS keys; derived keys are short-lived.

### Application secrets

- Unchanged from base sclawion. Secret Manager, per-secret IAM,
  rotation via scheduled Cloud Function publishing `secrets.rotated`
  events.
- **Customer API tokens** (e.g., a deploy token for customer's GitLab)
  are stored separately per customer in Secret Manager with
  `customer_id` as part of the secret name; per-tenant IAM bindings.

## Audit & detection

Beyond the base sclawion audit posture, CLAWPATH adds these signals:

| Signal | Source | Detect what |
|--------|--------|-------------|
| `sig.egress` span with unexpected `isd_as` | OTEL → Cloud Trace | Path policy violation (should be impossible if SIG is healthy) |
| SCION path-failover event | SIG logs → BigQuery | Network instability; potential path-injection attempt |
| TRC rotation in any in-use ISD | SCION control service | New TRC must be reviewed before allowing new paths |
| Per-customer egress volume spike | BigQuery aggregation | Compromised swarm or runaway agent |
| Same `swarm_id` running > budget wallclock | Firestore + alert | Stuck swarm |
| `secrets.rotated` followed by emitter 401 | Cloud Logging | Rotation missed a consumer |

## Incident response additions

### "We see traffic to a customer ISD we don't have permission for."

Block at SIG: `gcloud run services update sig --update-env-vars=SCION_BLOCKED_ISDS="<isd>"`.
Investigate path-policy bug. Rotate SIG service to clear in-memory
path cache.

### "Our SIG appears compromised."

1. `gcloud run services delete sig` to remove the deployment.
2. Customer SIGs will refuse new sessions.
3. Investigate via Cloud Audit Logs for the SIG's GSA.
4. Redeploy SIG from clean image (Binary Auth ensures this) and
   re-issue AS keys via the SCION control service.

### "Customer reports traffic from us they didn't expect."

1. Pull all `sig.egress` spans for that `customer_id` in the last 24h
   from Cloud Trace.
2. Cross-reference with `swarm_id` → user chat message → user identity.
3. If unauthorized swarm: check `swarms/{swarm_id}` document in
   Firestore, see who registered it, when, with what budget.

### "TRC rotation by an ISD operator we don't trust."

1. Path service auto-fetches new TRC; we must approve.
2. If we don't approve within the rotation window, paths to that ISD
   stop validating.
3. Decision: approve and update path policy, or remove the ISD from
   our allowed set.

## Contractual schedule (template excerpt)

For customer agreements, recommended technical schedule items:

> **Schedule X — Network connectivity (CLAWPATH)**
>
> 1. Customer shall participate in {ISD-name} as AS {AS-number}.
> 2. Customer shall operate a SIG (SCION-IP Gateway) at their network
>    edge, peering with Provider's SIG at ISD-AS {provider-AS}.
> 3. Provider shall route all agent egress destined for Customer's
>    network exclusively over SCION paths whose terminating AS is
>    Customer's AS as defined in (1).
> 4. Provider shall log per-request SCION path-id and ISD-AS chain
>    and retain such logs for 400 days, available to Customer on
>    request.
> 5. Either party shall notify the other within 24 hours of any TRC
>    rotation event in any ISD used for traffic between the parties.
> 6. Provider's SIG and AS keys shall rotate per the ISD's policy
>    (typically 6 months for TRC, monthly for AS keys).

## Open questions before first regulated customer

- Whether to insist on per-customer ISD (Option A) or accept
  per-customer AS in a shared ISD (Option B). Bias: A for finance,
  healthcare, government; B otherwise.
- Whether to operate our own SCION ISD or rely on a third party
  (Anapaya). Bias: rely on a third party until customer contracts
  justify the operations cost.
- Customer-controlled keys (CMEK with HSM-backed key) for per-customer
  CMEK. Currently we offer per-env CMEK; per-tenant requires Terraform
  refactor and a higher pricing tier.
- Whether to build a "SCION path browser" UI for customer auditors, or
  whether SQL-over-BigQuery is enough. Bias: SQL until a customer
  asks.

## Pre-launch security checklist

- [ ] Threat model review by an independent security engineer.
- [ ] Penetration test against ingress LB and SIG.
- [ ] Tabletop exercise: TRC compromise scenario.
- [ ] Tabletop exercise: malicious agent emits egress to wrong customer.
- [ ] Verify that BigQuery audit retention applies to SCION-related
      log entries.
- [ ] Verify that path-policy violations actually trigger the alert.
- [ ] Confirm SIG image is in Binary Authorization attestor allowlist.
- [ ] Document key rotation runbook for the SCION control service.
- [ ] Customer-facing security whitepaper (this document, polished).
