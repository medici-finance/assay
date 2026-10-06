---
brief: assay:assay:forge-neutral:35
title: Ruling resolver onto the read verb — the decision-record ruling check reads through deskread, accepts only a User author, and holds no credential of its own
why: >-
  statusgen's ruling resolver decides whether a decision record's ruling link points at a real,
  unedited comment written by a human allowed to decide. It is the last statusgen path that
  builds its own forge client, and the last place statusgen reaches for an ambient credential:
  `rulingForgeClient` takes a token from the environment or, failing that, from `gh auth token`.
  Moving its two reads onto the desk-tools read verb takes away that fallback, but it also
  rewrites every input the control decides on: how a deleted comment is detected, how a comment
  is bound to its issue, how an edit is detected, and how a bot is told from a human. That is a
  change to a control, not just a read, so it has its own brief and a human gate.
wave: 6
depends: ["forge-neutral/18", "forge-neutral/33"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
outcome: none
authored: 2026-10-06 by forge-neutral authoring session (split from forge-neutral/18 on review of its authoring change)
sources:
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md (forge-neutral/18) — moves every other statusgen forge-CLI site onto `deskread` and adds `issue` to `deskread`'s CI-transport kind set; its Verify row 3 excludes `statusgen/decisionruling.go` because this brief owns it"
  - "docs/streams/forge-neutral/brief-33-forge-reads-for-statusgen-s-remaining-sites.md (forge-neutral/33) — adds `Comment.UpdatedAt` (its Task 2.8), the comment id, author type and URL on the `comments` kind, and the `issue` kind (its Task 3); this brief consumes all of them"
  - "#2253 and its ruling (option a): which credential statusgen's CI-only modes read under. `--corroborate`, the mode this resolver runs in, is one of them; the CI-transport brief in #2314 carries that ruling's work"
  - "statusgen/decisionruling.go — `resolveRuling` (`:228`), its comment read (`:262`), the comment-to-issue binding (`:285-292`), the edit check (`:296-307`), the bot check (`:311-313`), the issue read (`:354`), `resolveDecisionRulings` (`:620`) and `rulingForgeClient` (`:633-648`, the `gh auth token` fallback at `:643`)"
  - "statusgen/corroborate.go:1481 — the one caller, which builds the client"
  - "statusgen/ghfetch.go — `ghClient` (`:40`), `newGHClient` (`:52`), `GetIssueComment` (`:198`), `GetIssue` (`:205`)"
  - "tools/desk/internal/deskkit/forge.go:68-73 — `Account.Type`: `User`, `Bot`, `Organization` or `Mannequin`; empty is could-not-check, never `User`"
  - "tools/desk/cmd/deskautolane/forge.go:193-201 — the precedent this brief follows: an empty author type is could-not-check, and any type other than `User` is refused"
  - "tools/desk/internal/deskkit/forge_github.go:1749-1761 — the GitHub comment read re-adds the `[bot]` suffix to a bot's login from the same `__typename` it reports as the author type"
  - "freshness-checked 2026-10-06 @ 35c303e47: `decisionruling.go:643` is the only forge-CLI launch in the file; row 3 of forge-neutral/18 prints 32 over non-test statusgen and 31 with this file excluded; the widened credential grep (row 4 below) matches 13 lines across `decisionruling.go` and `corroborate.go`"
exec-tier: strong
exec-tier-why: >-
  Every input the ruling check decides on is re-derived from a different read with a different
  failure shape. A mapping that turns could-not-check into "no such comment", "not edited" or
  "a human" passes a happy-path test suite and accepts a ruling that today is refused.
gate-why: >-
  The ruling resolver is the control that decides whether a decision record was ruled on by a
  human allowed to decide. This brief rewrites its inputs: deleted-comment detection moves from
  an HTTP status to an absence in a complete thread, the comment-to-issue binding moves from a
  parent-issue URL to the thread the comment was found in, edit detection moves to fields
  another brief adds, and the bot check moves to a forge-reported author type that must be
  `User`. It also moves the credential the resolver reads under, from an environment token or
  an ambient `gh` login to the minted read role locally and the workflow-token transport in CI.
  The human confirms that no input the control decides on can read as accepted when it could not
  be checked, and that the resolver holds no credential of its own afterwards.
decision-trigger: start
domain: complicated
consumers:
  - "statusgen/decisionruling.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/decisionruling_test.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/decisionruling-mutations.json: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/corroborate.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — the one call at `:1481`; the file's other sites are forge-neutral/18's)"
  - "statusgen/ghfetch.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — `GetIssueComment` and `GetIssue` go once nothing calls them)"
  - "tools/desk/cmd/deskread: out-of-scope (the `comments` and `issue` kinds and their fields are forge-neutral/33's; `issue` on the CI-transport kind set is forge-neutral/18's)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (no operation added; every read here already exists or is forge-neutral/33's)"
version: 1
id: 61620d38-216a-4c21-b390-046d3d03402c
---

# Brief 35 — Ruling resolver onto the read verb

## Context

**A design record is required before dispatch.** This brief is risk-gated
(`sensitive-data: yes`), so under brief-rule 48 it may not leave `todo` until its `design:` key
names an approved `DR-<slug>` record in the decisions register, and its sign-off is the
`gate: human` decision issue. Neither exists yet: the record is written and approved by the
human, not by the authoring session, and the decision issue is #2317, filed at this brief's first review dispatch. Dispatching
this brief without them is refused by `statusgen --lint`.

files:
- `statusgen/decisionruling.go` — `resolveRuling` (`:228`); its comment read (`:262`) and the
  status switch after it; the comment-to-issue binding (`:285-292`); the edit check
  (`:296-307`); the bot check (`:311-313`); its issue read (`:354`); `resolveDecisionRulings`
  (`:620`); `rulingForgeClient` (`:633-648`), whose `gh auth token` fallback is at `:643`.
- `statusgen/decisionruling_test.go` and `statusgen/decisionruling-mutations.json` — its tests
  and mutation manifest.
- `statusgen/corroborate.go:1481` — the one caller, which builds the client today.
- `statusgen/ghfetch.go` — `GetIssueComment` (`:198`) and `GetIssue` (`:205`), the two raw
  reads the resolver uses.

**What changes for the control.** This brief changes no accept or refuse rule's meaning, and no
outcome that refuses today may pass afterwards. What it changes is every input those rules read:
- **Deleted comment.** Today an HTTP 404 or 410 on the comment read. After: the comment's id is
  absent from a thread `deskread comments` read completely.
- **Comment-to-issue binding.** Today the comment payload's `issue_url` is compared with the
  link. After: the comment is looked up only in the linked issue's own thread, so a comment
  that sits on another issue is simply not found there.
- **Edit.** Today `created_at` and `updated_at` from the REST payload. After: `createdAt` and
  forge-neutral/33's `updatedAt` from the `comments` kind.
- **Bot.** Today `user.type == "Bot"` or a `[bot]` login suffix. After: the author type the
  forge reports must be `User` (Task 2).
- **Credential.** Today `GH_TOKEN`, then `GITHUB_TOKEN`, then `gh auth token`. After: none of
  its own. The resolver reads under whatever identity `deskread` resolves: the minted read role
  locally, and the workflow-token transport #2253's ruling chose in CI (the CI-transport brief
  in #2314, which opens its kind set at `issues`, `trust` and `comments`; forge-neutral/18 adds
  `issue`). With neither available, every ruling is `unresolvable-link`, a refusal.

