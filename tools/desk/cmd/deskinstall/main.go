// Command deskinstall is the Go-native install path for the pinned Assay
// toolchain — the Windows equivalent of the Unix `sudo make desk-install`, and
// (by construction) a cross-platform installer the Unix side can converge on.
//
// It mirrors the Unix acquire→verify→place flow step for step:
//
//	detect platform (windows-amd64 / windows-arm64 via runtime.GOOS/GOARCH)
//	  → resolve the pinned tag + sha256 for that platform from the plugin's
//	    shipped paired-versions.yaml  (a pinned repo tag, NEVER a floating ref)
//	  → download the release asset (statusgen-windows-<arch>.exe and
//	    desk-tools-windows-<arch>.tar.gz) for that tag
//	  → compute sha256 and compare; a mismatch is a hard REFUSE, not a warning,
//	    so no unverified bytes are ever placed
//	  → place the verified binaries on a PATH-resolvable dir.
//
// The sha256-verify-or-refuse step is the single load-bearing control on this
// surface: it is the one thing standing between a substituted release asset and
// a Windows adopter running unverified bytes. It runs FIRST, after the download
// and before ANY placement — a mismatch leaves nothing installed. The value it
// checks against is itself pinned in the reviewed, version-committed
// paired-versions.yaml, so the check and the pin fail for different reasons.
//
// For the very first install (chicken-and-egg: you need a binary to run this
// installer), a ~5-line PowerShell bootstrap — scripts/bootstrap-windows.ps1 —
// fetches only statusgen-windows-<arch>.exe and hash-verifies it before
// executing anything; from there this Go subcommand acquires and verifies the
// rest. The security-critical hash-verify therefore lives in ONE tested Go
// implementation, and PowerShell is confined to a trivial, auditable bootstrap.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// manifestName is the plugin-shipped pin manifest the install tag + expected
// sha256 are resolved FROM. It is a pinned, version-committed artifact — the
// installer never resolves a floating or rolling release ref.
const manifestName = "paired-versions.yaml"

const usage = `deskinstall — Go-native, version-pinned, sha256-verified toolchain installer.

usage:
  deskinstall --manifest <paired-versions.yaml> --dest <dir> [--platform <os-arch>]
  deskinstall --harness cursor --forge <github|gitlab> --repo <path> [--bundle <dir>] [--check]
  deskinstall --version
  deskinstall -h | --help

MODE 1 — acquire, verify, place (the pinned toolchain binaries). It resolves the
pinned tag + per-platform sha256 from the manifest (a pinned repo tag, never a
floating ref), downloads the release assets for the detected platform, VERIFIES
their sha256 and REFUSES on any mismatch (nothing is placed), then installs the
verified binaries into --dest.

Windows assets (from the release build matrix):
  statusgen-windows-amd64.exe      / statusgen-windows-arm64.exe
  desk-tools-windows-amd64.tar.gz  / desk-tools-windows-arm64.tar.gz

  --manifest   path to the pin manifest (` + manifestName + `). Required.
  --dest       PATH-resolvable directory the verified binaries are placed in. Required.
  --platform   override auto-detection (e.g. windows-amd64). Default: this host.

A successful run prints the .assay/ledger.jsonl path (component-model.md §5):
this mode's own effects are inside the boundary and record no line there, but
the path is where a later outside-effect component would, and it is what
` + "`deskdisable`" + ` reads when reversing one.

MODE 2 — harness placement (Cursor's install mechanism IS file placement: no
marketplace, no per-harness plugin manifest). Places the packaging roster's
skills into --repo/.cursor/skills, plugins/assay/references/*.md as a SIBLING
references/ tree so the skills' ../../references/*.md includes resolve, the
generated .cursor/rules/assay.mdc if the bundle carries one, and writes the
shared AGENTS.md bindings fragment between stable assay:bindings delimiters
(forge-substituted: gh on github, glab/--forge gitlab on gitlab). Idempotent —
a second identical run changes no byte.

  --harness    harness to place for. Only "cursor" is supported today.
  --forge      github | gitlab — which desk-transport vocabulary the AGENTS.md
               bindings name. Required with --harness.
  --repo       the adopter repo root the tree is placed into. Required with --harness.
  --bundle     the plugins/assay-shaped source tree to place FROM. Default:
               "plugins/assay" relative to the current directory (run this mode
               from within a checkout of the assay repo that ships that bundle).
  --check      re-derive what a run WOULD place and diff it against --repo;
               writes nothing. Exit 0 clean, 1 drift (names every drifted path
               and its kind: missing / extra / content-differs), 2 could-not-check.

--harness is mutually exclusive with --manifest/--dest: supplying both is a
refusal naming both modes, never a silent precedence.

exit (mode 1, and mode 2 without --check): 0 installed/placed · 5 refused
exit (mode 2 with --check): 0 clean · 1 drift · 2 could-not-check`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskinstall sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	var manifest, dest, platform, harness, forge, repo, bundle string
	var checkFlag bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --manifest needs a path")
				return deskkit.ExitRefused
			}
			i++
			manifest = args[i]
		case "--dest":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --dest needs a directory")
				return deskkit.ExitRefused
			}
			i++
			dest = args[i]
		case "--platform":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --platform needs an os-arch value")
				return deskkit.ExitRefused
			}
			i++
			platform = args[i]
		case "--harness":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --harness needs a value (e.g. cursor)")
				return deskkit.ExitRefused
			}
			i++
			harness = args[i]
		case "--forge":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --forge needs a value (github or gitlab)")
				return deskkit.ExitRefused
			}
			i++
			forge = args[i]
		case "--repo":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --repo needs a path")
				return deskkit.ExitRefused
			}
			i++
			repo = args[i]
		case "--bundle":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --bundle needs a directory")
				return deskkit.ExitRefused
			}
			i++
			bundle = args[i]
		case "--check":
			checkFlag = true
		default:
			fmt.Fprintf(stderr, "deskinstall: unknown argument %q\n%s\n", args[i], usage)
			return deskkit.ExitRefused
		}
	}

	if harness != "" {
		if manifest != "" || dest != "" {
			fmt.Fprintln(stderr, "deskinstall: --harness cursor (harness-placement mode) cannot be combined with "+
				"--manifest/--dest (acquire→verify→place mode) — refusing: two install modes given, choose one\n"+usage)
			return deskkit.ExitRefused
		}
		return runHarness(harness, forge, repo, bundle, checkFlag, stdout, stderr)
	}
	if checkFlag {
		fmt.Fprintln(stderr, "deskinstall: --check is only meaningful with --harness cursor\n"+usage)
		return deskkit.ExitRefused
	}
	if manifest == "" || dest == "" {
		fmt.Fprintln(stderr, "deskinstall: --manifest and --dest are both required\n"+usage)
		return deskkit.ExitRefused
	}

	err := Install(Options{
		ManifestPath: manifest,
		DestDir:      dest,
		Platform:     platform,
		Out:          stdout,
	})
	if err != nil {
		fmt.Fprintf(stderr, "deskinstall: REFUSED — %v\n", err)
		return deskkit.ExitRefused
	}
	return deskkit.ExitOK
}

