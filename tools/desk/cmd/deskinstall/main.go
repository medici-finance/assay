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
//	  → download the asset's Sigstore attestation bundle (<asset>.sigstore.json)
//	    and verify it attests THESE bytes as built by this repo's release
//	    workflow, run by the push of the pinned tag or by workflow_dispatch
//	    from main; any failure is a hard REFUSE (attest.go)
//	  → place the verified binaries on a PATH-resolvable dir.
//
// The attestation check is a second control that fails for a different reason
// than the sha256 check: the pin proves the bytes are the bytes the manifest
// names, the attestation proves the bytes the manifest names came out of the
// release workflow. No flag or option skips it.
//
// The sha256-verify-or-refuse step runs FIRST, after the download and before
// ANY placement — a mismatch leaves nothing installed. The value it checks
// against is itself pinned in the reviewed, version-committed
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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cursorbundle"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// manifestName is the plugin-shipped pin manifest the install tag + expected
// sha256 are resolved FROM. It is a pinned, version-committed artifact — the
// installer never resolves a floating or rolling release ref.
const manifestName = "paired-versions.yaml"

func main() {
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	os.Exit(deskkit.PerfRunMeasured("deskinstall", nil, os.Args[1:], func(a []string, measurement *perfrec.Session) (int, perfrec.Context) {
		sink := func(c perfrec.Context) {
			selected = c
			if measurement != nil {
				measurement.SelectContext(c)
			}
		}
		return run(a, os.Stdout, os.Stderr, sink), selected
	}))
}

// installFn is mode 1's domain entry point; the admission test wraps it to inject a
// fetcher and trust root while the real Install keeps every check.
var installFn = Install

// dispatch applies deskinstall's mode rules to the typed options the command tree
// resolved and runs the selected mode. Every refusal and exit code here is the one the
// hand-rolled parser had; the command tree only parses.
func dispatch(o options, stdout, stderr io.Writer) int {
	if o.harness != "" {
		if o.manifest != "" || o.dest != "" {
			fmt.Fprintln(stderr, "deskinstall: --harness cursor (harness-placement mode) cannot be combined with "+
				"--manifest/--dest (acquire→verify→place mode) — refusing: two install modes given, choose one")
			return deskkit.ExitRefused
		}
		if o.platform != "" {
			fmt.Fprintln(stderr, "deskinstall: --platform is only valid for binary installation")
			return deskkit.ExitRefused
		}
		if o.release != "" || o.pack != "" || o.dryRun || o.fetch || o.releases != "" || o.releaseHome != "" {
			return runVersionedHarness(o.harness, o.forge, o.repo, o.bundle, o.release, o.releases, o.releaseHome, o.pack, o.check, o.dryRun, o.fetch, stdout, stderr)
		}
		return runHarness(o.harness, o.forge, o.repo, o.bundle, o.check, stdout, stderr)
	}
	if o.dryRun || o.fetch || o.release != "" || o.releases != "" || o.releaseHome != "" || o.pack != "" {
		fmt.Fprintln(stderr, "deskinstall: release and preview options require --harness cursor")
		return deskkit.ExitRefused
	}
	if o.check {
		fmt.Fprintln(stderr, "deskinstall: --check is only meaningful with --harness cursor")
		return deskkit.ExitRefused
	}
	if o.manifest == "" || o.dest == "" {
		fmt.Fprintln(stderr, "deskinstall: --manifest and --dest are both required")
		return deskkit.ExitRefused
	}

	err := installFn(Options{
		ManifestPath: o.manifest,
		DestDir:      o.dest,
		Platform:     o.platform,
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

func runVersionedHarness(harness, forge, repo, bundle, release, releases, home, pack string, check, dryRun, fetch bool, stdout, stderr io.Writer) int {
	if harness != "cursor" {
		fmt.Fprintln(stderr, "deskinstall: only cursor supports versioned harness placement")
		return deskkit.ExitRefused
	}
	if pack != "" {
		if release == "" || check || dryRun || fetch || repo != "" || releases != "" || home != "" || forge != "" {
			fmt.Fprintln(stderr, "deskinstall: --pack requires --release and optional --bundle only")
			return deskkit.ExitRefused
		}
		if bundle == "" {
			bundle = bundleDirDefault
		}
		var encoded bytes.Buffer
		if err := cursorbundle.Pack(bundle, release, &encoded); err != nil {
			fmt.Fprintln(stderr, err)
			return deskkit.ExitRefused
		}
		if err := os.WriteFile(pack, encoded.Bytes(), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return deskkit.ExitRefused
		}
		fmt.Fprintf(stdout, "generated %s for %s; publish with its composition SHA-256\n", cursorbundle.AssetName, release)
		return deskkit.ExitOK
	}
	if repo == "" || (forge != "github" && forge != "gitlab") || (check && dryRun) {
		fmt.Fprintln(stderr, "deskinstall: require --repo and --forge github|gitlab; choose at most one of --check and --dry-run")
		return deskkit.ExitRefused
	}
	if release != "" {
		if bundle != "" {
			fmt.Fprintln(stderr, "deskinstall: --release cannot use checkout --bundle")
			return deskkit.ExitRefused
		}
		if releases == "" {
			releases = filepath.Join(repo, deskkit.ReleasesDir)
		}
		src := deskkit.CompositionSource{ReleasesDir: releases, ReleaseHome: home, Announce: stderr}
		if fetch {
			src.Fetch = deskkit.HTTPFetch
			src.CursorFetch = deskkit.HTTPFetchCursorBundle
		}
		if !validRelease(release) {
			fmt.Fprintln(stderr, "deskinstall: --release requires strict vX.Y.Z")
			return deskkit.ExitRefused
		}
		comp, found, err := src.Load(release)
		if err != nil || !found || comp.Umbrella != release {
			fmt.Fprintf(stderr, "deskinstall: selected Cursor release unavailable or inconsistent: %v\n", err)
			return deskkit.ExitRefused
		}
		var cleanup func()
		bundle, cleanup, err = cursorbundle.Acquire(src, comp)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return deskkit.ExitRefused
		}
		defer cleanup()
	} else if fetch || releases != "" || home != "" {
		fmt.Fprintln(stderr, "deskinstall: acquisition options require --release")
		return deskkit.ExitRefused
	}
	if bundle == "" {
		bundle = bundleDirDefault
	}
	if dryRun {
		plan, err := cursorbundle.Prepare(cursorbundle.Options{BundleDir: bundle, RepoRoot: repo, Forge: forge})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return deskkit.ExitRefused
		}
		fmt.Fprintln(stdout, "DRY-RUN Cursor placement (writing nothing under --repo)")
		plan.Report(stdout)
		return deskkit.ExitOK
	}
	return runHarness(harness, forge, repo, bundle, check, stdout, stderr)
}
func validRelease(tag string) bool {
	return regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(tag)
}
