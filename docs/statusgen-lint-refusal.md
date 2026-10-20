# Configuration refusal at the lint boundary

`statusgen --lint` must not report a clean source evaluation after the process's
configuration loader has refused its input. The shared failure shape is any
nonempty `scanConfig.Problems`, whether produced by unknown-key validation,
schema validation, value validation, or config-file access/ownership checks.

The entrypoint now stops this class at one boundary, immediately after the
existing effective-config echo and before selecting any lint evaluator. It
prints the loader's existing refusal diagnostics plus
`statusgen: configuration REFUSED; trust-dependent checks could-not-check` on
stderr, and exactly
`LINT: REFUSED configuration; trust-dependent checks could-not-check` on stdout,
then exits **5**. It emits no source `PROBLEM:` line: the source tree was not
judged. A valid source evaluation remains exit 0/PASS or exit 1/FAIL; usage errors
remain exit 2. `go run` can wrap the binary's exit status.

This rule does not require `Configured()` for every reporting run: an accepted
partial CI configuration keeps its existing per-check semantics. It changes no
loader policy, transport, known-key set, or non-lint mode. Help/version continue
to finish before configuration loading. The separate unknown-key policy hold in
[#1261](https://github.com/medici-finance/assay/issues/1261) is unchanged.

## Guard register

| Rule | Owner / enforcement | Evidence | Cost and removal condition |
|---|---|---|---|
| Refused configuration cannot enter a lint evaluator | `statusgen/main.go`, one pre-dispatch check over the loader's typed Problems list | `TestLintConfigRefusal` runs the binary with unknown-key, unsupported-schema and missing-file inputs plus a valid roster | One branch; no text matcher or per-caller refusal list. A future entrypoint refactor must preserve the same precondition before invoking an evaluator. |

This is a command precondition, not an authoring-guidance lint rule. The
precondition cannot be removed while ordinary evaluators can return PASS
without consuming the loader's refusal state. Counting source findings instead
would let differential lint demote a process refusal as pre-existing.

## Invocation and consumer inventory

| Consumer | Invocation / handling | Route |
|---|---|---|
| Ordinary local lint | `statusgen --root DIR --lint` (also Go flag spelling `-lint`) | Fixed at entrypoint; process test |
| Repeated roots | `statusgen --root A --root B --lint` | Same process precondition before root ownership/evaluation; process test |
| Differential lint | `statusgen --lint --diff-base REF` plus root/budget/changed/scope flags | Same process precondition before self-shell/demotion; process test |
| `statusgen/difflint.go` | Inner `--root DIR --lint` with optional budget/changed/scope | A refused parent never reaches the runner; child processes enforce the same boundary |
| `statusgen/lintaudit.go` | Historical `--root DIR --lint`; tallies PROBLEM/NOTICE lines only | Advisory firing audit, not a clean-lint verdict; unchanged |
| `tools/desk/cmd/deskevidence/lintdiff.go` | Before/after `statusgen --root DIR --lint` | Nonzero with zero PROBLEM lines is Unverifiable; new refusal uses this existing route |
| `tools/desk/cmd/deskpreflight/main.go` | `statusgen-lint` check, `statusgen --root DIR --lint` | Any nonzero result fails the preflight check; unchanged |
| Installer `plugins/assay/scripts/assay-install.sh` | `statusgen --root COPY --lint` | Requires exit 0 to prove installation; refusal cannot prove installation |
| `.github/workflows/assay-statusgen.yml` and staged counterpart | PR lint, post-merge board validation and reconcile validation use `--root . --lint` | Nonzero fails the shell step; unchanged |
| `.github/workflows/verify-gate-close.yml` and staged counterpart | Captures `statusgen --root . --lint` in a conditional | Nonzero retains refusal; unchanged |
| `.github/workflows/windows-ci-leg.yml` and staged counterpart | `statusgen.exe --root .. --lint` | Requires success; unchanged |
| Shipped author-brief, adopt, install, intake-desk, PR-review-desk and verify-desk skills | Prescribe `--lint` proof or review checks | Continue to require success; exit 5 is could-not-check for trust, never a clean source result |
| `writeguard` | Classifies `--lint` as read-only | No verdict interpretation; unchanged |
| deskboard/deskdispatch | References lint guarantees or consumes board/brief data; no lint subprocess at the inventoried sites | No invocation change |

The regression plants an unsupported schema independently of the reported
unknown-key mechanism, without changing either producer. Both must stop at the
same boundary, as must a missing file. Valid controls traverse all three lint
forms so an unconditional refusal cannot pass the test.
