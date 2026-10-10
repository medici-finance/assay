# Threat model — medici-finance/assay

Guidance for the Anthropic OSS Scanner. Read before scanning; it tells you
where the attack surface is, what a finding is worth, and what to skip.

## What this project does, and where untrusted input enters

Assay is a methodology and tooling bundle for running AI-agent "desks"
against one or more git forges (GitHub today, GitLab support in tree). The
shipped artifacts are:

- **`tools/desk/` (Go, ~60 commands under `cmd/`)** — the desk-tools suite.
  Agents and CI loops shell out to these binaries for every interaction with
  a forge: reading boards, posting comments, opening PRs, minting GitHub App
  tokens, and enforcing guardrails on what an agent may write or push.
- **`statusgen/`, `tools/*` (Go)** — board generator/linters and small
  single-purpose tools that parse markdown stream docs and YAML/JSON config.
- **`plugins/` (markdown + JSON)** — Claude Code plugin bundle: skills,
  agents, and hooks. Hooks execute `tools/desk` binaries on tool-call events.
- **`tools/cellctl`** — container orchestration for running agent cells
  (images, mounts, network policy).

Untrusted input enters primarily as **forge content**: issue titles/bodies,
PR titles/bodies/diffs, review comments, branch names, tag names, and file
paths fetched over the GitHub/GitLab API and then processed by desk-tools —
parsed, templated into prompts, passed to `git`/`gh` subprocesses, matched
against guard policies, or written into stream documents. Treat every string
sourced from a forge API response as attacker-controlled: anyone can open an
issue or PR on a public repo, and the whole point of the tooling is to
ingest exactly that. Secondary untrusted surfaces: YAML/JSON config files,
markdown frontmatter in stream docs, and CLI arguments supplied by an agent
session (a prompt-injected agent is within the threat model — that is why
the guard binaries exist).

## Components that matter most

1. **The guard binaries** — `writeguard`, `deskpushguard`, `deskpathguard`,
   `desksourceguard`, `repohardenguard`, `clusterguard`, `claimguard`. These
   are the enforcement layer standing between an autonomous (possibly
   prompt-injected) agent and the repository/forge. **A bypass of any guard
   — writing outside the allowed tree, pushing to a protected ref, evading a
   path or command check — is the single most valuable finding class here.**
   Pay attention to TOCTOU between policy check and action, symlink/hardlink
   tricks, path canonicalization (`..`, case-folding, unicode, alternate
   spellings of the same file), and argument-injection through filenames or
   branch names that begin with `-`.
2. **Credential and token handling** — `desktoken`, `deskapps`, and the
   `internal/deskkit` code that reads GitHub App private keys (PEM), mints
   JWTs and installation tokens, and passes them to subprocesses or writes
   them to disk. Findings: key/token material in logs, stdout, temp files,
   world-readable paths, error messages, or process lists; tokens scoped
   wider than requested; cache poisoning between roles.
3. **Command execution from forge data** — anywhere an issue/PR/comment
   string, branch name, or file path reaches `exec`, `sh -c`, `git`, or `gh`
   without strict quoting/allowlisting. This is the classic injection class
   for this codebase and should be hunted hard.
4. **`cellctl` container orchestration** — mount scope escapes, unintended
   host path exposure, network-policy gaps, or privilege escalation between
   the orchestrator and the cell workload.
5. **`deskcomms` / `commsgw` message signing** — signature verification
   bypasses, replay, key-confusion.
6. **Parsers** — markdown/frontmatter/YAML parsers across `statusgen` and
   `tools/*`: panics, unbounded resource use, or correctness bugs that flip
   a policy decision (e.g. a gate reading "pass" from attacker-shaped
   input). In scope, but lower than the above unless they subvert a guard or
   gate.

## How to exercise it

- Every Go module builds with `go build ./...` and tests with `go test ./...`
  from its own directory (no workspace file; each `go.mod` is independent).
- `tools/desk` is the large one: `cd tools/desk && go test ./...` runs the
  guard and forge-transport unit tests, including adversarial-path cases —
  extend those patterns.
- `tools/untrustcorpus` and `deskscanuntrusted` carry a corpus of hostile
  input samples used to test the trust gate; it is the best seed material
  for new injection attempts.
- `tools/skillslint`, `tools/winparity`, and the `*_test.sh` suites under
  `tools/` exercise the shell layer with `bash -n` and golden-output tests.
- Note on test baselines: parts of the `tools/desk` suite expect a desk
  operator environment (a roster config naming the forge, a git identity,
  credential files). Failures whose message is about resolving the forge,
  reading the roster, or a missing credential are environmental, not
  regressions — the tools fail closed by design when unconfigured. Judge a
  change by whether it introduces a *new* failure mode, not by the absolute
  count.

## Severity rubric

- **Critical:** remote (i.e. via forge content on a public repo, no local
  foothold) bypass of a guard binary leading to unauthorized write/push;
  exfiltration or disclosure of a GitHub App private key or installation
  token; command injection from forge content executed with the operator's
  credentials; container escape in `cellctl`.
- **High:** guard bypass requiring a local foothold or a cooperative victim
  config; token scope widening; signature-verification bypass in comms;
  injection requiring a less-common code path.
- **Medium:** path traversal confined to the workspace; a parser correctness
  bug that flips a gate decision without code execution; DoS of a single
  tool run via forge content.
- **Low:** DoS requiring operator action; hardening gaps without a
  demonstrated exploit path.

## Please leave alone

- **Fixture secrets.** Testdata, `fixtures/`, `untrustcorpus` samples, and
  `*_test.go` files contain deliberate dummy keys, tokens, and "leaked"
  strings used to test the scanners. They are not real credentials — do not
  report them.
- **Documentation/methodology prose** (`spec/`, `docs/`, `plugins/`
  skill text). Prompt-injection observations about markdown wording are out
  of scope unless they demonstrate a concrete bypass of a guard binary or
  hook.
- **Shell-script quoting issues on paths the operator already controls**
  (their own checkout layout). Word-splitting on forge-derived strings *is*
  in scope — the distinction is who controls the string.
- **Vulnerabilities in upstream platforms** (GitHub, GitLab, Claude Code
  itself). Report those to the vendor.
- **Go stdlib/toolchain advisories** not yet exploitable through any code
  path in this repo.
