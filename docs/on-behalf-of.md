# On-behalf-of: the composite-identity principal trailer

**Status: initial landing (multi-principal/01).** One trailer format, six writers, a
witness annotation, and a lint. This document is the format spec, the resolution
contract, and — as important — what the trailer does **not** prove.

## Why

Every desk role shares one GitHub App across every session that ever runs it. A review
posted by the reviewer App, an Evidence row committed by the verifier App, a PR opened by
the worker App — none of it says whose fleet, whose driver, whose session actually
produced the write. GitLab's shared-service-account audit model ships the fix to copy:
every audited action records "service account **on behalf of** `@human`". This is that
model, generalised to a fleet that has not yet grown a second human, so the plumbing is in
place before it needs to disambiguate between two.

## The trailer

```
On-behalf-of: human:<who>
On-behalf-of: human:<who> mode:unattended
```

- `human:<who>` — the human the write is on behalf of, in the form the TARGET REPO'S
  VISIBILITY calls for: the lowercased human **login** on a repo the roster states is
  `:private`, and the roster's neutral **name** for that same human — the
  `ASSAY_HUMAN_LOGIN_MAP` key, i.e. the given name the map already carries as that
  human's public form — on every other target. See "Which form" below.
- ` mode:unattended` — appended only when the resolving session carried no live
  interactive roster beacon (a cron loop: `scanloop`, `verifyloop`, …, as opposed to a
  desk/worker session with a registered role). Its absence means the session was
  attended.

It appears in three shapes, depending on the surface:

| Surface | Shape |
|---|---|
| A git commit (an Evidence row) | a trailing line in the commit message, blank-line separated |
| An issue or PR comment / body | a trailing line in the body, blank-line separated |
| A witness Runner cell (`## Evidence` table, `statusgen verifyrun`) | `on-behalf-of human:<login>` inside the cell's trailing parenthetical, e.g. `assay-verifier-app[bot] @ a1b2c3d (on-behalf-of human:ada) (ci-env)` |

The witness-cell form omits the `On-behalf-of:` key (it is a table cell, not a
line-oriented record) and is never suffixed with `mode:unattended` — `verifiedrunneragree.go`'s
`runnerKey` already drops everything from the first `(` onward, so any text placed inside
a trailing parenthetical never perturbs the runner-agreement comparison.

## Which form: the target repo's visibility decides

A trailer stamped onto a write to a **public** repo is world-readable for good — a posted
review cannot be edited afterwards, and a commit message cannot be taken back out of
history. So the form the trailer names is chosen from the target repo's **configured**
visibility (`ASSAY_ALLOWED_REPOS`' `:public` / `:private` token, read with no network call
— the same value `VisibilityRiskClassed` reads):

| Target | The trailer names |
|---|---|
| A repo the roster states is `:private` | the mapped login — unchanged |
| Everything else — `:public`, an unstated visibility, a repo admitted only by an `owner/*` pattern, a repo the roster does not carry, or no repo at all | the roster's neutral **name** for that human |

The rule is stated in the fail-closed direction on purpose, exactly as
`VisibilityRiskClassed` states its own: *everything except a stated-private repo takes the
neutral form.* A wrong "public" costs audit precision; a wrong "private" is a disclosure
that cannot be withdrawn. Every entry point therefore takes the target repo as a
**mandatory** argument — a verb cannot resolve a principal without saying where the write
is going, and a package test (`TestEveryWriteVerbNamesItsTarget`) reads the verbs' source
to hold that.

**When the roster carries no neutral name** for the blessing authority, a public-target
write **refuses** (exit 5) naming the `ASSAY_HUMAN_LOGIN_MAP` entry to add. The two
alternatives were both rejected: stamping the login is the disclosure this split exists to
prevent, and writing with no trailer at all would retire the presence guarantee every
check downstream is built on. The refusal text does not quote the login it is declining to
disclose.

**Not yet covered: the witness annotation.** The `statusgen` witness cell (below) is a
separate module's read-only mirror and still renders the login. It lands in a brief's
`## Evidence` table, so on a public repo it carries the same exposure; closing it means
teaching the attribution lint to accept the neutral name as well, which is a change to the
lint's contract rather than to this resolver, and is tracked separately.

## Resolution

Implemented once, in `tools/desk/internal/deskkit/principal.go` (`ResolvePrincipal`), and
mirrored read-only in `statusgen/principal.go` for the witness/lint side (statusgen and
the desk tools are separate Go modules that deliberately share no code — the
documented-duplicate pattern `rosterconfig.go` already uses).

**Order:** the calling session's own roster beacon (`deskroster`'s
`<StateDir>/roster/<session>.json`) decides ATTENDED vs UNATTENDED; `ASSAY_BLESS_LOGIN`,
read *exclusively* through the roster's config-home file (never an environment variable —
see `rosterconfig.go`'s `ClassWrite` rule, which this resolver reuses rather than opening
a second route), supplies the login itself.

**An environment variable is never consulted for the login.** A session could otherwise
set its own principal by exporting one — exactly the self-report failure mode the
roster's file-only-for-write-tools rule already closes for every other trust decision.
`DESK_SESSION` / `CLAUDE_SESSION_ID` name *which* beacon file to open; they never supply
the login.

**Three states** (`PrincipalState` in `principal.go`):

| State | Meaning |
|---|---|
| `PrincipalUnresolved` (zero value) | No roster is configured — or the target needs the neutral form and the roster states none. Every write verb **refuses** (exit 5) rather than write without a principal. |
| `PrincipalAttended` | Resolved to the bless login; the calling session has a live roster beacon. |
| `PrincipalUnattended` | Resolved to the **same** bless login; no live session beacon was found. |

