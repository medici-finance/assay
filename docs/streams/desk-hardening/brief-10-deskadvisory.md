---
brief: desk-hardening/10
title: deskadvisory — recompute-at-the-gate verification for security-advisory fixes
wave: 0
depends: []
unblocks: []
effort: M
gate: human
gate-why: handles embargoed pre-disclosure security content, and is a deliberate bounded exception to the compiled-in C-4 repo allowlist — it fetches an untrusted tree from a repo that is deliberately NOT in allowedRepos. Both the exception's boundary and the fetch hardening need a human on the review; a drift toward "fetch any repo the caller names" would be a general-purpose hole in the desk's strongest gate.
why: A security fix developed under a GitHub Security Advisory currently gets LESS verification than a typo PR — no CI, no App verdict, no desk review — because advisory temporary private forks have no Actions and no App can reach them. For a Flux base an unverified advisory merge is a change that can deploy itself. This is the last check before a merge nothing can stop.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [250]
schema: brief-v1
authored: 2026-07-31 by intake-desk (Opus session)
sources: ["docs/design/advisory-fix-pipeline.md (merged efbef969; content is the third revision, doc self-labels as rev 2)", "medici-finance/assay-toolkit#250", "medici-finance/assay-toolkit#251 (design + two review rounds)", "freshness-checked 2026-07-31 @ efbef969"]
---

# Brief 10 — deskadvisory

Build the tool docs/design/advisory-fix-pipeline.md specifies. **Read that document first — it
is the spec, and it records why several obvious designs were rejected.** This brief is the
contract; the design doc is the reasoning.

`gate: human` is derived, not chosen: the tool handles **embargoed pre-disclosure security
content** (`sensitive-data: yes`), and it operates on a repository outside the compiled-in C-4
allowlist as a deliberate bounded exception. Both need a human on the review.

## Context

files:
- **create** `tools/desk/cmd/deskadvisory/` — `main.go`, `advisory.go`, and tests
- **comment-only amendment** tools/desk/internal/deskkit/config.go — `allowedRepos`,
  `IsAllowedRepo`. **No behaviour change**: the set is not widened and the function is not
  touched. The only edit is the carve-out comment required by task step 7.
- **mirror** tools/desk/cmd/deskgit/exec.go and `deskgit.go` — the fetch hardening to copy
  (`--refmap=`, `--upload-pack=git-upload-pack`, `--no-recurse-submodules`, explicit positional refspec, `scrubbedEnv`)
- **spec** docs/design/advisory-fix-pipeline.md

facts:
- advisory temporary private forks (TPFs) have **no GitHub Actions** — `/actions/permissions` and
  `/actions/runs` 404 (design doc M1). There is no CI to read; the checks must be run locally.
- a TPF is **not** a fork by the API: `fork=false`, `parent=null`, base `forks_count=0` (M5).
- the authoritative base→TPF link is `GET /repos/{base}/security-advisories/{ghsa}` →
  `.private_fork.full_name` (M7). It lives on the **base** repo.
- `deskkit.IsAllowedRepo` makes no network call and trusts no caller input; that property is not
  to be altered by this brief.
- live fixture available while it exists: base `medici-finance/canton-k8s`, advisory
  `GHSA-895h-mc6v-42wr`, fork `medici-finance/canton-k8s-ghsa-895h-mc6v-42wr`.

consumers: this brief introduces a **bounded C-4 exception**, so every reader of the C-4 rule is a
consumer of its meaning — `deskkit.IsAllowedRepo` (unchanged, still the trust root: fixed-here by
*not* touching it), `cmd/deskgit`/`cmd/deskpost`/`cmd/deskboard` (unchanged; they must keep
refusing TPF slugs — out-of-scope), and docs/design/advisory-fix-pipeline.md (already states the
exception — fixed-here by keeping code and doc in step). **No new repo may be added to
`allowedRepos` by this brief.**

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not publish, merge, or modify any advisory**, and do not push to a TPF. This tool is
  read-and-verify only. Its output informs a human's merge decision; it never takes it.
- Treat fetched TPF content as **untrusted**. It is a security fix under embargo, authored
  wherever the advisory came from.
