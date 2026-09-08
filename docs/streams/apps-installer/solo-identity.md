# Solo identity mode — spec of record

Solo is the pilot tier of the [`deskapps`](./design.md) installer: the ramp on which an adopter
experiences the whole Assay workflow with **zero GitHub Apps**. `deskapps` creates no App and exits
after Screen 0 (design §7), having written a roster that names the operator's own login. This
document is the spec that §7 defers to. It fixes *whether and how* the desk verbs run on that single
login, so the mode **refuses rather than degrades** wherever one login cannot stand in for the
separation the role Apps otherwise provide.

The decision behind this spec is settled: see [§ Human decision](#human-decision). This is the
design of record; the implementation brief is authored after — and cites — that ruling.

## 1. What Solo is

Solo replaces every role's minted App-installation token with **one credential: the operator's own
user token**, obtained once from `gh auth` at boot. The six desk roles do not disappear — they
become **labels on the body of what is posted**, not distinct GitHub identities. Every action shows
the operator's login in the audit trail, and the controls that a bot-per-role layout provides by
*attribution* are provided in Solo by two other things instead:

- **GitHub's own refusal to let a login approve its own pull request.** A review authored by the
  PR's own author can only be a `COMMENT`; GitHub rejects `APPROVE` / `REQUEST_CHANGES` on your
  own pull request. In Solo the operator is the author of every PR, so no self-approval is
  physically possible — the verdict lands as a comment and the operator decides.
- **The human merge gate.** Merging is the operator's in every tier; in Solo it is the only place
  the "second pair of eyes" can live, so the mode is explicit that the operator reads before merging.

Where neither of those is enough — a repository whose **ruleset** requires a *bot* identity for a
required check or a protected action — Solo **refuses** the action and says so, rather than acting
under a login the ruleset will reject or, worse, that satisfies a gate the ruleset meant a distinct
actor to satisfy. See [§6](#6-refusals).

## 2. Turning it on — env and roster shape

Solo is selected by naming the operator login and providing nothing else:

```
ASSAY_SOLO_LOGIN=<login>     # the operator's own GitHub login; presence of this var IS Solo mode
```

`ASSAY_SOLO_LOGIN` is the chosen switch for three reasons: it is a single unambiguous signal (a
mode, not an inferred state), it names the login the whole mode pivots on so the roster and the
boot line can echo it, and it is refused-when-empty like every other identity input (fail closed).

When `ASSAY_SOLO_LOGIN` is set, the installer writes a roster in which:

- `ASSAY_TRUSTED_LOGINS` contains `<login>` (the operator is a trusted human author).
- `ASSAY_HUMAN_LOGIN_MAP` maps the operator's name to `<login>` (so the operator counts as an
  accountable human for the public-review and own-PR-neglect axes).
- `ASSAY_TRUSTED_BOT_SLUGS` is **empty** — there are no bot identities in Solo, so there is nothing
  to trust as a bot, and leaving it non-empty would be folklore no App backs.
- `ASSAY_BLESS_LOGIN` is `<login>` — the operator is their own blessing authority.

Alternatives considered and rejected:

- **Reuse the existing `<ROLE>_TOKEN` env override for all six roles.** `desktoken` already lets
  `<ROLE>_TOKEN` point a role at a token file (`envOr(prefix+"_TOKEN", …)` in
  `tools/desk/cmd/desktoken/desktoken.go`); pointing all six at one user-token file *works today*
  but is invisible — no mode is declared, the boot line cannot say "you are in Solo", and the
  preflight cannot explain why its scopes check went dark. Solo needs a *named* mode, so the env
  override is the mechanism, not the switch. `ASSAY_SOLO_LOGIN` is the switch that makes the mode
  legible; under the hood it arranges the per-role token resolution to land on the one user token.
- **Infer Solo from "no App keys present."** Rejected: absence is not intent, and a half-provisioned
  Full-suite machine (keys still downloading) would masquerade as Solo. A mode must be declared.
- **A boolean `ASSAY_SOLO=1` without a login.** Rejected: the mode needs the login named to write
  the roster and to print it on every boot line; a bare boolean forces a second lookup.

## 3. Per-verb behaviour under Solo

Every desk **write verb** does exactly one of three things in Solo: runs on the operator's user
token, runs on the user token but carries the role as a **label** in the body/provenance stamp, or
**refuses**. No verb silently mints an App token (there is no App key to mint from).

| Verb | Under Solo | Identity shown | Note |
|---|---|---|---|
| `desktoken <role>` | Returns the operator's user token for every role (the `<ROLE>_TOKEN` path resolves to the one user-token file); mints nothing | operator login | No `.perms` sidecar is written for a user token — see [§5](#5-preflight). |
| `deskpost` | Runs on the user token; the role rides as a body label / provenance stamp, not as an identity | operator login | A verdict comment reads "role: reviewer" in its body; the actor is the operator. |
| `deskpr create`/`update` | Runs on the user token; opens the draft PR authored by the operator | operator login | The operator is the PR author, which is *why* the reviewer path downgrades to `COMMENT` ([§6](#6-refusals)). |
| `deskfile --raised-by <role>` | Runs on the user token; `<raised-by:role>` is stamped in the issue body as a label | operator login | The stamp records which role *voice* raised it; the account is the operator's. |
| `deskevidence` | Runs on the user token; Evidence rows are committed under the operator's git identity | operator login | The verifier is the operator, so evidence and the brief it verifies share one login — the human read at merge is the separation. |
| `deskflip` | **Refuses** the ready-flip that depends on a *bot* `APPROVE`; permitted only where the gate the flip satisfies does not key on a bot identity | operator login | In Solo no bot can approve, so a review-gated ready-flip has no bot verdict to key on; the operator readies (or merges) by hand. See [§6](#6-refusals). |

The rule the table encodes: **a role in Solo is a label, never an identity.** The token is always the
operator's; the role names the *voice*, and the body carries it so a later reader can still see which
desk function produced the artifact.

## 4. The trust gate under Solo

The trust gate (`tools/desk/internal/deskkit/trust.go`) is fail-closed and reads the roster from
outside every ref the tools evaluate. In Solo:

- **Accepted login classes:** the operator's login, admitted through `ASSAY_TRUSTED_LOGINS`
  (`TrustedAuthor`) and `ASSAY_HUMAN_LOGIN_MAP` (`TrustedPublicAuthor` / `TrustedHumanAuthor`), and
  the operator as their own `ASSAY_BLESS_LOGIN`. No other class is admitted.
- **`ASSAY_TRUSTED_BOT_SLUGS` holds nothing.** There are no role Apps, so the bot-slug set is empty
  and `TrustedPublicAuthor`'s `[bot]` / `app/` branches match nobody. This is deliberate and
  documented, not an omission: a non-empty bot set in Solo would name a trust no App backs.
- **`deskfile --raised-by <role>`** stamps `raised-by:<role>` as a body **label** on the filed
  issue. It attributes the *role voice*; it does not change the author, which stays the operator.
  The trust gate still evaluates the *author login* (the operator), never the label.
- **Public-repo auto-review** stays governed by the higher `TrustedPublicAuthor` bar. In Solo the
  operator qualifies as a mapped human, so their own diffs are eligible — but the operator is also
  the *author*, so the reviewer path can only comment ([§6](#6-refusals)); the trust bar and the
  self-approval refusal are independent controls that both hold.

## 5. Preflight

The desk preflight (`tools/desk/internal/deskkit/preflight.go`) reports every check as one of
checked-clean / checked-failed / **could-not-check**, and the third is never rounded up to a pass.
Under Solo, two checks are **could-not-check by construction**, and the boot line must say so rather
than print a reassuring green:

- **`app-scopes-vs-duties`** is **could-not-check**. It reads the granted scopes from the `.perms`
  sidecar written next to a *minted App token*; a user token has no `.perms` sidecar (nothing writes
  one), so there is no recorded grant to compare against the role's duties. The check reports
  could-not-check — reading a bare permission listing as a grant is exactly what preflight refuses
  to do — and the boot line states: *"Solo mode: app-scopes-vs-duties is could-not-check (a user
  token has no recorded grant); GitHub enforces the operator's own account scopes at call time."*
- **cold-mint** is **could-not-check** for the same root cause: there is no App key to mint from, so
  the mint step has nothing to exercise. The boot line says so; it is not a failure the operator can
  fix, so it is reported as itself, not as red.

Every other preflight check that does not depend on an App grant runs normally. The boot line for
Solo therefore names the mode, the operator login, and the two could-not-check checks together, so
that could-not-check reads as *"by design in this mode,"* never as *"unverified, might be broken."*

## 6. Refusals

Solo refuses — rather than degrades — in these cases:

- **A repository whose ruleset requires a bot identity.** If a branch-protection **ruleset** or a
  required status check keys on a specific bot login (a required `<slug>[bot]` review, a check that
  only a role App can post), Solo has no such identity and must **refuse** the action, naming the
  ruleset and the identity it expects. Acting under the operator's login here is not merely useless
  — if the operator's login *could* satisfy a gate a ruleset meant a distinct actor to satisfy, it
  would silently defeat the control. Refuse and tell the operator this repo has outgrown Solo.
- **Any attempt to `APPROVE` your own pull request.** GitHub refuses a login approving its own pull
  request; a review by the PR author can only be `COMMENT`. In Solo the operator authors every PR,
  so the review verb **downgrades to `COMMENT` and says so** in one line — it does not error, it
  posts the verdict as a comment and states that approval is withheld because GitHub does not permit
  a login to approve its own pull request. The ready-flip that would depend on an `APPROVE` is
  therefore never available in Solo; the operator readies and merges by hand after reading the
  comment.
- **`deskflip` on a review-gated flip.** With no bot able to `APPROVE`, a ready-flip that keys on a
  bot approval has no verdict to key on; `deskflip` refuses it and directs the operator to the
  comment-form verdict and the human merge gate.

A refusal in Solo is a signal, not a wall: each one names the exact control the mode cannot provide,
which is also the operator's cue that they have outgrown the tier ([§7](#7-exit-criteria)).

## 7. Exit criteria

Solo is a ramp; these are the signals, visible to the operator, that it is time to move to
**Read + Act** (or Full suite):

- **A repo refuses an action because its ruleset requires a bot identity** ([§6](#6-refusals)). This
  is the hard boundary: the repo cannot be driven from Solo at all, and the refusal says so.
- **Verdicts piling up as comments the operator must action by hand.** When the operator is spending
  the session posting and re-reading their own comment-form verdicts instead of running the loop,
  the "no bot to approve" cost has become the workload the higher tiers remove.
- **A desire to run loops while away.** Solo runs nothing without the operator (every loop is
  started by hand); the first time the operator wants a review verdict on a new PR head to appear
  without them, they want `<prefix>-act`.
- **The audit trail showing one login for author and reviewer where a reader needs to see two.**
  When "who reviewed this?" must resolve to someone other than the author, one login can no longer
  answer it, and only a distinct bot identity can.

The installer surfaces these as the "you may have outgrown Solo" note; the move itself is a
re-run of `deskapps` at the Read + Act tier, which is additive — the operator's own login keeps
merging and deciding exactly as before.

## Human decision

**The question.** Solo replaces every role token with the operator's own user token, so bot
attribution (one login per role) is gone and one login authors, reviews-by-comment, and merges. That
weakens the identity model the desk tools otherwise enforce and must not be introduced by an
implementer's judgement. The driver confirms the mode's boundaries — which verbs run under it, what
the roster records, and that it refuses on a repo whose ruleset expects a bot identity — before any
implementation code is authored.

**The options, and each one's consequence for the README tier table:**

| Option | Consequence for the tier table |
|---|---|
| **1. Adopt the spec as written.** Solo is a supported pilot tier exactly as specified above: user token for every role, roles as labels, GitHub's self-approval refusal + the human merge gate as the restored controls, could-not-check preflight lines by construction, and refusal on a bot-identity ruleset. | The **Solo** row stays as it is (0 bot identities; verdicts as the operator's comments; merging and every decision the operator's), and gains the explicit callout — carried by apps-installer/07 — that Solo means the operator does far more than in the other tiers. |
| **2. Adopt with named changes.** Same mode, but the driver alters a named boundary (e.g. a different switch than `ASSAY_SOLO_LOGIN`, a narrower verb set, or a stricter refusal rule). | The Solo row is amended to match the named change; the "what stays the operator's" column is re-scoped to the altered boundary. |
| **3. Reject Solo as a supported mode.** No zero-App tier ships; the pilot ramp starts at Read + Act. | The **Solo** row is removed from the tier table and the "pilot ramp" language moves to Read + Act; the design's §7 becomes a recorded non-goal. |

**The standing ruling.** Decision issue **#467** records the driver's ruling on 2026-09-05:
**adopt Solo as specified (option 1)**, with the condition that the install page and runbook state
plainly that Solo means the operator does far more than in the other tiers — a callout carried by
apps-installer/07. This spec is written to that ruling and does not re-ask it; it is cited here as
the standing decision so the implementation brief that follows can point at both the ruling and this
spec.
