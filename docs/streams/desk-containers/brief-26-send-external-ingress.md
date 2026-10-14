---
brief: assay:assay:desk-containers:26
title: "External ingress for `send` — an opt-in authenticated listener, the untrusted `external` sender class, per-sender rate limits, a request/reply audit log, and Tier A status with an external-safe projection"
why: >-
  People outside the cell, such as a collaborator, a dashboard or another team's tool, want to
  ask "is your desk running, and what is it doing?" without being given access to the
  machine. Comms today is a local socket only, so they cannot ask at all. This brief opens one
  narrow, opt-in door: authenticated callers, a separate untrusted sender class, a strict rate
  limit, every request and reply logged, and answers built by code from an allow-listed view of
  the status that cannot carry paths, tokens or private names.
wave: 10
depends: ["desk-containers/25"]
unblocks: ["desk-containers/27"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver ruling 2026-10-14 (relayed by the coordinator): external, untrusted parties may ask a desk for status; ingress is a new explicitly opt-in listener; decide mTLS client certs or signed tokens; per-sender rate limits; audit every request and reply; replies never contain secrets, paths, tokens or private repo names"
  - "tools/desk/cmd/commsgw/a2a.go — an existing cross-cell A2A JSON-RPC server fronted by mutual TLS (`RequireAndVerifyClientCert` against a configured client CA), not started in local-only mode; the handshake refuses an unauthenticated peer before application code runs"
  - "docs/cellctl-comms.md — local-only by default; 'no cross-cell listener'; standalone gateways keep their mTLS requirements"
  - "desk-containers/25 — the sender-class model and the Tier A responder this brief exposes"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no external sender class, no per-sender rate limit, no external-safe status projection"
gate-why: >-
  sensitive-data: yes. This is the first network-reachable path into a cell. A reply that
  carried a filesystem path, a token, a pid tied to a host, a private repository name or a
  personal login would leak private cell state to an untrusted party. What the human confirms:
  the authentication choice; that the listener is off unless explicitly configured; that an
  external caller can only reach Tier A, answered from an allow-listed projection, never the
  full status document; and that the rate limit and the audit log are on whenever the
  listener is.
exec-tier: strong
exec-tier-why: (a) the auth choice and the projection allow-list are design decisions; (c) network auth, rate limiting and leak filtering, where a gap survives happy-path tests
domain: complicated
consumers:
  - "tools/desk/cmd/commsgw (external listener, auth, rate limit, audit, external Tier A): follow-up desk-containers/26 (this brief)"
  - "tools/desk/internal/comms (the `external` sender class: Tier A only): follow-up desk-containers/26 (this brief)"
  - "tools/desk/internal/cellop (the external-safe status projection): follow-up desk-containers/26 (this brief)"
  - "docs/cellctl-comms.md (an 'External ingress' section, including how to issue and revoke a caller credential): follow-up desk-containers/26 (this brief)"
  - "the console cockpit (shows external requests from the audit log): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 26 — External ingress for `send`

## Context
files:
- `tools/desk/cmd/commsgw/` — external listener (opt-in), caller authentication, per-sender rate limiter, request/reply audit log, external Tier A route
- `tools/desk/internal/comms/` — `external` sender class (Tier A verb only in this brief)
- `tools/desk/internal/cellop/external_projection.go` (planned) — the allow-listed status view
- `docs/cellctl-comms.md` — §External ingress
- `changelog/desk-containers-26-external-ingress.md` (planned)

single-point-of-failure: the caller credential check at the listener — behind it, the `external` class's Tier-A-only ACL, the allow-listed projection (no free text, no paths), the outbound scrub, the rate limiter, and the audit log that `doctor` reads.

facts:
- **Off by default.** The listener starts only when the manifest has an `external` block with a bind address, credential material and a rate limit. A missing or partial block means no listener. `cellctl comms <cell> check` reports which applies.
- **Authentication (the human decision below).** The recommended option reuses the existing A2A mutual-TLS front with a separate external-caller client CA. Each caller gets its own client certificate, the certificate's subject is the caller id, and revocation means removing it from that CA bundle. The handshake refuses an unauthenticated peer before any application code runs.
- **The `external` class** is assigned only from a verified caller credential. External callers may send only `ask status` in this brief. Any other verb is refused by the ACL, and free text is brief 27.
- **External-safe projection.** An allow-list, not a deny-list: role name; state (`up` | `down` | `paused` | `unknown`); heartbeat age bucket; queue depth; cell outcome. It excludes paths, pids, model names, repository or issue names, logins, token or cache detail, and error text. The reply is built by code from these fields only.
- **Outbound scrub.** Every external reply passes a scrub that refuses (rather than redacts) a reply containing any configured withheld token, an absolute path, a token-shaped string, or an email/login shape. A refusal is audited, and the caller gets a fixed `unavailable` answer.
- **Rate limit.** A per-caller-id token bucket (configured rate and burst) plus a global cap. Over the limit, the caller gets a fixed `rate-limited` reply, and the event is audited.
- **Audit.** One append-only JSON line per request and per reply: time, caller id, verb, outcome, and the reply digest, never the reply body. `doctor` reports an external request with no reply record, and a refused reply.

## Human decision
Should a cell be able to open an opt-in network door so outside callers can ask its desks for status, and how should those callers prove who they are?

The door is closed unless the operator configures it. Callers can only ask "status". The answer is built by code from a short allow-list (role, up/down, heartbeat age, queue depth) and is checked again before it leaves, so it cannot include paths, tokens, private names or error text. Every caller is rate-limited, and every request and reply is logged.

Options:
1. **Mutual TLS client certificates** (recommended) — reuses the gateway's existing mTLS front. Each caller holds its own certificate. The handshake rejects strangers before any code runs. Revoke a caller by removing their certificate. This costs certificate issuance per caller.
2. **Signed bearer tokens** — the operator issues each caller a token signed with a cell key, carrying an expiry and a caller id, over server-side TLS only. Easier to hand out, but a leaked token works until it expires, and rejection happens in application code rather than at the handshake.
3. **Do not open external ingress** — external status stays unavailable; briefs 26 and 27 are not built.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Never bind the listener in a default configuration, a test default, or an example that could be copied unchanged.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the opt-in `external` manifest block and its `check` validation.
2. Implement the chosen authentication on the listener; derive the caller id and the `external` class from it.
3. Add the `external` class (Tier A only) to the ACL, the projection, the outbound scrub, the rate limiter and the audit log.
4. Docs (issuing and revoking a caller credential, the projection fields, the audit format); changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/commsgw/ ./internal/comms/ ./internal/cellop/ -run 'TestExternal' -count=1` | exit 0 |
| 2 | check | `go -C tools/desk test ./cmd/commsgw/ -run 'TestExternalUnauthenticatedRefusedAtIngress' -count=1` | exit 0 — a caller with no credential, an unknown credential, or a revoked one is refused at the listener; the audit log shows the refusal and no request reaches the ACL |
| 3 | check | `go -C tools/desk test ./cmd/commsgw/ -run 'TestExternalListenerOffByDefault' -count=1` | exit 0 — with no `external` block, or a partial one, nothing binds (port probe fails) |
| 4 | check | `go -C tools/desk test ./internal/comms/ -run 'TestExternalClassTierAOnly' -count=1` | exit 0 — an authenticated external caller sending any verb other than `ask status` is refused by the ACL, even with the listener's own verb check stubbed out |
| 5 | check +mutation | `go -C tools/desk test ./cmd/commsgw/ -run 'TestOutboundScrubBlocksWithheldToken' -count=1` | exit 0 — a projection deliberately seeded with a configured withheld token, an absolute path and a token-shaped string is refused by the scrub, and the caller gets `unavailable`; a mutant that skips the scrub fails |
| 6 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestExternalProjectionAllowList' -count=1` | exit 0 — the projection's JSON keys equal the allow-list exactly; adding a field to the status document does not add it to the projection |
| 7 | check | `go -C tools/desk test ./cmd/commsgw/ -run 'TestExternalRateLimitTrips' -count=1` | exit 0 — burst+1 requests from one caller inside the window yield exactly one `rate-limited` reply and an audit line; another caller is unaffected |
| 8 | gate:human +flow +dereference | On a disposable cell with the listener configured on a loopback address: an authenticated caller asks status; an unauthenticated caller tries; the authenticated caller exceeds its rate | reply 1 carries only allow-listed fields; attempt 2 fails at the handshake (or token check); attempt 3 is rate-limited; the audit log shows all three with reply digests |
| 9 | check | `statusgen --root . --lint` | exit 0 |
| 9a | check | `statusgen --root . --consumers --brief desk-containers/26` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (see gate-why). The reviewer answers both core-system questions: what single control stands between an untrusted caller and private cell state, and which rows prove a lower layer holds with the upper one bypassed (rows 4 and 5 bypass the listener check and the projection respectively).