single-point-of-failure: the control is the resolver's mapping from a `deskread` result to a
ruling outcome. Every unknown must map to a refusal. Two layers stand behind it. First, the
reader returns a three-state result per read, so a caller that does not handle could-not-check
does not compile against the offline reader, which returns it for every call. Second, the
mutation manifest re-runs the test suite against named changes to the mapping (row 3), which
trips on a different signal (a mutant surviving) in a different place (the harness) from the
mapping's own tests.

facts — measured at `35c303e47`:

- `decisionruling.go:643` is `exec.Command("gh", "auth", "token")`, the last step of
  `rulingForgeClient` (`:633-648`). It is the file's only forge-CLI launch. forge-neutral/18's
  row 3 counts 32 lines over non-test statusgen, and 31 with this file excluded.
- `resolveRuling(c *ghClient, …)` (`:228`) and `resolveDecisionRulings(c *ghClient, …)`
  (`:620`) take statusgen's own HTTP client. Their only production caller is
  `corroborate.go:1481`, which passes `rulingForgeClient()`.
- The bot check at `:311-313` refuses `type == "Bot"` or a login ending `[bot]`. An author of
  type `Organization` or `Mannequin` passes it today and is refused only if its login is not
  in the human map.
- On the GitHub comment read the seam would serve, the backend appends `[bot]` to the login
  exactly when `__typename` is `Bot` (`forge_github.go:1755-1756`) and reports the same
  `__typename` as `Account.Type` (`:1761`). The suffix check and the type check therefore read
  one signal, not two.
