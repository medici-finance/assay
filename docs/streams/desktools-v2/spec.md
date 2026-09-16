# desktools-v2 — scoping document

**Status:** draft — proposed 2026-09-16; not yet ruled. Per `spec/lifecycle-v1.md` §8.2 a
`draft` document is the plan of record for nothing and no downstream control watches it. The
stream that cites it (`docs/streams/desktools-v2/README.md`) is therefore `status: parked`:
its briefs are authored and kept, but shelved out of Next-up and never dispatched until a
human rules this document `approved` (§8.4 `draft → approved` rides the ruling PR) and flips
the stream `active`. `approved` is the human's call, not this session's.

> **Why `draft`, not the word "proposed".** `spec/lifecycle-v1.md` §8.1 defines the
> machine-readable header states as exactly `draft | approved | routed`. A `**Status:**`
> line whose first token is none of those leaves the document *unclassified* (legacy) and a
> conforming detector ignores it. `draft` is the methodology's token for "a working proposal,
> not yet ruled" — the thing "proposed" means in prose — so it is used here rather than an
> unclassified literal. Nothing is faked `approved`.

## 1. The problem — the forge abstraction leaks

The desk tools were written ad-hoc, one verb at a time, while the desk flows were still
changing week to week. That was the right call then: a tool built to a flow that is about to
change should not be over-architected. The flows have since solidified, and the ad-hoc
origin now shows as a recurring, single-shape bug: **GitHub-specific facts are hardcoded
*past* the forge seam, and each hardcoding generates its own defect.**

There already is a seam. `tools/desk/internal/deskkit/forge.go` declares one `Forge`
interface — the ~four-dozen operations a shipping desk tool consumes — and two complete
backends implement it: `forge_github.go` (seated on the official `go-gh` REST/GraphQL
client, handed an already-minted token, refusing an empty one) and `forge_gitlab.go`. The
interface is deliberately FROZEN at the operations a shipping tool consumes, with token
minting and budget/secret/rate wrapping held OUTSIDE it.

The defect is not the seam. It is that **callers reach around it.** Concrete, each its own
filed issue:

| Issue | The reach-around |
|---|---|
| #1145 | `cellctl gen_shims`' `HOME` override hides `gh`'s ambient credential from any shimmed verb that shells `gh`. |
| #1146 | `deskdispatch` attaches the token only to a child literally named `gh`, not to a script that itself shells `gh`. |
| #628 | an inherited `GH_TOKEN` forces ONE installation across every scan repo. |
| #1019 | `deskclose` hardcodes the `pullRequest` GraphQL query shape and so cannot target an issue. |
| #1201 | `deskpushguard` hardcodes the remote name `"origin"`. |
| #884 | the push-transport guard does not expand `insteadOf` / `pushInsteadOf`, so a rewritten URL evades it. |
| #305 / #834 / #838 | the forge-CLI shell-exec ban is a permit-register RATCHET (`tools/desk/internal/forgeban/allowlist.go`, `const allowedInvocationCeiling` — **now 5**, down from 24), not a closure-to-zero. |
| #1223 | the read path still shells `gh` where it could run a native installation-token client (the concrete pilot; see §4). |

