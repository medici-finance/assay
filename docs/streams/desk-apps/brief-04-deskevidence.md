---
brief: desk-apps/04
title: deskevidence — verify-desk commits Evidence as verifier-app[bot]
why: >-
  Today verify-desk's Evidence rows are trusted prose ("runner: opus-verifier") anyone could write;
  a `verified` row backed only by an agent's word is the [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) unbacked-row gap. Committing Evidence
  via the API as verifier-app[bot] makes authorship machine-checkable (statusgen brief 07 then backs
  the row by the commit ACTOR), so a verified row means "the verifier App committed this," not
  "someone typed it."
wave: 2
depends: ["desk-apps/03", "desk-apps/02"]
unblocks: ["desk-apps/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (tamper-evident Evidence via verifier-app[bot]; closes [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md))", "memory verify-desk-operating-mode (verify-desk commits Evidence/status straight to main today)", "desk-tools/03 deskpost (the App-identity-posting precedent)"]
---

# Brief 04 — deskevidence (verify-desk Evidence as verifier-app[bot])

**BLOCKED-ON-HUMAN:** full live verification requires the verifier App to exist (created by human:<name> via
guide 02). The tool + offline tests land at model-gate; the live "posts as `verifier-app[bot]`"
Verify row waits for the App.

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskevidence/` (new); uses `deskkit` (desk-tools/01) + `desktoken`
(brief 03). Cross-repo note: Evidence lands in the brief's repo (oit or
assay-toolkit) — the tool is repo-agnostic.
facts:
- `deskevidence <repo> <branch> --evidence-file F [--verified-row <stream/NN>]`: commits the filled
  Evidence rows to the brief's file on `<branch>` (or main, per the brief's lifecycle) using the
  verifier App token from `desktoken verifier`.
- Commits attribute to `assay-verifier-app[bot]` (GitHub App API commits are tamper-evident; PAT
  commits are not — the whole point, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) §2).
- Inherits deskkit: C-3 secret-scan the evidence body, C-5 audit + idempotency `(repo, file,
  headSHA, bodyDigest)`, C-6 kill-switch, C-7 weakest-verb (commit only — no push-to-main, no merge;
  main-landing is human:<name>'s/the ruleset's call), C-10 fail-closed.
- `main` push stays human-gate (CLAUDE.md "What stays prompted": push to main in any form). On main,
  deskevidence stages the commit; the push is human:<name>'s (or sanctioned via the brief-08 ruleset +
  desk-app, brief 06). On a branch, deskevidence commits + the worker/verify flow pushes.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. Implement per facts; `deskkit.Guard()` first; audit every commit; commit author = verifier-app.
2. Tests (fake-git/fake-API): commit attributes to `verifier-app[bot]`; idempotent re-commit at same
  head → noop; oversize/secret-bearing evidence → exit 5 no-commit; kill-switch → exit 3; non-main
  branch only unless sanctioned.
3. Integration note for verify-desk skill: the verify-desk loop calls `deskevidence` instead of
  hand-editing + `git commit` (skill edit, coordinated with the verify-desk owner).

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskevidence/... -count=1` | exit 0; incl. attribution-to-verifier-app + idempotency + refusal tests |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `DESK_TOOLS_DISABLED=1 go run ./tools/desk/cmd/deskevidence oit main --evidence-file /dev/null; echo $?` | 3 |
| 4 | **(live, blocked-on-human)** commit a real Evidence row on a branch; `gh api /repos/.../commits/<sha>` shows `commit.author.name`/actor = `assay-verifier-app[bot]` | actor is verifier-app[bot] |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model + `/security-review` (auth/identity + the tamper-evident-actor claim). Reviewer confirms
the commit actor is verifiably `verifier-app[bot]`, the tool cannot push main, and Evidence is
idempotent + secret-scanned.