- `Account.Type` documents `User`, `Bot`, `Organization` and `Mannequin`, and says empty is
  could-not-check, never `User` (`forge.go:68-73`). `deskautolane` already refuses a sign-off
  whose author is not a `User` and treats an empty type as could-not-check
  (`deskautolane/forge.go:193-201`).

## Ground rules
- NEVER git push, trigger workflows or run mutating commands against a forge. Commit only per
  the task instructions.
- Stop at `implemented`. You do not set verified or done.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Don't guess. In
  particular: if `deskread`'s `comments` kind does not carry `databaseId`, `authorType`,
  `createdAt` and `updatedAt`, if there is no `issue` kind, or if `issue` is not on the
  CI-transport kind set, stop. Those are forge-neutral/33's and forge-neutral/18's to land.
- **No credential of the resolver's own.** Delete `rulingForgeClient` and add nothing in its
  place: no environment read, no token file, no CLI login, no client built in
  `decisionruling.go` or at its call site.
- **No outcome moves from refused to accepted.** Two refusals change their reason (Task 1);
  none disappears.
- Touch no file under `tools/desk/` and no workflow.

## Task

### 1. Move the two reads onto the reader

- `resolveRuling` and `resolveDecisionRulings` take forge-neutral/18's `forgeReader` interface,
  not `*ghClient`. `corroborate.go:1481` passes the reader the run already holds.
- The comment read becomes `deskread comments` on the link's issue, selecting the comment by
  `databaseId`. The issue read becomes `deskread issue`.
- Delete `rulingForgeClient` with its `GH_TOKEN` and `GITHUB_TOKEN` reads and its `gh auth
  token` fallback. Delete `GetIssueComment` and `GetIssue` from `ghfetch.go` once nothing calls
  them.
- The mapping moves from HTTP status to the envelope, and every unknown still refuses:
  - the verb exits non-zero, its output does not parse, its schema is not 1, or the item is in
    `partial` → `unresolvable-link`, with the partial reason in the detail;
  - the thread was read completely and the linked id is not in it → `deleted-comment`. The
    `comments` kind returns a whole thread or a `partial`, so a missing id in a complete thread
    is an observed absence. A comment that exists on a different issue now reads the same way;
    today it is `unrelated-issue`. Both refuse;
  - the linked number is a change, not an issue → the `issue` kind reports the kind mismatch in
    `partial` → `unresolvable-link`; today it is `unrelated-issue`. Both refuse;
  - an empty or unparseable `createdAt` or `updatedAt` → `unresolvable-link`; the two differing
    → `edited-comment`;
  - under the offline reader every ruling is `unresolvable-link`.
- The two test expectations that change (`TestRuling_CommentOnDifferentIssue`,
  `TestRuling_PullRequestNotIssue`) change from one refusal reason to another. Manifest entries
  keyed on the HTTP-status lines are re-pointed at the envelope mapping so each still reddens.
- Ruling links are a GitHub URL grammar, so a GitLab ruling link stays out of scope and is
  refused as malformed, as today.

### 2. Only a `User` rules

- An empty author login or an empty author type → `unresolvable-link`. Empty is
  could-not-check, never `User` (`forge.go:68-73`).
- Any author type other than `User`, compared case-insensitively → `bot-author`. That covers
  `Bot`, `Organization`, `Mannequin` and any type the forge adds later. The detail names the
  type. This follows `deskautolane/forge.go:193-201`.
- Keep the `[bot]` suffix check, but do not count it as a second layer. The GitHub backend adds
  the suffix from the same `__typename` it reports as the type (`forge_github.go:1755-1756`,
  `:1761`), so the two checks rest on one signal. The `User`-only rule is the control.

## Verify (executable — no prose-only DoD items)

