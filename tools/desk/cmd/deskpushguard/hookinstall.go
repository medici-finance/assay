// hookinstall.go implements `deskpushguard hook-install`: writes the platform-correct
// pre-push hook shim(s) into a .githooks directory (core.hooksPath), resolving the
// deskpushguard binary from PATH rather than baking in a hardcoded absolute install
// path.
//
// Before this (example-stream/12, closing the "Push-guard shim" needs-port row from
// docs/streams/example-stream/portability-audit.md — the real stream slug is withheld
// from shipping copy-set files per the corpus withheld-path guard), the ONLY way to populate
// .githooks/pre-push was `make desk-hook-install` / the PowerShell
// `Target-DeskHookInstall`, and both copied the committed `tools/desk/hooks/pre-push`
// shim VERBATIM — a `#!/bin/sh` script that `exec`s the fixed unix install path
// `/opt/desk-tools/bin/deskpushguard`. That literal has no Windows equivalent and no
// portable resolution: a Windows install lands the binary somewhere on PATH (a
// per-user install dir), never at `/opt`.
//
// `hook-install` is the fix: it resolves `deskpushguard` from PATH at hook RUN time
// (`command -v deskpushguard` in the unix shim), so the same shim works regardless of
// where the binary was installed, and on Windows it additionally writes the
// `pre-push.cmd` pair Git for Windows' native (non-sh) hook resolution needs. Both
// installers (`make desk-hook-install`, `scripts/build-windows.ps1`'s
// `Target-DeskHookInstall`) call this subcommand once the binary has been built,
// rather than copying the static file, so both convergent install paths agree.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// unixShim is the sh pre-push hook shim, written on every platform (Git for Windows
// ships its own sh.exe and runs this exact shim; native cmd.exe/PowerShell git hook
// resolution on Windows uses the .cmd pair below instead). It resolves the guard
// binary from PATH at run time — `command -v` — rather than a baked-in absolute
// install path, which is the one property that makes it portable across install
// locations and operating systems.
const unixShim = `#!/bin/sh
# Desk push guard -- git pre-push hook shim.
# Committed by ` + "`deskpushguard hook-install`" + ` into .githooks/pre-push (core.hooksPath);
# covers all worktrees. Resolves the deskpushguard binary from PATH at run time -- no
# absolute install-location literal is baked into this shim, so it works regardless of
# where deskpushguard was installed.
set -e
exec "$(command -v deskpushguard)" "$@"
`

// windowsCmdShim is the .cmd pair written alongside pre-push on Windows targets, for a
// git invocation that resolves hooks natively (cmd.exe) rather than through Git for
// Windows' bundled sh. It calls the .exe by name, relying on PATH exactly as the unix
// shim relies on `command -v`.
const windowsCmdShim = `@echo off
rem Desk push guard -- git pre-push hook shim (Windows .cmd pair for pre-push).
rem Written by deskpushguard hook-install; resolves deskpushguard.exe from PATH.
deskpushguard.exe %*
`

// legacyUnixShim is the exact formerly shipped hook, not a tool-name ownership
// heuristic. Only line-ending conversion is accepted; local edits remain foreign.
const legacyUnixShim = `#!/bin/sh
# Desk push guard — git pre-push hook shim.
# Committed into .githooks/pre-push (F-13 core.hooksPath); covers all worktrees.
# ` + "`make desk-install`" + ` refreshes it on each install run.
set -e
exec /opt/desk-tools/bin/deskpushguard "$@"
`

// writeHooks writes the pre-push hook shim(s) for targetOS into dir (normally
// ".githooks"). On a non-windows target it writes only "pre-push" (the PATH-resolving
// sh shim). On "windows" it writes BOTH "pre-push" (for Git for Windows' bundled
// sh.exe) and "pre-push.cmd" (the native pair). targetOS is a parameter rather than a
// bare runtime.GOOS read so both halves of the pairing are exercisable from a test
// suite running on any single host OS — production callers pass runtime.GOOS.
//
// force mirrors the Makefile's `FORCE=1` / PowerShell's `-Force`: without it, an
// existing hook that this installer did not itself write is left alone and reported
// as a typed error, never silently clobbered.
func writeHooks(dir, targetOS string, force bool) (installed []string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("hook-install: cannot create %s: %w", dir, err)
	}

	type shim struct {
		path, content string
		mode          os.FileMode
	}
	shims := []shim{{filepath.Join(dir, "pre-push"), unixShim, 0o755}}
	if targetOS == "windows" {
		shims = append(shims, shim{filepath.Join(dir, "pre-push.cmd"), windowsCmdShim, 0o644})
	}
	// Check the whole pair before changing either half. A foreign Windows
	// partner must not leave a newly installed or migrated Unix half behind.
	var pending []shim
	for _, sh := range shims {
		write, err := shimNeedsWrite(sh.path, sh.content, force)
		if err != nil {
			return nil, err
		}
		if write {
			pending = append(pending, sh)
		}
	}
	for _, sh := range pending {
		if err := os.WriteFile(sh.path, []byte(sh.content), sh.mode); err != nil {
			return installed, fmt.Errorf("hook-install: cannot write %s: %w", sh.path, err)
		}
		if err := os.Chmod(sh.path, sh.mode); err != nil {
			return installed, fmt.Errorf("hook-install: cannot set mode on %s: %w", sh.path, err)
		}
		installed = append(installed, sh.path)
	}

	return installed, nil
}

// runHookInstall parses `hook-install [--dir <githooks-dir>] [--force]` and writes the
// hook shim(s) for the running host's GOOS. It is the CLI entry point writeHooks is
// wrapped in; production always passes runtime.GOOS, so the windows pair is written
// only when actually running on Windows (the cross-OS parametrisation exists for the
// test suite, not for a flag an operator would pass).
func runHookInstall(args []string, stderr io.Writer) int {
	dir := ".githooks"
	force := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "deskpushguard hook-install: --dir requires a path argument")
				return deskkit.ExitRefused
			}
			dir = args[i]
		case "--force":
			force = true
		case "-h", "--help":
			fmt.Fprintln(stderr, "usage: deskpushguard hook-install [--dir <githooks-dir>] [--force]")
			return deskkit.ExitOK
		default:
			fmt.Fprintf(stderr, "deskpushguard hook-install: unknown argument %q\n", args[i])
			return deskkit.ExitRefused
		}
	}

	installed, err := writeHooks(dir, runtime.GOOS, force)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitRefused
	}
	if len(installed) == 0 {
		fmt.Fprintf(stderr, "deskpushguard hook-install: %s already carries the deskpushguard hook(s) (idempotent skip)\n", dir)
		return deskkit.ExitOK
	}
	for _, p := range installed {
		fmt.Fprintf(stderr, "deskpushguard hook-install: wrote %s\n", p)
	}
	return deskkit.ExitOK
}

// shimNeedsWrite distinguishes exact current content, known legacy content, and
// foreign hooks. Explicit --force retains its existing foreign-overwrite contract.
func shimNeedsWrite(path, content string, force bool) (bool, error) {
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("hook-install: cannot read %s: %w", path, err)
	}
	normalized := strings.ReplaceAll(string(existing), "\r\n", "\n")
	if normalized == content {
		return false, nil
	}
	if (content == unixShim && normalized == legacyUnixShim) || force {
		return true, nil
	}
	firstLine := strings.SplitN(string(existing), "\n", 2)[0]
	return false, fmt.Errorf("hook-install: refusing to clobber existing non-deskpushguard hook %s (%q) -- pass --force to overwrite", path, firstLine)
}
