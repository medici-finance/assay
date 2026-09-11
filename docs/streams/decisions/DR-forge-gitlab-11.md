---
id: DR-forge-gitlab-11
date: "2026-09-11"
title: "Guard-read custody: deskroster reads as the session role; repohardenguard reads as a dedicated read-only auditor identity through one enumerated hardening-read op"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Pass the operator's ambient CLI credential into the Forge client (a `gh auth token` read, or a `glab` shell-out) — ruled out: it is exactly the lane the forge-gitlab custody work retires. Both backends refuse a client without an explicitly minted token BY DESIGN so that no desk action silently runs as whatever identity is active; a hardening run's evidence is only worth something when it names the role that produced it; and on GitLab the read would itself be the forge-CLI shell-out the ban forbids. The `--as-app=false` fallback was already retired fleet-wide for the write verbs on the same reasoning."
  - "Run repohardenguard as an existing desk role's token (desk / reviewer / worker) — ruled out: nothing is gained (those roles are not repository admins either, so the admin-gated fields stay unreadable) and least privilege is lost — a GET-only tool whose own header says every setting it reads belongs to the human would carry a credential that can post, file, flip and push. Retained for deskroster only, whose two reads are display annotations inside a window that already holds that role's token; a second identity there would be ceremony."
  - "Provision the auditor identity WITH administrative write rights so that ruleset bypass lists and the security-and-analysis block become readable — ruled out: it buys visibility of two field families at the price of a standing settings-changing credential on an operator's machine beside a GET-only tool. The guard's three-state rule already handles the alternative honestly: an admin-gated field a non-admin cannot see is could-not-check with a re-run-as-admin hint, never a pass and never an absence."
  - "Add a generic `Get(endpoint)` method to the Forge so the checklist's arbitrary `gh api <endpoint>` cells can be served unchanged — ruled out: it is the passthrough the closed surface exists to forbid; `TestForgeNoPassthrough` fails on it by name and by argument, and a path-shaped argument re-opens every endpoint by every method behind one call."
  - "Narrow the guard to only what BOTH forges expose (visibility, file presence, branch/tag protection presence) so one checklist serves both — ruled out: it weakens the GitHub-side control (secret scanning, workflow permissions, bypass lists, vulnerability reporting would stop being checked) to make the forges look symmetric. The stream's standing rule is the other way: never demote the layer a forge CAN enforce to match one it cannot; add the per-forge kinds instead and record what CE cannot express as `not available — <tier>` rows."
  - "Keep repohardenguard as a permitted CLI wrapper and amend forge-gitlab/08's Verify row 3 to a ratchet assertion (the proposal on the deskroster migration PR) — ruled out by the driver's ruling on the closure-to-zero question (option B): the residual shell-outs are GitHub-only and do not run on GitLab at all, so they are open work, not an accepted remainder."
accepted:
  - "One more forge identity per account, on both forges: a GitHub App with `Metadata: read`, `Contents: read`, `Administration: read` and no write permission, and a GitLab service account with a `read_api`-scoped PAT under the rotate-on-mint custody file contract. Provisioning it is an adopter step documented in the adopter guides; a fleet that has not provisioned it gets a Refused (exit 5) naming the custody file, never a fallback."
  - "Under the auditor identity the rows GitHub shows only to an admin or to a ruleset writer (`security_and_analysis.*`, `[name=…].bypass_actors`) report could-not-check and are re-run by a human administrator — the same outcome the guard gives a non-admin operator today. Coverage of those rows is not bought with a write grant."
  - "repohardenguard gains a roster CONFIG read (the repo→forge map) for forge resolution only; it still consults no write-authorisation set. Its checklist grammar changes: `read <kind>` / `read file <path>` replaces `gh api <endpoint>`, and the old form is a parse refusal naming the vocabulary — an adopter's checklist document must be re-cut when it re-pins."
  - "The hardening-read surface is per forge by construction: a kind returns the forge's own settings document, a checklist is authored per forge, and a kind the resolved forge does not serve is a named could-not-check. Nothing is approximated across forges; the GitLab kinds are a follow-on brief, and until it lands a GitLab-resolved run authenticates and reports every row could-not-check."
---

**Status at authoring (2026-09-11): PROPOSED.** This record is authored with the brief it governs
(`forge-gitlab/11`) so the alternatives are weighed BEFORE the human gate rather than reconstructed
after it. The `decided-by` stamp binds when the driver (`human:<name>`) records an option on the
brief's decision-gate issue; the recorded ruling and the issue number are then appended to this
body in the implementing change (the register is append-only — the file is amended, never
replaced). If the driver records an option other than the recommended one, this record is amended
to say which and why, and the brief's Task is re-cut against it before dispatch.

**The decision.** Three `gh` shell-outs remain in the desk tree after `forge-gitlab/08` shipped its
shell-exec ban as a ratchet, and the driver ruled on the closure-to-zero question (option B, 2026-09-11)
that they are open work: `deskroster`'s two display reads and `repohardenguard`'s GET choke point.
The permit register names the blocker as custody — routing an ambient-credential tool through the
seam "changes WHO performs the write, which is a token-custody decision, not a transport change".
The decision has two halves:

- `deskroster` reads (`GetPullRequest`, `ListOpenChanges` — both already enumerated) run as the
  SESSION's own role token, resolved from the loop identity the window presents. The migration
  landed as the open deskroster PR under exactly this shape; this record ratifies the shape.
- `repohardenguard` runs as a dedicated read-only **`auditor`** role: a seventh `desktoken` role,
  minted through the existing role-parameterised GitHub App path and read through the existing
  GitLab token-file path, whose forge-side grant carries no write scope. Its arbitrary
  `gh api <endpoint>` reads become ONE enumerated operation, `RepoHardeningRead(repo, kind)`,
  over a CLOSED kind vocabulary — each kind a fixed endpoint literal per backend, validated before
  a request exists, refused by name where a backend has no such document. File-presence rows
  route through the existing `ReadFile` op.

**The constraint behind it.** Two controls that fail in the quiet direction: a guard token whose
scope nobody drew (it reads settings today; it could change them tomorrow), and a read that
"works" by borrowing whichever login is active (it runs on the operator's laptop; it never runs
on GitLab). The design's answer is two independent layers: the enumerated surface — a kind, not
a path; a ratchet on shell-outs; a reflection check against the inventory — so the tool's own
code cannot be aimed at a write endpoint; and the forge's permission model — a read-only App /
`read_api` PAT — so a write attempted with the guard's token outside our code is refused by the
forge itself. The brief's negative-path Verify row proves the second with the first bypassed.

**What this record does not decide.** It does not close the six permit rows that remain after
this brief and the deskroster PR (advisory, digest, dispatch, disposition, merge, push-guard);
they invoke the CLI through wrappers the closure row's grep does not see, each carries its own
named blocker in the register, and each needs its own custody answer — this record's shape (the
session role for in-window reads; a per-purpose narrow identity for an out-of-window tool) is a
precedent for them, not a ruling. It does not fix the GitLab kinds (the follow-on brief). It does
not, by itself, prove the approver differs from the brief's author — the same
attribution-not-identity limit `lifecycle-v1.md` §7.1.2 and `registers-v1.md` §7.4 declare. And it
does not attest the design is correct: that the alternatives were weighed is recorded here; whether
the kind set and the grant are right is the review gate's judgement, then the change's own
validation after it lands.
