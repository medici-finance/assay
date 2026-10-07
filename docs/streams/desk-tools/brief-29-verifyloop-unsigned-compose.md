---
brief: assay:assay:desk-tools:29
title: "`verifyloop verdict --unsigned-out <file>` — compose the verdict-v1 payload with no key, so a fenced runner composes and the host signs"
why: >-
  Today `verifyloop verdict` runs a brief's Verify rows, composes the verdict-v1 payload and
  signs it in one process, and it resolves the verifier's private key before it reads the queue.
  So the process that executes arbitrary Verify-row shell commands has to be able to read the
  verifier key. A downstream fenced-runner adopter wants the rows to run in a sealed container
  (no network, no key mounted) and the signature to happen afterwards on the host that holds the
  key. A signing route for a file already exists (`deskverdict sign --payload <file>` signs any
  JSON it is handed); the composing half does not, because no mode of `verifyloop verdict`
  composes without first resolving a key. This brief adds that mode, and gives the signer the
  checks that bind what it signs to what the composer wrote and to what the host dispatched, so
  the code that runs untrusted row commands never needs the key in reach and the host signs the
  same bytes today's combined path would have signed.
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The change sits on the verifier key's custody boundary and on what a verdict signature
  attests. The Verify rows a run executes are arbitrary shell, started with `sh -c` from the repo
  root as the same user as the runner, before the payload is composed (`verdictrun.go:41-46`).
  On today's combined path a malicious or buggy row can read the key the runner already
  resolved, which allows unlimited forgeries until the key is rotated. On the split path the key
  never enters the container, but the same row shell controls the composed payload: it can plant
  or replace the output file, or leave a process behind that writes it after the composer exits,
  and `deskverdict sign` signs any JSON it is handed with no schema check (`sign.go:43-52`). That
  signing lane exists today (desk-tools/28); this brief makes it the documented route for runner
  output. So the split trades key theft for payload shaping. The brief narrows the shaping in the
  signer — the signature is bound to the digest the composer printed and to the repo, head and
  time the host dispatched — but within that dispatched scope a hostile row can still invent
  results, and no layer in this repo stops it (the single-point-of-failure line says why). The
  verifier key is the credential in question (sensitive-data: yes; the same call
  desk-containers/02 made for a key-custody design). The human confirms four things: (1) the
  trade — payload shaping bounded to the dispatch, in exchange for the key never being in reach
  of row commands; (2) the new mode never resolves, opens or reads a key at any location the
  resolver consults, and the rows that prove it are discriminating; (3) the unsigned payload is a
  file only, never a fenced issue body, so nothing downstream can mistake it for a verdict; (4)
  the host contract under Task 9 and the signer changes that enforce it.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
outcome: none
id: 5b7aa5be-b580-4522-a9f1-056b65e8913b
domain: complicated
authored: 2026-10-08 by a desk authoring session (subagent), from a downstream fenced-runner adopter's request; revised the same day after the security and correctness reviews
sources:
  - "A downstream fenced-runner adopter's request: run check/check:ci rows inside a network-none container that never holds the verifier key, write the canonical UNSIGNED verdict-v1 payload to a file, and sign on the host afterwards with `deskverdict sign --payload <file>`"
  - "docs/streams/desk-tools/brief-04-runner-verdict-batching.md: the deterministic runner this brief extends (execute rows, batch ~5 min, sign, print the would-be verdict body)"
  - "docs/streams/desk-tools/brief-28-deskverdict-role-keys-and-scan-delta-verify.md: `deskverdict sign --key verifier`, the host-side signer this brief pairs with and extends"
  - "tools/desk/cmd/verifyloop/verdictrun.go:41-46 `shellExec`: every row runs as `sh -c <command>` with `cmd.Dir = root`, same user, same environment as the runner; :99-131 `cmdVerdict` builds `verdictRunConfig` with no `exec` or `out` field set, so it always uses `shellExec` and `os.Stdout`; :136-142 `runVerdict` calls `resolveVerifierPEMPath(cfg.pem)` FIRST, before `deriveHead` (:148-151), `scanAwaiting` (:162) and any row; :216-234 `emitBatch` ALWAYS calls `composePayload` then `signPayload`, and `dryRun` changes only the trailing text; :269-284 `resolveVerifierPEMPath` returns an override or a non-empty `VERIFIER_PEM` as-is (no stat), else `deskkit.FindConfigFile(\"verifier-app.pem\")` (:276), failing closed with exit 6; :32 the package comment's invariant 'an unsigned verdict is never emitted'"
  - "tools/desk/internal/deskkit/confighome.go:46-58 `ConfigHomeDirs` (`ASSAY_CONFIG_HOME` when set, then `~/.config/assay`); :64-73 `FindConfigFile` returns the first `<dir>/<name>` whose `os.Stat` succeeds and is not a directory (:68) — so a FIFO there is found, and `~` expands through `os.UserHomeDir`, which reads `HOME` on unix"
  - "tools/desk/cmd/verifyloop/verdictpayload.go:84 `composePayload(repo, head, ts, meta, rows)`, `ts` as RFC3339 UTC; :127-148 `signPayload`: `json.Marshal`, `deskkit.CanonicalizeJSON`, `os.ReadFile(pemPath)`, `ParseRSAPrivateKeyPEM`, `SignVerdictCanonical`, `AssembleVerdictBody`"
  - "tools/desk/cmd/verifyloop/main.go:281 (the `verdict` usage line) and :305-309 (the `verdict` prose: '--dry-run composes + signs + prints without filing … A missing verifier PEM is a loud envelope error and nothing is signed')"
  - "tools/desk/cmd/deskverdict/sign.go:22-89 `cmdSign`: reads `--payload` with `os.ReadFile` through the path it is given, following links (:43), canonicalises ANY JSON with no schema check (:48), resolves the signer key (`--pem`, then the role's env override, then `<role>-app.pem`, :54-63), prints the body (:82) and, when the payload path ends `.json`, ALSO writes the sibling `<name>.out` with `os.WriteFile` (:83-86, `siblingOutPath` :144-149), which follows a link and on failure only prints a note; tools/desk/cmd/deskverdict/main.go:68 the `sign` usage line"
  - "tools/desk/cmd/deskverdict/verify.go:54 reads the body, :60-69 resolves and parses the public key and exits 6 when none is configured, and only then (:71) checks the body's structure and signature"
  - "tools/desk/internal/deskkit/verdict.go: `CanonicalizeJSON` (idempotent), `SignVerdictCanonical` (RS256 PKCS#1 v1.5 — deterministic for a given key and input), `AssembleVerdictBody` (role=verifier shorthand of `AssembleVerdictBodyForRole`); :391 the structural refusal text 'no ```verdict-payload block found in body'"
  - "statusgen/transcribeverdict.go:266-276 `verdictParseBody`: a body with no signature trailer is a structural CouldNotCheck, never trusted"
  - "statusgen/consumers.go:575-592: `--consumers --brief` reports COULD-NOT-CHECK (exit 2) when the brief file is not in the diff against the base, and prints the merge-commit form `--base $(git merge-base M^1 M^2)` for a merged brief"
  - "freshness-checked 2026-10-08 @ 36113a1dd (origin/main) — verdictrun.go, verdictpayload.go, main.go and verdictrun_test.go under tools/desk/cmd/verifyloop/, sign.go, verify.go, main.go and deskverdict_test.go under tools/desk/cmd/deskverdict/, tools/desk/internal/deskkit/verdict.go and confighome.go re-read at that commit, and the live checks under `facts:` run against binaries built from it (the revision's checks ran at 0e0aae4f1, whose code is unchanged from 36113a1dd)"
