# Role-bound Git readers

`deskdispatch` refreshes an existing change's source branch using the dispatching
session's bound role. `CheckoutGitRead` reads the configured `origin` through
`gitcore.RemoteURLs`, checks the expected repository and the credential's HTTPS
host and port, then supplies authentication directly to gitcore's in-process
transport. The configured URL is retained. A failed refresh or a tip different
from the forge's source head stops admission before a claim or worktree is created.

`deskgit heads --repo owner/name` exposes the same read transport across a process
boundary. It reads every advertised branch and emits one versioned JSON envelope
with the repository, `complete: true` and a `heads` array (possibly empty). Errors
emit no successful envelope. The role comes from `DESK_LOOP`; there is no caller
role override, ambient login, credential environment or persistent config change.
Heads reads retain audit and breaker visibility without charging outward-write
budgets. Both the bounded and fallback ledger readers use the shared explicit
read classification; unknown operations and results keep conservative charging.

`deskboard` always adds `--role-claims` to its pinned `statusgen --next-up` child.
Statusgen uses `deskgit heads` for that flag or a nonempty `DESK_LOOP`. A missing
loop identity, unavailable custody, failed subprocess, truncated or partial
advertisement, wrong repository/schema, or missing heads array remains unknown
claims. There is no retry through native Git. Existing `claimsKnown` degradation
and `--require-claims` behavior still decide whether to render or refuse the board.
The completeness check occurs before claim filtering and capping.

A generic statusgen invocation without `--role-claims` and without a desk loop
retains its existing bounded native-Git branch listing. Offline lint retains its
existing zero-network behavior. The new statusgen flag and deskgit command must
be released together; a board using an older pinned statusgen fails visibly on
the unsupported flag rather than silently reading with ambient credentials.

## Consumer inventory

- Existing-change source refresh: `deskdispatch/worktree.go`, repaired here.
  Source/head equality and the exact remote-tracking refspec remain enforced.
- Sibling-root claim reads: `deskboard/dispatch.go` → pinned statusgen next-up →
  `statusgen/claimtransport.go` → `deskgit heads`, repaired here. Both dispatch and
  throughput use this path. Generic native-Git listing remains outside desk mode.
- Other dispatch subprocesses: local Git probes, worktree allocation, local roster
  writes and the separately authenticated claim/decision children retain their
  existing owners. The allocation guard no longer allows a native network fetch.
- Other board subprocesses: pinned statusgen gate/awaiting/local metadata reads do
  not use the branch-listing transport; forge API reads retain their existing
  resolved-role adapters.
- `statusgen newbrief` freshness and `statusgen mergecheck --fetch` are separate
  opt-in refresh consumers, outside the board/resume call chains. Their native
  fetches need a separate refscope-preserving migration; this repair does not
  claim to change them. The open follow-up scope and requested worker owner are
  retained in [issue #3327](https://github.com/medici-finance/assay/issues/3327#issuecomment-6241634796); a separate issue filing hit the filing rate limit. Generic Git transport operations across other commands
  retain their independently qualified owners.

The transport implementation, dependency versions and CLI entrypoints undergoing
other migrations are retained. Fetch URL compatibility remains gitcore's owner;
no second URL-rewrite resolver is introduced. Statusgen links only the data-only
forgeread envelope and continues to obtain authenticated reads through desk tools.
