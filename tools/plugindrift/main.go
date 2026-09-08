// plugindrift — drift checker for the `plugins/assay` skills bundle.
//
// The bundle is the CANONICAL home of the five method-text skills since the
// harness-portability/02 authority flip: SOURCES.yaml declares them under
// `canonical:` with no upstream, so there is nothing to re-fetch and nothing to
// drift against. The checker's live job is COVERAGE — every `skills/*/SKILL.md`
// accounted for as canonical-here or authored-here — plus validating any
// remaining `files:` rows that a future port may add back, re-fetching each such
// source at its recorded commit and at the source repo's current default branch,
// and reporting per file:
//
//	in-sync      head blob == the recorded blob
//	behind       head blob differs; N commits have touched the path since
//	moved        the recorded path no longer exists at head
//	unreachable  the source could not be read (network, auth, or not a remote)
//
// N IS A LOWER BOUND, NOT THE ANCESTRY DISTANCE. It comes from GitHub's
// `commits?since=<date>` filter (see ghClient.CommitsSince), which selects on a
// commit's own date. A commit authored before the recorded commit's timestamp but
// merged into the default branch after it is reachable from head, yet the filter
// drops it. Measuring the same five bundled paths by ancestry
// (`git rev-list --count <recorded>..origin/main -- <path>`) gave 136 where the
// date filter gave 131. The in-sync / behind / moved VERDICT is unaffected — that
// is decided by blob-sha comparison, not by N — so this understates severity,
// never a false in-sync. Fixing it needs an ancestry-aware source (a local clone,
// or per-commit path checks over the compare API), which is a follow-up, not a
// tweak: tracked in tools/README.md.
//
// Advisory by design: it exits 0 even on drift, so a stale bundle informs rather
// than blocks. `--fail-on-drift` makes any non-`in-sync` source fatal (exit 1),
// including `unreachable` — an unreadable source is an unknown, and the strict
// mode fails closed on unknowns.
//
// Cross-reference origins are reported but never decide the verdict; they exist
// for entries whose authoritative source is not remotely addressable.
//
// Before checking drift it checks COVERAGE: every `skills/*/SKILL.md` in the
// bundle must be pinned in `files:`, declared canonical-here in `canonical:`,
// or declared in `unported:` with a reason. "Every pinned file is in-sync" says
// nothing until the pinned set is known to be the whole set, so an unaccounted
// file exits 2 in either mode.
//
// Reads GitHub through the `gh` CLI, so it inherits the caller's auth and needs
// no token of its own. No network, no gh, or an unauthenticated gh yields
// `unreachable` for every remote source — never a false `in-sync`.
//
//	go run ./tools/plugindrift                     # advisory, exit 0
//	go run ./tools/plugindrift --fail-on-drift     # exit 1 on drift
//	make plugin-drift
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultManifest    = "plugins/assay/SOURCES.yaml"
	defaultMarketplace = ".claude-plugin/marketplace.json"
)

