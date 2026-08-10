# desk-tools — zero-prompt workflow plumbing (scoping)

**Date:** 2026-07-10 (v2 — post adversarial filter, 3 independent critics, ~30 findings folded
in; v1 same day) · **Origin:** INTAKE I-23 · **Authored:** Fable desk session, human:<name>-directed
**Decision owner:** human:<name>. The cutover brief (06) is `gate: human` — enabling zero-prompt outward
verbs is his explicit trade to accept, not a default.

## Problem

The permission system currently conflates two layers:

1. **Workflow verbs** — the standing-authorized motions of the methodology's loops: read PRs
   across the three repos, post the reviewer verdict, flip a converged PR ready, open and update
   a worker's draft PR, reply on findings, manage worktrees, run the board. Policy already
   authorizes every one of these (CLAUDE.md: reviews are ALWAYS posted; workers ALWAYS open
   draft PRs and reply on findings; the desk flips ready).
2. **Payload** — what an individual PR/brief actually changes: DAML, k8s, funds paths, deploys.

Both layers prompt today. A prompt on layer 1 asks the human to re-approve a decision the
methodology already made — a **nuisance alarm** in ISA-18.2 terms: not actionable, pure fatigue,
and it trains click-through, which *erodes* the layer-2 prompts that ARE actionable. Measured
from 50 recent session transcripts: the loops' top prompting patterns are all layer-1
(`gh -R <repo> pr view/diff` reads defeated by the `-R` flag; `gh pr comment/ready/create/review`
~70 combined; worker follow-up pushes and replies every review iteration; `go run …/deskboard.go`).

**A second, uncomfortable fact the filter surfaced:** the shared checkout's untracked
`.claude/settings.local.json` has accumulated ~1,179 rules including `Bash(gh *)`,
`Bash(git push *)`, and `Bash(go run *)` — in the most privileged window, the prompt is
already NOT the brake the threat model assumed. Part of this stream's job is to REPLACE that
accreted wide surface with the narrow one, not merely add tools beside it.

**Target state (human:<name>, 2026-07-10):** *"workflow no permissions, actual doing work might have
some."* The reviewer/verifier/worker/next-job loops never prompt for their own machinery;
prompting concentrates on the payload, gated by each brief's `gate`/`risk`/`gate-why`.

## Principle

**Where humans sit (human:<name>, 2026-07-10):** humans are valuable at (1) **initiating** — deciding
what to do, the initial prompts and briefs; (2) **approving** — reviewing and merging PRs
before anything goes live; (3) **deciding** — the verification gates and `gate: human` calls
where larger judgment lives. They should NOT be in the day-to-day loop mechanics. The three
human gates are deliberate and stay; everything between them is machinery.

Replace broad allowlist rules with **purpose-built tools that carry the policy in code** (the
standing tool-first preference, 2026-07-08/10). Each tool does one workflow verb with a narrow,
auditable surface; the allowlist names those binaries and nothing wider. The prompt's safety
value is not deleted — it is **relocated into the tool as an enforced precondition**, and where
a tool cannot enforce it, the doubt routes back to a human (C-10).

**The write/read line:** outward WRITES get binaries (they carry the risk); pure READS get
either a deskboard verb or a narrow allowlist rule — a read rule carries none of TM-2/TM-4's
risk, so a binary is only justified where the read needs logic (the ACTION classification,
cross-repo aggregation, drift banners), not for prompt-count alone.

## What stays prompted (deliberate — not gaps)

- **Push to main** in any form (verify-desk Evidence/doc commits included) — main is the deploy
  trigger; this stays human-adjacent by design.
- **`gh issue create`** (verify-desk VERIFY:FAIL filings) — rare, judgment-carrying.
- **Everything payload**: the brief's own build/test/deploy commands beyond the standing allows.
- **`sudo make desk-install`** — the install gate itself (C-1).

## Threat model (what zero-prompt opens, and the answer to each)

