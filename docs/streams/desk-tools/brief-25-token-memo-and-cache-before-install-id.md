---
brief: assay:assay:desk-tools:25
title: "One token lookup per owner per process — a memo in front of the minter, and `desktoken` consulting its cache BEFORE it resolves the install id"
why: >-
  A desk verb's forge reads all authenticate as a minted App installation token, and the
  path that obtains one has no memo at any layer. `deskkit.RoleTokenForOwner` forks the
  `desktoken` binary on EVERY call, and the resolver seam above it
  (`forgeresolve.go`'s `githubCustody`) is documented as reading the credential "at call
  time, not cached" — so a board read that touches one repository twenty times forks the
  minter twenty times for a credential that has not changed. Measured on one operating
  desk host in a quiet window: 140 `desktoken` invocations per `deskboard actions --delta`
  tick over 10 allowed repositories, roughly 13.5k a day, peaking at 13 a second. Almost
  none of them need to reach GitHub — 99.7 % land inside the minter's own 50-minute reuse
  window — but each pays the full process start plus the shared substrate's audit-log
  parse, and the rows those invocations write are the single largest contributor to the
  ledger that makes every other verb slower. Underneath sits the second half of the same
  defect: `desktoken` resolves the installation id BEFORE it consults its token cache,
  because the cache path is keyed by that id, so a "reuse cached token" call still signs a
  JWT and spends a `GET /app/installations` round trip to rediscover an installation that
  changes only when somebody installs or uninstalls the App. This brief fixes both ends of
  one path: a per-process memo keyed (role, owner) in front of the minter, and a
  cache-first resolution order in `desktoken` backed by an on-disk install-id cache. It
  adds no credential, relaxes no custody check, and changes no token's lifetime.
wave: 2
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1036]
schema: brief-v2
authored: "2026-09-14 by a worker-desk authoring session, from issue #1036 and its
  caller-side comment, both re-read against the source below on the same day"
sources:
  - "#1036 — `desktoken` resolves the install id via `GET /app/installations` before checking the token cache, so every 'reuse' is still an API round trip. Its Observed section names the resolution site, the cache path shape and the `os.Stat`; its Ask names the three fixes; its Verify names the three assertions this brief's rows 8, 9 and 11 implement."
  - "#1036's caller-side comment (desk performance review, 2026-09-14) — the measured fork counts, the call chain from `tools/desk/cmd/deskboard/forge.go` through `deskkit.ForgeFor` to `exec.Command(\"desktoken\", …)`, the `deskflip` double mint, and the ask for a per-process memo keyed (role, owner)."
  - "freshness-checked 2026-09-14 @ e428134c (origin/main) — every fact below re-read against the source at that commit."
  - "`tools/desk/internal/deskkit/roletoken.go` § `tokenMinter` / `RoleTokenForOwner` / `RoleTokenForRepo` — the ONE place a desk verb turns a (role, owner) pair into a token. `tokenMinter` is `exec.Command(\"desktoken\", role, \"--repo\", owner)`; `RoleTokenForOwner` calls it, reads the file it names, and returns. There is no cache, no memo and no reuse check in the file."
  - "`tools/desk/internal/deskkit/forgeresolve.go` § `GitHubCustodyMinterFunc` / `githubCustody` / `ResolveForge` — the resolver seam. Its doc states the credential is \"read AT CALL TIME, not cached\", which is a statement about the BASE-URL override reaching the Forge, not a decision that a token must be re-minted; the memo below preserves it exactly, because the memo sits under the seam and the seam still runs on every call."
  - "`tools/desk/cmd/deskboard/forge.go` § `init` / `forgeFor` — the per-repo resolution every board read goes through, installed as the custody hook. `forgeFor` is called from `board.go`, `health.go`, `stalled.go`, `zeroci.go` and `prstate.go`."
  - "`tools/desk/cmd/deskflip/flip.go` § `checkAppToken` and `tools/desk/cmd/deskflip/exec.go` § `init` — the explicit `mintTokenFn` lookup that gives the app-token condition its role-and-path refusal, followed by `ResolveForge`, whose GitHub custody calls the hook, which calls `mintTokenFn` again."
  - "`tools/desk/cmd/desktoken/desktoken.go` § the GitHub mint path — install-id resolution (env override, else `resolveMintOwner` + PEM read + JWT + `resolveInstallID`), then the cache path `<role>-token-<installID>`, then `--ttl`, then `--fresh`, then the cache `os.Stat` reuse check, then the mint. The order is the defect."
  - "`tools/desk/cmd/desktoken/desktoken.go` § `resolveInstallID` — `GET <API base>/app/installations` with the signed JWT, matching `account.login` case-insensitively, failing closed when no installation matches; and `cacheMaxAge = 50 * time.Minute` with the comment recording that installation tokens live about 60 minutes."
  - "`tools/desk/cmd/desktoken/desktoken.go` § `permsPath` / `writePerms` — the existing sidecar precedent: a 0600 file named `<tokenPath>.perms`, written beside the cache at mint time, whose failure to write is explicitly NOT fatal to the mint. The owner sidecar this brief adds is the same shape with a different payload and a stricter read."
  - "`tools/desk/internal/deskkit/confighome.go` § `ConfigHomeDirs` / `FindConfigFile` / `ConfigHomeWritePath` — the App-credential search path: read across it, write to its head. Every file this brief adds obeys it rather than inventing a second location."
  - "`tools/desk/cmd/desktoken/desktoken_test.go` § `TestCacheReuseFresh` — the existing test whose own comment reads \"GET /app/installations resolves before cache check\", and which therefore has to stand up an installations server to exercise a pure cache hit. It is the in-tree witness that the defect is real, and it is the test row 8 inverts."
  - "`tools/desk/cmd/desktoken/mutations.json` and `tools/desk/internal/deskkit/mutations.json` — the two mutation specs the changed packages already carry, which row 15 extends rather than replaces."