- **Run identity.** The tool rides the invoking human's ambient GitHub identity — the credential
  resolvable by `gh auth token` (preferred, via `GH_TOKEN` or `GITHUB_TOKEN`) falling back to
  git's credential helper. It must NOT persist the token on disk. The mechanism:
  `scrubbedEnv` is applied to the parent environment first to strip inherited
  execution vectors; the tool then appends its own `GIT_ASKPASS` pointing at a
  script it controls (the vector the allowlist closes is *inheritance* of an
  ambient askpass, not a value the tool sets deliberately after scrubbing).
  **`deskgit`'s shared `envAllowlist` must NOT gain `GIT_ASKPASS`** — if a
  resolution requires that, report NEEDS_CONTEXT. The fetch invocation must also
  carry `-c credential.helper=` (cleared) so nothing reaches the operator's
  credential store. Prefer `GIT_ASKPASS`; if the implementation cannot use it,
  `-c http.extraheader=` with the header value sourced from the environment (not
  written into argv) is an alternative — never put the token on the command line
  where `ps` and `/proc/<pid>/cmdline` can read it. The required capability is
  advisory-read + contents-read
  on the base repo and contents-read on the private fork — admin or security-manager on the base.
  When the credential lacks advisory-read, GitHub returns 404 for both "does not exist" and
  "access denied"; the tool must report "advisory could not be resolved or access denied" without
  disambiguating, and must exit non-zero. Verify rows 3-5 are to be run with this identity, not
  a broad PAT.
- **Embargo barrier.** Tool output that contains embargoed advisory identifiers (GHSA id, TPF
  slug, or fork name) or any content fetched from a TPF stays on the operator's terminal. Do not
  paste it into PR bodies, issues, board artifacts, or Evidence. Evidence for Verify rows
  involving a live advisory must redact identifiers (see the row-5 caveat below).

## Task

Build `deskadvisory check <base-repo> <ghsa-id>` — note the arguments are the **base repo and
advisory id**, never a TPF slug. Steps, in order, each failing closed:

1. **Admit the base.** Refuse unless `<base-repo>` is in `allowedRepos` (`deskkit.ExitRefused`).
   This is the ordinary C-4 gate, unwidened.
2. **Resolve the fork authoritatively** via the base repo's advisory object (M7). Refuse if the
   advisory does not exist, carries no `private_fork`, or is in a state that should not have one.
   Nothing is trusted from a repository name — there is no name-pattern derivation in this tool.
3. **Resolve the fork's head SHA live**, and confirm the fork reports `private: true`. Refuse
   otherwise.
4. **Fetch that tree with deskgit-grade pins** — `--refmap=`, `--upload-pack=git-upload-pack`,
   `--no-recurse-submodules`, an explicit positional refspec, and `scrubbedEnv`. Auth must be
   ephemeral (follow the Run-identity ground rule: `GIT_ASKPASS` set after `scrubbedEnv`,
   with `-c credential.helper=` cleared; never persisted to a git config file or credential
   cache). The fetched tree is ephemeral —
   land it in a temporary directory outside any existing repo checkout, with best-effort cleanup
   on exit. Reuse `deskgit`'s helpers rather than re-implementing them; if they must be exported
   to share, do that rather than fork the logic.
5. **Run the repo's declared check list** at that SHA. The check-list **definition** must be read
   from a trusted source only — the base repo's default branch via the GitHub API, or per-repo
   data committed in this repo — **never** from the fetched TPF tree. (A TPF copies
   `validate.yml` from the base, per design M1; reading it from the fetched tree would make the
   tool execute checks declared by the untrusted content being checked — a self-pass and local
   code-execution path.) The runner must treat the fetched tree strictly as **input data**: run
   only tools that are already on `PATH` or shipped in this repo, never execute scripts, hooks,
   or binaries from inside the fetched tree, and never source configuration that controls
   execution from it. If a declared check cannot be expressed as trusted-tool-over-untrusted-data
   (kubeconform-shaped: cheap, deterministic, secretless), the tool refuses (`ExitUnverifiable`)
   rather than improvising. **Where and how the check list is stored is still the implementer's
   design decision** — per-repo data, not compiled-in logic — but the source must be the base
   repo (via API) or this repo, never the TPF.
