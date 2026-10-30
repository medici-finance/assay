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

A Bash call that runs the gh CLI's token read (`gh auth token`, or `gh auth status`
asked to print the token) is refused while the hook's own environment has
`DESK_LOOP` or `DESK_SESSION` non-empty; with neither set the hook stays silent.
The stored gh login is whichever account last authenticated on the machine, never
the session's role identity. The refusal names `desktoken <role>` as the only
sanctioned path and says a failed or empty mint is a stop to report.

- **Split.** The core classifies (`Classification.ReadsAmbientForgeToken`, pure and
  lexical); the adapter reads the two variables and renders the refusal.
- **Placement.** It runs right after the Bash input decodes, before checkout
  discovery: it guards a credential, not a path, so a session outside any checkout
  is refused too. Malformed hook input or tool input stays a no-decision (fail open).
- **What matches.** The refusal covers the executed forms the lexer
  (`deskcore/writepolicy/forgetoken.go`) judges, and its tests pin each one. It is a
  lexical backstop, not a shell, and it does not claim every executed form. The read
  is a gh invocation whose first positional is `auth` and a later positional is
  `token` (case-insensitive). The other read forms are `gh auth status` asked to
  print the token (`--show-token`, any true spelling of it, or a short cluster with
  `-t`), `gh auth git-credential`, and `gh config get … oauth_token`. A gh flag's
  value (`-u`, `-h`, `-s`, …) is not a positional, so `gh auth switch --user token`
  is allowed. The executed forms judged are:
  - a command word after assignments, redirections and reserved words;
  - every word after a command wrapper or runner, or after a command word computed
    at run time, each tried once. The wrappers and runners are a closed set, named
    in code: the shared `commandWrappers` (`env`, `nohup`, `time`, `command`,
    `exec`, `sudo`, `nice`, `stdbuf`, `timeout`, `xargs`, `caffeinate`, `arch`,
    `setsid`, `ionice`) and the scan's own `forgeRunners` (`doas`, `run0`,
    `busybox`, `unbuffer`, `script`, `flock`, `watch`, `su`, `runuser`,
    `chronic`);
  - the scripts of a shell's `-c`, `eval`, `env -S`, and runners that hand their
    argument to a shell;
  - here-strings fed to a shell, and the arguments of the pipe stage immediately
    before a shell (`echo … | sh`);
  - plain substitutions (below), quoted or not;
  - heredoc bodies whose introducing line names a shell anywhere (piped, wrapped,
    or after an assignment).

  Quoting (including ANSI-C `$'…'` with escapes decoded, and locale `$"…"`),
  escaping, or a line continuation inside a word does not hide the read. A word
  with one part computed at run time (parameter expansion, a substitution, brace
  expansion, a glob) may stand for any one component of the read. One such
  component with the rest literal is judged a read.
- **Inert data.** Masking conventions hold: comments, heredoc bodies written to a
  non-shell under a quoted delimiter, and quoted prose (`git commit -m "…"`) do not
  match. An unquoted argument such as `echo gh auth token` is not a gh invocation.
- **Modeled constructs.** The scan models these exactly and judges the commands
  they spell: words, blanks, escapes and line continuations; `'…'`; `$'…'` when
  closed and holding no `\'`; `$"…"`; `"…"` holding only text, escapes, `$name`
  and plain expansions; `$name`; a plain `${…}` (no quote, backslash, backquote,
  `$`, brace, paren or newline inside); a plain `$(…)` (no paren, backslash,
  backquote, `#`, `<<`, `case`, bracket or brace other than a plain `${…}`, `$`
  before a quote, paren or bracket, and only simple quotes inside), which every
  shell ends at its first `)` and whose text is lexed as a script; comments;
  operators, subshell parentheses and group braces; redirections and
  here-strings; and a heredoc whose delimiter holds no substitution and whose
  delimiter line exists (exactly the word; leading tabs stripped for `<<-`),
  with, under an unquoted delimiter, a body holding no line continuation and
  only plain expansions.
- **Unmodeled constructs fail closed (desk-decided).** Everything else is
  decided at one point, without placing the construct's end: a non-plain or
  nested `$(…)`, backquotes, `$((…))`, `((…))`, `$[…]`, a non-plain `${…}`, a
  `$name[` inside double quotes, a `name[…]` subscript holding blanks or shell
  syntax, `<(…)` and `>(…)`, a `(` written against a word (functions, arrays,
  glob qualifiers), any unclosed quote or span, a `$'…'` holding `\'` (a POSIX sh
  ends it elsewhere), a heredoc delimiter holding a substitution, a heredoc with
  no delimiter line, and an unquoted heredoc body holding a line continuation
  (the shells join the lines before comparing the delimiter) or a non-plain
  expansion. From the start of the pipeline holding the construct to the end of
  the script, the call is refused if the read's trigger appears, and the scan
  stops otherwise. The trigger is a whole word `auth`, `token`, `oauth_token` or
  `git-credential`, or the words `status` and `gh` together, seen through quotes,
  backslashes, line continuations and decoded escapes. Every read the judge
  refuses holds one of them. False refuses are accepted; false allows are not.
  This departs from the policy's allow-and-log default and is recorded on the
  PR that added it. Nesting deeper than eight levels, and a scan that exhausts
  its work budget, are refused outright whatever the text spells.