exec-tier: strong
exec-tier-why: >-
  (b) this is a CACHE in front of a CREDENTIAL, and the failure that matters is silent: a
  memo or a glob that returns the right-shaped token for the wrong identity does not error,
  it authenticates as somebody else. Every happy-path test passes either way — only a row
  that mints two distinct identities and asserts the second never receives the first's
  token distinguishes a correctly keyed cache from one that merely works on a single-owner
  machine. The second hazard is an availability one in the other direction: a cache-first
  order that accepts a stale install id turns a fast path into a hard failure for the whole
  TTL, so the invalidation route needs a row of its own rather than a comment.
consumers:
  - "tools/desk/internal/deskkit/roletoken.go: fixed-here — `RoleTokenForOwner` gains the memo, the age guard and the mint counter. Its signature, its refusal texts and its never-print-the-token property are unchanged."
  - "tools/desk/cmd/desktoken/: fixed-here — the GitHub mint path's resolution ORDER, the owner sidecar, the install-id cache and its invalidation. No change to what is minted, to the token's lifetime, to the 0600 custody checks, or to the path-not-value output contract."
  - "tools/desk/cmd/deskflip/: fixed-here — one comment correction and one test. The app-token condition keeps its explicit lookup (it is the only thing that carries the token PATH into the refusal message); what changes is that the resolver's repeat lookup no longer forks a second process."
  - "The other eleven binaries that install the custody hook (`deskboard`, `deskclose`, `deskdisposition`, `deskevidence`, `deskfile`, `deskpost`, `deskpr`, `deskreply`, `deskroster`, `issueboard`, `scanloop`): fixed-here BY INHERITANCE and untouched by the diff — each calls the same `RoleTokenForOwner`, so each gets the memo without a line of its own. `deskboard` and `deskflip` carry the two Verify rows that prove it; the rest are the same call path and are not separately asserted."
  - "tools/desk/internal/deskkit/forgeresolve.go: out-of-scope (read, NOT changed — the custody seam still runs on every call and still reads the base URL at call time; the memo sits BELOW it, inside `RoleTokenForOwner`, so the seam's documented property is preserved rather than re-litigated)."
  - "The GitLab custody path (`gitlabCustody`, `tools/desk/cmd/desktoken/gitlab.go`): out-of-scope (it reads a provisioned PAT file rather than minting, and reaches it without passing through `RoleTokenForOwner`'s GitHub branch; the rotate-serialisation fix landed separately in #1041 and is neither extended nor weakened here)."
  - "The shared substrate's audit-log parse cost per invocation (#1035): out-of-scope (this brief reduces the NUMBER of invocations, not the cost of one; the parse itself is a separately filed brief and neither depends on this one nor is depended on by it)."
  - "A cross-process, on-disk token memo: out-of-scope (the on-disk reuse window IS `desktoken`'s existing 50-minute cache, which this brief makes reachable without a network call; a second on-disk layer in front of it would be two caches of one fact)."
version: 1
id: b968c3d4-cec0-4dac-8c4b-fe34f65bceac
---

# Brief 25 — A token memo per (role, owner), and `desktoken` reading its cache before it resolves the install id

## Dependencies
None. The memo is a package-private addition inside `RoleTokenForOwner` and the resolution
reorder is internal to `desktoken`'s GitHub mint path; neither needs a deliverable from any
other brief in this stream, and no brief in this stream needs one from it. It is filed as a
wave-2 brief because it is a performance change to a credential path rather than an
independent feature, not because it waits on anything.

