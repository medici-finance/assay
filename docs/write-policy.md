# Shared write policy and the writeguard adapter

**Status:** source implementation contract, 2026-10-20, approved for bounded
extraction in [#3752](https://github.com/medici-finance/assay/issues/3752#issuecomment-6299695731).
Installed hooks, publication and managed qualification remain separate gates.
The exact source/API/profile/build handoff is in [the census](write-policy-consumers.md).

## Outcome and ownership

One importable evaluator owns write-isolation decisions. The package is
`github.com/medici-finance/assay/deskcore/writepolicy`, inside the existing pure
`deskcore` module. It is separate from `deskcore/policy`'s approved role/forge grants:
local workspace isolation cannot grant forge authority. Publish compatible module
versions and qualify actual external consumers; a repository-relative replace is
source integration, not release proof. Do not create a second shared-core module.

The `S-write-policy` row in [contracts.md](contracts.md) records the core owner;
its marker baseline and checked consumers move with the implementation. `S-worktree` remains allocation, identity and lifecycle;
process leases, scratch preservation, push authorization and credentials keep their
existing owners. A workspace allocation grants no additional permission.
Existing authorized Git effects may need allocation-specific administration/object/ref
writes outside the checkout. Their adapter retains that scoped authority; generic
agent writes to protected store/config/control paths do not inherit it. Qualify an
owned staged commit as well as refusal of foreign refs/config; do not make ordinary
Git work impossible by declaring the entire common directory unwritable.

## Core and adapters

| Layer | Owns | Forbidden capability |
|---|---|---|
| `deskcore/writepolicy` | Typed operation/path checks, pure shell-command classification, exemption rules, sentinel protection and structured outcomes/reasons | Reading environment/config/files, resolving symlinks through I/O, Git, clocks, process/callout execution, network or credentials |
| `cmd/writeguard` | Existing stdin/JSON hook protocol; configuration and activation; trusted Git/path/claim observations; optional callout execution; mapping outcomes to harness responses | A second policy implementation or new authority source |
| Managed launcher/harness adapter | Map validated allocation, repository, generation, session and sandbox roots into evaluator inputs | Inferring an allocation from cwd, path prefix or an agent-supplied identifier; widening sandbox roots to satisfy a hook |
| Harness/container | Actual write-access confinement | Treating a library verdict as a filesystem sandbox |

`Classify(Request, LexicalContext)` freezes explicit lexical context, path-like
command tokens and literal masking in an immutable classification.
`Evaluate(Evaluation)` accepts that classification with its ID, protected roots,
explicit path/claim/context/callout observations, and the named profile. It returns
`Outcome` with decision, reason, schema revision, details and outstanding observation
requests. [policy.go](../deskcore/writepolicy/policy.go) is the exact Go surface;
[census and handoff](write-policy-consumers.md#implemented-api-and-profile-handoff)
describes the request/observation round trip and versions.
No `workspace`, desk-internal or command import enters the core. Adapters map a
validated workspace record to values; the values do not authenticate themselves.

Classification cannot observe the filesystem or execute a shell to resolve an
expression. The adapter resolves the requested paths (including nearest existing
ancestors for new files), supplies those observations, and reports failed or dynamic
resolution explicitly. Independent tests must cover this round trip, not just a
lexical path comparison. Revalidate relevant identity at the effect boundary;
a pre-tool observation is not protection against all later filesystem races.

## Behavior and compatibility

The evaluator distinguishes `allow`, `deny` and `unknown`, with stable reason codes
and applicable policy/evaluator revisions. Unknown is not a clean evaluation. The
adapter owns protocol mapping, so extraction preserves these existing distinctions:

- Legacy malformed hook input or Git-discovery failure yields no decision, as today.
- A configured callout can only add refusals; timeout, invalid answer or execution
  failure blocks. Preserve configuration-source restrictions and report the existing
  invalid-roster/unconfigured fallback rather than silently claiming it fail-closed.
- The shared-homed exemption requires the current claim and home/cwd conditions.
  Sentinel writes are checked before the exemption. Preserve expiry, scoping and
  path-alias handling; do not let an agent mint or renew its own exemption.
- Compiled shell checks remain heuristic; unknown interpreter/script effects are
  not newly claimed as covered. Preserve activation and missing-binary behavior.
- Managed admission requires independently validated allocation/session/repository
  identity and compatible versions. Missing or conflicting mandatory evidence holds
  that managed operation with a reason; it never falls back to legacy permission.
  This source profile is not a qualified or activated managed deployment.

A deny from the core cannot be cleared by a callout or adapter. Keep existing hook
JSON/exit/stdout behavior, including structured JSON denial on exit zero. Managed
decisions do not replace OS/harness confinement or operator activation gates.

## Ambient forge-credential refusal

A Bash call whose text holds a word of the gh CLI's token read (`gh auth token`,
or gh asked to print its stored token) is refused while the hook's own environment
has `DESK_LOOP` or `DESK_SESSION` non-empty; with neither set the hook stays
silent. The stored gh login is whichever account last authenticated on the
machine, never the session's role identity. The refusal names `desktoken <role>`
as the only sanctioned path and says a failed or empty mint is a stop to report.

- **Split.** The core classifies (`Classification.ReadsAmbientForgeToken`, pure and
  lexical); the adapter reads the two variables and renders the refusal.
- **Placement.** It runs right after the Bash input decodes, before checkout
  discovery: it guards a credential, not a path, so a session outside any checkout
  is refused too. Malformed hook input or tool input stays a no-decision (fail open).
- **What refuses.** A trigger word anywhere in the command text refuses
  (`deskcore/writepolicy/forgetoken.go`). The guard does not parse the shell: it has
  no lexer, no command positions and no quoting or heredoc model. Its one piece of
  shell modelling is the normalization below, and that keeps every reading it
  models instead of choosing one, so any reading that completes a trigger word
  refuses. The trigger words are a whole word `auth`,
  `token`, `oauth_token` or `git-credential`, or the words `gh` and `status`
  together, matched case-insensitively. Before matching, one pass drops quotes, a
  `$` before a quote, backslashes and line continuations, and decodes `\xHH`,
  `\uHHHH`, `\UHHHHHHHH` and octal escapes. A word starts and ends at a blank, a
  newline or one of ``;&|()<>`{},``, and at a backslash sequence that a shell,
  `echo -e` or printf may read as a boundary: a line continuation (a shell joins
  it mid-input but drops it at the end of its input, such as the last body line
  of a quoted heredoc fed to a shell), `\n`, `\t`, `\r`, `\cI`, `\cJ`, `\cM`, or a
  numeric escape that decodes to a blank, newline or operator byte. Each such
  sequence is read both ways, as part of the word and as a boundary. So
  `GH_TOKEN`, `desktoken`, `--token-file`, a path component such as `./auth/x`,
  and a test name holding `Token` do not match. The match is one pass per trigger
  word, linear in the command's length.
- **Accepted false refusals (desk-decided).** Text naming a trigger word as data is
  refused like a read. Examples: `echo 'gh auth token'`, `grep -n 'auth' notes.md`,
  `rg -n token src/`, a comment, a heredoc written to a file that says "token",
  `mkdir -p auth`, `gh auth status`, `gh auth login`, `gh pr view token`,
  `gh run list --json status,name`, `git status && gh pr view 1`, and a commit
  message, PR body or comment passed inline (`-m "…"`, `--body "…"`) that says
  "auth" or "token". The way through is a file: `git commit -F <file>`,
  `--body-file <file>`, or a pattern file (`grep -f <file>`). This departs from the
  policy's allow-and-log default and is recorded on the PR that added it.
- **Coverage.** The guard covers accidental or habitual ambient token reads through
  the gh CLI's commands by a cooperating session. It does not cover a credential
  read that bypasses those commands: through a git credential helper, the OS
  keychain, or a direct read of the gh CLI's config file. Deliberate evasion
  (dynamic evaluation, decoding, command names built from parts) is out of scope
  for this guard and is left to other layers: no operator credential in verify containers, and server-side App
  permissions. This scope is a relay of the driver's answer
  (https://github.com/medici-finance/assay/issues/4821#issuecomment-6523907010),
  which the driver's merge of the PR that added this section ratifies.

| # | Verify | Method | Expected |
|---|---|---|---|
| 1 | Hook-level refusal, variants, one test per form class, the accepted false refusals, trigger-free gh use allowed, silence and fail-open | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientToken\|TestHookMentionsRefused' -count=1` | PASS |
| 2 | Core classification: trigger words anywhere, escapes decoded, trigger-free text allowed however nested | `cd deskcore && go test ./writepolicy/ -run 'TestAmbientForgeTokenClassify\|TestForgeANSICDecode\|TestForgeMentionsRefused\|TestForgeNoTriggerAllowed' -count=1` | PASS |
| 3 | Positive control: a matcher that never fires is caught | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestWriteHookParity/production-mutations/ambient-token-unmatched' -count=1` | PASS (mutant caught) |
| 4 | Operator `!` input is untouched (paired control, manual; re-run after any harness change) | One session, hook armed, `DESK_LOOP` or `DESK_SESSION` set in the hook's environment. (a) The agent runs a control command through its Bash tool, e.g. `gh auth token >/dev/null`. (b) The operator types the identical command with the `!` prefix in the same session. | (a) refused with `ambient forge credential read`; (b) runs with no refusal. A refusal in (b) means the harness now routes operator input through PreToolUse: stop and re-decide the gate. |
| 5 | The match decides within a deadline on long inputs: wrapper chains, repeated ANSI-C spans, many heredoc operators, long runs of quoted spans, many substitutions, deep nesting and a megabyte of text, at the core and at the hook. The shared masking `Classify` runs first is not bounded while #4854 is open | `cd deskcore && go test ./writepolicy/ -run 'TestForgeWrapperChainLinear\|TestForgeScanWorkBounded' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientTokenWrapperChain\|TestAmbientTokenScanDeadline' -count=1` | PASS |
| 6 | Shapes a shell lexer could misplace (heredoc delimiters, `((`, `$((`, `$[`, `${…}`, `name[…]`, substitutions, backquotes, process substitution, unclosed spans, quote forms) refused with a trigger word and allowed without, and the message idioms refused when they name one, at the core and at the hook | `cd deskcore && go test ./writepolicy/ -run 'TestForgeArithHeredoc\|TestForgeShiftSiblings\|TestUnmodeled\|TestForgeReviewShapes\|TestForgeIdiomOutcomes' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientTokenArithHeredoc\|TestAmbientTokenSubstHeredoc\|TestHookUnmodeled\|TestHookReviewShapes\|TestHookIdiomOutcomes' -count=1` | PASS |
| 7 | One test per class the latest reviews found (word span, command prefix, dynamic arity, clobber operator, string run as a command, pass-through), each red on the earlier shell-model scan, at the core and at the hook | `cd deskcore && go test ./writepolicy/ -run 'TestForgeWordSpan\|TestForgeCommandPrefix\|TestForgeDynamicArity\|TestForgeClobberOperator\|TestForgeStringToCommand\|TestForgePassThrough' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestHookReviewClasses' -count=1` | PASS |
| 8 | A backslash sequence read both ways: a line continuation a shell drops at the end of its input (the last body line of a quoted heredoc fed to it), and an escape decoding to a blank or a newline at a word's end or start, at the core and at the hook | `cd deskcore && go test ./writepolicy/ -run 'TestForgeWordBreak' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestHookWordBreak' -count=1` | PASS |

## gh output filter check

In a desk session (the same two variables as above) a Bash call is refused when its
text holds the word `gh` and a `--jq` or `-q` filter, written out in the call, that
reads beyond the JSON output. The list of what counts is in the code
(`deskcore/writepolicy/ghfilter.go`, `Classification.ReadsOutsideGhOutput`). Ordinary
selection and formatting filters keep working. The check runs on any gh command.

The text is read as shell words with their quotes and backslashes removed, so a flag
and its value count however they are quoted, with any redirections between them skipped.
A string inside a filter is only taken as a string when the flag and the value are
plainly quoted. A quoted word that could hold a command is read again, and so is a
substituted command. What the shell expands in a filter is not filter text. A brace
expansion in a flag or a value is refused without being worked out.

**Limit: this is not a complete control.** It reads against that list, which was
checked on gh 2.102.0 alone, and on `sh`, `dash`, `bash` and `zsh` with their usual
settings. These are out of scope:

- A read with no filter, and printing a variable with a plain shell command.
- Other commands in the call, and other gh flags.
- A flag, a filter or the gh word that the shell computes, decodes from an escape or
  matches against files, including one assembled from a variable.
- Words changed by the session's shell settings, aliases or functions from outside the call.

The check reads words without parsing the shell or tying a flag to its command, so some
text that is not a gh filter is refused; the refusal names the way through.

| # | Verify | Method | Expected |
|---|---|---|---|
| 1 | Core: refused and allowed rows, generated words checked against real shells, and work bounded on long inputs | `cd deskcore && go test ./writepolicy/ -run 'TestGhFilter' -count=1` | PASS |
| 2 | Hook: refused in a desk session, generated words, ordinary filters allowed, silent outside one | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestHookGhFilter' -count=1` | PASS |
| 3 | Positive control: a matcher that never fires is caught | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestWriteHookParity/production-mutations/gh-filter-unmatched' -count=1` | PASS (mutant caught) |

## Consumers and migration

| Consumer | Disposition / owner | Retirement proof |
|---|---|---|
| Existing `cmd/writeguard` and command-hook wiring | desktools-v2/66 extracts policy and retains binary/protocol | Old command-local decision functions removed; common fixtures exercise import and built binary |
| Target local harness adapter | graph-execution/22 consumes /66 directly | Actual adapter/library decisions and independent sandbox refusal remain required now that the legacy CLI parser work (desktools-v2/31) is withdrawn |
| CLI parsing of writeguard and other guards | Unchanged: the desktools-v2/31 parser port is withdrawn and writeguard keeps its current parser | The hook adapter reaches the same evaluator; other guards retain their own meanings |
| Managed launch, tool hooks and host/container path mapping | managed workspace harness integration | Admitted allocation flows to hook and sandbox; missing/stale/foreign context refuses before launch/write; legacy hook integration still works |
| Other actual duplicate write-isolation evaluators | /66 discovers and records each site; migrate bounded sites there or route a named follow-up | Source-bound census with no unowned site or competing policy; calling an allocator alone does not require a writepolicy import |
| Statusgen checkout/audit lifecycle | shared workspace-library migration | Uses the workspace owner, not a blanket import of write policy; only a discovered duplicate evaluator adds a migration obligation |
| Push, credential, scratch, process and retirement guards | Out of scope: independent semantic/trust boundaries | Existing tests and enforcement remain; no blanket consolidation |

The implementation census includes current and planned consumers, source/query
revisions, exclusions, adapter owner and retirement condition. It lives at the
`docs/write-policy-consumers.md`; it is a checked inventory of sites, not
another semantic-owner registry. Public census/contracts contain neutral source
locators only. Coordinated consumers require exact producer API/profile versions;
installed hook/pin activation follows separate adopter review.

## Acceptance and quality

Preserve the existing source-bound regression cases, including false positives and
known coverage limits. Use independent expected outcomes, then compare the direct
library and real built hook on identical observations: shared write denied, owned
worktree allowed, symlink/ancestor escape, leading cd, literal/heredoc text, unresolved
shell target, shared exemption, sentinel write and configured-callout failure.
Exercise POSIX and supported Windows semantics; unsupported native observations
remain unknown. Record path/Git/callout invocation counts and elapsed time for the
same fixture workload before/after; no performance saving or new budget is invented.

Core tests run without filesystem/environment/clock access. Boundary checks inspect
transitive imports and effect calls with a planted I/O helper as a negative control;
behavioral fixtures also prove explicit observations determine the answer. A separate
external-module consumer builds with `GOWORK=off`; library/hook/version fixtures
must fail on a skipped deny, callout clearing a deny, sentinel exemption reordering,
missing managed binding or a duplicate local evaluator. Qualify actual release
contents and supported sandbox integrations before activating managed consumers.
