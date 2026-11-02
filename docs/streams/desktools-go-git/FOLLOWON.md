# desktools-go-git — the named follow-on

Brief 08 turned the git-exec counter into a ratchet pinned at **205** sites (51 direct
spawns + 154 seam call sites, measured on main `c2ac8845a9ea`): the count may fall but
may not grow. Getting from 205 toward the deskmerge fence is follow-on work, in three
parts. This file names that work. It authors no brief; choosing and sequencing the
migration briefs is a separate decision.

## 1. Remove the git binary from the agent environment

The boundary [spec.md](spec.md) always named, unchanged:

- **Agent commit verbs.** Agents run `git add` / `commit` / `checkout` themselves inside
  the linked worktrees `deskwt` creates. Dropping the binary there needs a desk-tool verb
  family that covers the agent's own commit loop.
- **A linked-worktree replacement.** go-git has no linked-worktree support (no `git
  worktree add`, list, lock or prune). Agent-facing worktrees stay real linked worktrees
  while agents still run `git`; replacing them is a design brief of its own. Op family 21
  (`worktree`) stays open until then, deskmerge's scratch worktrees included.
- **verifyloop's `pull --rebase` race.** go-git supports neither rebase nor a
  non-fast-forward pull. The durable-Evidence push race needs a redesign (regenerate on
  the new tip, or the contents API); the push half already runs on `gitcore.Push`.

## 2. Desk tools written after the inventory freeze

Brief 01 froze 25 op families across the tools that existed then (baseline 117 sites).
Briefs 03-07 migrated that inventory. Tools and features written afterwards added git
work that no migration brief owned, which is why the count stood at 206 rather than near
the fence. By tool, at the pin (direct spawns + seam call sites):

| Tool | Direct | Seam | Sites | What the sites do |
|---|---:|---:|---:|---|
| `deskwt` | 0 | 73 | 73 | `runGit(` throughout: role init, worktree create/lock/prune, branch collision, adopt-finished, transport checks, stale registrations, branch GC (`log -p` / `patch-id`), `status --porcelain`. Mostly the linked-worktree family (part 1) |
| `cellctl` | 19 | 16 | 35 | `git.go` (its own direct spawns and `gitOut(` helper), `reset.go` worktree listing and branch enumeration, `cell.go` git-dir probe, `deskd.go` origin URL read |
| `deskkit` (internal) | 5 | 12 | 17 | dispatch-record config read, verifier tracking checkout and attestation, preflight, kill switch, forge resolution, publish-identity reads, credential-helper chain |
| `deskevidence` | 5 | 11 | 16 | the decision gate (`decisiongate.go`), the receipt gate (`receiptvalidate.go`) and the flip path (`flip.go`); one seam line (`github.go`) is a part-3 over-count |
| `deskmerge` | 0 | 11 | 11 | `runGit(` lines around the trial merge: scratch worktree add/remove/prune (family 21), merge and abort, conflict listing, push; see brief 07 |
| `verifyloop` | 5 | 0 | 5 | verdict run, native dispatch, pending reads |
| `deskdispatch` | 0 | 5 | 5 | dispatch worktree setup, worker packet HEAD read |
| `deskpushguard`, `deskgit` | 0 | 8 | 8 | foreign-commit read and `ls-remote` head check; deskgit's remaining seam lines. One deskpushguard line launches `gh` (part 3) |
| `deskpathguard` | 1 | 2 | 3 | re-derive helper (`rederive.go`); one seam line (`forge.go`) is a part-3 over-count |
| `commsloop` | 2 | 0 | 2 | native dispatch |
| `scanloop` | 1 | 0 | 1 | lane read |
| `deskclaim` | 1 | 0 | 1 | the claim liveness probe (`liveness.go`) |
| `deskclaim-ref` | 1 | 0 | 1 | `gogit.go` |
| other single sites | 7 | 0 | 7 | `fanoutloop` board, `deskreconcile`, `deskrebaseline`, `deskmonitor`, `deskdisable`, `cellscratch`, `loopengine` |
| `gitversion`, `regression` (internal) | 2 | 0 | 2 | the git version probe; a regression fixture environment |

The named post-freeze tools are **cellctl, verifyloop, commsloop, deskevidence's decision
and receipt gates, scanloop, deskclaim's liveness probe and deskwt**. The rest of the
table is listed so a migration brief can scope against the full set, not to claim each
row is post-freeze. The remaining 18 sites are the `gh` and role-mint seams, the generic pass-throughs and the two guard fixtures described in part 3.

## 3. Over-counts the ratchet carries

The counter is textual. Two kinds of counted line launch no `git` process in production.
Both stay counted: narrowing the pattern is a reviewed change of its own, and brief 08
does not widen any exclusion to lower the number.

- **`execCommand(` matches non-git seams.** 26 counted lines use `execCommand(`. Of those,
  5 launch `git` (`deskwt` `status --porcelain` and its branch-GC `log`/`patch-id`
  pair, `deskgit`, `deskpushguard`'s foreign-commit read). 14 never launch git: the role
  mint helper in `deskclose`, `deskrestamp`, `deskfile`, `desklabel`, `deskevidence`,
  `deskdisposition`, `repohardenguard`, `deskpathguard` and `deskrelease`; `gh` in
  `deskdisposition`, `deskadvisory` and `deskpushguard`; `ssh -G` in `deskwt`; and
  `deskadvisory`'s check-tool runner. The other 7 are generic `execCommand(name, args...)`
  pass-throughs (`deskclose`, `deskflip`, `deskmerge`, `deskpr`, `deskboot`, `deskreply`,
  `deskdigest`) whose callers decide the program; each needs reading before it is
  classed.
- **Guard fixtures.** `tools/desk/internal/gitquiet/testdata/guard/wrapper/wrapper.go` and
  `tools/desk/internal/gitquiet/testdata/guard/wrappernotests/wrapper.go` hold one `exec.Command("git", ...)` each. They are
  fixtures for the gitquiet guard, not production code, but they are not `_test.go`
  files, so they count.

A narrowed pattern would lower the pin by up to 16 (14 non-git seams + 2 fixtures) in the
same change that narrows it, and must show the narrowed counter still catches the
mutation probe.
