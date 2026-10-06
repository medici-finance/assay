---
brief: assay:assay:forge-neutral:35
title: Human-ruling resolvers onto the read verb — the decision-record ruling check and the transcribe lanes' sign-off check read through deskread, accept only a User author, and hold no credential of their own
why: >-
  statusgen has two resolvers that decide whether a human ruled. The ruling resolver decides
  whether a decision record's ruling link points at a real, unedited comment written by a human
  allowed to decide. The sign-off resolver decides whether a ruling's sign-off link points at a
  comment by the blessing authority, and its answer arms or keeps inert the unattended transcribe
  lanes. The ruling resolver is the last statusgen path that builds its own forge client, and the
  last place statusgen reaches for an ambient credential: `rulingForgeClient` takes a token from
  the environment or, failing that, from `gh auth token`. Moving both resolvers onto the
  desk-tools read verb takes away that fallback and the sign-off resolver's `gh` launch, but it
  also rewrites inputs the two controls decide on: how a missing or deleted comment is detected,
  how a comment is bound to its issue, how an edit is detected, and how a bot is told from a
  human. That is a change to controls, not just reads, so it has its own brief and a human gate.
wave: 6
depends: ["forge-neutral/18", "forge-neutral/33"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
decision-issue: 2317
issues: [2317]
schema: brief-v2
outcome: none
authored: 2026-10-06 by forge-neutral authoring session (split from forge-neutral/18 on review of its authoring change)
sources:
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md (forge-neutral/18) — moves every other statusgen forge-CLI site onto `deskread` and adds `issue` to `deskread`'s CI-transport kind set; its Verify row 3 excludes `statusgen/decisionruling.go` because this brief owns it, and its row 19 pins the one line it leaves in `statusgen/transcribescan.go` to `ghCommentResolver`, which this brief also owns"
  - "statusgen/transcribescan.go — `ghCommentResolver` (`:104-129`, its `gh` launch at `:109`), `parseCommentURL` (`:136-159`), the R-7 enactment gate `transcribeEnactmentGate` (`:180-216`, the blessing-authority login and id check at `:205`, the `User` check at `:209`); `ghAuthorResolver` at `:76` is a plain trust read and stays forge-neutral/18's"
  - "statusgen/transcribeverdict.go — the R-6 enactment gate `transcribeVerdictEnactmentGate` (`:423-459`; login and id at `:448`, `User` at `:452`), which reads the same `commentResolver`"
  - "statusgen/main.go:1980,1987,1995 — the three modes that pass `ghCommentResolver` to the gates (`--transcribe-scan`, `--transcribe-scan-delta`, `--transcribe-verdict`)"
  - "docs/streams/forge-neutral/brief-33-forge-reads-for-statusgen-s-remaining-sites.md (forge-neutral/33) — adds `Comment.UpdatedAt` (its Task 2.8), the comment id, author type and URL on the `comments` kind, and the `issue` kind (its Task 3); this brief consumes all of them"
  - "#2253 and its ruling (option a): which credential statusgen's CI-only modes read under. `--corroborate`, the mode this resolver runs in, is one of them; the CI-transport brief in #2314 carries that ruling's work"
  - "statusgen/decisionruling.go — `resolveRuling` (`:228`), its comment read (`:262`), the comment-to-issue binding (`:285-292`), the edit check (`:296-307`), the bot check (`:311-313`), the issue read (`:354`), `resolveDecisionRulings` (`:620`) and `rulingForgeClient` (`:633-648`, the `gh auth token` fallback at `:643`)"
  - "statusgen/corroborate.go:1481 — the one caller, which builds the client"
  - "statusgen/ghfetch.go — `ghClient` (`:40`), `newGHClient` (`:52`), `GetIssueComment` (`:198`), `GetIssue` (`:205`)"
  - "tools/desk/internal/deskkit/forge.go:68-73 — `Account.Type`: `User`, `Bot`, `Organization` or `Mannequin`; empty is could-not-check, never `User`"
  - "tools/desk/cmd/deskautolane/forge.go:193-201 — the precedent this brief follows: an empty author type is could-not-check, and any type other than `User` is refused"
  - "tools/desk/internal/deskkit/forge_github.go:1749-1761 — the GitHub comment read re-adds the `[bot]` suffix to a bot's login from the same `__typename` it reports as the author type"
  - "freshness-checked 2026-10-06 @ 35c303e47: `decisionruling.go:643` is the only forge-CLI launch in the file; row 3 of forge-neutral/18 prints 32 over non-test statusgen and 31 with this file excluded; the widened credential grep (row 4 below) matches 13 lines across `decisionruling.go` and `corroborate.go`"
  - "re-checked 2026-10-06 @ e6cb7d2a0 with BSD and GNU grep: 18's row 3 without its path filter prints 32 and with it 31; `transcribescan.go` carries two of those lines, `:77` in `ghAuthorResolver` and `:109` in `ghCommentResolver`, with identical text, which is why no path or text filter can split them and row 19 of 18 splits them by enclosing function"
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
  The sign-off resolver feeds the R-7 and R-6 enactment gates, which arm the unattended
  transcribe lanes. This brief rewrites its inputs the same way: one comment read by id becomes
  a selection by numeric id from the linked issue's thread, and the author type the gates
  require to be `User` comes from the comments kind instead of the REST user object.
  The human confirms that no input either control decides on can read as accepted when it could
  not be checked, and that neither resolver holds a credential of its own afterwards.
decision-trigger: start
domain: complicated
consumers:
  - "statusgen/decisionruling.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/decisionruling_test.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/decisionruling-mutations.json: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/corroborate.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — the one call at `:1481`; the file's other sites are forge-neutral/18's)"
  - "statusgen/ghfetch.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — `GetIssueComment` and `GetIssue` go once nothing calls them)"
  - "statusgen/transcribescan.go `ghCommentResolver`: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — the sign-off resolver at `:104-129` and its launch at `:109`; `ghAuthorResolver` at `:76` in the same file is forge-neutral/18's)"
  - "statusgen/transcribescan_test.go, statusgen/transcribeverdict_test.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — the reader-backed sign-off mapping test of row 7; the existing gate tests inject a `commentResolver` fixture and stay as they are)"
  - "statusgen/signoffresolver-mutations.json: follow-up forge-neutral/35 (this brief; a new manifest for the sign-off mapping, row 7; flips to fixed-here when the implementation adds the path)"
  - "statusgen/main.go: follow-up forge-neutral/35 (this brief; flips to fixed-here when the implementation edits the path — the three `ghCommentResolver` arguments at `:1980`, `:1987` and `:1995`; the file's other changes are forge-neutral/18's)"
  - "statusgen/transcribeverdict.go: out-of-scope (the R-6 gate at `:423` reads the resolver through the unchanged `commentResolver` type, so its accept and refuse rule stays byte-identical; the file's own forge-CLI sites are forge-neutral/18's)"
  - "tools/desk/cmd/deskread: out-of-scope (the `comments` and `issue` kinds and their fields are forge-neutral/33's; `issue` on the CI-transport kind set is forge-neutral/18's)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (no operation added; every read here already exists or is forge-neutral/33's)"
version: 1
id: 61620d38-216a-4c21-b390-046d3d03402c
---

# Brief 35 — Human-ruling resolvers onto the read verb

## Context

**A design record is required before dispatch.** This brief is risk-gated
(`sensitive-data: yes`), so under brief-rule 48 it may not leave `todo` until its `design:` key
names an approved `DR-<slug>` record in the decisions register, and its sign-off is the
`gate: human` decision issue. The decision issue exists: it is #2317, filed at this brief's
first review dispatch and named by `decision-issue:`. The design record does not exist yet: it
is written and approved by the human, not by the authoring session. Dispatching this brief
without it is refused by `statusgen --lint`.

**Two resolvers, one brief.** statusgen has two resolvers that decide whether a human ruled,
and this brief owns both. The ruling resolver (`decisionruling.go`) decides whether a decision
record's ruling link is a real, unedited comment by a human allowed to decide. The sign-off
resolver (`ghCommentResolver` in `transcribescan.go`) resolves a ruling's `**Sign-off:**` link
to its author and body, and the R-7 and R-6 enactment gates (`transcribescan.go:180`,
`transcribeverdict.go:423`) arm the unattended transcribe lanes only when that author is the
blessing authority by login and numeric id, is of type `User`, and the body names the ruling.
Moving either resolver rewrites the inputs its control decides on, so neither moves under
forge-neutral/18's model gate.

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
- `statusgen/transcribescan.go` — `ghCommentResolver` (`:104-129`), whose one `gh` launch is at
  `:109`, and `parseCommentURL` (`:136-159`), which today keeps only the repository and the
  comment id. `ghAuthorResolver` (`:76`) in the same file is a trust-predicate read and stays
  forge-neutral/18's.
- `statusgen/main.go` — the three `ghCommentResolver` arguments (`:1980`, `:1987`, `:1995`).
- `statusgen/transcribescan_test.go` and `statusgen/transcribeverdict_test.go` — the gate tests;
  they inject a `commentResolver` fixture, so they keep passing unchanged.

**What changes for the ruling control.** This brief changes no accept or refuse rule's meaning, and no
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

**What changes for the sign-off control.** The two enactment gates keep their rule byte for
byte: the `commentResolver` type stays `func(url string) (authorIdentity, string, error)`, and
the gates' login, id, `User` and body checks are not edited. What changes is how the resolver
fills those inputs:
- **Which comment.** Today one REST read of the comment by its id, and the URL's issue number is
  ignored. After: `deskread comments` on the issue or change number in the URL, selecting the
  comment by numeric `databaseId`. A comment that is not in the linked thread is not found, so
  a link whose number and comment id disagree no longer resolves.
- **Missing comment.** Today an HTTP error from `gh api`. After: the id is absent from a thread
  the kind read completely. Both return an error, and an error keeps the lane inert.
- **Author type.** Today `.user.type` from the REST payload. After: the comments kind's
  `authorType`, which the GitHub backend takes from `__typename`. An empty type is
  could-not-check and returns an error; it is never filled as `User`.
- **Author id.** Today `.user.id`. After: the comments kind's author id. A zero or missing id
  returns an error, ahead of the gate's own `author.ID == 0` refusal.
- **Edit.** No change. The sign-off gates have no edit check today and this brief adds none.
  Adding one would change the gate's rule, which is a separate decision.
- **Credential.** Today the ambient `gh` login. After: none of its own, as for the ruling
  resolver.

single-point-of-failure: for each resolver, the control is its mapping from a `deskread`
result to an outcome, and every unknown must map to a refusal. Go does not make a caller handle
a returned value, so the three-state reader result is a convention the mapping tests check,
not a layer the compiler enforces. The layers behind each mapping are these:
- **Ruling resolver.** The second layer is the human-login map. An author is accepted only if
  its login maps to a human allowed to decide (`decisionruling.go:316-343`).
  That is a different signal (operator configuration) from the type the forge reports, so a
  type mapping that wrongly reads a bot as `User` is still refused unless the bot's login is in
  the human map. The map does not catch a bot whose login is in it, which is why row 3's
  non-`User` cases use mapped logins and why mutation (c) and (d) exist.
- **Sign-off resolver.** The second layer is the gates' pin on the blessing authority's login
  AND numeric id from the roster (`transcribescan.go:205`, `transcribeverdict.go:448`). A
  mapping that reads a non-`User` author as `User` still arms nothing unless that author is the
  blessing authority's own account by id. The `User` check (`:209`, `:452`) is the only layer
  for a non-`User` account carrying that id, and row 7's cases cover it.
- **Both.** The mutation manifests re-run the tests against named changes to each mapping
  (rows 3 and 7). That trips on a different signal, a surviving mutant, in a different place,
  the harness, from the mapping's own tests.

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
- Re-checked at `e6cb7d2a0`: `ghCommentResolver` (`transcribescan.go:104-129`) runs
  `gh api repos/<repo>/issues/comments/<id>` at `:109` and returns login, id, `.user.type` and
  body. `main.go` passes it to all three transcribe modes (`:1980`, `:1987`, `:1995`). The
  line at `:109` reads the same as `ghAuthorResolver`'s launch at `:77`, so forge-neutral/18's
  row 19 tells them apart by enclosing function, not by text or path.
- A second edit signal for the ruling resolver, `lastEditedAt` (set by content edits only), is
  not on the seam's comment read and forge-neutral/33 does not add it. This brief uses
  `createdAt` and `updatedAt` as today and does not add the field.

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
  `decisionruling.go` or at its call site. The same holds for the sign-off resolver: it reads
  through the run's reader and nothing else.
- **Do not edit either enactment gate.** `transcribeEnactmentGate` and
  `transcribeVerdictEnactmentGate` keep their checks unchanged, and `commentResolver` keeps its
  signature. Only the resolver behind it changes.
- **No outcome moves from refused to accepted.** Two refusals change their reason (Task 1);
  none disappears.
- Touch no file under `tools/desk/` and no workflow.

## Task

### 1. Move the ruling resolver's two reads onto the reader

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

### 2. Only a `User` rules a decision record

- An empty author login or an empty author type → `unresolvable-link`. Empty is
  could-not-check, never `User` (`forge.go:68-73`).
- Any author type other than `User`, compared case-insensitively → `bot-author`. That covers
  `Bot`, `Organization`, `Mannequin` and any type the forge adds later. The detail names the
  type. This follows `deskautolane/forge.go:193-201`.
- Keep the `[bot]` suffix check, but do not count it as a second layer. The GitHub backend adds
  the suffix from the same `__typename` it reports as the type (`forge_github.go:1755-1756`,
  `:1761`), so the two checks rest on one signal. The `User`-only rule is the control.

### 3. Move the sign-off resolver onto the reader

- Replace `ghCommentResolver` with a resolver built from the run's `forgeReader` that keeps the
  `commentResolver` signature. `main.go:1980`, `:1987` and `:1995` pass it in place of
  `ghCommentResolver`. Delete `ghCommentResolver` and its `gh` launch at `transcribescan.go:109`.
- `parseCommentURL` also returns the issue or change number from the URL path. A URL whose
  number does not parse is an error, as an unparseable URL is today.
- The read is `deskread comments` on that number, selecting the comment by `databaseId` equal
  to the URL's comment id. The mapping returns an error, which keeps the lane inert, for every
  unknown:
  - the verb exits non-zero, its output does not parse, its schema is not 1, or the item is in
    `partial`, including a change whose conversation thread the kind cannot read;
  - the thread was read completely and the id is not in it. Another comment in the same thread,
    even one by the blessing authority that names the ruling, is never taken in its place;
  - an empty author login, a zero or missing author id, or an empty `authorType`. Empty is
    could-not-check, never `User`;
  - under the offline reader, every sign-off.
- Otherwise it returns the author's login, numeric id and `authorType` unchanged, and the body.
  It does not normalise the type's case or fill a default. The gates compare `Type` with `User`
  exactly, and that comparison stays theirs.
- Add a mutation manifest for the mapping, `statusgen/signoffresolver-mutations.json`
  (planned), in the shape of `decisionruling-mutations.json`, holding mutations (e), (f) and
  (g) below.

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
| 7 | check:ci +mutation | `cd statusgen && go test . -run '^TestSignoffReaderMappingRefuses$' -count=1 -v > "${TMPDIR:-/tmp}/b35-r7.out" 2>&1 && grep -F -e '--- PASS: TestSignoffReaderMappingRefuses' "${TMPDIR:-/tmp}/b35-r7.out" && muhar -spec signoffresolver-mutations.json` | **negative path**: `TestSignoffReaderMappingRefuses` (planned) drives the reader-backed sign-off resolver through a fake reader, first on its own and then through both `transcribeEnactmentGate` and `transcribeVerdictEnactmentGate`. On its own, each Task 3 case returns an error: verb failure, bad schema and a `partial` item; a complete thread without the URL's comment id, where the thread also holds a blessing-authority comment naming the ruling under another id; an unparseable number in the URL; an empty author login, an author id of 0, and an empty `authorType`; and the offline reader. Through each gate, every one of those cases, plus authors of type `Bot`, `Organization` and `Mannequin`, leaves the lane INERT. Every non-`User` case uses the blessing authority's own login and numeric id, so only the type check can refuse it. The one positive case (a `User` author with the blessing authority's login and id, in a complete thread, whose body names the ruling) arms both gates. The `muhar` leg reports the baseline green, the control caught, and mutations (e), (f) and (g) caught, with none not caught |
| 8 | check +dereference | `test -f statusgen/transcribescan.go && awk '/^func /{fn=$0; sub(/^func (\([^)]*\) )?/, "", fn); sub(/\(.*/, "", fn)} /"gh"/ && !/^[[:space:]]*\/\// {print fn}' statusgen/transcribescan.go \| wc -l` | output is `0`: no non-comment line of `transcribescan.go` carries the literal `"gh"`, in any function. This is forge-neutral/18's row 19 run after this brief: 18 leaves exactly one line, in `ghCommentResolver`, and this brief removes it. Measured at `e6cb7d2a0` with BSD awk and GNU awk: `2` (`ghAuthorResolver`, `ghCommentResolver`). Row 5 counts the same line in its whole-tree total; this row names the file so a reviewer sees which resolver it is |

### Named mutations for the `+mutation` rows

| Row | The mutation that must redden it |
|---|---|
| 3 | (a) read a `partial` thread as a complete thread without the comment, so the outcome is `deleted-comment` where it must be `unresolvable-link`. (b) treat an empty `updatedAt` as equal to `createdAt`. (c) narrow the `User` requirement back to a `Bot` check, so `Organization` and `Mannequin` authors with a mapped login pass to the human map and are accepted. (d) drop the author-type check entirely, so a `Bot` author with a mapped login and no `[bot]` suffix is accepted. Each must fail the row on the reason, and (c) and (d) are added to the manifest as named entries so the `muhar` leg reddens on them too |
| 7 | (e) fill an empty `authorType` with `User`, so a comment by the blessing authority's login and id that the forge did not classify arms the lane. (f) select the comment by author login, or take the first comment in the thread, instead of by `databaseId`, so another blessing-authority comment naming the ruling arms the lane when the linked one is missing. (g) on a verb failure or a `partial` item, return a zero identity with no error, so the resolver reports a read where none happened. (g) is caught by the resolver-level cases even though the gates' own `author.ID == 0` check would refuse it; that is the point of testing the resolver on its own |

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
| The sign-off resolver reads an unclassified author as `User`, so the blessing authority's id on an account the forge did not classify arms a transcribe lane | row 7, mutation (e) |
| The sign-off resolver picks some other blessing-authority comment from the thread when the linked one is missing or was deleted | row 7, mutation (f) |
| A failed or partial sign-off read returns an empty identity with no error, and only the gate's id check happens to stop it | row 7, mutation (g) |
| The move edits a gate's check, for example to adapt to the new type field, and loosens what arms a lane | ground rule "Do not edit either enactment gate"; Review confirms the diff leaves both gate functions unchanged |
| `ghCommentResolver` is left behind or re-expressed in a form row 5's text count cannot see | row 8 names the file; Review confirms the launch at `:109` is gone, not hidden |
| The control is moved to a fresh read path whose inputs a human never reviewed | the `gate: human` sign-off and the design record required before dispatch |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter; `sensitive-data: yes` — see `gate-why` and the two "What
changes" notes in `## Context`). The reviewer confirms that neither resolver holds a credential
of its own after the move (no token read, no `gh auth token`, no client built in
`decisionruling.go` or at `corroborate.go:1481`, no `gh` launch in `transcribescan.go`), that
every unknown in both mappings refuses, that only a `User` author can rule or sign off, and that
the diff leaves `transcribeEnactmentGate` and `transcribeVerdictEnactmentGate` unchanged. The
human signs off on #2317 against the approved design record.