exec-tier: strong
exec-tier-why: >-
  (a) design decisions with a security consequence (what the new mode refuses, what it writes,
  what the signer now refuses, what a signature now attests) and (c) key-custody and file-path
  plumbing whose dangerous failures — the unsigned branch quietly still resolving or opening the
  key, or the signer reading or writing through a link — pass every happy-path test.
consumers:
  - "tools/desk/cmd/deskverdict/sign.go: fixed-here (gains `--expect-sha256`, `--expect-repo`, `--expect-head` and `--not-before`, reads the payload only as a regular non-link file, and creates the `.out` sibling exclusively; with none of the new flags its signed output for a regular payload file is unchanged, which the existing tests in deskverdict_test.go prove and row 8 keeps unedited)"
  - "tools/desk/cmd/deskverdict/verify.go: out-of-scope (unchanged and read-only toward this brief; row 3 (e) proves it refuses the bare payload on structure with a public key supplied, and (f) that it accepts the host-signed body)"
  - "statusgen/transcribeverdict.go: out-of-scope (never sees the unsigned payload: it reads issue bodies, and an unsigned payload is written only to a file; a body without a signature trailer is already CouldNotCheck there)"
---

# Brief 29 — `verifyloop verdict --unsigned-out <file>`: compose with no key, sign on the host

## Dependencies
None. `deskverdict sign --payload` (desk-tools/28) and the deterministic runner (desk-tools/04)
are both in the tree at the freshness stamp. This brief adds one mode to the runner and binding
checks to the signer; it changes neither the verdict primitives in `deskkit` nor the verifier.

## Context
files:
- `tools/desk/cmd/verifyloop/verdictrun.go` — the flag-parsing seam, the new flag, the unsigned branch of `runVerdict`, the amended package comment
- `tools/desk/cmd/verifyloop/verdictpayload.go` — one shared canonical-bytes helper used by BOTH `signPayload` and the unsigned writer
- `tools/desk/cmd/verifyloop/main.go` — usage line and `verdict` prose, including the host contract
- `tools/desk/cmd/verifyloop/verdictunsigned_test.go` (planned) — rows 1, 3, 4
- `tools/desk/cmd/verifyloop/verdictunsigned_unix_test.go` (planned) — row 2 (FIFO canaries, `//go:build unix`)
- `tools/desk/cmd/deskverdict/sign.go` — the binding flags, the regular-file payload read, the exclusive `.out` write
- `tools/desk/cmd/deskverdict/main.go` — the `sign` usage line
- `tools/desk/cmd/deskverdict/signbinding_test.go` (planned) — row 11
- `tools/desk/cmd/deskverdict/signpaths_unix_test.go` (planned) — row 12 (links and FIFOs, `//go:build unix`)
- `changelog/` — one fragment for the implementing PR

single-point-of-failure: two faults, two answers. (a) A key read by the new mode: the single control is the unsigned branch's code path (it never reaches `resolveVerifierPEMPath`, `signPayload` or any key location the resolver consults) — behind it, the operator's fence, which mounts no key (outside this repo), and the consumers' refusal of an unsigned body (`deskverdict verify` exits 6 on a bare payload; statusgen's transcriber treats a body with no signature trailer as CouldNotCheck). (b) A forged, malformed or substituted payload: the single control is the host's signing step, enforced in `deskverdict sign` by the digest, repo, head and not-before checks and the regular-file read (Task 8), under the host contract (Task 9) — behind it, NONE. A second layer is infeasible in this design: the row shell and the composer run as one user in one container, so nothing the composer writes or prints (the file or its digest line) is out of a hostile row's reach, and the only witness outside the fence would be a host that re-runs the rows, which is the combined path this brief exists to avoid. The signer checks bound a forgery to the repo, head and time window the host dispatched; they do not prevent it. Option 2 in the Human decision moves the composer out of the fence and is the design that would add a layer.

