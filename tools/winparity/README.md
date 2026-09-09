# `winparity` — Windows-build ↔ Makefile target parity guard

The Windows counterpart to the root `Makefile` is
[`scripts/build-windows.ps1`](../../scripts/build-windows.ps1). It mirrors the
Makefile's `.PHONY` target set by hand, and a hand-kept mirror drifts: a target
added to one file and not the other ships a Windows build that has quietly lost
(or invented) a target. `winparity` is the check that reddens on that state —
the same shape of guard as `tools/skillslint`'s guardrail byte-diff and
`tools/pairedversions`' front-door consistency check.

## What it asserts

Two things about `scripts/build-windows.ps1`:

1. **Target parity.**

   > the set of targets between the `MAKEFILE-PARITY TARGETS (BEGIN/END)` markers
   > in `scripts/build-windows.ps1` **equals** the set of targets on the
   > `Makefile`'s `.PHONY:` line.

   Set equality, not subset — a target present on either side but not the other
   is a failure, in both directions.

2. **Windows PowerShell 5.1 cleanliness (#678).** The script must stay parseable
   by `powershell.exe` (5.1), not only by `pwsh` (7+). Concretely it must be
   **ASCII-only** and contain **no `>>>`** in any string: 5.1 lexes `>>>` as a
   redirection operator (a `ParserError` before compile) and its non-UTF-8
   default encoding mangles em-dashes and other non-ASCII in error/log strings.
   The scan reads the file as bytes, so this leg catches a regression on Linux
   CI before it reaches a native Windows host — no PowerShell needed to run it.

## Three-state, fail-closed

`checked-clean` is exit 0. A checked disagreement (`DRIFT`) and a
`could-not-check` (a file it could not read, or a `.ps1` with no marker block)
are both non-zero and are reported **as themselves**; a source it could not read
has cleared nothing and never renders green
([`docs/three-state-instrument-rule.md`](../../docs/three-state-instrument-rule.md)).

## Usage

```
go run . --root ../..     # from tools/winparity/
winparity --root .        # exit 0 = in parity, 1 = drift or could-not-check, 2 = usage error
```

It reads only two files under `--root` (`Makefile` and
`scripts/build-windows.ps1`) and contacts no network. Exit is fail-closed over
both assertions: `checked-clean` (exit 0) requires target parity AND a
5.1-clean script; any drift, any 5.1 regression, or any could-not-check reddens.

`scripts/build-windows.ps1` runs this guard as a **preflight** before executing
any target, so a drift is caught on the Windows side before a build runs, not
just in CI.
