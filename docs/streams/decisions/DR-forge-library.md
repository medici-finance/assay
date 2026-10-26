---
id: DR-forge-library
date: "2026-10-25"
title: "One shared forge library for reads and writes: the desk verbs and deskread become thin wrappers, statusgen links it for its forge calls, the library owns the per-role-and-repo credential, gh/glab launches are ratcheted to at most 7 and git uses go-git; supersedes DR-forge-neutral-36"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2395#issuecomment-6404972196"
alternatives:
  - "Keep DR-forge-neutral-36 (library-first for offline and frozen inputs only; every online statusgen read stays on the deskread verb) — superseded by the ruling, which names it. It kept two forge transports alive: the in-process seam inside desk-tools and a process bridge for statusgen."
  - "Promote deskkit to an importable package wholesale — not chosen. The ruling asks for one library with a credential no caller can read; exporting deskkit would publish its custody, guards and every internal helper as API, which the non-CI target architecture rules out (a catch-all package or deskkit wholesale)."
  - "Keep each command as the integration boundary and add a forge service process — not chosen. The ruling makes the command-line tools thin wrappers around a linked library; a service boundary for credential custody remains future work and is not delivered here."
accepted:
  - "desktools-v2 builds one shared forge library for reads and writes. deskread and the write commands (deskpost, deskpr, deskreply, deskfile, deskevidence, deskrun and the rest) become thin command-line wrappers around it."
  - "statusgen links the library for its forge calls instead of running a desk-tools command. A credential is therefore present inside the statusgen process, held only in the library's private field; this reverses DR-forge-neutral-36's rule that no credential moves into a process that has none today."
  - "The library owns the credential. Each client is constructed for one role and one repository, the token is held in a private field no caller can read, and there is never a fallback to an ambient token."
  - "DR-desktools-v2-03, DR-forge-neutral-34, DR-forge-neutral-14, DR-forge-neutral-17 and DR-forge-gitlab-11 move into the library unchanged."
  - "gh/glab: at most 7 launch sites across desk-tools and statusgen, held by a ratchet that only goes down. Every other forge call goes through the library."
  - "git: go-git. A git launch is allowed only where go-git cannot do the operation, on its own down-only ratchet."
  - "DR-forge-neutral-36 is superseded. Its record stays in the register; work already delivered under it keeps its Evidence."
---

**Ruling recorded (2026-10-25): one shared forge library for reads and writes.** The driver's
ruling, posted under the driver's own login on #2395 and not edited since, reads in full
(verbatim; the record only wraps its lines):

> Supersedes DR-forge-neutral-36.
> 1. One shared forge library for reads and writes. desktools-v2 builds it. deskread and the
>    write commands (deskpost, deskpr, deskreply, deskfile, deskevidence, deskrun and the rest)
>    become thin command-line wrappers around it.
> 2. statusgen links the library for its forge calls, instead of running a desk-tools command.
> 3. The library owns the credential. Each client is set up for one role and one repo. The
>    token is held in a private field that no caller can read, and there is never a fallback to
>    an ambient token. These rules move into the library unchanged: DR-desktools-v2-03 (an App
>    token minted for the repo), DR-forge-neutral-34 (the CI read-only workflow-token opt-in),
>    DR-forge-neutral-14 and -17 (run and retry credentials refuse a human-bound entry), and
>    DR-forge-gitlab-11 (the read-only auditor identity for the hardening guard).
> 4. gh/glab: at most 7 launch sites across desk-tools and statusgen, held by a ratchet that
>    only goes down. Every other forge call goes through the library.
> 5. git: use go-git. A git launch is allowed only where go-git cannot do the operation, on its
>    own ratchet.

Ruling: https://github.com/medici-finance/assay/issues/2395#issuecomment-6404972196

This record transcribes the ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4). The human act is the driver's comment, not this file. No rationale
was given with the ruling, and this record supplies none beyond what the five points state.