| # | Threat | Answer |
|---|--------|--------|
| TM-1 | **Local forgeability of the review gate.** Permission rules are per-repo, not per-session: granting `deskpost` grants it to EVERY session, so a worker (or a runaway) could post bot reviews unprompted. | Cannot be fully engineered away (no per-session permissions exist in the harness). Mitigations: C-2 (`ready`/`review` verify their own preconditions incl. head-pinning), C-5 (audit + idempotency + limits), branch protection as the hard backstop. **Accepted residual, signed at cutover** — and the sign-off must name the FULL surface: (a) the gate stays tamper-evident from GitHub's side, honor-system locally (already true today via `mint-reviewer-token.go`); (b) `.claude/settings.json` and `settings.local.json` are agent-writable, so allowlist rules themselves are honor-system (cutover drill includes a settings-diff check; brief 06 purges the accreted local.json surface); (c) the kill-switch file is agent-deletable — it stops faithful runaway loops (the observed 2026-07-09 incident class), not adversarial ones; Guard logs DISABLED-file transitions so disarming is at least visible. |
| TM-2 | **Prompt injection / exfiltration.** Loops read semi-untrusted text; zero-prompt outward writes remove the human circuit-breaker where injection aims. | **Honest framing: C-3 is an accidental-paste seatbelt plus friction, not a complete exfiltration defense.** Structured writes, size caps, and the secret scan block accidental leakage and naive exfil; a deliberately encoding adversary can evade pattern filters, and `deskpr`'s branch push is inherently an unfiltered-content channel (committed code cannot be reliably content-filtered — `deskpr` runs the same secret scan over the diff-vs-default as a seatbelt, and refuses on hits, but this is best-effort). **Recorded residual**: deliberate injection-driven exfil is mitigated by PR threads/pushes being human-visible and audited, not prevented. Backstop: everything lands in the three private repos human:<name> reads. |
| TM-3 | **The allowlisted-source/binary trap.** An allow rule executes whatever is at the named path at runtime; `~/.claude/*` and the repo are agent-writable. | **C-1: root-owned install (human:<name>, 2026-07-10).** Binaries live in `/opt/desk-tools/bin/` installed via `sudo make desk-install` — the sudo password IS the manual permission gate; no agent can write there. A committed sha256 manifest + the deskboard drift banner make staleness/drift visible. The pre-existing `Bash(go run …/deskboard.go*)` rule is REMOVED at cutover. **Recorded exemption**: `Bash(go run ./tools/statusgen*)` keeps the run-from-source shape — accepted because statusgen is lint/board-only (no outward writes, no credentials); revisit if it ever grows a write verb. |
| TM-4 | **Runaway amplification.** A looping agent opened 4 unauthorized PRs on 2026-07-09 WITH prompts; prompts were the accidental rate-limiter. | C-5: outward-write budget sized to legitimate peak — **10 per tool per rolling hour, counting ALL attempts** (any result — a refusal-loop must trip it), grouped by tool; per-(key) idempotency; C-6 kill switch (one `touch`, suite-wide, <5s). Note 30/hr would NOT have stopped the observed incident; 10/hr + idempotency does. |
| TM-5 | **Filesystem delete surface** (worktree cleanup). | C-8: resolved-path prefix allowlist, dirty/unpushed refusal, shared-checkout refusal by identity, no force verb. |

## Design constraints (binding on EVERY tool — briefs restate the ones they implement)

- **C-1 Root-owned pinned binaries (the manual permission gate).** Tools live in `../assay-toolkit/tools/desk/`
  (Go), reviewed via the normal PR loop, installed by IAN running `sudo make desk-install` →
  `/opt/desk-tools/bin/<tool>`, root-owned 0755. The allowlist names those paths. No `go run`
  rule for any agent-writable source path. Each binary embeds `sourceSHA`+`builtAt` (ldflags)
  and emits them in `--version` and every audit record; `make desk-install` also writes
  `../assay-toolkit/tools/desk/MANIFEST.sha256` (committed) and the cutover + weekly drill verify installed
  hashes against it. `deskboard` prints a **STALE banner** on every run when installed
  `sourceSHA` ≠ origin/main's `tools/desk` tree (drift is a per-boot banner, not an archaeology
  exercise); each loop skill's boot step checks it.