6. **Report** pass/fail with the SHA actually checked, and exit non-zero on fail.
7. **Amend the prohibition this exception contradicts, in the same commit.**
   tools/desk/internal/deskkit/config.go documents C-4 as a closed set that nothing may widen, and
   `IsAllowedRepo`'s comment says a caller receiving `false` must refuse with `ExitRefused`. A
   reader of that file will find `deskadvisory` operating on a repo outside the set and no
   indication it is sanctioned. Add a **carve-out comment beside `allowedRepos`/`IsAllowedRepo`**
   naming `deskadvisory`, stating its boundary (admission starts from an allowlisted base; the
   fork is reached only via that base's advisory `private_fork` pointer; the set itself is never
   widened), and citing docs/design/advisory-fix-pipeline.md and this brief. **Do not change any
   behaviour in that file** — comment only.

   This step exists because #261 records a recurring stream defect: desk-hardening changes grant
   authority while leaving the contradicting rule unamended, so the contradiction is discoverable
   only by whoever happens to read both. An exception that is not written where the rule lives is
   indistinguishable from a violation.

Follow the existing deskkit exit conventions (0 ok, 5 refused, 6 unverifiable). Network failure or
any state the tool cannot establish is a **refusal**, never a pass — the whole point of the tool
is that it is the last check before an unstoppable merge.

