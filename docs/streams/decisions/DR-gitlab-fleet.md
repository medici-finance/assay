---
id: DR-gitlab-fleet
date: "2026-10-25"
title: "GitLab fleet provisioner ported to its own Go command (deskgitlab): the brief 30 compatibility report is approved as listed, and the landed command carries explicit maintenance ownership and cross-command parity obligations"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Reject or narrow the compatibility report (option 2 on the brief's decision issue) — not taken: the driver chose option 1, approving the report as listed in #3155."
  - "Consolidate into deskfleet through a separately reviewed compatibility-preserving migration, then retire deskgitlab and reroute the shell adapter — not chosen here: it stays the reopening path named in obligation 5, and the ownership question it belongs to is still open on #2987."
  - "Retire the shell script before porting it, the option that predates the merged implementation — not available: the port has landed, and this record cannot silently delete or retire the landed command."
accepted:
  - "The migration lands with exactly the compatibility changes listed in the \"Compatibility report\" section of #3155's description: the legacy flag forms kept, the deliberate help changes, the one environment credential source, and the refusal-side changes listed there. Approval covers only those listed changes; nothing unlisted is authorised, and silence never authorises a weaker check."
  - "The landed `tools/desk/cmd/deskgitlab` command and the `tools/create-fleet-gitlab.sh` adapter onto it are kept, under the maintenance and parity obligations below."
  - "This record is separate from the ownership question on #2987, from independent verification of brief 30 and from final human acceptance; none of those is decided here."
---

## State and provenance

**Ruling recorded (2026-10-25): option 1 — approve the compatibility report.** The question was
put as the two options in the brief's `## Human decision`
(`docs/streams/desktools-v2/brief-30-port-the-gitlab-fleet-provisioner-to-a-go-command.md`):
(1) approve the report; (2) reject or narrow it. The driver answered on the brief's decision
issue, [issue #4398](https://github.com/medici-finance/assay/issues/4398), under the driver's
own login, in the
[ruling comment](https://github.com/medici-finance/assay/issues/4398#issuecomment-6399372132)
(2026-10-25T15:14:42Z), which reads:

> desktools-v2/30: 1 — approve the compatibility report in medici-finance/assay#3155.

The driver supplied no further rationale; none is attributed here. This record transcribes that
ruling into the register; it does not mint a new one. The brief records the ruling in its own
frontmatter (`decision-issue: 4398` and a `ruling:` link to the comment above) and cites this
record as its `design:`.

The ruling comment names the brief by its board id, not this record by its `DR-gitlab-fleet`
id, so this record carries no `ruling:` link of its own: one would fail the corroboration check
as `record-not-named` (`spec/registers-v1.md` §7.5). Corroborating this record's own stamp needs
a ruling comment that names `DR-gitlab-fleet`; until one exists, this record claims only what
the ruling above says.

The implementation that the report describes is
https://github.com/medici-finance/assay/pull/3155 (merged 2026-10-16). The ownership decision
between this separate command and the existing `deskfleet` remains open on
https://github.com/medici-finance/assay/issues/2987; this record does not rule it, and it does
not retrospectively authorise the earlier dispatch or merge while that issue was unruled.

## Bounded direction

Keep `tools/desk/cmd/deskgitlab` as a supported provisioner and preserve the shell create
adapter, under the provisional maintenance contract below, until the ownership question on
#2987 is ruled.

The desk-tools component maintainer owns the command, adapter and parity contract together. The
worker desk routes defects and future edits to that owner; the review desk checks the
cross-command contract; independent verification remains with the verify desk. A command-local
implementer cannot declare cross-command parity by inspecting only its own handler. These are
role obligations, not an assignment to an unnamed individual.

## Maintenance and parity obligations

1. Keep the provisioner and renewal role tables aligned with their canonical shared definitions. Any edit to a shared role, membership, scope or renewal table must enumerate both `deskgitlab` and `deskfleet` consumers, update parity controls and show both consumers' results.
2. Preserve the shell create adapter's declared flags, help, output and exit ordering, host defaults, paid-tier/avatar settings, generated-token modes and safe output-directory behavior. Enumerate adopter docs and install/adopt skill references whenever that contract changes.
3. Preserve environment-only owner-token input for this command, no token material in argv/help/errors, generated-token and writer-identity file permissions of 0600, and the existing no-redirect REST boundary. This direction does not authorize widening `deskfleet` custody or moving secrets between custody models.
4. A parity failure blocks accepting the affected contract change. Fix the mismatch or obtain a specifically scoped human contract ruling; never silently weaken a parity assertion, omit a consumer or treat one command's success as evidence for both.
5. At the next proposed consolidation, renewal-owner change, script retirement or token-custody change, reopen ownership explicitly with a consumer migration and compatibility report. No date-based automatic retirement, new provisioning authority, release or live operation follows from this choice.

## Approval boundaries and follow-up

The ruling approves the compatibility report exactly as listed in #3155 — token scope,
custody-file permissions, idempotency and secret-free surfaces included as that report states
them — and nothing beyond it. Independent verification of brief 30's Verify table and final
human acceptance remain separate and are not claimed here. The review follow-ups recorded on
#2987 (the out-dir mode-mask test covers group-write modes only; the writer-identity file's
0600 mode has no test) remain work to inspect; this record does not declare those tests
present or passed.
