---
id: DR-desktools-v2-10
date: "2026-09-21"
title: "One outbound-write check at the forge write seam, keyed on the target's visibility, with layered overrides: credential, personal-data and self-containment refusals overridable with the audited flag; a withheld identifier on a public or unknown target and a ruling claim never"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Everything overridable, audited (option 2) — ruled out: put to the driver on #1319 and not taken; the ruling selected option 1. Under option 2 a withheld identifier reaches a public repository on any session's written reason, and the audit row, whose identity is self-reported, is only how it is found afterwards."
  - "Nothing new overridable (option 3) — ruled out: put to the driver on #1319 and not taken. Personal-data and withheld refusals would be hard stops everywhere, the option most likely to strand work on a false positive such as a phone-shaped id, and a block with no way through has pushed work off the sanctioned transport before."
  - "Hold for more design of the layers or the visibility key (option 4) — ruled out: not taken; the ruling selected option 1 and the same ruling approved the stream spec the design comes from."
accepted:
  - "Every outward write a desk tool makes passes one function, `deskkit.OutboundCheck`, before it leaves the machine. It refuses with a rule id and a location and never edits the author's text."
  - "The check sits where every write already goes: the Forge that `ResolveForge` returns is wrapped, so every text-carrying method is checked, and the push path (deskpr before it pushes, and the deskpushguard pre-push hook) checks the branch name, every commit message and the added lines."
  - "For every target it runs the credential scan, the ruling-claim guard and a personal-data pass (e-mail addresses and phone-number shapes; forge no-reply addresses and reserved example domains are allowed). For a public target, and a target whose visibility the roster does not state, it also runs the self-containment categories and the configured withheld register."
  - "Credential, personal-data and self-containment refusals may be overridden with the one audited flag every write verb takes; each override leaves an audit row holding the rule id and a digest of the text, never the text."
  - "A withheld-identifier refusal on a public or unknown target is not overridable from any verb: the way through is to reword, or for a human to change the configured withheld set. A ruling-claim refusal is never overridable."
  - "The check moves existing scans; it drops none. Every refusal a verb's own scan made before the change, the shared check still makes."
---

**Ruling recorded (2026-09-21): option 1 — layered overrides.** The question was put as the
four options in the brief's `## Human decision`
(`docs/streams/desktools-v2/brief-10-one-outbound-write-check.md`): (1) layered overrides;
(2) everything overridable, audited; (3) nothing new overridable; (4) hold. The driver chose
option 1 in the desk session, and the desk relayed that choice to the brief's decision issue,
[issue #1319](https://github.com/medici-finance/assay/issues/1319). The driver then ratified it
under the driver's own login, never a role App, in the
[ratifying comment](https://github.com/medici-finance/assay/issues/1319#issuecomment-5753805199)
(2026-09-21T00:17:19Z). The same ruling flipped `docs/streams/desktools-v2/spec.md` to
approved, and the brief records it as "Ruled 2026-09-21 (#1319): option 1 — layered
overrides." This record transcribes that ruling into the register; it does not mint a new one.
The ratifying comment does not name this record's id, so the record carries no `ruling:` link
(one would fail the corroboration check as `record-not-named`). Its approval is corroborated
the way the register's other placeholder records are: by the driver's own approval of the pull
request that lands this file.

**The decision.** One outbound-write check, run before a write leaves the machine, replaces
the per-verb scans. Its layers are chosen by the target's configured visibility, read from the
roster with no network call. A target the roster does not list is treated as public. The ruling
settles the part the brief's `gate-why` puts to the human: which refusals the audited override
may pass. Credential, personal-data and self-containment false positives are real, so those
stay overridable. A withheld identifier reaching a public repository is the failure the check
exists to prevent, and the override's identity is self-reported, so no written reason lets
one through.

**The constraint behind it.** The brief's `single-point-of-failure` line names the check
itself as the one control. Three layers stand around it. First, structure: `ResolveForge` is
the only construction site, and a ban on naming a backend type outside `deskkit` keeps any verb
from holding an unchecked Forge. Second, completeness: a test walks the `Forge` interface and
fails when a text-carrying method is not routed through the check. Third, out of band: a
deployment's own merge-gate sweep over the merged tree catches on a different signal, in a
different component, after the push.

**What this record does not decide.** It does not change the configured withheld set or any
roster visibility entry. Those stay human edits to configuration. It does not decide the
follow-on work that `desktools-v2/11` unblocks. It does not, by itself, prove the approver is a
different identity from the brief's author (`registers-v1.md` §7.4). And it does not attest the
design is correct: that is the brief's Verify rows and its mutation entry.
