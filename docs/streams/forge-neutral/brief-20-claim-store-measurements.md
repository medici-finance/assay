---
brief: assay:assay:forge-neutral:20
title: Measurements — what the reviewer writes, who reads claims, and what the file store can know about where it runs
why: >-
  The plan to move dispatch claims off the forge and drop the reviewer's repository write
  rests on things nobody has measured: that the claim is the reviewer's only repository write,
  that every claim reader is known, and that the file store can tell when it has been pointed
  at a network filesystem or started inside a container, where it is not supported. A wrong
  assumption here becomes a silent double-dispatch or a guard that never fires. This brief
  turns the spec's could-not-check rows into dated reads before code is built on them.
wave: 1
depends: []
unblocks: ["forge-neutral/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them"
  - "tools/desk/cmd/deskclaim/main.go:30-35 — the recorded reason dispatch claims left the machine-local directory on 2026-08-13"
  - "tools/desk/internal/deskkit/claim.go:217-323 — the shipped exclusive-create-under-lock primitive whose guarantees rows S2 and S4 are about"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): two sweeps (every reviewer-role forge write; every claim reader) whose misses are invisible unless the sweep method itself is re-derivable"
domain: complicated
version: 1
id: e90319ae-edc8-4dc2-8127-f9e609c4704c
---

# Brief 20 — Claim-store measurements

## Context
files:
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §3.1 gains two inventory tables; §3.3 rows S2 and S4 move from could-not-check to
  a dated result.
- `changelog/<branch-slug>.md` (planned) — the per-PR fragment.

This brief writes no tool code. It is documentation backed by reads on **fixtures** the
measuring operator owns.

Withdrawn from the first draft by ruling (spec §10 D, H): the shared-container-volume
measurement (containers use the served store, permanently) and every forge-behaviour read
(the forge-ref store is being removed, so nothing is built on them).

facts:
- Reviewer-role forge calls are constructed through `ForgeFor(repo, "reviewer")` or a reviewer
  token mint. Known sites at `c67cc371`: `tools/desk/cmd/deskpost/`, `tools/desk/cmd/deskflip/`,
  `tools/desk/cmd/deskdispatch/dispatch.go:872-918` (the claim),
  `tools/desk/cmd/deskclose/superseded.go`, `tools/desk/cmd/desklabel/`.
- Known claim readers outside the claim tool (spec T8): `tools/desk/cmd/desksupervise/live.go`,
  `tools/desk/cmd/deskpost/claimliveness.go`, `tools/desk/cmd/fanoutloop/land.go`,
  `tools/desk/internal/loopengine/writescope_io.go`, `tools/desk/cmd/deskroster/` (list join),
  and skill bodies that tell the reader to `git ls-remote` the claim namespace.
- A result row is: filesystem or platform, question, exact command, observed outcome, date.
  A row that could not be measured stays **COULD-NOT-CHECK** with the reason.
- The file-store race probe is: N processes each attempt an exclusive create of the same path
  under the shipped directory lock; the row records how many reported success. Exactly one is
  the only passing count.

## Ground rules
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief may start while the spec is `draft` — it is the measurement the spec's approval
  leans on. It changes no code.
- Record commands and outcomes only. No credential value enters the document.
- Do not convert a could-not-check into a result by reasoning or from memory. If the
  filesystem or platform is not available, the row stays could-not-check and says what is
  missing.

## Task
1. **Reviewer write inventory.** For every desk verb that can act as the reviewer role, list
   each forge operation and the permission it needs on each forge; mark read / pull-request
   write / issue write / repository write. Expected answer: "repository write: the dispatch
   claim only". Any other row is a finding — file it and name it in the PR body.
2. **Claim reader inventory.** Every site that reads, lists or releases a dispatch claim
   without going through the claim tool, with file:line and what it does when the namespace is
   empty. This is brief 22's work list.
3. **S2 — network filesystems.** Run the race probe on a local disk (the control) and on any
   network filesystem you can reach from a plain host process. Record filesystem type, mount
   options, N, and the success count.