Test names are planned, created by the implementer.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go build ./... && go test ./... -count=1` | exit 0 |
| 2 | check | `statusgen --root . --lint` | `LINT: PASS`, exit 0 |
| 3 | check:ci +mutation | `cd statusgen && go test . -run '^TestRulingReaderMappingRefuses$' -count=1 -v > "${TMPDIR:-/tmp}/b35-r3.out" 2>&1 && grep -F -e '--- PASS: TestRulingReaderMappingRefuses' "${TMPDIR:-/tmp}/b35-r3.out" && muhar -spec decisionruling-mutations.json` | **negative path**: `TestRulingReaderMappingRefuses` (planned) drives `resolveRuling` through a fake reader. Each case in Task 1 and Task 2 gives its refusal: verb failure, bad schema and a `partial` item give `unresolvable-link`; a complete thread without the id gives `deleted-comment`; a change number gives `unresolvable-link`; an empty author login, an empty author type, and an empty or unparseable `createdAt` or `updatedAt` each give `unresolvable-link`; author types `Bot`, `Organization` and `Mannequin` each give `bot-author`; differing times give `edited-comment`; the offline reader gives `unresolvable-link` for every ruling. Every non-`User` case uses a login that IS in the human map, so only the type check can refuse it. No case that refuses at the base passes after the move. The `muhar` leg (the harness built from `tools/desk/cmd/muhar`) reports the baseline green, the control caught, and every manifest entry caught, with none not caught |
| 4 | check | `test -f statusgen/decisionruling.go && test -f statusgen/corroborate.go && { grep -n -F -e rulingForgeClient -e '"auth", "token"' -e GH_TOKEN -e GITHUB_TOKEN -e os.Getenv -e os.LookupEnv -e newGHClient -e ghClient -e GetIssueComment -e '"net/http"' statusgen/decisionruling.go statusgen/corroborate.go; test $? -eq 1; }` | exit 0 after the move: neither the resolver's file nor its caller's file names the old client, a token variable, an environment read, a client constructor or `net/http`. Only grep's no-match status passes. A match, a missing file or a grep error fails the row. Measured at `35c303e47` with BSD and GNU grep: 13 matching lines, exit 1. The `ghClient` pattern is what holds `resolveRuling` to the reader interface. The row errs strict: a later, unrelated `os.Getenv` in either file also fails it, and that is wanted |
| 5 | check +dereference | `test -d statusgen/ && { grep -rnF '"gh"' statusgen/ --include='*.go' \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+_test\.go:' \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`. This is forge-neutral/18's row 3 with no path excluded, so it is the stream's completion count for statusgen: 18's row takes every other file to 0, and this row adds `decisionruling.go`. What a `0` cannot see is listed in 18's row 3 Expect. Measured at `35c303e47`: `32` |
| 6 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/35` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

### Named mutations for the `+mutation` rows

| Row | The mutation that must redden it |
|---|---|
| 3 | (a) read a `partial` thread as a complete thread without the comment, so the outcome is `deleted-comment` where it must be `unresolvable-link`. (b) treat an empty `updatedAt` as equal to `createdAt`. (c) narrow the `User` requirement back to a `Bot` check, so `Organization` and `Mannequin` authors with a mapped login pass to the human map and are accepted. (d) drop the author-type check entirely, so a `Bot` author with a mapped login and no `[bot]` suffix is accepted. Each must fail the row on the reason, and (c) and (d) are added to the manifest as named entries so the `muhar` leg reddens on them too |

## Pre-mortem → detection map

*"This shipped and was wrong — what went wrong?"*

| Failure mode of the work | Caught by |
|---|---|
| A `partial` comment read is taken as absence, so an unreadable thread reads as a deleted comment | row 3, mutation (a) |
| An unreadable update time is taken as unedited, so an edited ruling comment passes | row 3, mutation (b) |
| Only `Bot` is refused, so an `Organization` or `Mannequin` author whose login is in the human map is accepted as a human ruling | row 3, mutation (c) |
| The type check is lost in the move and the `[bot]` suffix is relied on alone, though both come from one `__typename` | row 3, mutation (d) |
| An empty author type is read as `User`, so an author the forge did not classify is accepted | row 3 (empty-type case) |
| `gh auth token` is removed but re-expressed, or another token source is added for the resolver or at its call site, so the brief quietly keeps or swaps an ambient credential | row 4; Review confirms the resolver holds no credential |
| `resolveRuling` keeps taking `*ghClient` and a client is built somewhere else for it | row 4 (`ghClient`, `newGHClient`) |
| A refusal reason changes and a reviewer reads it as a loosened control | Task 1 names both changes; row 3 asserts each still refuses |
| The resolver runs in CI with no transport for the `issue` kind, so every ruling refuses in CI | **no row here**: `issue` on the CI-transport kind set is forge-neutral/18's, with its own row; a ruling that cannot be read is `unresolvable-link`, a refusal, so the failure is loud, not a pass |
| The control is moved to a fresh read path whose inputs a human never reviewed | the `gate: human` sign-off and the design record required before dispatch |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter; `sensitive-data: yes` — see `gate-why` and "What changes for
the control" in `## Context`). The reviewer confirms that the resolver holds no credential of
its own after the move (no token read, no `gh auth token`, no client built in
`decisionruling.go` or at `corroborate.go:1481`), that every unknown in the mapping refuses,
and that only a `User` author can rule. The human signs off on the decision issue against the
approved design record.