It is INDEPENDENT of the separately filed brief covering the shared substrate's per-invocation
audit-log parse (#1035): that one makes each invocation cheaper, this one makes there be fewer
of them. Either can land first, neither changes the other's Verify rows, and there is no typed
edge between them in this stream.

## Context

single-point-of-failure: **the cache-identity binding — the `.owner` sidecar that records
which App and which account a cached token file was minted for, and the (role, owner) key on
the in-process memo.** Behind it are four layers, each tripping on a different signal in a
different component: (1) the FILENAME, which already carries the installation id
(`<role>-token-<installID>`) and is not changed here, so a wrong-identity reuse needs the
sidecar to lie AND the installation id to collide, not either alone; (2) the SINGLE-MATCH
rule on the cache probe — two fresh candidates, a missing sidecar, a sidecar that does not
record exactly this App and this account, or an account name outside the accepted character
set all fall through to authoritative resolution, so the probe fails closed on absence and
ambiguity rather than on content alone; (3) the memo's LIFETIME, which is one process and at
most 45 minutes, strictly inside the minter's own 50-minute window and well inside the
token's ~60-minute life, so a bad entry cannot outlive the verb that made it and no disk
state carries it to the next one; (4) the EXCHANGE, which is the only consumer of a cached
install id and fails closed against GitHub — an installation id that no longer belongs to
this App cannot mint anything, and its 404 invalidates the cache entry rather than being
retried under the wrong installation. The layers are genuinely independent: the sidecar is a
content match on disk, the single-match rule is a count, the memo bound is a clock in
memory, and the exchange is a remote authority.

risk note — all four risk answers are `no`, and the change is nonetheless inside the
credential plane, which is why the answers are argued rather than asserted. `regulatory` and
`customer`: nothing here is visible outside the machine; no forge write changes, no message
changes, no artifact changes. `irreversible`: every new file is a cache under the
App-credential search path, deletable at any time with no loss — deleting them restores
exactly today's behaviour, one network call at a time. `sensitive-data`: no token VALUE
enters a new file, an error, a log line or an audit record; the sidecar records an App name
and an account name, both of which the existing cache FILENAME already discloses, and the
install-id cache records an integer already printed in every `desktoken` audit row's detail.
The 0600/0700 custody checks on the key, the cache and its directory are read-only
references here — a reviewer who finds one of them relaxed, or finds any token value in a
new file, flips `sensitive-data` to yes and takes the human gate.

The `risk-files-crossread` lint NOTICE fires here and is answered rather than ignored: the
declared paths sit under `tools/desk/internal/deskkit/` and `tools/desk/cmd/desktoken/`,
both security-path triggers for this repo, and all four risk answers are `no`. The answers
stand because every change is a REORDER or a MEMO of a lookup whose result is unchanged: no
custody check is removed, moved or made conditional (the 0600 token-mode check, the private
key's mode check and the directory's 0700 check all run exactly where they run today, on
exactly the same paths); no refusal becomes a fallback; and the one new remote-facing
behaviour is an invalidation that makes a failure MORE likely to be reported, not less. Rows
11, 12, 13, 14 and 15 are the mechanical evidence, and a reviewer who finds a control read
or written by any of it flips the answer.

files:
- `tools/desk/internal/deskkit/roletoken.go` (existing) — the memo: a package-level map keyed
  `(role, owner)` behind a mutex, the age guard, the mint counter and its exported reader,
  and the test-only reset.
- `tools/desk/internal/deskkit/roletoken_memo_test.go` (planned) (new) — rows 2 to 5.
- `tools/desk/cmd/desktoken/desktoken.go` (existing) — the cache-first resolution order, the
  owner sidecar (write on mint AND on reuse), the install-id cache read/write/invalidRoute,
  and the `--fresh` bypass.
- `tools/desk/cmd/desktoken/installidcache_test.go` (planned) (new) — rows 8 to 14.
- `tools/desk/cmd/desktoken/mutations.json` (existing) — the new mutation entries for the
  cache-first order and the sidecar match.
- `tools/desk/internal/deskkit/mutations.json` (existing) — the new mutation entries for the
  memo key and its age guard.
- `tools/desk/cmd/deskboard/tokenforks_test.go` (planned) (new) — row 6.
- `tools/desk/cmd/deskflip/tokenforks_test.go` (planned) (new) — row 7.
- `tools/desk/cmd/deskflip/exec.go` (existing) — a comment correction only (see facts).
- `tools/desk/README.md` (existing) — a short subsection on the two caches, where they live,
  and how to clear them.
- `changelog/<implementing branch slug>.md` (planned) (new) — the per-PR fragment this repo's
  changelog check requires.

facts (read at `e428134c`, 2026-09-14):

- **There is exactly ONE place a desk verb turns a (role, owner) pair into a token, and it
  has no memo.** `deskkit.RoleTokenForOwner` (roletoken.go) validates role and owner, calls
  `tokenMinter`, reads the file whose path the minter printed, and returns. `tokenMinter` is
  `exec.Command("desktoken", role, "--repo", owner)`. `RoleTokenForRepo` is the same function
  keyed by a slug. Nothing between them caches, and nothing above them does either.
- **The file's own doc comment already states the invariant the memo must be keyed on, and
  claims a caching behaviour that does not exist.** `RoleTokenForOwner`'s comment closes
  "So the owner is the unit, and every caller keys its cache on it." No caller keys a cache
  on it; there is no cache to key. The comment is right about the KEY and wrong about the
  state of the code, which is precisely why the memo belongs at this function rather than in
  each of the callers the comment imagines.
- **Eight non-test call sites reach it, and twelve binaries install the custody hook that
  routes through it.** Direct callers: `tools/desk/cmd/deskboard/forge.go`, `tools/desk/cmd/deskdispatch/exec.go`,
  `tools/desk/cmd/deskflip/exec.go`, `tools/desk/cmd/deskgit/deskgit.go`, `tools/desk/cmd/deskpost/github.go`,
  `tools/desk/cmd/deskroster/forge.go`, `tools/desk/cmd/issueboard/forge.go`, `tools/desk/cmd/scanloop/forge.go`, plus
  `forgeresolve.go`'s own default. Twelve `cmd/` packages install
  `deskkit.SetGitHubCustodyMinter`. One memo at the shared function therefore covers all of
  them; twelve per-binary memos would be twelve chances to key one of them wrong.
- **`deskboard`'s reads are per-repo loops over that hook.** `tools/desk/cmd/deskboard/forge.go`
  installs the custody minter and defines `forgeFor(repo)`; the board's reads call it from
  `board.go` (twelve sites), `health.go` (two), `stalled.go` (three), `zeroci.go` (three) and
  `prstate.go` (one). **Correction to the dispatch note, recorded rather than dropped**
  (worker-kit clause 7): the note and the issue comment both say "22 call sites". The measured
  figure at this commit is 22 OCCURRENCES of `forgeFor(` in non-test files, of which one is
  the definition — so 21 call sites. The conclusion the number supports (that the board
  resolves a forge per repo per read, with no shared resolution) is unchanged; the figure is
  corrected here rather than repeated.
- **The measured cost, from #1036's comment, restated as the baseline this brief moves:** 140
  `desktoken` invocations per `deskboard actions --delta` tick over 10 allowed repositories —
  14 per repository per tick — about 13.5k a day on one host, peaking at 13 a second; 99.7 %
  of them reported `reused cached`. The brief's target is the first number, and the Verify row
  that gates it is row 6, which asserts the in-process fork count rather than re-running the
  field measurement.
- **`deskflip` performs the same lookup twice per run.** `tools/desk/cmd/deskflip/exec.go`'s `init`
  installs a custody hook that calls `mintTokenFn`; `tools/desk/cmd/deskflip/flip.go`'s `checkAppToken`
  calls `mintTokenFn` directly and then calls `ResolveForge`, whose GitHub custody calls the
  hook, which calls `mintTokenFn` again. The hook's own doc comment says letting the resolver
  repeat the lookup "would mint twice per run" — and that is exactly what the code does. The
  comment describes an intent the structure never delivered.
- **`desktoken` resolves the installation id BEFORE it consults the cache, and cannot do
  otherwise today, because the cache path is derived from the id.** In order: App binding and
  prefix, PEM path resolution, App id, then install id — env `<PREFIX>_INSTALL_ID` if set,
  else owner resolution, PEM read, key parse, JWT sign, `resolveInstallID` — then
  `defaultToken := ConfigHomeWritePath("<role>-token-<installID>")`, then `--ttl`, then
  `--fresh`, then the cache `os.Stat` and its under-50-minute reuse return. Every step of the
  JWT-and-API block runs even when the answer is "the token on disk is 5 minutes old".
- **The in-tree test already documents the defect in its own comment.**
  `desktoken_test.go`'s `TestCacheReuseFresh` sets up an installations server with the comment
  "GET /app/installations resolves before cache check", and only then seeds the fresh cache
  whose reuse it is testing. A pure cache hit currently cannot be exercised without a server.
- **`resolveInstallID` costs a signed JWT plus one API call and is answering a question that
  rarely changes.** It signs with the App key, `GET`s `/app/installations`, and matches
  `account.login` case-insensitively against the owner, failing closed when nothing matches.
  An installation id changes only when the App is installed or uninstalled on that account.
- **The sidecar pattern this brief reuses already exists next to the same file.**
  `permsPath(tokenPath)` is `tokenPath + ".perms"`, written 0600 by `writePerms` immediately
  after the mint, and its write failure is deliberately NOT fatal — the token is the
  deliverable. The owner sidecar is the same shape, with one difference that matters: a
  missing or non-matching `.perms` degrades a preflight check to could-not-check, whereas a
  missing or non-matching `.owner` must REFUSE the fast path and fall through to resolution.
- **The App-credential search path is already a solved question.** `ConfigHomeDirs()` is the
  `ASSAY_CONFIG_HOME` head plus the shipped default; `FindConfigFile(name)` returns the first
  existing match and the full searched list; `ConfigHomeWritePath(name)` writes to the head.
  Both new files read across the path and write to its head, so a deployment that points the
  knob at its provisioning directory keeps key, cache, sidecar and install-id cache in one
  place — the whole point of that resolver.
- **The token's own lifetime is the ceiling every cache here sits under.** `cacheMaxAge` is 50
  minutes and its comment records that GitHub's installation tokens live about 60. No cache
  this brief adds may be allowed to serve a token past that window, which is what fixes the
  memo's own bound below 50 rather than at it.

facts — the design:

- **The memo.** A package-level `map[roleOwnerKey]memoEntry` in `roletoken.go` behind a
  `sync.Mutex`, where the key is the pair `(role, owner)` AFTER the existing trimming, and the
  entry is `{token, path string; at time.Time}`. `RoleTokenForOwner` consults it before
  calling `tokenMinter` and populates it after a successful lookup. It is per PROCESS: no file
  is written, nothing is shared between verbs, and a new invocation starts empty.
- **The memo's age guard is 45 minutes, and the number is derived rather than chosen.** An
  entry older than `roleTokenMemoMaxAge` is discarded and the minter is called again. 45 is
  strictly under `desktoken`'s own 50-minute reuse threshold, which is itself under the
  token's ~60-minute life: a memo that outlived the minter's window would hand back a token
  the minter would have replaced, which is the one way a memo could make a verb FAIL where it
  previously succeeded. The constant carries a comment naming `cacheMaxAge` as the value it
  must stay under, so the two cannot drift apart silently.
- **Failures are never memoised.** A `tokenMinter` error, an empty path, an unreadable file
  or an empty token produces the existing typed refusal and writes NOTHING to the memo. A
  transient failure must not pin a refusal for the life of a long-running verb, and a memo
  that caches errors is indistinguishable from one that caches successes until the first
  outage.
- **The mint counter.** A process counter incremented on each real `tokenMinter` call, read by
  an exported `RoleTokenMints() int`. It exists because "how many times did this process fork
  the minter" is the assertion three Verify rows need and there is otherwise no way for a test
  in another package to make it: counting calls INTO `RoleTokenForOwner` counts memo hits, and
  counting audit rows measures a different tool's behaviour. It is a plain integer under the
  same mutex, carries no identity, and never appears in output.
- **`desktoken`'s new resolution order**, replacing the block described above. The env
  override stays FIRST and authoritative — it is the documented short-circuit and nothing
  below may overtake it:
  1. `<PREFIX>_INSTALL_ID` set → use it, exactly as today.
  2. Else resolve the owner (`resolveMintOwner`, local, unchanged). If `--fresh` was given,
     skip straight to step 5: `--fresh` means "the App or its permissions may have changed",
     and a fast path that honoured a cache would make it a no-op for the rest of the window,
     which is the same defect `--fresh` was added to fix for the token cache.
  3. **Cache probe.** Across `ConfigHomeDirs()`, glob `<role>-token-*` where the suffix is all
     digits. A candidate is ACCEPTED only when its mode is 0600, its mtime age is under
     `cacheMaxAge`, and its `.owner` sidecar exists and records exactly this App name and this
     account. If exactly one candidate is accepted, take the installation id from its
     filename and continue with no network call at all. Two or more accepted candidates, or
     none, falls through.
  4. **Install-id cache.** Read `<appName>-install-<owner>` across the search path; accept
     when its mode is 0600 and its mtime age is under `installIDMaxAge` (24 h) and its content
     is all digits. Accepting gives the id without a network call; the token cache is then
     checked exactly as it is today.
  5. **Resolve.** PEM read, JWT sign, `resolveInstallID` — byte-for-byte the path that runs
     today — then write the install-id cache (0600, to the search path's head, best-effort).
- **The owner sidecar.** `<tokenPath>.owner`, 0600, one line: the App name, a single space,
  the account name. It is written at mint time beside `writePerms`, and ALSO on the cache-reuse
  path when it is absent or does not match — which is what lets an existing cache file, minted
  before this change, be adopted by the fast path on the invocation AFTER the first rather
  than never. Writing it does not touch the token file's mtime, so adopting a cache does not
  extend its life by a second.
- **Why the sidecar rather than encoding the owner in the filename.** The filename
  `<role>-token-<installID>` is read by `--ttl`, by the `--fresh` removal, by the `.perms`
  sidecar path and by operators; changing its shape would orphan every cache file on every
  machine at once and would make `--fresh` silently stop removing the file it means to remove.
  A sidecar adds a fact without moving one.
- **Account names are validated before they are ever put in a path.** Only
  `[A-Za-z0-9._-]+`, non-empty, is accepted for the install-id cache filename and for the
  sidecar match. Anything else is not sanitised, escaped or rewritten — it simply means no
  cache is read and none is written, and resolution proceeds. A credential-plane cache is not
  a place to be clever about a path.
- **A cached install id that is no longer valid self-heals in one run.** The only consumer of
  the id is the token exchange. When that exchange returns 404 AND the id came from a cache
  (either probe), `desktoken` removes the install-id cache entry and refuses with a message
  naming the removal and stating that the next run re-resolves. It does NOT silently retry
  in-process: one remote failure mapping to one refusal keeps the audit row honest about what
  happened, and the next invocation is a second of work away.
- **`deskflip`'s second lookup stops being a second FORK; the explicit lookup stays.**
  **Correction to the dispatch note, recorded rather than dropped** (worker-kit clause 7): the
  note asks for the second mint to be "removed" so that `ResolveForge` reuses the hook's
  token. Read against the source, the explicit `mintTokenFn` call in `checkAppToken` cannot be
  deleted without losing something: it is the ONLY thing that carries the token PATH into the
  app-token condition's refusal and into its OK line, and `ForgeResolution` carries no path
  field to replace it with. Adding one would change a shared resolver's public struct for one
  verb's message — a wider change than this brief's scope, and a worse one. So the double
  MINT is removed exactly where it costs: with the memo in place, the resolver's repeat
  lookup is a memo hit and forks nothing, and `deskflip` forks `desktoken` once per run. The
  stale comment in `exec.go` is corrected to describe what the code now actually does.
- **What this brief does NOT change.** No token value in any new file, error, log line or
  audit row. No change to `cacheMaxAge`, to the minted token's lifetime, or to the exchange.
  No change to the 0600 token-mode check, the private-key mode check, or the 0700 directory
  mode. No change to the path-not-value output contract or to `tokenPathNotice`. No change to
  any refusal's exit code. No change to the GitLab path.

## Ground rules
- **A cache may never serve an identity it cannot prove.** Every fast path in this brief is
  gated on a positive match of BOTH the App name and the account, and anything short of that
  — absent, ambiguous, malformed, unreadable, oddly named — falls through to authoritative
  resolution. There is no "probably the same installation".
- **The memo lives in memory and dies with the process.** No token, path, or memo entry is
  written to disk by `roletoken.go`, and no environment variable turns the memo into a
  cross-process one.
- **No custody check is moved, relaxed, or made conditional.** The mode checks on the key, the
  cache file and the cache directory run where they run today, on the same paths, on every
  path through the code including the new fast ones. Removing or weakening one is the
  security-gate refusal in the worker kit, not a shortcut.
- **The env override stays first.** `<PREFIX>_INSTALL_ID` is the authoritative short-circuit
  and no cache may overtake, override or invalidate it.
- **`--fresh` bypasses everything new.** It already bypasses the token cache; it must bypass
  the probe and the install-id cache too, or it stops meaning what it means.
- **Best-effort writes, fail-closed reads.** A failure to WRITE a sidecar or an install-id
  cache is swallowed exactly as `writePerms`' is — the token is the deliverable. A failure to
  READ one, or any doubt about what it says, is a fall-through to resolution, never an
  assumption.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Deliverables

1. **The memo — `tools/desk/internal/deskkit/roletoken.go`.**
   - `roleTokenMemoMaxAge = 45 * time.Minute`, with a doc comment naming `desktoken`'s
     `cacheMaxAge` as the value it must stay under and why.
   - A package-level memo map keyed on the trimmed `(role, owner)` pair behind a `sync.Mutex`,
     holding `{token, path, at}`.
   - `RoleTokenForOwner` consults the memo after its existing role/owner validation and before
     `tokenMinter`; on a miss it performs today's lookup unchanged and stores the result; on a
     failure of any kind it stores nothing and returns today's typed refusal unchanged.
   - `RoleTokenMints() int`, the process fork counter, and a package-private reset used by
     tests in this package.
   - No change to any signature, refusal text, or to the rule that the token VALUE never
     appears in an error, a log line or an audit record.
2. **The resolution order — `tools/desk/cmd/desktoken/desktoken.go`.** The five-step order
   above, implemented as small named helpers rather than inline in the existing block:
   `probeCachedInstallID(role, appName, owner)`, `readInstallIDCache(appName, owner)`,
   `writeInstallIDCache(appName, owner, id)`, `invalidateInstallIDCache(appName, owner)`,
   `ownerSidecarPath(tokenPath)`, `readOwnerSidecar(path)`, `writeOwnerSidecar(tokenPath,
   appName, owner)`, and `cacheOwnerName(owner) (string, bool)` for the character-set
   validation. `installIDMaxAge = 24 * time.Hour`.
3. **The sidecar writes.** At mint time beside `writePerms`; on the reuse path when the
   sidecar is absent or does not match, without touching the token file's mtime.
4. **The 404 invalidation route.** When the exchange fails 404 and the installation id came
   from either cache, remove the install-id cache entry and refuse with a message naming the
   removal and the fact that the next run re-resolves. An id from the env override or from a
   fresh resolution is NOT an invalidation case.
5. **`--fresh` bypass.** `--fresh` skips the probe and the install-id cache and resolves
   authoritatively, in addition to the cache and sidecar removal it already performs.
6. **`deskflip` — `tools/desk/cmd/deskflip/exec.go`.** The `init` doc comment corrected to state what the
   code does and why one fork per run is now the outcome. No behavioural change in this file.
7. **Tests, one file per package as named under `files:`**, covering every Verify row below,
   including the two negative controls (rows 5 and 12) and the fail-first evidence the worker
   kit requires.
8. **Mutation entries** added to the two existing specs: `tools/desk/cmd/desktoken/mutations.json` and
   `internal/deskkit/mutations.json`.
9. **Docs — `tools/desk/README.md`.** A short subsection: what the two caches are, where they
   live (the App-credential search path), how long each is good for, and that removing any of
   the files is always safe and costs one network call.
10. **Changelog fragment** under `changelog/`.
11. **Nothing else.** No cross-process memo, no change to `cacheMaxAge` or to token lifetime,
    no new environment variable, no change to the GitLab path, no relaxation of any mode check,
    and no change to `ForgeResolution`.

## Definition of done

- One process that asks for the same (role, owner) token N times forks `desktoken` once,
  and asking for a different role or a different owner forks again.
- A `deskboard` run over N repositories spanning M distinct accounts forks the minter at most
  M times, measured by `RoleTokenMints()` rather than by narration.
- `deskflip` forks the minter exactly once per run.
- With a warm token cache, a matching owner sidecar and no `<PREFIX>_INSTALL_ID`, `desktoken`
  makes ZERO network calls — asserted against a transport that fails every request.
- With a stale token cache and a warm install-id cache, `desktoken` makes exactly one call
  (the exchange) and zero installation lookups; with everything cold it makes exactly two.
- Two Apps bound to the same role on the same account never read each other's cache file, and
  a sidecar naming a different App is a fall-through, not a match.
- `--fresh` resolves authoritatively with every cache warm.
- A 404 from the exchange on a cached id removes the install-id cache entry and says so.
- Both mutation specs are GREEN at baseline, catch their positive control, and catch every new
  mutation.
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and `statusgen --root .. --lint` reports
  `LINT: PASS`.
- The brief's board row is `implemented` and the changelog fragment is present.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 — the memo, the reordered mint path and the new tests compile with no call-site churn beyond the files named under `files:` |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleTokenMemoForksOncePerOwner$' -count=1` | exit 0 — a counting `tokenMinter` stub: 20 calls for one (role, owner) pair fork exactly ONCE and every call returns the identical token and path; the same 20 calls spread over two accounts fork exactly TWICE |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleTokenMemoRemintsPastMaxAge$' -count=1` | exit 0 — both directions: an entry stamped 44 minutes old is REUSED and forks nothing, an entry stamped 46 minutes old is discarded and forks again, and the constant is asserted to be strictly less than `desktoken`'s 50-minute reuse threshold |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleTokenMemoDoesNotCacheFailures$' -count=1` | exit 0 — a stub that fails the first call and succeeds the second: the first call returns the existing typed refusal, the second FORKS AGAIN and succeeds, and the refusal text and error type are byte-identical to the pre-memo ones for all four failure shapes (minter error, empty path, unreadable file, empty token) |
| 5 | check +mutation | `cd tools/desk && go test ./internal/deskkit/ -run '^TestRoleTokenMemoNeverCrossesIdentities$' -count=1` | exit 0 — the SPOF row and a NEGATIVE control: a stub minting a distinguishable token per (role, owner) pair is driven over two roles and two accounts in an interleaved order; every one of the four combinations receives ITS OWN token and never another's. Mutation: keying the memo on role alone, or on owner alone, REDDENS this test |
| 6 | check:ci +flow | `cd tools/desk && go test ./cmd/deskboard/ -run '^TestBoardReadsForkTokenMinterOncePerOwner$' -count=1` | exit 0 — the brief's headline claim, end to end: a board read driven over N configured repositories spanning M accounts against a recorded backend, with `deskkit.RoleTokenMints()` sampled before and after, forks the minter at most M times — asserted as a number, with N > M so the row can fail |
| 7 | check:ci | `cd tools/desk && go test ./cmd/deskflip/ -run '^TestFlipForksTokenMinterOncePerRun$' -count=1` | exit 0 — one flip run over a stubbed forge: `RoleTokenMints()` advances by exactly 1, proving the explicit condition lookup and the resolver's custody lookup now share one fork; the condition's OK line still names the token path and its refusal still names the role and the path |
| 8 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestWarmCacheMakesNoNetworkCall$' -count=1` | exit 0 — #1036's first Verify assertion: a fresh 0600 token cache plus a matching `.owner` sidecar, no `<PREFIX>_INSTALL_ID`, and an `httpClient` whose transport FAILS every request — the run exits 0, prints the cached token path, audits `reused cached`, and the transport records zero requests |
| 9 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestStaleCacheMintsExactlyOnce$' -count=1` | exit 0 — #1036's second Verify assertion: a token cache stamped 51 minutes old with a warm install-id cache mints exactly ONE token, records exactly ONE request path (the exchange) and ZERO installation lookups; with every cache cold the same run records exactly TWO requests, in the order installations then exchange |
| 10 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestInstallIDCacheHitAndExpiry$' -count=1` | exit 0 — an install-id cache stamped 23 hours old is used (zero installation lookups); one stamped 25 hours old is not (exactly one lookup, and the file is rewritten with a current mtime); a file whose content is not all digits is ignored rather than parsed |
| 11 | check +mutation | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestTwoAppsOnOneOwnerNeverShareACacheFile$' -count=1` | exit 0 — #1036's third Verify assertion: two App bindings for one role on ONE account, each minted in turn — the two cache files have different names, neither run reads the other's file, and a candidate whose `.owner` sidecar names the OTHER App is rejected by the probe and falls through to resolution. Mutation: dropping the App-name half of the sidecar comparison REDDENS this test |
| 12 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestProbeFallsThroughOnAmbiguityAndMalformedInput$' -count=1` | exit 0 — the NEGATIVE control, four ways: two equally fresh accepted candidates, a candidate with no `.owner` sidecar, a candidate whose mode is not 0600, and an account name carrying a character outside `[A-Za-z0-9._-]`. Each falls through to authoritative resolution — one installation lookup, correct result — and the malformed-account case is asserted to have constructed NO path from the account name |
| 13 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestFreshBypassesProbeAndInstallIDCache$' -count=1` | exit 0 — with a fresh token cache, a matching sidecar AND a warm install-id cache, `--fresh` still performs exactly one installation lookup and one exchange, removes the old cache and its `.perms`, and writes a new sidecar |
| 14 | check:ci | `cd tools/desk && go test ./cmd/desktoken/ -run '^TestExchange404OnCachedInstallIDInvalidatesIt$' -count=1` | exit 0 — a 404 from the exchange on an id taken from the install-id cache removes that cache file, refuses with a message naming the removal, and exits non-zero; the same 404 on an id from `<PREFIX>_INSTALL_ID` removes NOTHING, proving the invalidation is scoped to cached ids |
| 15 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec cmd/desktoken/mutations.json && go run ./cmd/muhar -spec internal/deskkit/mutations.json` | exit 0 — both specs: baseline GREEN, positive control CAUGHT, and every new mutation CAUGHT — the memo keyed on role alone, the memo's age guard widened past `cacheMaxAge`, failures memoised, the sidecar's App-name comparison dropped, the probe's single-match rule relaxed to first-match, the account character-set check removed, and `--fresh` allowed to honour the probe |
| 16 | check:ci | `cd tools/desk && go test -timeout 300s ./internal/deskkit/... ./cmd/desktoken/... ./cmd/deskflip/... ./cmd/deskboard/... -count=1` | exit 0 — every existing test of the four touched packages, unchanged; in particular `TestCacheReuseFresh`, `TestCacheExpiredMintsNew`, `TestInstallIDOverride`, `TestFreshDeletesCacheAndPermsThenMints` and the per-role key-resolution tests still pass against the reordered path |
| 17 | check:ci | `cd tools/desk && gofmt -l internal/deskkit/roletoken.go cmd/desktoken cmd/deskflip cmd/deskboard > /tmp/dt25-fmt.out; test ! -s /tmp/dt25-fmt.out` | exit 0 |
| 18 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |
| 19 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers --brief assay:assay:desk-tools:25; echo $?` | 0 — the routing claims in `consumers:` are RESOLVED against this branch's own diff, not counted: every entry marked `fixed-here` is touched by the diff and every `out-of-scope` one is not. Exit 2 is COULD-NOT-CHECK (no diff to take — a fully merged tree), which is reported AS ITSELF and never as a pass |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The memo returns one identity's token to another — the credential defect that does not error | row 5 (four interleaved combinations, each asserted to get its own token) + row 15's key-narrowing mutations |
| The memo outlives the token it holds, so a long verb authenticates with an expired credential | row 3 (both directions, plus the constant asserted under the minter's own threshold) + row 15's widened-guard mutation |
| A transient mint failure is memoised and pins a refusal for the life of the process | row 4 (fail-then-succeed, all four failure shapes) + row 15's failure-memoising mutation |
| The fork count does not actually drop, because a caller bypasses `RoleTokenForOwner` | rows 6 and 7 (the counter sampled across a real board read and a real flip, asserted as a number) |
| The cache probe adopts a file minted by a DIFFERENT App on the same account | row 11 (two bindings, one account) + row 15's sidecar-comparison mutation |
| The probe picks one of several plausible candidates instead of failing closed | row 12 (two equally fresh candidates fall through) + row 15's first-match mutation |
| An account name from configuration is interpolated into a path | row 12 (the malformed-account case asserts no path is built from it) + row 15's character-set mutation |
| A cached install id goes stale and every call fails for a whole TTL with no way out | row 14 (the 404 invalidation, and its scoping to cached ids only) |
| `--fresh` becomes a no-op because a new cache short-circuits it | row 13 + row 15's `--fresh` mutation |
| The env override stops being authoritative because a cache overtakes it | rows 10 and 14 (the override path asserted to skip every cache and to invalidate nothing) + row 16 (`TestInstallIDOverride` unchanged) |
| A custody mode check is skipped on one of the new fast paths | row 12 (the not-0600 candidate falls through) + row 16 (the existing mode tests unchanged) + review |
| The reordered mint path changes behaviour for a case no new row covers | row 16 (the four packages' existing suites, named) |
| The token VALUE reaches the new sidecar, the install-id cache, an error or an audit row | review-only — a read of the two writers and the four new error strings; the existing `TestTokenNeverPrintedToOutput` covers stdout and stderr but not a file this brief adds |
| The memo makes a genuinely concurrent verb serialise on the mutex | review-only — the critical section holds no I/O; a reviewer confirms `tokenMinter` is not called with the lock held |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

| # | Exit | Key observed output |
|---|------|---------------------|
| — | — | not yet run — this brief is authored, not implemented |

## Review

Gate: model (all four risk answers no). Model-gated because both hazards are mechanically
bounded: the identity hazard by row 5's four interleaved combinations, row 11's two-Apps
case, row 12's four fall-through shapes and row 15's key- and comparison-narrowing
mutations; the availability hazard by rows 3, 4, 13 and 14, which between them pin the memo
under the token's life, keep failures out of it, keep `--fresh` authoritative and give a
stale cached id a route back. The reviewer confirms in the verdict: (1) that the memo's key
is the full (role, owner) pair and that nothing in the fast path can return a token minted
for another pair; (2) that `roleTokenMemoMaxAge` is strictly under `desktoken`'s
`cacheMaxAge`, and that the two constants reference each other so they cannot drift; (3)
that no custody or mode check is skipped, moved or made conditional on any new path, and
that the env override still runs first and is unaffected by every cache added here; and (4)
that no token VALUE is written to the owner sidecar, the install-id cache, any new error
string, or any audit detail.