**What this tool must not do:** claim to gate the merge (it cannot — the advisory merge is an
admin action), store or accept a cached verdict, or accept an arbitrary repo to fetch. If the
implementation drifts toward "fetch any repo the caller names", stop and report NEEDS_CONTEXT.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOFLAGS=-buildvcs=false go test ./cmd/deskadvisory/...` | exit 0 |
| 2 | `cd tools/desk && GOFLAGS=-buildvcs=false go vet ./cmd/deskadvisory/...` | exit 0 |
| 3 | `go run ./tools/desk/cmd/deskadvisory check medici-finance/not-in-allowlist GHSA-895h-mc6v-42wr; echo $?` | exit `5`; stderr names the C-4 refusal and the repo |
| 4 | `go run ./tools/desk/cmd/deskadvisory check medici-finance/canton-k8s GHSA-0000-0000-0000; echo $?` | non-zero; stderr says the advisory could not be resolved or access was denied — **not** a pass |
| 5 | `go run ./tools/desk/cmd/deskadvisory check medici-finance/canton-k8s GHSA-895h-mc6v-42wr` | resolves fork `medici-finance/canton-k8s-ghsa-895h-mc6v-42wr`; prints the head SHA it checked |
| 6 | `cd tools/desk && for p in refmap upload-pack scrubbedEnv no-recurse-submodules; do if ! grep -qF "$p" cmd/deskadvisory/*.go; then echo "MISSING: $p"; exit 1; fi; done` | exit 0, no `MISSING:` line. **One grep per pin, not an alternation**: `grep 'a\|b\|c\|d'` succeeds when ANY ONE branch matches, so it would report "all four pins present" with three of them absent — the exact defect this row exists to catch (#262) |
| 7 | **Mutation:** delete the `IsAllowedRepo` call from step 1, re-run row 1 | a test **FAILS**. If the suite still passes, the admission test is not wired and row 1 proves nothing |
| 8 | **Mutation:** make the advisory-resolution error path return success, re-run row 1 | a test **FAILS** (fail-closed on unresolvable advisory is asserted, not assumed) |
| 9 | `cd tools/desk && GOFLAGS=-buildvcs=false go test ./internal/deskkit/...` | exit 0 — C-4 behaviour untouched by this brief |
| 10 | See fenced block below | every changed line (`+`/`-` not `+++`/`---`) is a `//` comment; row 9 backs this up |
| 11 | `grep -qF deskadvisory tools/desk/internal/deskkit/config.go && grep -qF advisory-fix-pipeline.md tools/desk/internal/deskkit/config.go` | exit 0 -- carve-out names both the tool and the design doc |
| 12 | `git clone --depth 1 https://github.com/medici-finance/canton-k8s /tmp/ck8s && cd tools/desk && DESKADVISORY_BASE_CLONE=/tmp/ck8s GOFLAGS=-buildvcs=false go test -count=1 -v -run PassesOnRealBase ./cmd/deskadvisory/` | PASS. **The shipped check list must be able to return PASS on the base repo's own `main`** — a list that cannot is not a gate, it is an outage: every advisory fix would FAIL regardless of content and the verdict would carry no information. Needs `kubeconform` on PATH; the test skips without the env var, so CI does not run it |
| 13 | **Mutation:** in `runChecks`, make `invertExit` treat any non-zero exit as a pass (delete the `default:` arm of the exit-code switch), re-run row 1 | a test **FAILS**. `grep` exits 1 for "no match" but **2 for an error** — a guard that malfunctioned must never be indistinguishable from a clean tree |
| 14 | **Mutation:** delete the `files < c.MinFiles` work-floor in `runChecks`, re-run row 1 | a test **FAILS**. A path that EXISTS but holds nothing is the vacuous pass `os.Stat` alone cannot see: `grep` over an empty dir exits 1, `kubeconform` over one exits 0 |
| 15 | **Mutation:** delete the `compiledOutputMatch` assertion in `runChecks`, re-run row 1 | a test **FAILS**. Exit 0 alone cannot distinguish "validated the tree" from "found nothing to validate" |
| 16 | **Mutation:** delete the `advisoryStatesWithFork` gate in `resolveFork`, re-run row 1 | a test **FAILS**. The TPF exists only pre-disclosure (M7 measured the fixture at `state:"draft"` WITH a fork; published reports `private_fork:null`), so a gate admitting only `published` refuses every advisory the tool can check |
| 17 | **Positive control:** `cd tools/desk && GOFLAGS=-buildvcs=false go test -count=1 -run PositiveControl ./cmd/deskadvisory/` | exit 0. Drives the **shipped** checkdef over trees carrying planted defects (a `${VAR:-default}` in `deploy/scripts`, an access-key-shaped string) and requires each to be caught, then requires the same guard to PASS once removed. Without it every "clean tree" pass in the package is vacuous |
| 18 | **Inventory + doc parity:** `cd tools/desk && GOFLAGS=-buildvcs=false go test -count=1 -run 'Inventory\|DocParity\|EmbeddedCheckdefs' ./cmd/deskadvisory/` | exit 0. `go/ast` over `run()`'s dispatcher (every `case` label must be exercised by a `run([]string{…})` call), reflection over `checkSpec`'s JSON fields, and the README's shipped-check table vs the embedded checkdef. Each guard fails closed when it can see nothing, so a blind guard cannot report "all classified" |

Row 10 command (runs from repo root):

```bash
cd tools/desk && git diff -U0 origin/main -- internal/deskkit/config.go | grep -E '^[+-][^+-]'
```

Rows 7, 8 and 13-16 are mandatory and are the point of the table. A guard that cannot be shown to
fail when removed has not been tested — that failure mode was hit twice in one day on 2026-07-31
(a `grep -q '${'` fail-closed check that matched nothing, and a coupling test whose regex matched
~400 lines away), which is why this stream carries the mutation-test rule at brief 01.

Rows 12-18 were added after review. They exist because this tool's entire value is **not
overstating what is enforced**, and the first two implementations of it both shipped a decision
layer that could print PASS having examined nothing — first over a path that did not exist, then
over one that existed and was empty, and separately whenever a `grep` guard *errored* rather than
finding the tree clean. Rows 13-15 are the mutation form of those three; row 12 catches the
adjacent failure where the check list is so strict it can never pass, which is equally
uninformative; row 17 is the positive control without which every absence assertion in the suite
is unfalsifiable; row 18 keeps the enumerations (dispatcher cases, schema fields, documented
checks) from going stale silently.

Row 5 depends on the live fixture advisory existing and the operator holding advisory-read on
`canton-k8s`. If the fixture has been published and its fork deleted, substitute a
**purpose-created drill advisory only** — never a real embargoed advisory whose identifiers would
be recorded in a durable artifact. If a verifier runs against a live advisory, Evidence must
redact identifiers: "a live draft advisory, identifiers withheld; run witnessed by <identity>."
The general rule (see Ground rules: Embargo barrier) applies — embargoed identifiers and TPF
content stay on the operator's terminal, never pasted here.

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     The `verified` status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review

<!-- reviewer verdict recorded here -->
