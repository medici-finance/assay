# Resident operating rules — single source

This file is the ONE source of the Assay resident operating rules — the
always-loaded operating rules every session runs under. Every per-harness
delivery artifact is GENERATED from it, so the ten rules exist in exactly one
place and divergence is impossible, not merely detected.

Do not hand-edit the generated artifacts. Regenerate them with:

    go run ./tools/harnessgen resident          # write the artifacts
    go run ./tools/harnessgen resident --check   # CI: fail if they drift from this source

Generated artifacts:

- `plugins/assay/hooks/resident-rules.payload.txt` — the Claude Code SessionStart
  payload text (`inject-resident-rules.sh` wraps it as the `systemMessage` JSON).
- `plugins/assay/codex/AGENTS-assay.md` — the Codex `AGENTS.md` fragment, framed as
  one section so it composes into an adopter's existing `AGENTS.md` (HP/01 §3.1,
  HP/03 ruling A1: Codex CLI for v1).

The `## Header`, `## R<N>` and `## Footer` sections below are the machine-read
content; the prose above is not. Rule-content changes are their own PRs — never
smuggled into a plumbing change.

The Header carries a `{{VERSION}}` token, resolved by the generator and never
hand-typed: it reads the `version` field out of
`plugins/assay/.claude-plugin/plugin.json` and substitutes `v<version>` into
every generated artifact. Do not replace the token with a literal version
number — that reintroduces the exact drift `--check` now catches (#730:
the banner said "v0.1.0" long after the manifest moved to "1.0.0").

## Header

RESIDENT OPERATING RULES (assay plugin {{VERSION}}). These are the project-agnostic rules the desk skills rely on. Violate none without the human driver's explicit say-so.

## R1 EVIDENCE-NOT-CLAIMS
EVIDENCE-NOT-CLAIMS: every assertion needs a verifiable artifact (command output, file hash, log line) — never a bare text claim. Your own self-report is untrustworthy. Verify before asserting.

## R2 ISOLATION
ISOLATION: own worktree for every implementer, NEVER the shared checkout. Never git restore/clean a checkout you didn't create. Path-specific git add only — never -A. Check git rev-parse --show-toplevel before first write — abort if it resolves to the shared checkout.

## R3 NEUTRAL-DISPATCH WORDING
NEUTRAL-DISPATCH WORDING: when dispatching reviewers or workers, describe the work in plain correctness language (wrong values, forked state, fails-to-fire). Frame the task by the observable defect, not by speculative intent — it keeps the dispatch precise, actionable, and free of unfounded assumptions about cause.

## R4 OUT-OF-REPO PROTOCOL
OUT-OF-REPO PROTOCOL: files outside the repo (~/.claude/**) have no worktree isolation and edits go live instantly. Briefs touching them must declare exact paths in Context. At most ONE such brief in flight at a time. Apply edits LAST, commit in the ~/.claude stopgap repo.

## R5 NO ATTRIBUTION LINES anywhere
NO ATTRIBUTION LINES anywhere: no Co-Authored-By in commits, no Generated-with-Claude-Code in PRs/issues/comments.

## R6 MODEL-TIER AWARENESS
MODEL-TIER AWARENESS: this session can be silently downgraded. On probe (the human driver asks your model): present env model line verbatim, keep working. On assertion of downgrade: stop synthesis/judgment/composition, fall back to verification and transcription.

## R7 REDACTION
REDACTION: private repo -> full defect detail on PR (worker needs file:line + mechanism). Redact only genuinely secret MATERIAL (tokens/keys/PII), never defect descriptions.

## R8 GIT PUSH POLICY
GIT PUSH POLICY: NEVER push to main or merge without the human driver's explicit say-so. Branch push + draft PR is standing-authorized. Never trigger workflows or mutating kubectl.

## R9 SHARED-VALUE DISCIPLINE
SHARED-VALUE DISCIPLINE: a brief changing a value other components read must enumerate consumers and verify the flow end-to-end — not just the changed site.

## R10 CLASS-SWEEP RULE
CLASS-SWEEP RULE: a fix at one site is almost never alone. Grep for siblings and route each one (fixed-in-this-brief / follow-up brief / out-of-scope).

## Footer

See the assay skill bodies for the full operating manual. These are the compressed rules — the skill bodies carry the reasoning and the war stories.
