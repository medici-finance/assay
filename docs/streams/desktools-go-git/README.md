---
stream: desktools-go-git
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# desktools-go-git Stream

Move the desk tools' git operations off the external `git` binary and in-process onto
`go-git`, with the App installation token handed directly to the transport
(`BasicAuth{Username: "x-access-token"}`). The tools then carry **no git-binary
dependency and no credential-helper / `:443`-`insteadOf` machinery** — the whole
`git`-binary escape-hatch class (config-named program execution, env injection,
`insteadOf` substitution, hooks, `PATH` trust) collapses structurally rather than
being fenced vector-by-vector. This extends the in-process-over-HTTPS posture that
`desktoken` / `deskpost` / `deskevidence` / `deskrelease` already hold from the REST
API to the git pack transport.

The migration is a **seam-swap, not a rewrite**: each tool already routes git through a
single per-tool `exec.go` seam, so ~90 call sites across ~14 tools (25 op families)
move by swapping the seam behind behaviour goldens. One decided EXCEPTION —
`deskmerge`'s human-gated three-way trial merge, which `go-git` cannot express — stays
on the `git` binary as the sole sanctioned caller, fenced and documented. Fully
removing the binary from the **agent** environment (agent commit verbs +
linked-worktree replacement) is a named follow-on stream, out of scope here. See
[spec.md](spec.md) for the full thesis, decisions, and boundaries.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [inventory freeze + `gitexec` single-seam contract + golden harness + counting CI gate](brief-01-inventory-and-seam-contract.md) | 1 | L | todo | — | — |
| 02 | [`gitcore` transport + in-process auth (BasicAuth) + go-git pin](brief-02-gitcore-transport-auth.md) | 2 | L | todo | — | — |
| 03 | [migrate read/plumbing verbs (read-heavy tools)](brief-03-migrate-read-plumbing.md) | 3 | L | todo | — | — |
| 04 | [migrate `deskpushguard` detection reads (parity + mutation test)](brief-04-migrate-deskpushguard-reads.md) | 3 | M | todo | — | — |
| 05 | [migrate fetch + retire bespoke hardening (`deskgit` / `deskadvisory`)](brief-05-migrate-fetch-retire-hardening.md) | 4 | M | todo | — | — |
| 06 | [migrate push + retire ambient-credential machinery + preflight probe](brief-06-migrate-push-retire-ambient.md) | 4 | M | todo | — | — |
| 07 | [`deskmerge` exception — fence the trial merge, migrate the rest](brief-07-deskmerge-exception-fence.md) | 3 | M | todo | — | — |
| 08 | [flip the drop-the-binary CI gate + CVE floor + file the follow-on](brief-08-flip-gate-and-cve-floor.md) | 5 | M | todo | — | — |

## Critical path

`desktools-go-git/01` (seam contract + goldens) -> `desktools-go-git/02` (gitcore +
in-process auth — **human gate**, the dependency-tree security review) ->
`desktools-go-git/03` (read/plumbing migration) -> `desktools-go-git/05` (fetch +
retire hardening — **human gate**, the security-posture review) ->
`desktools-go-git/08` (flip the gate).

The real blocker at the head is **02**: nothing downstream proceeds until the new
`go-git` dependency tree and the in-process-token auth path clear their human security
review, and that same review posture gates **05** (retiring the two bespoke hardening
layers). Those two human gates — not the mechanical seam swaps — are the pacing items.

## Dependency waves

- **Wave 1** — `desktools-go-git/01` (no dependencies; the inventory freeze, the single
  `gitexec` fallback seam, the golden harness, and the initially-advisory CI grep).
- **Wave 2** — `desktools-go-git/02` (depends on 01; the human-gated `gitcore` +
  transport/auth layer that every migration builds on).
- **Wave 3** — `desktools-go-git/03`, `desktools-go-git/04`, `desktools-go-git/07`
  (all depend on 01 + 02; parallelizable — read-heavy tools, `deskpushguard` detection
  reads, and the `deskmerge` non-merge verbs + fence).
- **Wave 4** — `desktools-go-git/05`, `desktools-go-git/06` (depend on 02 + 03; the
  transport verbs — fetch/hardening-retirement and push/ambient-credential-retirement).
- **Wave 5** — `desktools-go-git/08` (depends on 03, 04, 05, 06, 07; flips the CI gate
  to failing, pins the CVE floor, and files the follow-on agent-verbs stream).