func main() {
	root := flag.String("root", ".", "the repository root to scan")
	manifest := flag.String("manifest", defaultManifest, "Path to the sources manifest, relative to --root")
	marketplace := flag.String("marketplace", defaultMarketplace, "Path to the marketplace catalog whose external plugin pins are checked, relative to --root (empty string skips the check)")
	failOnDrift := flag.Bool("fail-on-drift", false, "Exit 1 when any non-advisory source is not in-sync (unreachable counts as drift)")
	// --marketplace-only scopes the run to the marketplace catalog's external
	// plugin pins, skipping the SOURCES.yaml manifest and its coverage check.
	// This is a SCOPE flag, not a verdict change: every origin's status
	// (in-sync/behind/moved/unreachable) and the advisory semantics are decided
	// by the same code either way — this only selects WHICH origins run. The
	// scheduled impeccable re-pin surface (ui-craft/04) uses it so its exit code
	// reflects the catalog pin's currency alone: a `behind` catalog pin is the
	// fork's designed steady state (advisory, exit 0), while `moved`/`unreachable`
	// is actionable pin-target drift / could-not-check (exit 1). Run over the full
	// manifest, that signal is swamped by the bundle's own pre-existing `behind`
	// SOURCES rows (dailies/intake-desk), which would pin the exit code to a
	// permanent, decoupled red.
	marketplaceOnly := flag.Bool("marketplace-only", false, "Check ONLY the marketplace catalog's external plugin pins; skip the SOURCES.yaml manifest + coverage")
	maxPages := flag.Int("max-pages", 5, "Pages of per-path commit history (100/page) to walk looking for the recorded commit")
	flag.Parse()

	if *maxPages < 1 {
		fmt.Fprintln(os.Stderr, "plugindrift: --max-pages must be at least 1")
		os.Exit(2)
	}

	var (
		m       Manifest
		cov     Coverage
		haveCov bool
	)
	// Root-joined manifest path, computed once so the run header (below) prints the
	// file actually read rather than the raw --manifest flag, in every mode.
	manifestPath := *manifest
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(*root, *manifest)
	}
	if !*marketplaceOnly {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: cannot read %s: %v\n", manifestPath, err)
			os.Exit(2)
		}

		if err := yaml.Unmarshal(data, &m); err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: cannot parse %s: %v\n", manifestPath, err)
			os.Exit(2)
		}
		if err := m.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: %s: %v\n", manifestPath, err)
			os.Exit(2)
		}

		// Coverage before drift: a file nobody pinned cannot be reported stale, so
		// "every pinned file is in-sync" means nothing until the pinned set is known
		// to be the whole set. A gap here is a manifest error, not advisory drift.
		cov, err = CheckCoverage(filepath.Dir(manifestPath), &m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: %s: coverage: %v\n", manifestPath, err)
			os.Exit(2)
		}
		haveCov = true
	}

	// Marketplace catalog: external plugin pins are provenance too (ui-craft/02,
	// ruling #1024). Parse errors — including an external source with no sha —
	// are manifest errors, same class as a bad SOURCES.yaml. In --marketplace-only
	// mode the catalog is the whole signal, so an empty --marketplace path here is
	// a usage error rather than a silent no-op.
	if *marketplace == "" && *marketplaceOnly {
		fmt.Fprintln(os.Stderr, "plugindrift: --marketplace-only needs a --marketplace path (got empty)")
		os.Exit(2)
	}
	var pins []MarketplacePin
	mpPath := ""
	if *marketplace != "" {
		mpPath = *marketplace
		if !filepath.IsAbs(mpPath) {
			mpPath = filepath.Join(*root, *marketplace)
		}
		mpData, err := os.ReadFile(mpPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: cannot read %s: %v\n", mpPath, err)
			os.Exit(2)
		}
		pins, err = ParseMarketplace(mpData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "plugindrift: %s: %v\n", mpPath, err)
			os.Exit(2)
		}
	}

	// In --marketplace-only mode the catalog pins are the ENTIRE signal, so an
	// empty pin set is a fail-closed condition, not a clean run. Without this a
	// renamed or removed impeccable entry (or any catalog that parses to zero
	// external pins) would fall through to the "0 origins is canonical-here" summary
	// and report CLEAN / exit 0 forever — the exact silent-green an unknown must
	// never become. Same class as the empty --marketplace guard above; fail closed.
	if *marketplaceOnly && len(pins) == 0 {
		fmt.Fprintf(os.Stderr, "plugindrift: --marketplace-only found 0 external plugin pin(s) in %s — nothing to check (renamed or removed catalog entry?)\n", mpPath)
		os.Exit(2)
	}

	client := newGHClient()
	var results []Result
	if !*marketplaceOnly {
		results = Check(client, &m, *maxPages)
	}
	results = append(results, CheckMarketplace(client, pins, *maxPages)...)
	summary := Summarize(results)

	if *marketplaceOnly {
		fmt.Printf("plugindrift: marketplace-only — external plugin pin(s) from %s\n", mpPath)
	} else {
		total := len(m.Files) + len(m.Canonical) + len(m.Unported)
		fmt.Printf("plugindrift: %s v%s — %d file(s) from %s\n", m.Bundle, m.BundleVersion, total, manifestPath)
	}
	if haveCov {
		fmt.Printf("coverage:    %d bundled %s — %d pinned, %d canonical, %d unported, 0 unaccounted\n",
			cov.OnDisk, coverageGlob, cov.Pinned, cov.Canonical, cov.Unported)
	}
	if mpPath != "" {
		fmt.Printf("marketplace: %d external plugin pin(s) from %s\n", len(pins), mpPath)
	}
	fmt.Println()
	for _, r := range results {
		origin := ""
		if r.Advisory {
			origin = " (cross-reference, advisory)"
			if r.Origin == "catalog" {
				origin = " (advisory — deliberately pinned; upstream movement is re-pin material, not drift)"
			}
		}
		fmt.Printf("%-11s  %s%s\n             %s\n", label(r.Status), r.File, origin, r.Detail)
	}
	if len(results) == 0 {
		// Post-flip state: no files: rows, so no origins to check. The coverage
		// line above is the whole signal — every bundled file accounted for as
		// canonical-here or authored-here, which is exactly "0 behind, 0
		// unreachable, 0 unaccounted".
		fmt.Println("\nPLUGINDRIFT: CLEAN (0 origins — every bundled file is canonical-here or authored-here)")
	} else {
		fmt.Printf("\n%s\n", summary.Verdict())
	}

	if summary.Drift {
		fmt.Println("plugindrift: re-port from the current source, or update SOURCES.yaml and PARITY.md to a newer commit.")
	}
	if *failOnDrift && summary.Drift {
		os.Exit(1)
	}
}

func label(status string) string {
	switch status {
	case StatusInSync:
		return "IN-SYNC"
	case StatusBehind:
		return "BEHIND"
	case StatusMoved:
		return "MOVED"
	default:
		return "UNREACHABLE"
	}
}