**What it supersedes.** [`DR-forge-neutral-36`](DR-forge-neutral-36.md) (2026-10-08) limited
library-first reuse to offline and frozen inputs, kept every online statusgen forge read on the
`deskread` verb, kept `deskkit` internal and kept credentials out of statusgen. Points 1 to 3 of
this ruling replace each of those limits. The earlier record stays in the register with a
superseded note; work delivered under it (forge-neutral/36, the `forgeread` module) keeps its
Evidence and is reused, not rewritten.

**Carried rules.** The five records named in point 3 keep their text. Where they name a
desk-tools mechanism, the mechanism now lives in the library:

- [`DR-desktools-v2-03`](DR-desktools-v2-03.md): reads authenticate only as a minted App
  token scoped to the repository being read; an unminted or empty token is refused with no
  ambient fallback. Its references to `deskkit.ForgeFor` now read as the library's client
  constructor; the rule is unchanged.
- [`DR-forge-neutral-34`](DR-forge-neutral-34.md): the CI workflow-token read transport, an
  explicit CI-only, read-only opt-in beside the App custody default.
- [`DR-forge-neutral-14`](DR-forge-neutral-14.md) and [`DR-forge-neutral-17`](DR-forge-neutral-17.md):
  the run and gate-approval verbs behind a roster run-credential binding that refuses a
  human-bound entry, and the widened run-log read with retry kept behind that same binding.
- [`DR-forge-gitlab-11`](DR-forge-gitlab-11.md): guard-read custody, the session role for
  roster reads and a dedicated read-only auditor identity for the one enumerated
  hardening-read operation.

**Placement (desk default, amendable at desktools-v2/75's human gate).** The ruling names the
owner (desktools-v2) and the rules, not a path. The plan places the library in a new
repository-root Go module, `forgeclient/`, beside `forgeread/`, `gitquiet/` and `deskcore/`:

- statusgen cannot import anything under `tools/desk/internal/`, so the library and the role
  credential provider it owns (desktools-v2/58's planned `tools/desk/internal/credential/`)
  must live outside that tree.
- It composes `forgeread`'s existing adapters. The `forgeread` root package stays
  dependency-free and offline (forge-neutral/36 row 11 unchanged), so a consumer that links only
  the root package still acquires no transport or credential. This is why the plan does not put
  the client in further packages of the `forgeread` module, as the draft forge-service amendment
  (`docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md`) suggested.
- It is a narrow, versioned package with named consumers (the desk-tools write verbs,
  `deskread` and statusgen), not a catch-all package and not `deskkit` exported wholesale
  (`docs/proposals/non-ci-architecture/11-target-architecture.md`).

**Operation surface.** Callers see enumerated typed operations and typed results, never a raw
URL, request, GraphQL document or token. This is the same shape the forge-service amendment
gives its role executor, so a verb can later move between in-process and over-a-channel without
an API change. Moving credential custody out of the session into such a service is future work;
this ruling does not deliver it.

**Briefs.** desktools-v2/75 builds the library and turns the verbs into wrappers, with the
gh/glab ratchet. desktools-v2/76 owns the go-git migration and the git-launch ratchet.
desktools-v2/77 retires the `deskread` bridge for statusgen. Each is `gate: human` and cites this
record with its `design:` key. forge-neutral/18 rows 21 to 23, forge-neutral/35's transport
paragraph and desktools-v2/08 row 10 are amended to this direction.

**The consequence level.** `consequence: major` is not in the ruling comment, which states no
level. It is a desk default declared in the pull request that adds this record, amendable by the
maintainer.

**What this record does not decide.** It signs off none of the three briefs; each keeps its own
human decision. It does not choose the module path finally (see Placement). It does not say which
7 gh/glab launch sites remain, or whether skill shell scripts outside desk-tools count toward
the 7; desktools-v2/75 measures and proposes, and its human gate decides. The only credential
holder it authorizes is the library's private field; it grants no new identity, no widened
token scope and no service. It does not attest the design is
correct: it records that the alternatives were weighed and names a human approver.
