---
id: DR-forge-neutral-13
date: "2026-09-10"
title: "Route deskpr/deskfile/deskclose through the forge resolver under minted App custody, per the identities #781 confirmed"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "deskpr — keep the `--as-app=false` ambient-credential fallback (the transition-period path) — ruled out: the token-refusing backends cannot serve an ambient identity, so retaining the flag leaves a branch that silently cannot work; the fallback (flag + branch) is removed and a no-token run REFUSES at the custody step rather than falling through to an ambient CLI credential."
  - "deskfile — mint the `--raised-by` role's OWN App token (so the filer identity equals the attributed role) — ruled out: #781 confirmed the session-role App performs the write with `--raised-by` staying a body/label attribution, as it is today; minting the raised-by role's App was named in the brief's Task 4 for the human to weigh and was not adopted."
  - "Perform the migration without a human confirmation of the acting identity per verb — ruled out: the permit register calls routing these three verbs through the resolver a token-custody decision (it changes WHO performs each write), not a transport change, so it is a security judgment no model self-certifies; #509 authorized the follow-on to do the work and #781 is the gate that fixes the identity each verb assumes."
accepted:
  - "The `--as-app=false` ambient fallback is retired fleet-wide: an operator (or a repo) without a minted session-role App token loses `deskpr`'s write path until a token is minted, where before it could fall through to an ambient `gh` credential. This is the intended fail-closed posture, not a regression."
  - "`deskfile` is re-homed from a documented no-App-token-on-any-path design onto session-role App custody; its interim GitLab named-refusal (#691) is superseded now the backend serves GitLab, while the could-not-check refusal on an UNRESOLVABLE forge is retained."
  - "`deskclose`'s `viewer{login}` whoami is replaced by the minted role's known login; the blessing-authority id-pin (`IsBlessAuthorityIDStrict`) is preserved by extending `ListComments` to carry the author's numeric id, so no fail-closed / two-role / merged-not-closed invariant is weakened by the re-seat."
---

The driver (`human:<name>`) ratified `forge-neutral/13` — routing the last three
ambient-credential outward-write verbs (`deskpr`, `deskfile`, `deskclose`) through the
forge resolver under minted App-identity custody — at the brief's own human decision gate.

The ruling was recorded on the brief's decision-gate issue,
[issue #781](https://github.com/medici-finance/assay/issues/781), which the driver closed
`approve-as-briefed` (COMPLETED). Per the decision-gate template the issue offered approve
as-briefed, approve with changes, or hold/reject; the recorded close is option 1 — approve
as briefed — confirming, per verb, the acting identity the brief's `gate-why` proposes:

- `deskpr` → the session-role worker App (its `--as-app=true` default path; the ambient
  fallback is retired);
- `deskclose` → the session-role App (DESK_LOOP-selected);
- `deskfile` → the session-role App, with `--raised-by` staying a body/label attribution.

This closes the human gate the brief's frontmatter declares (`gate: human`, `risk:
{sensitive-data: yes}`). The constraint behind the decision is that these three verbs reach
the forge under an ambient CLI credential by documented design — `deskfile` mints no App
token on any path, and `deskclose`'s `exec.go` states it "gates WHETHER and WHAT, never
WHO" — so routing them through the resolver changes WHO performs each write, which the
permit register explicitly classes a token-custody decision rather than a transport change.
The human confirms that custody change; #509 (the amendment that rescoped these three out
of brief 04) authorized the follow-on to perform it.

**What this record does not decide.** It does not adopt the alternative deskfile identity
(minting the `--raised-by` role's own App) — that was named in the brief's Task 4 for a
separate ruling and is not taken here. It does not, by itself, prove the approver is a
different identity from the brief's author — the same attribution-not-identity limit
`lifecycle-v1.md` §7.1.2 declares for verification, and `registers-v1.md` §7.4 for this
register. And it does not attest the chosen design is correct: that the alternatives were
weighed is recorded here; whether the identity each verb now assumes is right is the review
gate's judgement, then the change's own validation after it lands.