**Why both resolved states name the same login today.** Per the 2026-09-12 driver ruling
this brief relays: until a second human joins the roster, the
principal resolves to the roster's single blessing authority (`ASSAY_BLESS_LOGIN`)
regardless of who or what is driving the session. The session-beacon read exists so that
when a second human's attach-time handshake lands (deferred to roster v2), attended
sessions already have the plumbing to resolve to the session's own human rather than the
bless login — this landing's job is only to tell attended and unattended runs apart, not
yet to pick between two humans.

## The writers

Six verbs stamp the trailer, or refuse (exit 5) rather than write without one:

| Verb | Where it lands |
|---|---|
| `deskpost comment` / `review` / `security-review` | a trailing line on the posted comment/review body |
| `deskreply` (plain and `--workpad`) | a trailing line on the posted/edited comment body |
| `deskpr create` / `edit` | a trailing line on the PR body |
| `deskfile new` / `attach` | a trailing line on the issue body / comment |
| `deskevidence` | a trailing git-trailer line on the commit message (both the direct-write and the side-branch/draft-change lanes) |
| `deskflip` | see below — this verb has no body to stamp |

**Idempotency is unaffected.** The trailer is appended to the body/message sent to the
forge, never to the bytes an idempotency/dedup key is computed from. Two sessions posting
byte-identical semantic content — one attended, one not, or on different days — still
dedupe: the trailer text (which can legitimately differ between the two calls) never
enters the comparison. `deskpr edit`'s noop compare strips a live body's prior trailer
(`deskkit.StripOnBehalfOfSuffix`) before comparing against the caller's replacement, for
the same reason.

**`deskflip` is the deliberate exception.** Its one mutation
(`deskkit.ReadyFlip` — a draft→ready GraphQL/REST transition) carries no body or commit
message at all; there is no text surface to stamp a trailer onto. Its write is: resolve
the principal as a *precondition* of the mutation (refusing exit 5 rather than flip
without one), and name it in the printed success/dry-run line, so the record of who a
flip was on behalf of survives in the one place every other `deskflip` decision does —
stdout, which the desk's audit capture already retains.

`--dry-run` on every verb runs every other check, resolves the principal (refusing if
unresolvable), and prints the trailer it would have written before stopping. **This
changes the dry-run contract**: a dry run can no longer be used to check a write verb's
*other* preconditions in an environment with no roster configured — `deskflip --dry-run`
in particular now exits 5 on an unresolvable principal even when every other precondition
it checks would have passed. That is deliberate (the principal is a real precondition of
the mutation, not a preview-only concern), but it is a behavior change worth knowing about
before scripting against `--dry-run` in an unconfigured environment.

## The witness annotation

`statusgen verifyrun`'s witness `row()` renders the on-behalf-of suffix into the Runner
cell whenever the base runner names a shared App/bot identity (the `<slug>[bot]`
convention `executingRunner` itself already uses for a CI-actor runner) — never for a
`human:<name>` runner, which already names the acting human directly. Unlike the write
verbs above, this annotation is **best-effort**: if no principal resolves, the cell is
left unannotated rather than refusing the whole witness write. The write-side refusal
(the six verbs, above) is the actual enforcement point; the witness cell is a
*record* of what already landed.

## The lint

`statusgen --lint` (`principalAttributionProblems`, `statusgen/attribution.go`) reads
every brief's `## Evidence` section and checks two DISTINCT shapes, which are gated
differently — read both before assuming either governs the other:

- **Missing annotation.** An Evidence row run by an App/bot identity with **no**
  on-behalf-of annotation at all. This check is **cutover-gated, not roster-gated**: a
  row dated at or after `principalAttributionCutoverDate` (the date this write path
  landed) is a hard **PROBLEM** — the write path could have stamped it and did not. A
  row dated *before* the cutover is a **NOTICE** — no write path existed yet to stamp
  it, so it is grandfathered rather than condemned. This check runs **regardless of
  whether a roster is configured**: an unconfigured repo still gets the
  missing-annotation signal (as PROBLEM or NOTICE per the row's own date), because
  detecting the *absence* of an annotation needs no human map to check against.
- **Unrecognised annotation.** An Evidence row whose on-behalf-of annotation names a
  login **not** in this repo's roster human map (`ASSAY_HUMAN_LOGIN_MAP` / the bless
  login's own entry). This check IS roster-gated: with **no roster configured at all**,
  it has no human map to validate against and says nothing (a repo that has not adopted
  the roster gets no signal from a check it cannot answer, rather than a manufactured
  problem). It carries no cutover — an annotation that exists at all was written by a
  stamping path, so the retroactive-history problem the cutover solves does not apply
  here.

A human-run row (`human:<name>`) is never checked by either shape — it already names its
principal directly.

## What this does *not* prove

The witness file comment already states the residual for `executingRunner`, and it
applies here without modification: **the environment is not a cryptographic anchor.**
Whoever controls the process controls the roster's config-home file and the session
beacon that decide attended/unattended. The on-behalf-of trailer is evidence for a
reviewer reading a diff — a composite-identity record analogous to GitLab's, and the
precondition the later author-never-flipper check, the decision packet, and the
segregation-of-duties lint (multi-principal/03, /04, /09) build on — not an unforgeable
attestation. Its real strength, like the witness record it rides alongside, is that it
lands next to the tree SHA and commit it names, where a second person (or a later,
stricter check) can corroborate it.