The common thread is one sentence: **a `Forge` exists, but reaching around it is possible,
so it keeps happening.** Every row above is a place a GitHub fact (a subprocess name, a
remote name, a query shape, a token's scope) was written outside the two backends. v2's
thesis is to make each of those *structurally impossible to write*, not to fence them one at
a time.

## 2. What v2 is (and is not)

**v2 is the enforcement-and-native-client layer that makes reaching around the seam
impossible, plus the migration of the currently-unrouted leak sites onto it.** It is not a
second fork of the tools and not a rewrite of the two backends — both are complete and stay.

Four architectural commitments:

1. **ONE seam, and only the two backends may speak GitHub/GitLab.** Every transport,
   identity handoff, query construction, remote name, and subprocess name passes through
   `deskkit.Forge`. The GitHub REST/GraphQL host literal, the `gh` binary, the remote name
   `origin`, and a `pullRequest`-shaped GraphQL query are facts that may appear ONLY inside
   `forge_github.go` / `forge_gitlab.go` (and their tests).

2. **A ban-lint that makes the reach-around a red build, not a code-review catch.** A CI
   check that flags `gh`-subprocess names, a hardcoded `"origin"`, and `pullRequest`-shaped
   query literals anywhere outside the two backends. It generalizes the existing
   `forge-surface-control.yml` / `forgeban` permit-register (a ceiling) into a positive
   ban whose target is **zero**, and it ships advisory (counting) first so its baseline is
   recorded before it bites.

3. **A native installation-token client so no read shells `gh`.** `forge_github.go` already
   runs on `go-gh` with an explicitly-minted token and refuses an empty one; the reads that
   still shell `gh` (or a `gh`-shim, or a script that shells `gh`) route through the native
   client instead. This structurally retires the `HOME`-override credential-hiding class
   (#1145), the token-only-attaches-to-a-child-named-`gh` class (#1146), and the
   one-installation-forced-by-an-inherited-`GH_TOKEN` class (#628), because none of those
   vectors exists once the credential is a minted token handed to an in-process client.

4. **Incremental, tool-by-tool migration, with the old path REMOVED as each lands.** A verb
   is migrated onto the seam AND its reach-around deleted in the same change — never left as
   a dormant fallback. The ban-lint's counter is the progress metric; it reaches zero when
   the last reach-around is gone.

## 3. Boundary with the sibling streams — no duplication

Three streams already touch this surface. v2 does **not** re-author their work; it cites
them and owns only the gap none of them closes.

- **`forge-neutral`** (active) — makes the desk verbs the only sanctioned forge *write*
  path on GitHub *and* GitLab, and adds the resolver/custody that decides *which* forge and
  *which* identity performs a write (`forge-neutral/01`). Its progress metric is the
  `forgeban` ceiling (24 → 5 so far). **v2 depends on its resolver and never re-implements
  it.** Where a v2 brief needs "which forge / which identity", it consumes
  `forge-neutral/01`; v2's own contribution is the *ban* that makes a non-seam construction
  a red build, and the *native read client* forge-neutral's write-path scope does not cover.

- **`desktools-go-git`** (active) — moves the desk tools' *git* operations off the `git`
  binary onto in-process `go-git`, collapsing the git-transport escape-hatch class. The
  push-transport guard's remote-name and `insteadOf` gaps (#1201, #884) sit at the edge of
  its territory; v2 owns them from the *forge-assumption* angle (a guard that hardcodes a
  GitHub-shaped remote name is a reach-around), coordinates with `desktools-go-git` on the
  transport half, and does not migrate any git plumbing itself.

- **`desk-tools`** (active) — the general planning board for the current `tools/desk/`
  suite. v2 is the *architectural* successor initiative for the forge-abstraction slice
  specifically; routine current-tool briefs continue to live on `desk-tools`.

**Litmus for "belongs to v2":** the work makes reaching around the `Forge` seam impossible
(the ban-lint), removes a `gh` shell-out from a read (the native client), or migrates a
currently-*unrouted* forge-assumption leak site (#1145, #1146, #628, #1019, #1201, #884).
Write-path verb migration is forge-neutral's; git-transport migration is desktools-go-git's.

## 4. The pilot — #1223, native read-path client

#1223 ("retire `gh` shell-out in the read path for a native installation-token client") is
v2's first concrete migration and the proof the architecture pays. It is scoped to the READ
path deliberately: reads are idempotent, so the migration is provable against golden
outputs with no write risk, and the native client it stands up is the same one every later
write migration reuses. It is authored here as `desktools-v2/03`.

## 5. Definition of done for the stream

The stream is done when: the ban-lint is wired and **failing** (not advisory) with a count
of zero reach-around sites outside the two backends; every issue in §1's table is closed by
a landed migration whose old path was removed in the same change; and no read in the desk
suite shells `gh`.

## 6. Open questions for the approver

1. **Scope of the ban-lint's third pattern.** Banning a `pullRequest`-shaped GraphQL literal
   outside the backends is unambiguous for GraphQL; a REST path fragment is fuzzier. The
   proposal (brief-02) starts the pattern narrow (subprocess `gh`, remote `"origin"`,
   `pullRequest`/`mergeRequest` GraphQL blocks) and widens by evidence. Approve narrow-first,
   or specify the full pattern up front?
2. **Native-client boundary with forge-neutral.** v2's native read client and
   forge-neutral's resolver/custody meet at token minting. The proposal keeps minting in the
   identity layer (forge-neutral) and has v2 consume a minted token, matching `forge.go`'s
   existing contract. Confirm this split, or fold the read-client into forge-neutral?
3. **desktools-go-git handoff for #1201/#884.** The push-guard remote-name/`insteadOf` fix
   straddles both streams. The proposal has v2 own the forge-assumption half and coordinate
   the transport half. Confirm, or route #1201/#884 wholly to desktools-go-git?