- **Idiom outcomes.** `git commit -m "$(cat <<'MSG' … MSG)"` and
  `gh … --body "$(cat <<'EOF' … EOF)"` are unmodeled (a heredoc in a
  substitution). Both are allowed when the trigger is absent and refused when
  the message or body says, for example, "token" or "auth". The way through is
  `git commit -F <file>` or `--body-file <file>`. A plain substitution
  (`--body "$(cat body.md)"`) is lexed and judged, so its text is not matched.
- **Work bound.** The scan reads the raw command, with no masking pass of its own,
  and charges every step to a budget of 32 units per byte plus a fixed base: each
  script lexed (its length), each candidate command position and gh argument
  judged (its length), and each repeat of an already-lexed script. Every heredoc
  operator, quote and substitution is read once. So the scan's own work is linear
  in the command's length, and past the budget it stops and refuses. This bound
  covers the forge scan only. `Classify`, which runs before it, still applies the
  shared quoted-span masking, whose cost grows faster than linear on long runs of
  quoted spans; that shared cost predates this check and is tracked in #4854.
  Text piped or here-string-fed into a shell is lexed even when the shell also has
  `-c`, so a shell that ignores its stdin can still be refused.
- **Not covered.** The following are not judged and stay allowed:
  - a script sourced with `source` / `.`;
  - shell input read from a file or device whose text is not on the command line;
  - a read with more than one component computed at run time, and a show-token
    flag computed at run time;
  - arguments supplied at run time by another command (for example `xargs`);
  - command runners outside the closed sets above (for example `find -exec`, or a
    tracing, scheduling or namespace tool), whose later words are not judged as a
    command;
  - text that reaches a shell through a pass-through stage (`echo … | cat | sh`,
    `tee`): only the stage immediately before the shell is read;
  - other paths that turn a string into a command: tool aliases (including git
    aliases), build-tool evaluation, interpreter one-liners, OS scripting bridges,
    and encoded shell commands;
  - gh aliases;
  - scripts that read the token on the caller's behalf;
  - other ways to read the stored credential: its config file, a keychain query,
    or a git credential helper;
  - harness tools other than Bash;
  - remote or container execution.

  The refusal is a client-side backstop. It is not the only control.

| # | Verify | Method | Expected |
|---|---|---|---|
| 1 | Hook-level refusal, variants, one test per judged class, allowed look-alikes, silence and fail-open | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientToken' -count=1` | PASS |
| 2 | Core classification, including the depth fail-closed and ANSI-C decoding (a `$'…'` holding `\'` is unmodeled) | `cd deskcore && go test ./writepolicy/ -run 'TestAmbientForgeTokenClassify\|TestForgeANSICDecode' -count=1` | PASS |
| 3 | Positive control: a matcher that never fires is caught | `cd tools/desk && go test ./cmd/writeguard/ -run 'TestWriteHookParity/production-mutations/ambient-token-unmatched' -count=1` | PASS (mutant caught) |
| 4 | Operator `!` input is untouched (paired control, manual; re-run after any harness change) | One session, hook armed, `DESK_LOOP` or `DESK_SESSION` set in the hook's environment. (a) The agent runs a control command through its Bash tool, e.g. `gh auth token >/dev/null`. (b) The operator types the identical command with the `!` prefix in the same session. | (a) refused with `ambient forge credential read`; (b) runs with no refusal. A refusal in (b) means the harness now routes operator input through PreToolUse: stop and re-decide the gate. |
| 5 | The scan's own work is bounded for the shapes tested: wrapper chains, repeated ANSI-C spans, many heredoc operators on one line, long runs of quoted spans and many substitutions whose two ends differ each decide within the per-byte budget and within a deadline, and the hook decides those shapes within its deadline. The deadline is shown for these shapes only: the shared masking that runs first is not bounded while #4854 is open | `cd deskcore && go test ./writepolicy/ -run 'TestForgeWrapperChainLinear\|TestForgeScanWorkBounded' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientTokenWrapperChain\|TestAmbientTokenScanDeadline' -count=1` | PASS |
| 6 | Budget exhaustion and excess nesting are refused outright, whatever the text spells, including decoded and split spellings and a command naming no read | `cd deskcore && go test ./writepolicy/ -run 'TestForgeBudgetFailsClosed\|TestForgeUndecidedRefused' -count=1` | PASS |
| 7 | Heredoc placement: a delimiter line matches only exactly, data heredocs stay allowed, and `((`, `$((`, `$[`, `${…}` and `name[…]` shapes around a `<<` are refused with the trigger and allowed without, at the core and at the hook | `cd deskcore && go test ./writepolicy/ -run 'TestForgeArithHeredoc\|TestForgeShiftSiblings' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestAmbientTokenArithHeredoc\|TestAmbientTokenSubstHeredoc' -count=1` | PASS |
| 8 | Unmodeled constructs: one table per construct (command substitution, backquote, heredoc body, arithmetic, `((`, process substitution, unclosed span, quote forms) refused with each trigger spelling and allowed with none; the earlier reviews' span-end classes refused; the two idioms pinned both ways, at the core and at the hook | `cd deskcore && go test ./writepolicy/ -run 'TestUnmodeled\|TestForgeReviewShapes\|TestForgeIdiomOutcomes' -count=1`; `cd tools/desk && go test ./cmd/writeguard/ -run 'TestHookUnmodeled\|TestHookReviewShapes\|TestHookIdiomOutcomes' -count=1` | PASS |

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
