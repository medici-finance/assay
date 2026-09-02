---
brief: windows-port/02
title: Portability audit — enumerate + triage the shell-assuming surfaces
why: >-
  The Go tools already run on Windows; the glue around them does not, and until someone
  enumerates exactly which surfaces assume a POSIX shell, a `~/.config` home, or a `/`-path,
  every downstream brief guesses. This audit is the map the install path (03), the CI leg (04),
  and the adoption doc (05) all read: it says, per surface, works / needs-port /
  documented-workaround / out-of-scope — so no one ports a surface that was already portable,
  and no one ships one that silently breaks.
wave: 0
depends: []
unblocks: ["windows-port/03", "windows-port/04", "windows-port/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): a brief that enumerates and triages the shell-assuming surfaces — hooks, guard scripts, desk verbs that shell out, path separators, ~/.config vs %APPDATA%"
  - "survey (2026-09-01 @ origin/main): 11 .sh files; hooks are #!/bin/bash + jq invoked as bash \"…\"; guards are COMPILED Go (portable) but pre-push is a #!/bin/sh shim execing /opt/desk-tools/bin/deskpushguard; ~64 exec.Command git/gh sites; config-home hardcoded ~/.config/assay in rosterconfig.go:253; expandHome (appconfig.go:34) handles only ~/ with no %APPDATA%/XDG branch; XDG branch exists only in publicbless.go"
  - "harness-portability/README.md: the sibling stream that measured statusgen/tools/** as harness-agnostic argv CLIs — this audit does NOT re-measure the binaries, it audits the delivery/glue layer around them"
  - "freshness-checked 2026-09-01 @ origin/main: no per-OS portability triage doc exists under docs/streams/windows-port or elsewhere"
consumers:
  - "docs/streams/windows-port/portability-audit.md: fixed-here (the new triage table is the deliverable)"
  - "docs/streams/windows-port/brief-03-install-path.md: follow-up windows-port/03 (the install fork weighs whether a shell installer even runs, from this triage)"
  - "docs/streams/windows-port/brief-04-windows-ci-leg.md: follow-up windows-port/04 (the desk-verb smoke picks a verb this triage classes as windows-runnable)"
  - "docs/adopting-assay.md: follow-up windows-port/05 (the adopter doc lifts the documented-workaround rows verbatim)"
---

# Brief 02 — Portability audit: enumerate + triage the shell-assuming surfaces

## Context

files:
- **create** `docs/streams/windows-port/portability-audit.md` (planned) — a triage table plus a
  short recommendation per class. This is a documentary deliverable; it changes no code.

facts:
- **The binaries are out of scope — do not re-audit them.** `statusgen` and everything under
  `tools/**` are plain Go argv CLIs (measured, `harness-portability`, 2026-08-07). They
  cross-compile and run on Windows (brief 01 proves it). This audit covers the DELIVERY + GLUE
  layer only.
- **The surfaces to triage, with their starting facts** (verify each against the live tree —
  the reviewer will):
  - **SessionStart hooks** — `plugins/assay/hooks/inject-resident-rules.sh` and
    `inject-board-state.sh`, both `#!/bin/bash`, depend on `jq`, invoked from
    `plugins/assay/hooks/hooks.json` as `bash "${CLAUDE_PLUGIN_ROOT}/hooks/…"`. Windows Claude
    Code has no `bash`/`jq` by default. **The `hooks.json` `bash "…"` command string is the hard
    edge** — it is the load-bearing SessionStart mechanism (the resident rules arrive through it).
  - **Push guard shim** — `tools/desk/hooks/pre-push`, `#!/bin/sh`, `exec
    /opt/desk-tools/bin/deskpushguard "$@"`. The GUARD is compiled Go (portable); the SHIM and
    the absolute `/opt/desk-tools/bin` path are POSIX-only. Installed via `core.hooksPath` into
    `.githooks/pre-push`.
  - **Install path** — `sudo make desk-install` → root-owned `/opt/desk-tools/bin`; a POSIX
    Makefile target. (Its replacement is brief 03; this audit records WHY it needs replacing.)
  - **Config home** — hardcoded `~/.config/assay/…` (`rosterconfig.go:253` const;
    `roster.env`, `apps.env`, `claims/`, `writeguard-shared-ok`). `expandHome`
    (`appconfig.go:34`) handles only a literal `~/` via `os.UserHomeDir()`; there is no
    `%APPDATA%` / `XDG_CONFIG_HOME` branch (one XDG branch exists only in `publicbless.go`).
    On Windows `os.UserHomeDir()` returns `%USERPROFILE%`, so `~/.config/assay` resolves to
    `C:\Users\<u>\.config\assay` — it *works*, but it is not the Windows-idiomatic `%APPDATA%`.
  - **Desk verbs that shell out** — ~64 `exec.Command("git"|"gh"|…)` sites. `git`/`gh` exist on
    Windows; the triage question is whether any site assumes a POSIX-only invocation (a `/bin/sh
    -c`, a `/`-rooted path, a `sh`-only pipeline).
  - **Path separators** — `filepath.Separator` is used in a few spots
    (`tools/desk/cmd/writeguard/guard.go`); the risk is hardcoded `"/"` string-building elsewhere.
  - **`/tmp`** — shell scripts default `${TMPDIR:-/tmp}` (e.g.
    `plugins/assay/scripts/inbound-monitor.sh`); Go code should use `os.TempDir()`.
- **The triage vocabulary is fixed at four tokens**, one per surface row:
  `works` (runs on native Windows as-is) · `needs-port` (must change to run on Windows) ·
  `documented-workaround` (cannot be made native cheaply; ship a stated manual step, e.g.
  "install Git-Bash for the SessionStart hook") · `out-of-scope` (not part of the Windows claim,
  e.g. the container entrypoint).
- **This is an audit, not a fix.** No surface is ported here; each row states the disposition and
  points at the owning brief (03 for install, 04 for CI, or a follow-up issue) where the port
  lands.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done, and you port nothing here.
- Every `works` / `out-of-scope` disposition needs a one-line REASON — an unreasoned token is not
  a triage.
- If a surface's disposition is genuinely undecidable without the install-fork ruling, record it
  as `needs-port (disposition depends on the install-fork ruling)` rather than guessing.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Sweep the repo for every shell-assuming / path-assuming surface in the delivery+glue layer.
   Minimum coverage: the six classes in `facts:` (hooks, push-guard shim, install path, config
   home, shell-out verbs, path separators + `/tmp`). Pair every absence-grep with a positive
   control.
2. Build the triage table in `docs/streams/windows-port/portability-audit.md` (planned):
   `| Surface | File(s) | Assumes | Disposition | Owner | Note |`, one row per surface, the
   Disposition drawn from the four-token vocabulary.
3. Add a short **Recommendations** section: which surfaces MUST port for the end state (the
   install path + whatever the CI smoke touches), which get a documented workaround (the `bash`+`jq`
   SessionStart hooks are the likely candidate), and which are out-of-scope (the container
   entrypoint, the `.test.sh` suites).
4. State the **config-home recommendation explicitly** — keep `~/.config/assay` (works on Windows
   via `%USERPROFILE%`) vs add a `%APPDATA%` branch — with a recommendation and the one-line
   consequence of each, feeding 03/05.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/windows-port/portability-audit.md; echo $?` | `0` |
| 2 | The four disposition tokens are all used: `grep -cF -e 'works' -e 'needs-port' -e 'documented-workaround' -e 'out-of-scope' docs/streams/windows-port/portability-audit.md` | `>= 6` — at least six disposition mentions across the table (each of the six surface classes carries one) |
| 3 | **Dereferencing — the audit's hook fact is TRUE against the tree** (not just asserted): `grep -q 'inject-resident-rules.sh' docs/streams/windows-port/portability-audit.md && head -1 plugins/assay/hooks/inject-resident-rules.sh \| grep -qF '#!/bin/bash' && grep -qF 'bash "' plugins/assay/hooks/hooks.json; echo $?` | `0` — the doc names the hook, and the hook really is `#!/bin/bash` invoked via `bash "…"` |
| 4 | **Dereferencing — the config-home fact is TRUE**: `grep -q 'rosterconfig' docs/streams/windows-port/portability-audit.md && grep -qF '.config/assay' tools/desk/internal/deskkit/rosterconfig.go; echo $?` | `0` — the doc cites the const, and the const really carries `~/.config/assay` |
| 5 | **Dereferencing — the push-guard shim fact is TRUE**: `grep -q 'pre-push' docs/streams/windows-port/portability-audit.md && grep -qF '/opt/desk-tools/bin/deskpushguard' tools/desk/hooks/pre-push; echo $?` | `0` |
| 6 | Every triage row names an owning brief or issue: `grep -cE -e 'windows-port/0[1-5]' -e 'follow-up' docs/streams/windows-port/portability-audit.md` | `>= 6` — each disposition points somewhere it is owned |
| 7 | **Positive control** — a fabricated surface is NOT in the audit: `grep -qiF -e 'winsock' -e 'registry hive' docs/streams/windows-port/portability-audit.md; echo $?` | `1` |
| 8 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/02; echo $?` | `0` — the `consumers:` claim (the audit doc is created here) is proved by the branch diff |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f docs/streams/windows-port/portability-audit.md; echo $?` | 0 | `0` | 2026-09-02 | implementer |
| 2 | `grep -cF -e 'works' -e 'needs-port' -e 'documented-workaround' -e 'out-of-scope' docs/streams/windows-port/portability-audit.md` | 0 | `17` | 2026-09-02 | implementer |
| 3 | `grep -q 'inject-resident-rules.sh' docs/streams/windows-port/portability-audit.md && head -1 plugins/assay/hooks/inject-resident-rules.sh \| grep -qF '#!/bin/bash' && grep -qF 'bash "' plugins/assay/hooks/hooks.json; echo $?` | **1** (expected 0) — see note below | `1` | 2026-09-02 | implementer |
| 4 | `grep -q 'rosterconfig' docs/streams/windows-port/portability-audit.md && grep -qF '.config/assay' tools/desk/internal/deskkit/rosterconfig.go; echo $?` | 0 | `0` | 2026-09-02 | implementer |
| 5 | `grep -q 'pre-push' docs/streams/windows-port/portability-audit.md && grep -qF '/opt/desk-tools/bin/deskpushguard' tools/desk/hooks/pre-push; echo $?` | 0 | `0` | 2026-09-02 | implementer |
| 6 | `grep -cE -e 'windows-port/0[1-5]' -e 'follow-up' docs/streams/windows-port/portability-audit.md` | 0 | `21` | 2026-09-02 | implementer |
| 7 | `grep -qiF -e 'winsock' -e 'registry hive' docs/streams/windows-port/portability-audit.md; echo $?` | 1 | `1` | 2026-09-02 | implementer |
| 8 | `statusgen --root . --consumers -brief windows-port/02` | 0 | consumers claim corroborated against the branch diff (this brief file + `portability-audit.md` both present) | 2026-09-02 | implementer |

**Note on row 3 (checked-failed, reported as itself — not rounded to pass):** the three-part
command fails at its third clause. Parts 1–2 pass (0): the audit doc names the hook, and the
hook file really is `#!/bin/bash`. Part 3, `grep -qF 'bash "' plugins/assay/hooks/hooks.json`,
does not match — dereferenced with `python3 -c "..."` byte inspection, `hooks.json`'s JSON
string literal is `bash \"${CLAUDE_PLUGIN_ROOT}/…` (backslash then quote, the JSON escape for
an embedded `"`), not `bash "` (space then quote with no backslash) — so the fixed-string
search for `bash "` never matches the raw file bytes at all, regardless of what this audit
records. This is an escaping bug in the Verify row's own grep pattern (written against the
JSON-decoded shell command string, not the raw JSON source bytes), not a defect in
`portability-audit.md`'s content — the underlying fact the row exists to prove (the hook is
`#!/bin/bash`, invoked from `hooks.json` via a `bash "…"` command string once JSON-decoded) is
independently true and is what the audit doc states. No code was changed to work around this
(fixing it would mean editing `hooks.json`, which is out of this brief's scope — a
documentary deliverable that changes no code). Flagged for the reviewer and for whoever
authors a follow-up correction to this brief's Verify row 3 (e.g. `grep -qF 'bash \\"'` or a
plain `grep -qF 'bash'` presence check).

## Review
Gate: **model** (from frontmatter). All four risk answers are `no` — this is a documentary
triage, no code changes. The dereferencing rows (3-5) are the point of the review: the reviewer
confirms the audit's load-bearing facts hold against the live tree, so a confident-but-wrong
triage cannot pass as a well-formed one. The reviewer also checks the four-token vocabulary is
used consistently and every disposition carries a reason and an owner.