facts:
- Key resolution is the FIRST thing `runVerdict` does (`verdictrun.go:139`). Checked live at the stamp: with no key anywhere (empty `HOME`, `VERIFIER_PEM` and `ASSAY_CONFIG_HOME` unset) `verifyloop verdict --dry-run` exits 6 with "cannot find the verifier private key" even when the queue is empty — before the queue is read. No existing mode composes without a key.
- `--dry-run` DOES sign. Checked live: it printed the fenced canonical payload plus the `deskverdict-signature v1 alg=RS256 role=verifier` trailer and "signed verdict for 2 row(s) … — dry-run: not filed". Neither `--dry-run` nor the default mode files anything today; the only difference is the trailing text.
- With `VERIFIER_PEM` set, resolution does not stat the file; the key is OPENED only at sign time (`signPayload`'s `os.ReadFile`). Checked live: `VERIFIER_PEM` naming a nonexistent file runs every row and then exits 6 with "cannot read verifier key at …"; with an empty queue it exits 0 having opened nothing. So "VERIFIER_PEM set to a missing file" is NOT a discriminating negative for this brief — a run with no rows passes it whatever the code does. Rows 1 and 2 are built around that.
- The resolver has three sources, not one: `VERIFIER_PEM`, then `verifier-app.pem` under `$ASSAY_CONFIG_HOME`, then under `~/.config/assay` (`verdictrun.go:273-276`, `confighome.go:46-58`, `:64-73`). `FindConfigFile` needs only a non-directory (`confighome.go:68`), so a FIFO placed at either config-home location is found and opened by the signed path; row 2 puts a canary at all three.
- The Verify rows run as `sh -c` from the repo root, as the runner's own user and environment (`verdictrun.go:41-46`), before any payload is composed. Inside a fence, the rows and the composer therefore share one user and one container: a row can write the composer's output directory, leave a process running after the composer exits, and, with same-user process access, reach the composer's own output streams. This is why the host never treats anything from inside the fence as more than a claim, and why fault (b) above has no second layer.
- `deskverdict sign --payload <file>` accepts exactly the composed payload shape — and any other JSON: it checks no schema (`sign.go:43-52`). So "the key holder signed what it was handed" is not new: that lane exists today for any caller with the key. This brief documents it as the route for runner output and adds the checks under Task 8. It prints the signed body on stdout and, when the payload path ends `.json`, also writes a sibling `<name>.out`. Checked live: the fenced payload extracted from a `--dry-run` body, signed by `deskverdict sign --pem <same key>`, was BYTE-IDENTICAL (`cmp`) to the body `--dry-run` printed, and `deskverdict verify` on it exited 0. RS256 PKCS#1 v1.5 is deterministic, which is why byte identity is achievable at all.
- `deskverdict sign` follows links on both of its path lookups. Checked live at 0e0aae4f1: with `p2.out` a symlink to another file, `sign --payload p2.json` exited 0 and overwrote the link's TARGET with the signed body; with `link.json` a symlink to a payload, `sign --payload link.json` exited 0 and signed the target's bytes. Signing inside a directory the fence can write therefore lets the fence choose what the host reads and where it writes.
- `deskverdict verify` resolves the public key BEFORE it looks at the body (`verify.go:60-69`; the structure check is at `:71`). Checked live: on a bare payload with no public key configured it exits 6 with "no verifier pubkey configured" — and so would a validly signed body. Only with `--pubkey <matching key>` does a bare payload exit 6 with "no ```verdict-payload block found in body" (`tools/desk/internal/deskkit/verdict.go:391`). Row 3 (e) passes `--pubkey` and asserts that text.
- Go's `flag` help prints every flag with ONE dash: checked live, `verifyloop verdict --help` renders the existing flag as `  -pem string`. `flag.UnquoteUsage` takes the first back-quoted span in a usage string as the placeholder name, so a usage string with no back-quotes renders a string flag as `-<name> string`. Row 7 greps the single-dash form and Task 1's usage string carries no back-quotes.
- `cmdVerdict` (`verdictrun.go:99-131`) builds its config with no `exec` or `out`, so a test cannot observe rows through it. Task 2 adds a parsing seam; row 4 tests refusals there and run-time cases through `runVerdict` with an injected `exec` and `out`.
- The payload's `ts` is `cfg.nowFn()`; the CLI has no clock flag. So byte identity across two RUNS is provable only in a Go test that injects `now`, `exec`, `repo`, `head`, `session` and `runner` through `verdictRunConfig` (the existing tests already do: `TestRunVerdictDryRunEndToEnd`, helpers `writeTestKey`, `fakeExec`, `demoRoot`).
- The combined path may flush more than one payload per run (window `defaultBatchWindow` = 5m). The new mode composes ONE payload per run; equivalence is stated for a run the combined path also flushes once.
- `git rev-list` rejects `--diff-filter` (it prints its usage). `git log -1 --format=%H --first-parent --diff-filter=A HEAD -- <file>` on main returns the commit that brought `<file>` onto main's first-parent line: the squash commit, or the merge commit M itself, whose diff against M^1 is the whole implementing branch. Checked on a scratch repository with a `--no-ff` merge and a later commit. Rows 8 and 10 anchor on it, so they test the implementing change and not whatever is on main when the verifier runs.
- Precedent for a test that builds a sibling binary: `tools/desk/cmd/deskpr/deskpr_test.go` and `tools/desk/cmd/deskdispatch/verifierattestation_test.go` (`exec.Command("go", "build", "-o", …)`).
- Exit codes (`verifyloop` usage): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner. `deskkit.Refused` → 5, `deskkit.Unverifiable` → 6.

## Human decision
An automated verification tool runs each work item's check commands and then signs the results
with a private key, so a downstream reader can trust that the results came from the verifier.
Today one process does both: it loads the key before it runs anything, so the check commands —
which are arbitrary shell — run in a process that can read the key. A malicious or broken check
command can copy the key, and with the key anyone can sign any results until the key is replaced.

An adopter wants to split this in two. The check commands run inside a sealed container with no
network and no key; that container writes the results to a file, unsigned. Afterwards, on the
host that holds the key, the existing signing command signs that file. For the same results, the
signed output is byte-for-byte what today's combined process produces, so nothing downstream
changes.

What the split does not fix: the check commands still run in the same container, as the same
user, as the step that writes the results file. So the party in control of that file is the
check commands themselves. A malicious one cannot take the key any more, but it can decide what
the results file says, and the host will sign whatever the file says. The signing command
already signs any file it is handed today; this proposal makes it the documented route. The
recommended option makes the host refuse a file that does not match what the composing step
reported or what the host asked for (the same repository, the same commit, a fresh timestamp),
which stops stale or swapped files and confines a forgery to the one run the host dispatched. It
does not stop a hostile check command from inventing results for that run: nothing inside the
container can be trusted to witness them, and the only outside witness would be a host that
re-runs the checks — which is the combined process again.

So the choice is between two risks: on the combined path, a bad check command can steal the key
and forge anything, indefinitely; on the split path, it can forge only the results of the run it
is part of, and the key never leaves the host.

Options:
1. **Approve with the host-side binding (recommended).** The split mode, plus a signer that
   refuses a results file unless its fingerprint matches the one the composing step printed and
   its repository, commit and timestamp match what the host dispatched; that reads only an
   ordinary file, never a link; and that never writes its output through a link. A hostile check
   command can still invent results for the run it is part of, but cannot get a stale or
   swapped file signed and never reaches the key. Everything is in this brief.
2. **Approve the split, but keep the composing step outside the container.** A different design:
   the composing step runs on the host with no key loaded and sends each check command into the
   sealed container one at a time, so the check commands never share a process or a user with
   whatever writes the results. This is the only option that adds a second layer against forged
   results. It is a larger change; this brief would be withdrawn and a new one authored.
3. **Approve the split mode only, with no signer changes.** The host's checks are written down
   as a procedure but not enforced; the signer keeps signing any file it is handed, as it does
   today. The least protection of the approvals.
4. **Approve, but mark split-path signatures.** Record in the signed results that they were
   composed elsewhere. This changes the signed format, breaks byte identity with the combined
   path, and needs a format version bump — a separate, larger change.
5. **Reject.** Keep the combined process only; adopters who want the checks fenced from the key
   have no supported path.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Under `tools/desk/cmd/deskverdict/` change ONLY `sign.go` and the `sign` usage line in
  `main.go`. Do not change `verify.go`, `pubkey.go`, `keygen.go`, `helpers.go`,
  `tools/desk/internal/deskkit/verdict.go`, or any existing line of
  `tools/desk/cmd/verifyloop/verdictrun_test.go` or `tools/desk/cmd/deskverdict/deskverdict_test.go`
  (row 8 checks this). The existing signed path and the existing signer tests are the baseline
  the new behaviour is proven against.
- No key material is committed: tests generate keys with `writeTestKey` (verifyloop) or the
  package's existing key helper (deskverdict) into `t.TempDir()`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Flag.** In `cmdVerdict` add `--unsigned-out <file>` with this usage string, which carries
   no back-quoted span (so Go's flag help renders `-unsigned-out string`): "write the canonical
   verdict-v1 payload to this new file UNSIGNED and print its sha256; resolves and reads no key;
   sign it on the key-holding host with deskverdict sign --expect-sha256". The name says the
   custody fact (`unsigned`) and the shape (`-out` a file), and pairs with the signer's
   `--payload`. Carry it in `verdictRunConfig` as `unsignedOut string`.
2. **Parsing seam and refusals.** Move flag parsing into
   `parseVerdictFlags(args []string) (verdictRunConfig, error)`; `cmdVerdict` becomes parse,
   then `runVerdict`. It returns `deskkit.Refused` (exit 5) for:
   - `--unsigned-out` with `--pem` (a key path in an unsigned run is a caller error, never
     silently ignored);
   - `--unsigned-out` with `--dry-run` or with an explicitly passed `--window` (detect explicit
     flags with `fs.Visit`) — neither has a meaning in this mode, and a silent ignore would hide
     a caller who expected batching.

   Add `writeHook func(f *os.File, b []byte) error` to `verdictRunConfig` (nil means `f.Write`),
   so a test can fail the write after the file is created. Production never sets it.
3. **The unsigned branch of `runVerdict`.** When `cfg.unsignedOut != ""`, branch BEFORE the
   `resolveVerifierPEMPath` call, into its own function (for example `runVerdictUnsigned`) so the
   separation is visible in review. Its first step: if `os.Lstat(cfg.unsignedOut)` succeeds,
   refuse with exit 5 naming the path, before the queue is read or any row runs, and leave the
   existing entry untouched. Then derive repo/head/session/runner exactly as the signed path does
   (before any row runs), `scanAwaiting`, run every item's rows with `runBriefRows`, and compose
   ONE payload with `composePayload(repo, head, cfg.nowFn(), meta, rows)` over all rows (the
   batching window does not apply: cadence is the host's). The branch must not call
   `resolveVerifierPEMPath`, `signPayload`, `deskkit.FindConfigFile`, or read `VERIFIER_PEM` /
   `ASSAY_CONFIG_HOME`.
4. **One canonical form.** Add `canonicalPayloadBytes(p verdictPayload) ([]byte, error)`
   (`json.Marshal` then `deskkit.CanonicalizeJSON`) in `verdictpayload.go`, and make
   `signPayload` call it instead of its own two lines. The unsigned writer uses the same helper,
   so the bytes written are the bytes the combined path signs.
5. **Write, digest and cleanup.** The bytes to write are the canonical bytes plus one `"\n"`.
   Compute their SHA-256 BEFORE opening the file. Open the path with
   `os.O_WRONLY|os.O_CREATE|os.O_EXCL`, mode `0644`; write through `writeHook`; close and check
   the close error.
   - An `O_EXCL` failure (something created the path during the run) is exit 5 naming the path.
     The composer wrote nothing, so the existing entry is left untouched, and no digest is
     printed.
   - A write or close failure after a successful create is exit 6. The composer removes the
     file it created — after confirming with `os.SameFile` that the path still names the file it
     opened; if it does not, it removes nothing and names the path in the error — and prints no
     digest.
   - So on every non-zero exit the composer leaves no file of its own, and stdout carries no
     digest. A file that exists after a non-zero exit was not written by this run.
   `O_EXCL` proves only that the path did not exist at the open; the binding between the file
   and this run is the digest (Task 6), checked by the signer (Task 8).
6. **Stdout.** On success, exactly one line: `unsigned verdict payload for N row(s) across M
   brief(s) written to <file> sha256=<64 lowercase hex> — NOT signed; sign on the key-holding
   host: deskverdict sign --payload <file> --expect-sha256 <hex> --expect-repo <repo>
   --expect-head <head> --not-before <dispatch time>`. The digest is the SHA-256 of the exact
   bytes written (canonical bytes plus `"\n"`). Stdout never carries a ```` ```verdict-payload
   ```` fence or a `deskverdict-signature` trailer in this mode, so nothing that scrapes stdout
   for a verdict body can pick up an unsigned one.
7. **Empty queue.** No runner-executed rows: write NO file, print `verdict runner: no
   runner-executed (check/check:ci) rows in the Awaiting queue — nothing to compose`, print no
   digest, exit 0. The host's signal is the digest line from an exit-0 run (Task 9), never
   whether a file is present: with no digest there is nothing to sign, whatever the output
   directory holds.
8. **Signer binding (`deskverdict sign`).** In `sign.go`:
   - **Payload read.** `os.Lstat` the `--payload` path; refuse (exit 5) unless it is a regular
     file (`Mode().IsRegular()` — rejects a link, FIFO, device or directory without opening it).
     Open it read-only, `Stat` the open file, and refuse (exit 5) unless `os.SameFile` matches
     the `Lstat` result (the entry was swapped in between). Read the bytes ONCE from that open
     file; the digest, the field checks and the signature all use those same bytes.
   - **New flags**, each optional, each checked BEFORE the signer key is resolved, each a
     refusal (exit 5) with nothing on stdout when it fails:
     `--expect-sha256 <hex>` — the SHA-256 of the bytes read must equal it;
     `--expect-repo <owner/name>` and `--expect-head <sha>` — the payload must be a JSON object
     whose top-level `repo` / `head` string equals it;
     `--not-before <RFC3339>` — the payload's top-level `ts` must parse as RFC3339 and must not
     be earlier.
     With none of these flags, sign behaves as today for a regular payload file.
   - **`.out` sibling.** Create it with `os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL,
     0o644)` in place of `os.WriteFile`. `O_CREATE` with `O_EXCL` fails on ANY existing entry,
     including a link (dangling or not), and never writes through one. If the create fails, exit
     5 naming the path and saying the signed body is on stdout only; the existing entry is left
     untouched.
   - Add the four flags to the `sign` usage line at `tools/desk/cmd/deskverdict/main.go:68`.
9. **Host contract.** State it in the `verdict` prose in `tools/desk/cmd/verifyloop/main.go` and in the
   `--unsigned-out` changelog entry; the prose carries the literal phrases `the fence has fully exited`
   and `--expect-head`, which row 7 greps. The host:
   - **H1** runs the composer with explicit `--repo` and `--sha` and records the dispatch time;
   - **H2** signs only after the composer exited 0 AND printed the `sha256=` line, and only once
     the fence has fully exited with no row process left alive; any other exit, or no digest
     line, means do not sign — whatever file is present;
   - **H3** signs with `deskverdict sign --payload <file> --expect-sha256 <the printed digest>
     --expect-repo <repo> --expect-head <sha> --not-before <dispatch time>`, and refuses to
     continue on any non-zero exit;
   - **H4** takes the signed body from sign's stdout, never from a `.out` file inside a
     directory the fence could write.
10. **Docs.** Amend the package comment's invariant at `verdictrun.go:32` to "an unsigned verdict
    BODY is never emitted; an unsigned PAYLOAD is written only to an explicit `--unsigned-out`
    file". Add `[--unsigned-out <file>]` to the `verdict` usage line in `main.go`, and to the
    `verdict` prose one sentence that the mode reads no key, plus the host contract (Task 9).
11. **Tests** — the planned files under `files:`, one per Verify row below, anchored names as
    written there. Row 2's FIFO canaries and row 12's link and FIFO cases live in the
    `//go:build unix` files.
12. **This brief.** In this brief's `files:` list, drop the ` (planned)` marker from each path
    the implementing change creates. This also puts the brief in the implementing diff, which
    row 10 needs.
13. **Changelog fragment** under `changelog/` (`### Added`): the new flag, that it reads no key,
    the four signer flags and the host contract; under `### Changed`: `deskverdict sign` now
    refuses a payload that is not a regular file and refuses to replace an existing `.out`.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutNeverResolvesVerifierKey$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r1.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutNeverResolvesVerifierKey' "${TMPDIR:-/tmp}/b29-r1.out"` | exit 0, `--- PASS:` printed. **Negative path**: `HOME` is an empty temp dir, `VERIFIER_PEM` and `ASSAY_CONFIG_HOME` are set empty, `demoRoot` holds runner rows. Arm A: `runVerdict` with `unsignedOut` returns nil, the file exists and parses as a `verdictPayload` with every executed row. CONTROL arm in the same test, same environment: the signed path (`dryRun: true`) returns an error whose `deskkit` exit code is 6 and whose text contains `cannot find the verifier private key` — so the environment really has no key, and arm A's success is not vacuous. Arm B: `VERIFIER_PEM` names a REAL valid key from `writeTestKey`; the unsigned run's stdout and file contain neither ```` ```verdict-payload ```` nor `deskverdict-signature`. Mutation M1 turns arm A red |
| 2 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutNeverOpensVerifierPEMCanary$' -count=1 -timeout 120s -v > "${TMPDIR:-/tmp}/b29-r2.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutNeverOpensVerifierPEMCanary' "${TMPDIR:-/tmp}/b29-r2.out"` | exit 0, `--- PASS:` printed. **Negative path, discriminating, at every location the resolver consults.** `HOME` is pinned to a temp dir H and `ASSAY_CONFIG_HOME` to a temp dir C. Three FIFOs, each made with `syscall.Mkfifo`: canary A at a temp path named by `VERIFIER_PEM`, canary B at `C/verifier-app.pem`, canary C at `H/.config/assay/verifier-app.pem`. One detector goroutine per FIFO repeatedly opens it write-only and non-blocking (`os.O_WRONLY` combined with `syscall.O_NONBLOCK`); on a FIFO that open succeeds only while some reader has it open, and fails with `ENXIO` otherwise. Unsigned arm U1 (`VERIFIER_PEM` = A, `ASSAY_CONFIG_HOME` = C, all three canaries live): no detector ever succeeds, and the payload file is written. Unsigned arm U2 (`VERIFIER_PEM` and `ASSAY_CONFIG_HOME` set empty, canary C live): its detector never succeeds. CONTROL arms on the signed path (`dryRun: true`, `demoRoot` rows), one per location: K1 with `VERIFIER_PEM` = A fires A; K2 with `VERIFIER_PEM` empty and `ASSAY_CONFIG_HOME` = C fires B; K3 with both empty fires C. Each fired detector closes, the reader gets EOF and the run returns an error. A test whose control arm never fires fails — so no canary can pass on a broken detector. Mutation M2 turns U1 red (A); mutation M5 turns U1 red (B) and U2 red (C). This row covers the three sources the resolver consults; a read of a path the resolver never consults is outside it |
| 3 | check:ci +flow +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedComposeSignedByDeskverdictMatchesCombined$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r3.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedComposeSignedByDeskverdictMatchesCombined' "${TMPDIR:-/tmp}/b29-r3.out"` | exit 0, `--- PASS:` printed. Fixed `now`, `repo`, `head`, `session`, `runner`, `fakeExec`, one `demoRoot` (one flush). (a) Combined path, `pem` = a `writeTestKey` key, `dryRun: true`: capture the printed body up to and including the signature trailer line. (b) Unsigned path, same inputs, writes `payload.json` in `t.TempDir()` and captures stdout. (c) The file's bytes equal the fenced payload text inside (a)'s body plus `"\n"` — the file IS the combined path's canonical form — and the `sha256=` value on (b)'s stdout equals the hex SHA-256 of the file's bytes. (d) The test builds `deskverdict` from `../deskverdict` into `t.TempDir()` and runs `sign --payload <file> --pem <same key> --expect-sha256 <the printed digest> --expect-repo <repo> --expect-head <head> --not-before <now minus one minute>`: exit 0, and its stdout is BYTE-IDENTICAL to (a)'s body (`bytes.Equal`, diff printed on failure). (e) `deskverdict verify --body <file> --pubkey <the matching public key>` (the unsigned file) exits 6 AND its stderr contains `verdict-payload block found` — the 6 is the structural refusal, not a missing key (without `--pubkey` a signed body also exits 6). (f) `deskverdict verify --pubkey <the matching public key>` on the (d) output exits 0. Mutation M3 turns (c) red; mutation M8 turns (d) red |
| 4 | check:ci +mutation | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestUnsignedOutRefusals$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r4.out" 2>&1 && grep -F -e '--- PASS: TestUnsignedOutRefusals' "${TMPDIR:-/tmp}/b29-r4.out"` | exit 0, `--- PASS:` printed. Parse cases through `parseVerdictFlags`: `--unsigned-out f --pem k`, `--unsigned-out f --dry-run` and `--unsigned-out f --window 1m` each return an error whose `deskkit` exit code is 5. Run cases through `runVerdict` with an injected `exec` that records calls and an injected `out`: (i) an `--unsigned-out` path that already exists returns 5 with ZERO rows executed and the existing bytes unchanged; (ii) an `exec` that CREATES the output path with planted bytes while a row runs gives 5, and the planted bytes are unchanged afterwards; (iii) a `writeHook` that fails after the create gives a non-zero exit and the path does not exist afterwards; (iv) an empty queue returns nil, prints `nothing to compose`, and the path does not exist; (v) a normal run prints exactly one line containing `sha256=`. In cases (i) to (iv) `out` contains no `sha256=`. Mutation M4 turns (ii) red; mutation M9 turns (iii) red |
| 5 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRunVerdictMissingPEMFailsClosedAndFilesNothing$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5a.out" 2>&1 && grep -F -e '--- PASS: TestRunVerdictMissingPEMFailsClosedAndFilesNothing' "${TMPDIR:-/tmp}/b29-r5a.out" && go test ./cmd/verifyloop/ -run '^TestDryRunSignVerify$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5b.out" 2>&1 && grep -F -e '--- PASS: TestDryRunSignVerify' "${TMPDIR:-/tmp}/b29-r5b.out" && go test ./cmd/verifyloop/ -run '^TestRunVerdictDryRunEndToEnd$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5c.out" 2>&1 && grep -F -e '--- PASS: TestRunVerdictDryRunEndToEnd' "${TMPDIR:-/tmp}/b29-r5c.out" && go test ./cmd/verifyloop/ -run '^TestSignedBodyVerifiesAndTamperRefuses$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r5d.out" 2>&1 && grep -F -e '--- PASS: TestSignedBodyVerifiesAndTamperRefuses' "${TMPDIR:-/tmp}/b29-r5d.out"` | exit 0, four `--- PASS:` lines. The existing signed path is unchanged: a missing key still fails closed with exit 6 and files nothing, `--dry-run` still signs and verifies, and a tampered body is still refused. Row 8 proves these tests were not edited to pass |
| 6 | check:ci | `cd tools/desk && test -z "$(gofmt -l cmd/verifyloop/ cmd/deskverdict/)" && go vet ./cmd/verifyloop/ ./cmd/deskverdict/ && go test ./cmd/verifyloop/ ./cmd/deskverdict/ -count=1` | exit 0 — gofmt-clean, vet-clean, and both whole packages (new and existing tests, including every existing signer test) pass |
| 7 | check | `cd tools/desk && go run ./cmd/verifyloop verdict --help > "${TMPDIR:-/tmp}/b29-r7v.out" 2>&1; go run ./cmd/deskverdict sign --help > "${TMPDIR:-/tmp}/b29-r7s.out" 2>&1; grep -F -e '-unsigned-out string' "${TMPDIR:-/tmp}/b29-r7v.out" && grep -F -e '-expect-sha256 string' "${TMPDIR:-/tmp}/b29-r7s.out" && grep -F -e '-not-before string' "${TMPDIR:-/tmp}/b29-r7s.out" && grep -F -e 'unsigned PAYLOAD is written only to an explicit' cmd/verifyloop/verdictrun.go && grep -F -e '--unsigned-out' cmd/verifyloop/main.go && grep -F -e 'the fence has fully exited' cmd/verifyloop/main.go && grep -F -e '--expect-head' cmd/verifyloop/main.go && grep -F -e '--expect-sha256' cmd/deskverdict/main.go` | exit 0. Go's flag help prints one dash, so the help greps use the single-dash form; `-unsigned-out string` also proves the usage string carries no back-quoted span (one would replace `string` with the quoted text). The usage lines in both `main.go` files name the new flags, and the package comment's invariant is amended. The `verdict` prose states the host contract (Task 9), so dropping it reddens this row; the contract's enforceable half is pinned by rows 11 and 12. The help commands' own exit codes are not the assertion; the greps are |
| 8 | check | `a="$(git log -1 --format=%H --first-parent --diff-filter=A HEAD -- tools/desk/cmd/verifyloop/verdictunsigned_test.go)" && test -n "$a" && git diff --quiet "$a^1" "$a" -- tools/desk/internal/deskkit/verdict.go tools/desk/cmd/deskverdict/verify.go tools/desk/cmd/deskverdict/pubkey.go tools/desk/cmd/deskverdict/keygen.go tools/desk/cmd/deskverdict/helpers.go && git diff --output-indicator-old='<' "$a^1" "$a" -- tools/desk/cmd/verifyloop/verdictrun_test.go tools/desk/cmd/deskverdict/deskverdict_test.go > "${TMPDIR:-/tmp}/b29-r8.diff" && ! grep -q -e '^<' "${TMPDIR:-/tmp}/b29-r8.diff" && test -z "$(git ls-files '*.pem' '*.key')" && echo ok` | prints `ok`. Runs on merged main, at any later commit, with full history (not a shallow clone). `a` is the first-parent commit that brought `verdictunsigned_test.go` onto main — the squash commit, or the merge commit whose diff against its first parent is the whole implementing branch — so the diff is exactly the implementing change, whatever landed after it; an empty `a` fails `test -n`. The verdict primitives, the verifier and the untouched signer files are unchanged, every existing line of both existing test files is unchanged (additions are allowed; removed or edited lines are marked `<`, so the diff's own `---` header cannot match), and no key material is tracked |
| 9 | check:ci | `cd statusgen && go run . --root .. --lint` | exit 0, `LINT: PASS` |
| 10 | check:ci | `cd statusgen && go build -o "${TMPDIR:-/tmp}/b29-statusgen" . && cd .. && a="$(git log -1 --format=%H --first-parent --diff-filter=A HEAD -- tools/desk/cmd/verifyloop/verdictunsigned_test.go)" && test -n "$a" && t="$(mktemp -d)" && git clone -q --shared --no-checkout . "$t/c" && git -C "$t/c" checkout -q --detach "$a" && "${TMPDIR:-/tmp}/b29-statusgen" --root "$t/c" --consumers --brief desk-tools/29 --base "$a^1" > "${TMPDIR:-/tmp}/b29-r10.out"; rc=$?; rm -rf "$t"; test "$rc" = 0 && grep -q -e '^summary: [1-9][0-9]* corroborated, 0 disproved,' "${TMPDIR:-/tmp}/b29-r10.out" && echo ok` | prints `ok`. Runs on merged main with full history; `a` is the implementing commit as in row 8, and the gate runs on a throwaway shared clone checked out at `a` against `a^1`, so it judges exactly the implementing diff (Task 12 puts this brief in it). The `sign.go` entry is `fixed-here` and corroborates; the other two are `out-of-scope`, which the gate reports as UNCHECKED, not passed — their truth is the reviewer's call, backed by row 8 (neither file changed) and row 3 (e) and (f) |
| 11 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskverdict/ -run '^TestSignBindingRefusesMismatchBeforeKey$' -count=1 -v > "${TMPDIR:-/tmp}/b29-r11.out" 2>&1 && grep -F -e '--- PASS: TestSignBindingRefusesMismatchBeforeKey' "${TMPDIR:-/tmp}/b29-r11.out"` | exit 0, `--- PASS:` printed. A valid verdict-v1 payload file with known `repo`, `head` and `ts`. Refusal cases run with `--pem` naming a path that does NOT exist, so reaching key resolution would give 6: a wrong `--expect-sha256`, a wrong `--expect-repo`, a wrong `--expect-head`, a `--not-before` later than the payload's `ts`, and `--expect-repo` on a payload that is a JSON array each return 5 with empty stdout — the refusal comes before any key lookup. The matching case (all four flags correct, a real key) returns 0 and its stdout verifies with the matching public key; the same payload with none of the new flags also returns 0 with the identical body. Mutation M6 turns the wrong-digest case red; mutation M7 turns the wrong-head case red |
| 12 | check:ci +mutation | `cd tools/desk && go test ./cmd/deskverdict/ -run '^TestSignPathHandlingNeverFollowsLinks$' -count=1 -timeout 60s -v > "${TMPDIR:-/tmp}/b29-r12.out" 2>&1 && grep -F -e '--- PASS: TestSignPathHandlingNeverFollowsLinks' "${TMPDIR:-/tmp}/b29-r12.out"` | exit 0, `--- PASS:` printed. With a real key in every case: (i) `--payload` a symlink to a valid payload returns 5 with empty stdout; (ii) `--payload` a FIFO returns 5 without blocking; (iii) `<name>.out` pre-created as a symlink to a victim file returns 5, the victim's bytes are unchanged and the link is still a link; (iv) `<name>.out` pre-created as a regular file returns 5 and its bytes are unchanged; (v) a fresh directory returns 0 and `<name>.out` is a new regular file holding exactly the stdout body. Mutation M10 turns (iii) red; mutation M11 turns (i) red |

### Named mutations
Each was chosen so the row it names is the ONLY thing standing between it and a merge.
- **M1** — at the top of the unsigned branch, add `if _, err := resolveVerifierPEMPath(""); err != nil { return err }`. Row 1 arm A reddens (exit 6, key not found).
- **M2** — in the unsigned branch, add `if p := os.Getenv("VERIFIER_PEM"); p != "" { _, _ = os.ReadFile(p) }`. Row 1 stays green (arm A has `VERIFIER_PEM` empty; arm B's read succeeds silently); row 2's canary A fires in arm U1 and it reddens. This is why row 2 exists beside row 1.
- **M3** — in the unsigned writer, replace `canonicalPayloadBytes(p)` with `json.MarshalIndent(p, "", "  ")`. Row 3 (d) stays green — the printed digest follows the bytes written and `deskverdict sign` re-canonicalises — so only (c) catches a non-canonical file. This is why (c) is asserted separately.
- **M4** — drop `os.O_EXCL` from the composer's open flags. Row 4 case (ii) reddens (the planted bytes are overwritten).
- **M5** — in the unsigned branch, add `if p, _, ok := deskkit.FindConfigFile("verifier-app.pem"); ok { _, _ = os.ReadFile(p) }`. Row 1 stays green (arm A has an empty `HOME` and no config home, so nothing is found; arm B likewise); row 2's canary B fires in U1 and canary C in U2. This is why row 2 places a canary at each config-home location and pins `HOME` and `ASSAY_CONFIG_HOME`.
- **M6** — in `sign.go`, drop the `--expect-sha256` comparison. Row 11's wrong-digest case reddens (it reaches key resolution and exits 6, not 5).
- **M7** — in `sign.go`, drop the `--expect-head` comparison. Row 11's wrong-head case reddens.
- **M8** — compute the printed digest over the canonical bytes WITHOUT the trailing `"\n"`. Row 3 (c) reddens on the digest comparison, and (d) reddens because the signer's `--expect-sha256` refuses.
- **M9** — drop the composer's remove-on-failure step. Row 4 case (iii) reddens (the partial file is left behind).
- **M10** — write the `.out` sibling with `os.WriteFile` again. Row 12 case (iii) reddens (the victim file is overwritten through the link).
- **M11** — read the payload with plain `os.ReadFile(*payloadPath)`. Row 12 case (i) reddens (the link's target is signed).

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter; sensitive-data: yes). The human decision above is ratified
before dispatch; the option chosen is recorded on the brief's decision issue. If it is option
3 or 4, the brief is revised before it moves to in-progress; if it is option 2, this brief is
withdrawn and the new design authored in its place.

A reviewer answers both questions in the verdict:
1. What is the single control between the new mode and a key read, and is it acceptable? And
   what is the single control between a forged or substituted payload and a signature?
   (Expected: for the key read, the unsigned branch's code path; behind it, the operator's fence
   holds no key — outside this repo, so this brief cannot prove it — and an unsigned payload is
   refused by every consumer that reads a verdict body. Acceptable because the mode is opt-in, the
   fence is the point of the request, and rows 1 and 2 test the code path from two independent
   angles. For a forged payload, the host's signing step, enforced by Task 8's checks; behind it
   NONE, for the reason the single-point-of-failure line gives. Acceptable only as far as the
   human accepts the trade in the Human decision.)
2. Does any Verify row prove a lower layer catches the fault with the upper layer bypassed?
   (Expected: row 2 catches an open of the key at every location the resolver consults —
   `VERIFIER_PEM`, `$ASSAY_CONFIG_HOME` and `~/.config/assay` — not only through
   `resolveVerifierPEMPath`, so it holds when the structural separation row 1 tests is bypassed
   (M2, M5); it does not cover a path the resolver never consults. Row 3 (e) proves, with a
   public key supplied, that the consumer layer refuses the unsigned file on structure,
   independent of the fence. Rows 11 and 12 prove the signer refuses a mismatched, linked or
   non-regular payload whatever the composer did; nothing proves a lower layer for a forgery
   inside the dispatched scope, because none exists.)

The reviewer also confirms from the diff that `signPayload` now calls `canonicalPayloadBytes`
and nothing else changed in the signed path's behaviour, that no stdout line in the unsigned
mode can contain a payload fence, and that `deskverdict sign` with none of the new flags signs a
regular payload file exactly as before.
