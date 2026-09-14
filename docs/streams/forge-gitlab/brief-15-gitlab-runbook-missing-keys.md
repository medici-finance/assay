---
brief: assay:assay:forge-gitlab:15
title: The GitLab runbook's missing keys — forge binding, board-push credential, source-pin lane
why: >-
  An operator who follows the GitLab runbook end to end still cannot get a desk running, and the
  three things that stop them are all absent from the document rather than broken in the product.
  The key that BINDS a project to the GitLab backend is never named in the GitLab runbook at all,
  so every verb resolves the wrong forge on a correctly provisioned fleet. The credential the
  scaffolded board-regeneration job requires is named once, in a pointer that sends the reader to
  a section about a different credential — and that section does not exist under the number the
  pointer gives. And the source-pin lane a GitLab plus native-Windows adopter actually installs
  through is documented only in the other forge's runbook, so this reader is told nothing and
  writes pin lines one reader accepts and another refuses. Each cost a real adopter a round trip
  that ended in a product issue for a documentation gap.
wave: 4
depends: ["forge-gitlab/04"]
unblocks: ["forge-gitlab/16"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [719, 896]
schema: brief-v2
authored: 2026-09-14 by forge-gitlab wave-plan authoring session
sources:
  - "#719 — the board-push credential: the scaffolded GitLab CI job refuses without a masked project or group access token with repository-write scope, and that instruction exists only in the generated file's comment and the job log. The runbook's pointer sends the reader to fleet-PAT custody, which is a different credential. Live evidence in the issue: a regen job on a self-managed Community Edition instance stopped on exactly that refusal, and a developer-level identity could not create the variable"
  - "#896 — the follow-through gaps: its first half (the issue-lane scan scope) is already in the GitLab runbook; its second half, the source-pin lane, is not. A live install wrote source pin lines that one board reader accepts and another refuses, and the operator read a stale board as an all-clear"
  - "#795 — the same source-pin grammar reported from the field one round earlier; the board's desk-tools reader now accepts BOTH orderings, so the residue is documentation, not the reader"
  - "#651 / #652 — the front-door tracking pair: the chooser and the two install lanes landed for the other forge's runbook; this brief carries the GitLab-side pointer so the same reader is not sent down a lane written for a different forge"
  - "docs/adopting-assay-gitlab.md — the document under change. Measured 2026-09-14 @ 2c67b34f: the forge-binding key appears ZERO times; the board-push credential appears ONCE, in the runner section's pointer to `§2, token custody`; the source-pin lane appears zero times. Token custody is §5, not §2, so the pointer is wrong twice over — wrong section number and wrong credential"
  - "docs/adopting-assay.md — the other forge's runbook, which DOES carry the source-pin lane: it states that the source lane writes no pin line at all, that the literal lane name is never a field in the pin grammar, and it records the live install that wrote those lines and then failed a board read with no pin found"
  - "statusgen/init.go — the scaffolded GitLab CI job: the regeneration step refuses when the push credential is unset and prints the complete instruction. That refusal text is the authority this brief quotes; it is not paraphrased from memory"
  - "tools/desk/internal/deskkit/pins.go + tools/desk/cmd/deskboard/main.go — the pin readers. The desk-tools reader accepts the commit in either field position; the generic pin reader returns fields two and three positionally. The asymmetry is what a wrongly-written pin line exposes"
  - "freshness-checked 2026-09-14 @ 2c67b34f — all three absences re-measured in the GitLab runbook on this commit"
exec-tier: strong
exec-tier-why: "the deliverable is a setup guide whose every claim is actionable, and a confident wrong instruction here costs an adopter a boot cycle and produces a product issue for a doc defect (question a: the runbook must decide what the minimum credential and role actually are, which the product code constrains but does not state); the three sections also have to agree with two readers in two components and with the other forge's runbook (question b)."
domain: complicated
tier: free
consumers:
  - "docs/adopting-assay-gitlab.md: fixed-here (three new or corrected subsections, and the wrong pointer removed)"
  - "docs/adopting-assay.md: out-of-scope (the other forge's runbook already carries the source-pin lane; this brief cross-references it rather than forking a second copy that can drift)"
  - "statusgen/init.go: fixed-here (one pointer line added to the generated CI comment naming the new runbook subsection; no behaviour change, and the refusal text stays the authority)"
  - "plugins/assay/skills/install/SKILL.md: out-of-scope (the install skill's forge-neutral prerequisites are the neutral stream's own brief, not this one)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the status row)"
version: 1
id: cce224ba-25ea-4db2-b322-126f0fdd0eb3
---

# Brief 15 — The GitLab runbook's missing keys

## Context

Three gaps, each measured in the document rather than inferred.

**1. The forge binding is never named.** The roster key that binds a project to the GitLab
backend appears zero times in the GitLab runbook. Every verb in the suite resolves which forge
serves a repo before it does anything else, and without that binding a correctly provisioned
GitLab fleet is read as the default forge — which is exactly the class of failure the standing
field reports describe from the other end (a verb reaching the wrong host for a project that
does not live there). The runbook that provisions the fleet has to state the key that makes the
fleet resolvable.

**2. The board-push credential has no home.** The scaffolded GitLab CI job cannot push the
regenerated board with the job's own token; it needs a separate masked variable holding a
project or group access token with repository-write scope. That complete instruction exists in
the generated file's comment and in the job log. The runbook mentions the variable once, in the
runners section, as a pointer to "§2, token custody" — and token custody is §5, and it is about
the per-role fleet credentials, which are emphatically NOT this token. A reader who follows the
pointer arrives at the wrong credential in the wrong section.

**3. The source-pin lane is documented for the other forge only.** A GitLab plus
native-Windows adopter installs through the source lane. The other forge's runbook says
plainly that this lane writes no pin line at all, that the lane's own name is never a field in
the pin grammar, and records the live install that wrote such lines and then failed a board
read. The GitLab runbook says none of it, so this reader writes pin lines the desk-tools reader
happens to accept in either field order and a different reader refuses — and a stale board
reads as an all-clear.

files:
- `docs/adopting-assay-gitlab.md` — three changes:
  - the forge-binding key added to the post-provisioning boot section alongside the existing
    API-base and scan-scope keys, with its value shape and where it is set;
  - a first-class board-push-credential subsection: which token kind (project or group access
    token, not a human credential and not a per-role fleet token), which scope, which role when
    the default branch is protected, masked and protected visibility, and where in the settings
    it is created — and the runner section's pointer corrected to point at it;
  - a source-pin-lane subsection that states the rule and cross-references the other forge's
    runbook rather than restating it, so the two cannot drift.
- `statusgen/init.go` — one pointer line in the generated CI comment naming the new subsection.
  No behaviour change; the refusal text stays authoritative.
- `docs/streams/forge-gitlab/README.md` — the status row.
- `changelog/forge-gitlab-15-gitlab-runbook-missing-keys.md` (planned).

single-point-of-failure: the document is the only control between a correct provisioning run
and a boot that cannot finish, and a document cannot fail closed — it fails by being confidently
wrong. So the design puts a second, differently-shaped layer behind it: every credential and
scope claim the new text makes is DEREFERENCED by a Verify row against the artifact that
constrains it (the generated CI file's own refusal text, and the pin readers' own field
handling), so a claim written from memory goes red in the row rather than in an adopter's
terminal. The two layers fail on different signals — prose review catches adequacy, the
dereference rows catch wrongness — and neither substitutes for the other.

facts:
- Measured 2026-09-14 @ `2c67b34f` in `docs/adopting-assay-gitlab.md`: forge-binding key
  occurrences **0**; board-push variable occurrences **1** (the runner section's pointer);
  source-pin lane occurrences **0**. The API-base and scan-scope keys ARE present and are the
  shape the new section copies.
- The runbook's token-custody section is **§5**. The pointer in the runners section says §2.
  Both the number and the credential are wrong.
- The generated CI job's refusal text is the complete instruction and is the authority for the
  new subsection's scope claim. Quote it; do not paraphrase a scope from memory.
- The desk-tools pin reader accepts the commit in field two or field three. The generic pin
  reader returns fields two and three positionally. A pin line written in the lane's wrong
  shape is therefore accepted by one reader and refused by another — which is why the rule is
  "write no source pin line", not "write it carefully".
- This brief changes documentation only. No product behaviour, no new key, no relaxed check.

## Edition
Minimum GitLab tier: **free**. Every key and credential documented here is Free-tier: the
project or group access token, the masked CI variable, and the roster keys. The default branch
being protected at a maintainer-level role is a Free-tier setting and is the reason the new
subsection must state a minimum role rather than only a scope. No new degradation is disclosed.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT invent a default host, a default token lifetime, or a scope you have not read out of
  the generated file or the product code. An unverifiable instruction is a could-not-check line
  in the PR body, not a confident sentence in a runbook.
- Do NOT fork the source-pin lane's text into this runbook. Cross-reference it. Two copies of a
  pin grammar is how the next adopter gets the stale one.

## Task
1. Add the forge-binding key to the post-provisioning boot section: value shape, where it is
   set, and the symptom when it is absent (every verb resolves the wrong forge). Place it
   beside the existing API-base and scan-scope keys and follow their format exactly.
2. Add the board-push-credential subsection, with the token kind, scope, minimum role under a
   protected default branch, and variable visibility — every claim dereferenced against the
   generated CI file's own refusal text. Correct the runners section's pointer to target it,
   and remove the wrong section reference.
3. Add the source-pin-lane subsection: state the rule, name the symptom a wrongly-written pin
   line produces at each reader, and cross-reference the other forge's runbook for the full
   lane. No second copy of the grammar.
4. Add the one-line pointer in the generated CI comment naming the new subsection. Text only —
   the refusal itself is unchanged.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -c 'ASSAY_REPO_FORGES' docs/adopting-assay-gitlab.md` | `1` or more — the forge-binding key is named in the GitLab runbook | check |
| 2 | `grep -c 'STATUSGEN_PUSH_TOKEN' docs/adopting-assay-gitlab.md` | `4` or more — the credential has a subsection, not a single pointer | check |
| 3 | `! grep -n -e 'token custody' docs/adopting-assay-gitlab.md \| grep -q -e '§2'` | exit 0 — no `token custody` line still points at §2, so the wrong pointer is gone. Gating on the exit status, not on a count, because a count of zero exits non-zero on the success path | check |
| 4 | `S=$(sed -n 's/.*with the \([a-z_]*\) scope.*/\1/p' statusgen/init.go \| head -1); test -n "$S" && grep -c -- "$S" docs/adopting-assay-gitlab.md` | `$S` non-empty and the count `1` or more — the scope literal is LIFTED out of the generated job's own refusal text and then required in the runbook, so a scope written from memory makes this row red. This is the row that can fail on a well-formed but wrong document | gate:model +dereference +flow |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -run TestArtifactPin -v -timeout 120s` | exit 0; output contains `PASS` — the pin-reader behaviour the new subsection describes is the behaviour the reader has | gate:model +dereference |
| 6 | `grep -c 'adopting-assay.md' docs/adopting-assay-gitlab.md` | `1` or more — the source-pin lane is cross-referenced, not forked | check |
| 7 | `statusgen --root . --consumers` | exit 0 | check |
| 8 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The scope or token kind is written from memory and is already wrong | row 4 (dereferences the generated job's own refusal text) |
| The pin-lane description says something the readers do not do | row 5 |
| The new subsection is added but the wrong pointer is left in place, so the reader still lands on the fleet credential | row 3 |
| The forge-binding key is mentioned in passing rather than given a value shape | row 1 counts presence only — the value shape is review-only, and the reviewer is asked for it below |
| The source-pin lane is copied rather than referenced, and the copy goes stale | row 6 + Ground rules; drift between two copies is review-only |
| The minimum role under a protected default branch is asserted with no source | no row — the product code does not constrain it, so it is could-not-check unless taken from the forge's own published documentation and cited inline; say which in the PR body |

### Dispatch checklist
```
[x] 1. Rows discriminate — row 4 goes red on a confidently-wrong scope, which is precisely the failure a presence grep cannot see.
[x] 2. Facts dated — all three occurrence counts measured 2026-09-14 @ 2c67b34f, and each carries the command that re-establishes it.
[x] 3. Self-contained — the three gaps, the wrong pointer's exact text, and the two readers' behaviour are described here.
[x] 4. Risk answers match files: — documentation only; the sections describe credentials but mint, store and relax nothing: all four stay no.
[x] 5. gate-why n/a (gate: model, all four no).
[x] 6. Effort honest — three sections, one of them requiring a dereferenced credential claim: M, not S.
[x] 7. No shared value changes; consumers: names the other runbook as out-of-scope with the anti-drift reason.
[x] 8. Pre-mortem run; the three review-only items are named, including the one fact that has no in-repo source.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: does the forge-binding entry give a reader enough to WRITE the value — shape,
location, and the symptom when it is missing — or does it only prove the key was mentioned?