4. **S4 — what the tool can know.** On each supported OS you can reach, record whether a Go
   process can (a) determine the filesystem type of a directory and (b) determine that it is
   running inside a container or pod, and by what read. "Cannot be determined on this OS" is a
   result. (b) decides where the spec's §6 container guard can refuse. (a) decides no refusal:
   by ruling (spec §10 L) a network filesystem is a NOTICE on every boot, never a refusal, and
   (a) fixes which of the two NOTICE wordings — type named, or type not determined — each OS
   can print.
5. Update the spec's §3.3 status column. Where a result bears on the ruling recorded at the
   spec's §10 L — for example a network filesystem on which the race probe does not yield
   exactly one success — say so at the top of the PR body. Do not edit the ruling; a measured
   contradiction is the driver's to weigh.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | `2` — both work-list rows are still present |
| 2 | check +dereference | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | exit 0 and no line printed — each row quotes a probe count or a determination result, or says COULD-NOT-CHECK |
| 3 | gate:model +dereference | Re-run the local-disk control of Task 3 from the recorded command text | exactly one of N succeeded; a different count is checked-failed on this brief |
| 4 | check | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the reviewer write inventory |
| 5 | check | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the claim reader inventory or is the claim tool itself |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A reviewer-role write is missed and the narrowed reviewer later fails at that verb | row 4 |
| A claim reader is missed and goes blind when claims leave the forge | row 5 |
| A network-filesystem row is filled from general knowledge and presented as measured | row 2 (a measured row must carry a count) and row 3 |
| Container or filesystem detection is reported as possible on an OS where it was never tried | review-only — each S4 row names the OS and the read it used |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---|---|---|---|---|
| 1 | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | 0 | `2` | 2026-09-18 | worker (implementer) |
| 2 | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | 0 | (no line printed) | 2026-09-18 | worker (implementer) |
| 3 | `cd tools/desk && go test ./internal/deskkit/... -run '^TestAcquireConcurrentExactlyOneWinner$' -count=1 -v -timeout 60s` | 0 | `--- PASS: TestAcquireConcurrentExactlyOneWinner (0.68s)` — 16 racers, exactly 1 acquired (the test's own `wins != 1` assertion) | 2026-09-18 | worker (implementer) |
| 4 | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | 0 | `tools/desk/cmd/deskdispatch/dispatch.go`, `tools/desk/cmd/deskpost/claimliveness.go`, `tools/desk/cmd/deskpost/comment.go`, `tools/desk/cmd/deskpost/forgeclient.go`, `tools/desk/cmd/deskpost/label.go`, `tools/desk/internal/deskkit/modelstamp.go` — all six appear in §3.1a | 2026-09-18 | worker (implementer) |
| 5 | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | 0 | 14 files (`tools/desk/cmd/deskclaim-ref/claim.go`, `tools/desk/cmd/deskclaim-ref/main.go`, `tools/desk/cmd/deskdispatch/dispatch.go`, `tools/desk/cmd/deskdispatch/main.go`, `tools/desk/cmd/deskpost/claimliveness.go`, `tools/desk/cmd/deskpost/forgeclient.go`, `tools/desk/cmd/desksupervise/actions.go`, `tools/desk/cmd/desksupervise/live.go`, `tools/desk/cmd/fanoutloop/land.go`, `tools/desk/internal/deskkit/claimref.go`, `tools/desk/internal/deskkit/forge_gitlab.go`, `tools/desk/internal/deskkit/forge.go`, `tools/desk/internal/gitcore/claimref.go`, `tools/desk/internal/loopengine/writescope_io.go`) — all accounted for in §3.1b (two are the claim tool itself, two are comment-only mentions, two are library/definition files, the rest are readers) | 2026-09-18 | worker (implementer) |

Supplementary reads behind rows 1–3 (not separate Verify rows, cited in §3.3):
- S4(a) darwin: `syscall.Statfs_t.Fstypename` on this worktree → `"apfs"`; on the one reachable network mount (a local NFS mount) → `"nfs"` (read-only `statfs(2)`, macOS, arm64).
- S4(a) linux / S4(b): inside a locally-available Linux container image (no network pull), `uname -a` → `Linux … x86_64`, a Go probe found `statfs("/").Type = 0x794c7630` (overlay, named via Linux's own magic-number table, not a stdlib field), `/.dockerenv` present, `/proc/1/cgroup` = `"0::/\n"` (cgroup v2, no docker/kubepods substring), `/run/.containerenv` absent.
- S2 network-filesystem attempt: `mkdir`/`touch` against the NFS mount above both → `Permission denied`, despite the mount's ownership matching the measuring account's uid. Recorded as COULD-NOT-CHECK, not as a filesystem defect.

### Non-implementer verifier run — VERIFY: FAIL — 4/5 pass, 0 could-not-check, 1 fail — 2026-09-24 claude-opus-4-8-verifier

Runner is not the implementer; a documents-only diff run non-hermetically on darwin at merged main
`2a5c230efe9c29f17b9acf6aa798e3a0a2285fbb`, offline (`KUBECONFIG=/dev/null`). No row is `check:ci`,
so each is decided on its direct result. Row 5 FAILs as a check-definition staleness, filed at #1606.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---|---|---|---|---|
| 1 | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | prints `2` — both work-list rows present | PASS — exit 0; printed `2` (both work-list rows present) | 2026-09-24 | claude-opus-4-8-verifier |
| 2 | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | exit 0, no line printed — each row carries a count / determination / COULD-NOT-CHECK | PASS — exit 0; no line printed (each row carries a count / determination / COULD-NOT-CHECK) | 2026-09-24 | claude-opus-4-8-verifier |
| 3 | `cd tools/desk && go test ./internal/deskkit/... -run 'TestAcquireConcurrent.*OneWinner' -count=1 -v -timeout 60s` | exactly one of 16 racers acquires (the test's own `wins != 1` assertion) | PASS — exit 0; the exactly-one-winner concurrency test reports --- PASS (0.51s), 16 racers, one winner, its `wins != 1` assertion held. The brief's row-3 Verify cell is prose, so statusgen verifyrun reported row 3 could-not-run (exit 127); this is the recorded local-disk control of Task 3 run directly, and it passed | 2026-09-24 | claude-opus-4-8-verifier |
| 4 | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the §3.1a reviewer write inventory | PASS — exit 0; 6 files — deskdispatch/dispatch.go, deskpost/claimliveness.go, deskpost/comment.go, deskpost/forgeclient.go, deskpost/label.go, deskkit/modelstamp.go — all six present in §3.1a; inventory conclusion "repository write — the dispatch claim only" holds | 2026-09-24 | claude-opus-4-8-verifier |
| 5 | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the §3.1b claim reader inventory or is the claim tool itself | FAIL — exit 0; 16 files at merged main, but THREE are absent from §3.1b: deskdispatch/repairadmission.go, desksupervise/status.go, deskkit/forge_github.go. Expected condition NOT met → row FAILS. (All three match only on a comment/interface-doc line, not a functional read; and forgeclient.go, which §3.1b DOES list, no longer matches the grep — the pinned inventory has diverged from merged main.) | 2026-09-24 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — winners = 1 @ tools/desk/internal/deskkit/claim_test.go:53 (`if wins != 1`) — an exclusive create under one directory lock (O_CREAT|O_EXCL semantics of the shipped primitive) admits exactly one creator; all other racers fail EEXIST. So exactly one of N succeeds by construction, independent of N. This is the "exactly one is the only passing count" the brief pins, and it is correct.
RISK-VALUE: N/A for the remaining enumerated literal — racers = 16 @ tools/desk/internal/deskkit/claim_test.go:22 is a reversible test knob (the racer count N); raising or lowering it changes only the strength of the concurrency exercise, not the guarantee, and needs no derivation. No other literal, threshold, tolerance, timeout, or authority binding is introduced or changed by this documentation-only diff; the quoted Linux magic numbers (overlay 0x794c7630, NFS 0x6969, CIFS/SMB) are cited facts, not controls this brief pins.

Row 5 fails as a check-definition staleness: the pinned §3.1b inventory misses three comment-only mentions. Filed as #1606.

**Evidence correction (2026-09-24).** Row 5's Observed says the three files absent from §3.1b match only on a comment line, but its recorded `grep -rln` prints only filenames, not the lines behind them. The command that supports the claim is `grep -n -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' tools/desk/cmd/deskdispatch/repairadmission.go tools/desk/cmd/desksupervise/status.go tools/desk/internal/deskkit/forge_github.go`; re-run at the verified sha `2a5c230e` it prints three matches, each a `//` comment rather than a functional read: repairadmission.go line 219 ("lives in the same refs/dispatch namespace as the item claims, collides"), forge_github.go line 2167 ("refs/dispatch/<key>), verbatim from the ref field"), and status.go line 122 ("refs/dispatch claim, so it is session-influenced input"). No state, count or heading changes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `Re-run the local-disk control of Task 3 from the recorded command text` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:26b6e0457462 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | pass exit=0 | sha256:5c3e60f74ab7 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | pass exit=0 | sha256:d2556591d929 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run — VERIFY: FAIL — 4/5 pass, 0 could-not-check, 1 fail — 2026-09-27 claude-opus-5-5-verifier

Runner is not the implementer; documents-only brief, run non-hermetically on darwin at merged main
`b227b40768db08a0a91046899bc1877cf3c6d1ec`, offline (`KUBECONFIG=/dev/null`). The witness table
above records exit status only; rows 3 and 5 carry prose Expect cells the witness cannot decide, so
each is decided below on its direct result.

What changed since the 2026-09-24 runs (at `2a5c230e`): this brief and the spec are byte-identical
(unchanged hashes), and so are repairadmission.go and status.go. forge_github.go and
deskpost/forgeclient.go changed (unrelated commits), which moved the forge_github.go comment from
line 2167 to line 2259. #1606 is still open and §3.1b was not updated, so row 5's outcome is unchanged.

- Row 1 — PASS. exit 0, printed `2`.
- Row 2 — PASS. exit 0, no line printed; the S2 row carries COULD-NOT-CHECK for network filesystems, the S4 row carries "determined".
- Row 3 — PASS (run directly; the witness row is exit 127 only because the Verify cell is prose). The recorded local-disk control command from the S2 row, `cd tools/desk && go test ./internal/deskkit/ -run '^TestAcquireConcurrentExactlyOneWinner$' -count=1 -v -timeout 60s`, gave exit 0 and `--- PASS: TestAcquireConcurrentExactlyOneWinner (0.16s)`: 16 racers, exactly one winner (the test's `wins != 1` assertion held).
- Row 4 — PASS. exit 0; 6 files (deskdispatch/dispatch.go, deskpost/claimliveness.go, deskpost/comment.go, deskpost/forgeclient.go, deskpost/label.go, deskkit/modelstamp.go). A mechanical membership check against §3.1a found all six.
- Row 5 — FAIL. exit 0; 16 files. A mechanical membership check against §3.1b found 13 and missed the same three as on 2026-09-24: deskdispatch/repairadmission.go, desksupervise/status.go and deskkit/forge_github.go. Running `grep -n` with the same three patterns on those files prints only `//` comment lines: repairadmission.go:219 (the admission lease "lives in the same refs/dispatch namespace", and that lease is taken through the claim tool backend), status.go:122 (a security comment on the claim Owner field), and forge_github.go:2259 (the doc comment of the generic MatchingRefs primitive, the GitHub twin of forge_gitlab.go, which §3.1b already lists as a library file). Not one of them is a claim read that bypasses the claim tool. **Stale-shaped, check-definition:** the pinned inventory has drifted from merged main. No claim reader was missed. #1606 still tracks it.

Risk-bearing values: the enumeration covered this brief's Deliverables and diff, which is documents only (the spec §3.1a/§3.1b/§3.3 plus the Evidence). It found two literals, both unchanged since 2026-09-24:

RISK-VALUE: DERIVED — wins != 1 (winners = 1) @ tools/desk/internal/deskkit/claim_test.go:53 — the shipped primitive's O_CREATE|O_EXCL create (claim.go:273), taken under one directory lock, admits exactly one creator, and every other racer gets EEXIST and backs off. So exactly one of N succeeds by construction, whatever N is. That is the "exactly one is the only passing count" the brief pins.
Ranked last, so no verdict line: racers = 16 @ tools/desk/internal/deskkit/claim_test.go:22 is a reversible test knob. It only sets how hard the concurrency exercise pushes; the guarantee does not depend on it, and it needs no derivation.

VERIFY: FAIL — row 5 (check-definition staleness, #1606); rows 1–4 PASS.

### Non-implementer verifier re-run: 2026-10-02T21:25:38Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 5a108baba705c5f9b458501065fc52a12557c969

Runner is not the implementer. Documents-only brief, run non-hermetically on darwin at the merged
main above, offline (`KUBECONFIG=/dev/null`); the Go test ran under a throwaway HOME. Rows 4 and 5
carry prose Expect cells, so each was decided by a mechanical membership check of the printed file
list against the spec's §3.1a / §3.1b text, not by exit status.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---|---|---|---|---|
| 1 | `grep -c -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md` | `2` — both work-list rows are still present | PASS — exit 0; printed `2` | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `grep -e '^. S2 . ' -e '^. S4 . ' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn20-rows.txt && ! grep -v -e 'of [0-9][0-9]* succeeded' -e 'determined' -e 'COULD-NOT-CHECK' /tmp/fn20-rows.txt` | exit 0 and no line printed | PASS — exit 0; no line printed (S2 row carries "1 of 16 succeeded" and COULD-NOT-CHECK for the network filesystem; S4 row carries "determined") | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | Re-run the local-disk control of Task 3 from the recorded command text — the S2 row records it as `cd tools/desk && go test ./internal/deskkit/... -run '^TestAcquireConcurrentExactlyOneWinner$' -count=1 -v -timeout 60s` | exactly one of N succeeded | PASS — exit 0; `--- PASS: TestAcquireConcurrentExactlyOneWinner (0.12s)`; 16 racers, the test's `wins != 1` assertion held. The Verify cell itself is prose, so the recorded command was executed directly | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && grep -rln -e 'ForgeFor(.*"reviewer")' -e 'ReviewDispatcherRole' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the reviewer write inventory | PASS — exit 0; 6 files (deskdispatch/dispatch.go, deskpost/claimliveness.go, deskpost/comment.go, deskpost/forgeclient.go, deskpost/label.go, deskkit/modelstamp.go); all 6 found in §3.1a | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && grep -rln -e 'ClaimRefsPrefix' -e 'ClaimRefPath' -e 'refs/dispatch' --include='*.go' cmd internal \| grep -v _test.go \| sort` | every file listed appears in the claim reader inventory or is the claim tool itself | FAIL — exit 0; 16 files; 13 found in §3.1b, 3 absent: deskdispatch/repairadmission.go, desksupervise/status.go, deskkit/forge_github.go. Expected condition not met | 2026-10-02 | assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Execution witness** (`statusgen verifyrun --brief`, tool v1.0.31, at 5a108baba705): 4 of 5 rows
pass on exit status — row 1 pass (sha256:53c234e5e847), row 2 pass (sha256:e3b0c44298fc), row 3
could-not-run exit=127 (sha256:26b6e0457462; the Verify cell is prose, the shell has nothing to
execute), row 4 pass (sha256:5c3e60f74ab7), row 5 pass (sha256:d2556591d929). All five output
hashes are identical to the 2026-09-27 witness, so the row 4 and row 5 file lists are byte-for-byte
what they were then. The witness's row 5 pass is exit-status only and does not decide the prose
Expect; the membership check above does.

**What changed since the 2026-09-27 verdict (at b227b40768db), and why the verdict has not.**

- The spec (reviewer-write-boundary.md) is byte-identical: sha256 8317b9eae90a…, last touched by
  #1562. §3.1b was not updated.
- repairadmission.go, status.go, claim.go and claim_test.go are unchanged.
- Changed on main since: deskdispatch/main.go, deskpost/claimliveness.go, comment.go and
  forgeclient.go, deskkit/claimref.go, forge.go, forge_github.go, forge_gitlab.go and
  modelstamp.go, and gitcore/claimref.go. None of these edits adds or removes a file from either
  grep's output.
- Row 5, previously failing: still fails, same three files. `grep -n` with the row's three patterns
  on those files prints one line each, every one a `//` comment: repairadmission.go:219 ("lives in
  the same refs/dispatch namespace as the item claims"), status.go:122 ("refs/dispatch claim, so it
  is session-influenced input"), forge_github.go:2377 (doc comment, "refs/dispatch/<key>), verbatim
  from the ref field" — moved from line 2259 by unrelated edits). No claim read that goes around
  the claim tool was found; the pinned inventory has drifted from merged main. Check-definition
  staleness, tracked by #1606, which is still OPEN.
- Row 3, previously could-not-run in the witness: unchanged in the witness (prose cell, exit 127);
  passes when the recorded command is executed directly, as on both earlier runs.

**Risk-bearing values.** Enumeration covered this brief's Deliverables and diff (documents only:
spec §3.1a, §3.1b, §3.3, plus Evidence). Two literals, both unchanged since 2026-09-24:

RISK-VALUE: DERIVED — wins != 1 (winners = 1) @ tools/desk/internal/deskkit/claim_test.go:53 — the shipped primitive creates the claim with O_CREATE|O_EXCL (claim.go:273) under one directory lock, which admits exactly one creator; every other racer gets EEXIST. Exactly one of N succeeds by construction, whatever N is — the "exactly one is the only passing count" the brief pins.

Ranked last, no verdict line: racers = 16 @ tools/desk/internal/deskkit/claim_test.go:22 — a reversible test knob that sets only how hard the concurrency exercise pushes.

VERIFY: FAIL — row 5 (check-definition staleness: §3.1b omits three files whose only match is a code comment; #1606 open); rows 1–4 PASS.

### 2026-10-02 desk dispatch — re-verify at merged main 03065900ca21: 4/5 pass, row 5 still fails (check-definition staleness, #1606)

Non-implementer verifier re-run, 2026-10-03T UTC, assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), at merged main 03065900ca21 (confirmed against the forge's main head the same turn). Documents-only brief, run non-hermetically on darwin, offline (KUBECONFIG=/dev/null); the Go test and the witness ran under a throwaway HOME. Rows 4 and 5 have prose Expect cells, so each was decided by a mechanical membership check of the printed file list against the spec's section 3.1a / 3.1b text, not by exit status. Long tokens are abbreviated below: "the spec" = reviewer-write-boundary.md in this stream; the row 3 test is TestAcquireConcurrent...OneWinner (name shortened); file paths are given relative to tools/desk.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 as written: grep -c for the S2 and S4 row prefixes in the spec | `2` | PASS — exit 0; printed `2` | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (row file written to a scratch path instead of the shared tmp dir; same greps) | exit 0, no line printed | PASS — exit 0; no line printed (S2 carries "1 of 16 succeeded" and COULD-NOT-CHECK for the network filesystem; S4 carries "determined") | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | The local-disk control recorded in the spec's S2 row: cd tools/desk and go test ./internal/deskkit/... with -run on the exactly-one-winner race test, -count=1 -v -timeout 60s | exactly one of N succeeded | PASS — exit 0; `--- PASS` for the race test (0.11s), package ok; 16 racers, the test's `wins != 1` assertion held. The Verify cell itself is prose, so the recorded command was executed directly | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written: grep -rln for the reviewer ForgeFor call and ReviewDispatcherRole over cmd and internal, minus tests, sorted | every file listed appears in the reviewer write inventory | PASS — exit 0; 6 files (deskdispatch/dispatch.go, deskpost/claimliveness.go, deskpost/comment.go, deskpost/forgeclient.go, deskpost/label.go, deskkit/modelstamp.go); all 6 found in section 3.1a | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as written: grep -rln for ClaimRefsPrefix, ClaimRefPath and refs/dispatch over cmd and internal, minus tests, sorted | every file listed appears in the claim reader inventory or is the claim tool itself | FAIL — exit 0; 16 files; 13 found in section 3.1b, 3 absent: deskdispatch/repairadmission.go, desksupervise/status.go, deskkit/forge_github.go. Expected condition not met | 2026-10-03 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (statusgen verifyrun --dry-run, tool v1.0.31, throwaway HOME, at 03065900ca21): row 1 pass (sha256:53c234e5e847), row 2 pass (sha256:e3b0c44298fc), row 3 could-not-run exit=127 (sha256:26b6e0457462; the Verify cell is prose, so the shell has nothing to execute), row 4 pass (sha256:5c3e60f74ab7), row 5 pass (sha256:d2556591d929). All five output hashes are identical to the 2026-09-27 and 2026-10-02 witnesses, so the row 4 and row 5 file lists are byte-for-byte unchanged. The witness's row 5 pass is exit-status only and does not decide the prose Expect; the membership check above does.

What changed since the 2026-10-02 verdict (at 5a108baba705, landed by #2065), and why the verdict has not:

- The wake receipt's hashed inputs that changed on main: deskkit/forge.go, deskkit/forge_github.go and deskkit/forge_gitlab.go, all by #2046 (deskmerge App-token reads: cross-repo pull-request head/base detection and a walk-to-end comment read). #2063 (sops-block refusal) touched none of the receipt's inputs. Every other hashed input, including the spec (sha256 8317b9eae90a...) and claim_test.go, is byte-identical. The brief file changed only by the Evidence block #2065 appended.
- None of the #2046 edits adds or removes a claim-ref read; neither grep's output changed (same witness hashes).
- Row 5 still fails on the same three files. grep -n with the row's three patterns on them prints one line each, every one a code comment: repairadmission.go:219 ("lives in the same refs/dispatch namespace as the item claims"), status.go:122 ("refs/dispatch claim, so it is session-influenced input"), forge_github.go:2435 (doc comment, "refs/dispatch/<key>), verbatim from the ref field"; moved from line 2377 by #2046). No claim read that goes around the claim tool was found; the pinned section 3.1b inventory has drifted from merged main. Check-definition staleness, tracked by #1606, which is still open. The earlier blocker holds unchanged.
- Row 3 is unchanged: could-not-run in the witness (prose cell), PASS when the recorded command is executed directly, as on every earlier run.

Risk-bearing values. Enumeration covered this brief's Deliverables and diff (documents only: spec sections 3.1a, 3.1b, 3.3, plus Evidence). Two literals, both unchanged since 2026-09-24:

RISK-VALUE: DERIVED — wins != 1 (winners = 1) @ deskkit/claim_test.go:53 — the shipped primitive creates the claim with O_CREATE|O_EXCL (deskkit/claim.go:273) under one directory lock, which admits exactly one creator; every other racer gets EEXIST. Exactly one of N succeeds by construction, whatever N is: the "exactly one is the only passing count" the brief pins.

Ranked last, no verdict line: racers = 16 @ deskkit/claim_test.go:22 — a reversible test knob that sets only how hard the concurrency exercise pushes.

VERIFY: FAIL — row 5 (check-definition staleness: section 3.1b omits three files whose only match is a code comment; #1606 open); rows 1–4 PASS; status stays implemented.

## Review
Gate: **model** (from frontmatter — all four risk answers no; documents only). Reviewer records
verdict + date in the stream README table.
