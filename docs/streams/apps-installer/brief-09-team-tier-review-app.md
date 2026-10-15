---
brief: assay:assay:apps-installer:09
title: "Team tier gains `<prefix>-review` — a third App holds the reviewer role, so the act bot's PRs get a formal approval"
why: >-
  On the recommended Read + Act tier the act App both authors PRs and posts review verdicts, and
  GitHub refuses an App approving its own PR — so every PR on that tier loops in the review desk's
  CHECK state and never reaches the ready-flip, and the superseded lane refuses outright. A third
  App that holds only the reviewer role gives the tier a distinct approver, so the full-suite
  review-to-ready lane runs on it unchanged, with no gate loosened.
wave: 2
depends: ["apps-installer/02"]
unblocks: ["apps-installer/03", "apps-installer/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [3062]
schema: brief-v2
authored: 2026-10-15 by the-desk (house-the-desk-20261015T140050Z)
sources:
  - "#3062 — the finding: the Read + Act tier has no supported review-to-ready path (CHECK loop; deskclose superseded refuses one App behind worker and reviewer)."
  - "#3062 driver ruling C (ratified by the driver's own comment on that issue, 2026-10-15): add a reviewer App to the team tier; never loosen the approval gate. Supersedes the option-A approach in #3077."
  - "./design.md §2 Screen 0 tier table (Read + Act column), §5 avatars."
  - "apps-installer/02 — `TierManifests`, `teamReadPerms`/`teamActPerms`, `permsForFamilyRole`, `deskRoles` in tools/desk/cmd/deskapps/manifest.go; `writeBindings` in records.go."
  - "freshness-checked 2026-10-15 @ 3bf2bf99b (origin/main) — `TierManifests(\"team\", p)` returns two specs (`<p>-read` ReadOnly, `<p>-act` bound to all six roles); no `-review` App exists in any tier but family."
exec-tier: strong
exec-tier-why: >-
  Question (b) — the binding this writes (apps.env `REVIEWER_APP=`) is read by desktoken, while the
  approver identity deskflip and deskclose check is read from the roster; the change is only right
  when every reader sees reviewer ≠ worker, and a test of the writer alone passes either way.
consumers:
  - "tools/desk/cmd/deskapps/manifest.go: follow-up apps-installer/09"
  - "tools/desk/cmd/deskapps/page/tier.html: follow-up apps-installer/09"
  - "tools/desk/cmd/deskapps/page/setup.html: follow-up apps-installer/09"
  - "docs/streams/apps-installer/design.md: follow-up apps-installer/09"
  - "docs/streams/apps-installer/README.md: follow-up apps-installer/09"
  - "docs/desk-tools/deskapps.md: follow-up apps-installer/09"
  - "tools/desk/README.md: follow-up apps-installer/09"
  - "docs/adopting-assay.md: follow-up apps-installer/09"
  - "tools/desk/cmd/deskapps/records.go: out-of-scope (writeBindings writes one line per role on the spec it is given; the new role split flows through it unchanged — row 3 proves it)"
  - "docs/streams/apps-installer/brief-03-deskapps-install-prove.md: fixed-here"
  - "tools/desk/cmd/deskavatar/main.go: follow-up apps-installer/10"
  - "docs/desk-tools/deskavatar.md: follow-up apps-installer/10"
  - "tools/desk/cmd/deskflip/flip.go: out-of-scope (reads the approver from the roster's reviewer binding; the gate is unchanged by ruling)"
  - "tools/desk/cmd/deskclose/superseded.go: out-of-scope (its same-App refusal stays; it now passes for a three-App team install — row 4)"
  - "plugins/assay/skills/install/SKILL.md: follow-up apps-installer/07"
version: 1
id: 079eab46-d529-4d71-9ff9-b74478aa6e63
---

# Brief 09 — Team tier gains `<prefix>-review`

## Context
files:
- `tools/desk/cmd/deskapps/manifest.go`, `manifest_test.go`, `bindings_test.go` (team cases).
- `tools/desk/cmd/deskapps/page/tier.html`, `tools/desk/cmd/deskapps/page/setup.html` (team-tier copy).
- `tools/desk/cmd/deskclose/superseded_test.go` (one new case, no production change).
- `docs/streams/apps-installer/design.md` §2 Screen 0 table and §1 rule 4; the tier table in
  `docs/streams/apps-installer/README.md`.
- `docs/desk-tools/deskapps.md` (`--tier team` line); `tools/desk/README.md` §"Role→App binding"
  (the "two-App tier" prose); `docs/adopting-assay.md` (the Read + Act self-approval paragraph,
  and the per-App private-key step).
- `changelog/apps-installer-09-team-review-app.md` (new).

single-point-of-failure: author ≠ approver on the team tier. Three independent layers stand
behind it: (1) the manifests and bindings this brief writes put the reviewer role on its own App;
(2) at run time deskclose and deskflip compare the roster's worker and reviewer logins
(`deskkit.SameActor`) and refuse when they match — a different component, reading a different
file; (3) GitHub itself refuses an App approving a PR it authored. Row 4 bypasses layer 1 (a
hand-written roster binding both roles to one App) and proves layer 2 still refuses.

facts:
- Team tier after this brief, in run-board order (checked 2026-10-15 against manifest.go @ 3bf2bf99b):
  `<p>-read` (ReadOnly, `teamReadPerms`, unchanged, bound via `READ_APP=`);
  `<p>-act` (`teamActPerms`, unchanged) bound to worker, verifier, desk, issue-loop, intake-loop;
  `<p>-review` bound to reviewer only, permissions exactly `permsForFamilyRole("reviewer")`
  (contents:write, issues:write, pull_requests:write, checks:read, statuses:read, actions:read,
  administration:read). Never `administration:write`.
- Why the reviewer set equals the family reviewer's: the desk preflight's `app-scopes-vs-duties`
  check requires contents:write, issues:write and pull_requests:write of every role App; deskflip
  needs administration:read to read a required-check set (#1020). Reusing the function keeps the
  two tiers from drifting.
- Three Apps fit one sitting under GitHub's per-account creation throttle (about four Apps).
- `writeBindings` (records.go) writes `<ROLE>_APP=<spec.Name>` for each role on the spec, or
  `READ_APP=` for a ReadOnly spec; it is called per App once keyed. So the reviewer binding moves
  to `<p>-review` exactly when that App is keyed.
- Display label for the tier: **Read · Act · Review**. The `--tier team` flag value is unchanged.
- Private keys: since GitHub's 2026-10-13 App-management changes, an App's private keys are
  generated and listed on the App's **Credentials** tab, not the General settings page. The
  manifest flow returns the first key in the conversion, so deskapps needs no click there; the
  hand runbook's key step must name the right tab.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never change deskflip's approval gate, deskclose's same-App refusal, or the family tier.
- Never widen `teamReadPerms` or `teamActPerms`.

## Task
1. manifest.go: `TierManifests("team", p)` returns three specs in the order read, act, review.
   `<p>-act`'s `Roles` drops `reviewer`; `<p>-review` has `Roles: ["reviewer"]` and
   `Permissions: permsForFamilyRole("reviewer")`. Update the `deskRoles` comment that says every
   role is bound to `<prefix>-act`.
2. A team run against an existing `apps.state.json` that already has `<p>-read` and `<p>-act`
   keyed adds the `<p>-review` row as pending and never re-creates the keyed two. Until
   `<p>-review` is keyed, `REVIEWER_APP` stays as written; once keyed it reads `<p>-review`.
3. Page copy (tier.html, setup.html): the Read + Act column becomes Read · Act · Review with the
   design table values below; setup lists three App rows.
4. design.md §2 table, Read · Act · Review column: Bot identities 3 · Clicks 3 create · 3 install
   per org · Sittings 1 · Review verdicts posted by `<prefix>-review[bot]`, formal review ·
   Approves a PR: `<prefix>-review` · Merges: you · Author and approver: distinct actors · Keys on
   this machine: 3 · Audit trail: `<prefix>-read[bot]` · `<prefix>-act[bot]` ·
   `<prefix>-review[bot]`. §1 rule 4: "A team needs three." Mirror the change in the stream
   README's tier table, deskapps.md, and tools/desk/README.md's "two-App tier" prose (now three
   keys and six bindings).
5. docs/adopting-assay.md: the Read + Act self-approval paragraph states the tier now carries a
   reviewer App; the per-App key step names the Credentials tab.
6. deskclose superseded_test.go: add a case where the roster binds worker to `<p>-act` and
   reviewer to `<p>-review` — the lane is not refused for being one App.
7. changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestManifest' -count=1 -v 2>&1 \| grep -cE -e 'team tier: 3 manifests' -e 'team review App permissions equal family reviewer' -e 'no tier set carries administration:write'` | 3 | check |
| 2 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindingsWritten' -count=1 -v 2>&1 \| grep -cE -e 'REVIEWER_APP=example-review' -e 'WORKER_APP=example-act' -e 'READ_APP=example-read'` | 3, and the same command with `grep -c 'REVIEWER_APP=example-act'` prints 0 | check |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestTeamTierBindingsResolveThroughDeskkit' -count=1 -v` | exit 0 — the test writes apps.env through `writeBindings` for all three team specs into a temp config home, then reads it back through `deskkit.AppBinding`; logs `reviewer→example-review worker→example-act` and fails if the two are equal | check +flow |
| 4 | `cd tools/desk && go test ./cmd/deskclose/ -run 'TestSuperseded.*(SameApp\|TeamTier)' -count=1 -v` | exit 0 — the existing same-App case still refuses (exit 5 shape) and the new three-App team case is not refused for being one App | check |
| 5 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && /tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE -e 'example-read' -e 'example-act' -e 'example-review'` | 3 | check |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestTeamResumeAddsReviewOnly' -count=1 -v` | exit 0 — a state file with read and act keyed gains one pending `example-review` row; neither keyed row changes state | check |
| 7 | `n=$(/tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE 'example-(read\|act\|review)'); grep -E '^\| Bot identities' docs/streams/apps-installer/design.md \| awk -F'\|' '{gsub(/ /,"",$4); print $4}' \| grep -qx "$n" && echo match` | prints `match` — the design table's Read · Act · Review identity count equals the number of Apps the tool plans (run after row 5) | check +dereference |
| 8 | `cd tools/desk && go test ./cmd/deskapps/ -count=1` | exit 0 — mutations.json and the page tests stay green | check |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
