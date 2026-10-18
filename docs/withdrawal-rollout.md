# Withdrawal rollout and open work

Withdrawal retires remaining planned scope. It does not undo implementation, cancel a
worker, close a PR, satisfy a dependency, or attest verification. Preserve original
brief IDs, Evidence, stamps and receipts. Use an in-place amendment while the brief's
identity and purpose survive; use withdrawal when its remaining plan is no longer wanted.

## Order of operations

1. Land the schema/parser/readers together, publish their release, then upgrade each
   consumer's pinned statusgen and desk-tools. Confirm `statusgen --version --json`
   lists `brief-v3`. Keep existing brief-v1/v2 records unchanged; their old prose
   `superseded-by` keys remain tolerated. Never bulk-convert them merely to green CI.
2. Prepare the candidate board amendment. Only withdrawn briefs need brief-v3.
   Preserve partial implementation and verification history. When a future-state
   handoff governs the change, share its revision-bound `withdrawal-context` pointer
   across affected briefs. Use context-handoff-v2 if its impacts include already
   withdrawn work; v1 is deliberately unchanged. Product-target revision bindings
   remain owned by statusgen/23, and continuing observations by continuing-operations;
   this release adds no duplicate metric or portfolio taxonomy to brief frontmatter.
3. Export the open PR snapshot with the authorized forge reader. For GitHub the
   fields are `number,headRefName,headRefOid,files` on `gh pr list --state open`.
   Set a limit that covers the queue and verify enumeration separately. Then run:

   ```sh
   statusgen withdrawal-audit --root <candidate-tree> --prs <open-prs.json>
   ```

   The JSON report names matching PR heads, affected briefs and successors. It matches
   branch identities, brief/evidence files and stream READMEs; README matches are
   intentionally conservative. Coverage is always `snapshot-only`: indirect consumers,
   unlisted PRs, incomplete file lists and current claims remain could-not-check.
   The command is offline and never changes a PR, branch, claim or lifecycle cell.
4. For every affected PR, re-read its head and current claim before acting. Record the
   checkpoint and choose with its owner: retain useful work and retarget/re-scope it;
   finish independent evidence against the implementation that actually landed; or
   propose superseded/withdrawn disposition with the existing independent confirmation
   gates. Reconcile unacknowledged active work before publishing its amendment. Do not
   auto-close a PR merely because its brief is withdrawn. `deskboard stalled` reports
   `reconcile` for a withdrawn owner, even when its branch is far behind main.
5. Merge current main into affected open branches and run their actual required gates
   with the new readers. Fix schema/reader defects centrally; fix genuine stale plans
   in their owning PRs. Keep execution/review/human gates intact. Missing provenance is
   advisory in this rollout; malformed supplied provenance fails. Retiring scope is
   never a reason to erase failing evidence, weaken tests or treat unknown as passed.

An older statusgen deliberately refuses brief-v3 and names the needed schema, its
supported set and the local pin. Upgrade the reader before the authoring change. Do not
rename v3 records to v2 to hide that incompatibility. The first supporting release is
filled into `spec/brief-v1.md` at release cut; an unreleased build's version output is
capability evidence for that build, not evidence a consumer pin already supports it.

## Small measurement burden

Use the existing PR snapshot and audit report as the rollout worklist. Record only the
selected treatment, owner acknowledgement and unresolved obligations in the existing
handoff/workpad. No per-brief KPI, scheduled meeting, service or observation record is
required. This applies equally to the toolkit and software produced with it. Existing
outcome/observation records remain their single owners; workflow throughput does not
stand in for product benefit or settled verification debt.