// bundleDirDefault is the source tree's default location relative to the
// current working directory: `--harness cursor` is meant to be run from
// within a checkout of medici-finance/assay (the repo that ships
// plugins/assay), the same convention tools/harnessgen's own --root "."
// default uses.
const bundleDirDefault = "plugins/assay"

// runHarness is the CLI glue for MODE 2 (`--harness cursor`): flag validation
// and exit-code mapping only. All placement/check logic lives in
// harness_cursor.go's HarnessCursorPlace/HarnessCursorCheck, which tests call
// directly with an explicit (fixture) BundleDir — this function is exercised
// only for its own flag-parsing and mode-exclusivity behaviour.
func runHarness(harness, forge, repo, bundle string, check bool, stdout, stderr io.Writer) int {
	if harness != "cursor" {
		fmt.Fprintf(stderr, "deskinstall: unknown --harness %q: expected cursor\n", harness)
		return deskkit.ExitRefused
	}
	if repo == "" {
		fmt.Fprintln(stderr, "deskinstall: --harness cursor requires --repo <path>")
		return deskkit.ExitRefused
	}
	if forge == "" {
		fmt.Fprintln(stderr, "deskinstall: --harness cursor requires --forge github|gitlab")
		return deskkit.ExitRefused
	}
	if bundle == "" {
		bundle = bundleDirDefault
	}
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: forge}

	if check {
		findings, err := HarnessCursorCheck(opts)
		if err != nil {
			fmt.Fprintf(stderr, "deskinstall --harness cursor --check: could-not-check: %v\n", err)
			return exitHarnessCouldNotCheck
		}
		if len(findings) > 0 {
			fmt.Fprintln(stderr, "deskinstall --harness cursor --check: DRIFT")
			for _, f := range findings {
				fmt.Fprintf(stderr, "  %s\n", f)
			}
			return exitHarnessDrift
		}
		fmt.Fprintln(stdout, "deskinstall --harness cursor --check: clean")
		return exitHarnessClean
	}

	if err := HarnessCursorPlace(opts, stdout); err != nil {
		fmt.Fprintf(stderr, "deskinstall --harness cursor: REFUSED — %v\n", err)
		return deskkit.ExitRefused
	}
	return deskkit.ExitOK
}