- **C-2 Preconditions re-verified in-tool, with head-pinning.** A tool never trusts its
  caller's claim of state.
  - `deskpost review` REQUIRES `--head <sha>` — the SHA the verdict was formed against; refuses
    (exit 5) if the PR's current head differs (a verdict must not land on unreviewed code).
  - `deskpost ready` verifies immediately before acting: PR open+draft; the App's latest review
    is APPROVED **with `commit_id` == current head**; CI green per the per-repo table below;
    repo ∈ set. GitHub's ready mutation has no compare-and-swap, so: re-read head immediately
    before the flip; **after** the flip, re-read once more — if the head moved in the window,
    post a loud structured comment naming the race and exit non-zero (this remediation comment
    is the ONE sanctioned deviation from C-7's flip-only rule; un-drafting is not attempted).
  - `deskpr` verifies: non-default branch (detached HEAD → exit 6), ≥1 commit ahead, origin ∈
    repo set, no staged-uncommitted changes.
- **C-3 Structured outward writes (seatbelt, precisely specified).** Anything writing text to
  GitHub accepts only `--body-file` (no stdin/inline), size ≤16 KiB, and passes the secret scan:
  REFUSE on `ghp_`/`github_pat_`/`ghs_`/`gho_` prefixes, `AKIA[0-9A-Z]{16}`, `-----BEGIN` PEM
  headers, `eyJ`-prefixed 3-dot JWT shapes, `sops`/`ENC[` markers, and runs of ≥32 base64ish
  chars **EXCEPT exactly 40- or 64-char lowercase-hex runs (git SHAs — the methodology quotes
  them constantly; refusing them would brick every verdict)**. Test vectors for both directions
  (SHA passes, each token pattern refuses) are C-9 deliverables. Review bodies must additionally
  match the verdict schema — **defined by brief 03 in the tools/desk README (verdict-format
  section) as a deliverable**, and the loop skills adopt it at cutover; plain comments get scan
  + size only. `deskpr` runs the same scan over title, branch name, body, AND the diff-vs-default
  (best-effort — see TM-2's residual). No override flag exists anywhere.
- **C-4 Fixed scope.** The repo set (`oit`,
  `example-org/agent-runtime`, `example-org/medici-examples`) is compiled in; no flag widens it.
  No tool has merge, close, un-draft, branch-delete, force-push, issue-create, or
  workflow-dispatch verbs. **Per-repo CI table (compiled in, C-2 consumes it):**
  `oit`: PR checks REQUIRED — empty rollup = exit 6 (unverifiable);
  `agent-runtime`: same; `medici-examples`: known to have NO PR CI — empty rollup = green
  (non-empty red still refuses). SKIPPED/NEUTRAL check conclusions are ignored (match
  deskboard v1's ciState). App-installation parity across all three repos (incl. whether the
  App may un-draft/ready PRs) is VERIFIED at cutover, not assumed.
- **C-5 Audit + limits (concurrency-honest).** Every invocation appends one JSON line to
  `~/.claude/desk-tools/audit.jsonl`: `{ts, tool, verb, argsDigest, bodyDigest?, repo, pr,
  headSHA, result: ok|noop|refused|disabled|ratelimited|unverifiable, detail, sourceSHA,
  builtAt, sessionTag}`. `sessionTag` = `$CLAUDE_SESSION_ID` else a derived tag (worktree path
  + ppid) — self-reported, forensics not enforcement. Semantics, exactly:
  - **Ordering:** for outward writes, the audit line is appended AFTER the remote call returns;
    the crash window between call and append (rare duplicate on retry) is an accepted residual.
  - **Locking:** outward-write verbs hold an `flock` on the audit file across
    check→remote-call→append (stale-lock timeout 60s → exit 6) so parallel sessions can't
    double-post or double-count.
  - **Idempotency:** keys — `ready`: `(repo, pr, headSHA)`; `review`: `(repo, pr, headSHA,
    verdict)`; `comment`: `(repo, pr, headSHA, bodyDigest)` (two DIFFERENT comments at one head
    are both legitimate; identical re-posts are not); `deskpr`: `(repo, branch, headSHA,
    "pr-create")` — push-succeeded/create-failed reruns proceed to create; existing open PR for
    the branch → noop printing its URL. **Only prior entries with `result ∈ {ok, noop}` count
    as done** — a refusal followed by state change is a legitimate retry (C-10 forbids retrying
    past a LIVE failed verification, not retrying after the world changed). Noop prints loudly
    WHAT it deduplicated against.
  - **Rate limit:** outward writes only; ≤10 per tool per rolling hour, counting ALL attempts
    (any result), `ts ≥ now-1h`, grouped by `tool`. Breach → exit 4.
  - **Failure states:** no file/dir = empty history (bootstrap: create 0700/0600). Unreadable
    file or malformed line = exit 6 for the affected lookup, with the recovery procedure printed:
    a HUMAN moves the file aside to `audit.jsonl.corrupt-<ts>` (tools never truncate/rewrite);
    fresh file = fresh history — the post-rotation duplicate-post window is accepted and noted.
    File deletion fails open (resets limits + history): recorded residual; the deskboard banner
    prints the audit file's age/first-ts so a suspicious reset is visible.
- **C-6 Kill switch.** First action of every tool AND re-checked immediately before any outward
  write: if `~/.claude/desk-tools/DISABLED` exists (or `DESK_TOOLS_DISABLED=1`) → write the
  audit line (result=disabled — the audit write is exempt from "touch nothing"), print the
  DISABLED file's first line as the reason (empty → generic), exit 3. Guard logs
  presence/absence TRANSITIONS of the file (vs the last audit line's state) so disarming is
  visible after the fact. Deletion-fails-open is a recorded residual (TM-1c).
- **C-7 Weakest-verb design.** `deskpr` can ONLY create drafts on non-default branches and push
  follow-ups to an EXISTING open draft (no ready, no edit); `deskpost ready` only flips
  draft→ready (the post-race loud comment is the sole carve-out); `deskboard` is GET-only,
  proven by a PATH-shim test enumerating every gh/API invocation.
- **C-8 Worktree safety.** `deskwt` operates only under `/private/tmp/oit-*` and
  `<repo-root>/.claude/worktrees/`; prefix-match on the RESOLVED (`EvalSymlinks`) path; the
  shared checkout is refused by identity (git-common-dir parent), not just prefix. **Dirty** =
  staged/unstaged TRACKED changes, unpushed commits, or no upstream; untracked build artifacts
  (`node_modules`, `.daml/dist`) do NOT block removal. No force flag exists. (verify-desk's
  documented `../verify-desk-main` location is amended to a sanctioned prefix at cutover.)
- **C-9 Negative tests are deliverables.** Every refusal above ships with a test proving the
  tool REFUSES (and, where remote calls are involved, that NO call was made — fake-server hit
  recording). A brief is not `implemented` until its refusal tests exist and pass.
- **C-10 Fail closed on ambiguity (human:<name>, 2026-07-10).** *If in doubt, ask — never assume.* Any
  state a tool cannot POSITIVELY verify — API error during a precondition check, unexpected
  PR/branch state, unparseable input, partial response — is a refusal (distinct exit code + a
  message naming the manual/prompted path). One scoped exemption: in-tool token re-mint on
  expiry mid-operation is an allowed internal retry (it re-verifies nothing about the world);
  everything else: no tool proceeds on a best guess or retries past a live failed verification.
  Exit codes: 3 disabled, 4 rate-limited, 5 refused, 6 unverifiable, 0 success/noop.

## Console noise floor (the role-skill output contract)

The standing desk windows (the-desk, pr-review-desk, verify-desk, intake-desk) narrate
the board every sweep — full tables each iteration, per-item progress notes, "checked,
nothing new" paragraphs — and the signal (a needs-decision item, a verdict, an exception)
drowns in iteration noise. The tool half of this is `deskboard --delta` / `--quiet`
(issue-flow/10): the read tools print only what CHANGED. The skill half is a one-line-per-
quiet-iteration output contract — the **console noise floor** — the role windows adopt,
because a binary cannot gate what the model prints between tool calls.

**Three console output classes (a role window emits the multi-line ones ONLY when their
class is present; otherwise it earns one line):**

1. **Actionable** — multi-line, always printed: a new finding/verdict, a needs-decision
   item, an error/refusal (tool exit 3/4/5/6), or a question to the human. This is the
   signal; it is never compressed to one line.
2. **State change** — full tables printed ONLY when the board changed (a `--delta` sweep
   surfaced added/removed/field-changed rows) or on explicit request ("show me the board").
   On no change, see class 3.
3. **Quiet iteration** — ONE line when nothing actionable happened: timestamp, the surface
   swept, the delta count (`Δ 0` or `Δ +N`), and the next wake. Per-item progress narration
   ("checked PR #N, still CHANGES_REQUESTED…") goes NOWHERE — the mandatory audit line
   (C-5) already records the run, and restating it on the console is pure noise.

A quiet loop looks like:
`11:02Z swept prs (oit) — Δ 0 — next wake 11:12Z`

A state-change loop prints the quiet line's `Δ` segment replaced by the changed rows from
`--delta`, plus any actionable class. The contract is on the CONSOLE only: PR/issue thread
posts are a separate channel and remain fine when useful (issue-flow/10 scope correction).

Two properties of `deskboard`'s modes the window can rely on:

- **The quiet line's `Δ` is an unread badge, not a consumed one.** A `--quiet`-only sweep
  does not advance the snapshot, so drilling in with `--delta` after a non-zero `Δ` still
  shows the rows — the quiet loop cannot eat its own signal. The badge keeps accumulating
  until a `--delta` run renders it.
- **Both modes report transitions, not standing state.** A row that has been actionable for
  days is silent after its first sighting, so a window owing the class-1 "always print
  actionable" duty needs a periodic full sweep (`--table`, or `deskboard actions`) beside
  the quiet loop — `--delta`/`--quiet` alone cannot satisfy it.

## Tool suite

| Tool | Verb(s) | Replaces (prompting today) | Brief |
|------|---------|---------------------------|-------|
| `deskboard` | cross-repo board (PRs/actions/reviews/queue) + `diff`/`files` reads for reviewers + STALE/audit banners + `--delta`/`--quiet` console-discipline modes on `prs`/`queue`/`nextup` (issue-flow/10) | `gh -R` polling AND reviewer diff reads; skill-dir deskboard.go v1 | 02 |
| `deskpost` | `review --head` / `comment` / `ready` as the reviewer App | `gh pr comment/review/ready`, `mint-reviewer-token.go` | 03 |
| `deskpr` | `create` (draft-only) + `update` (follow-up push to own open draft) | worker `git push -u` + `gh pr create --draft` + iteration pushes | 04 |
| `deskwt` | worktree `add`/`remove` under allowed prefixes | `git worktree add/remove` | 05 |
| `deskreply` | worker-identity PR replies (NOT the App — the two voices must never share a tool) | worker `gh pr comment` during the fix→re-review cycle | 07 |
| (shared) | `deskkit`: config, audit+lock, kill switch, rate limit, version, secret-scan | — | 01 |

Worker-local `git commit` / `git merge origin/main` (keep-current) mutate nothing remote and are
handled as plain allowlist rules at cutover, not binaries (the write/read line above).

## Rollout

Wave 0: 01. Wave 1: 02, 03, 04, 05, 07 (parallel, all on 01). Wave 2: the **gate: human
cutover 06** (depends on all five tools, 02–05 + 07; effort M): human:<name> runs `sudo make desk-install`; allowlist swap (add the five
`/opt/desk-tools/bin/*` rules + the worker-local git rules; REMOVE the `go run …/deskboard.go`
rule); **purge/shrink `settings.local.json`** (enumerate its ~1,179 rules; wide execution/write
rules removed — this is a deliverable, not a footnote); skills wired to the tools (with raw-
command fallback documented for exits 3/5/6); pre-written one-command ROLLBACK patch (re-add
old rules + revert skill edits) committed before the swap; drills: zero-prompt full cycle
(desk half AND worker half), kill-switch <5s, refusal spot-check, binary-manifest verification,
settings-diff check; TM-1(a/b/c) residual sign-off. Critical path: **01 → 03 → 06**.
Maintenance owner: the process desk (Bob) via the methodology stream; extraction to
assay-toolkit is a later candidate, noted not planned.

## Out of scope

- Per-session permission enforcement (the harness has none; TM-1's residual follows).
- statusgen changes beyond none; sandbox configuration.
- Any merge/close/un-draft/deploy capability — merge remains human:<name>'s, always.
- Filtering committed code content beyond the best-effort diff scan (TM-2 residual).
