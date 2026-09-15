// Command deskversion is the adopter version marker: it reports
// which umbrella version a consumer repo is on, and which per-artifact versions
// that is made of, by assembling the answer from the `.assay-versions` pin file
// and the umbrella's composition manifest — records that already exist, never a
// fourth source of truth.
//
// It is a READ-ONLY reader. It reads the pin file and the umbrella's composition
// under --root and writes nothing; it never touches the platform's plugin install
// cache (`~/.claude/plugins`), which is a read-only input to `upgrade-assay`, not
// to this tool. The composition comes from `<releases>/<umbrella>.yaml` when the
// adopter has authored one, else from the release's published `checksums.txt` —
// materialised as `<releases>/<umbrella>.checksums.txt` for offline use, or
// or, ONLY under an explicit `--fetch`, fetched from the release home for exactly
// that tag (`--release-home` re-points it), with the URL printed to stderr before
// contact. Without `--fetch` the tool never reaches the network. Nothing fetched
// is cached or written.
//
// THREE STATES, ONE EXIT CODE EACH (deskkit contract, exitcodes.go):
//
//	known                → exit 0  — one umbrella version, consistent composition.
//	known-inconsistent   → exit 5  — records disagree; the report names the pair.
//	could-not-determine  → exit 6  — no/unreadable pin, no umbrella line
//	                                 ("no umbrella pin"), or unreadable composition.
//	                                 Never "assume latest".
//
// USAGE:
//
//	deskversion --root <consumer-repo> [--releases <dir>] [--fetch] [--release-home <owner/repo>]
//	deskversion --version
//
// --root defaults to ".". --releases defaults to <root>/releases (where a consumer
// materialises a composition manifest or the release's checksums.txt; the marker
// fixtures ship them there so the reader is exercisable offline).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskversion — the adopter version marker.

USAGE:
  deskversion --root <consumer-repo> [--releases <dir>] [--fetch] [--release-home <owner/repo>]
  deskversion --version

Reads the umbrella line + per-artifact lines from <root>/.assay-versions and
cross-checks them against the umbrella's composition: <releases>/<umbrella>.yaml
when authored, else derived from the release's checksums.txt — read from
<releases>/<umbrella>.checksums.txt when materialised, else — only under
--fetch — fetched from the release home for that tag, the URL printed to stderr
before contact. Without --fetch the tool never reaches the network.

Exit codes:
  0  known               — one umbrella version, consistent composition
  5  known-inconsistent  — records disagree (the report names which and how)
  6  could-not-determine — no/unreadable pin, no umbrella line, or unreadable
                           composition manifest (never "assume latest")`

// fetchFunc is the release-home fetcher; a package variable so tests swap in an
// offline fake and the production binary never needs the network in a test.
var fetchFunc deskkit.Fetcher = deskkit.HTTPFetch

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deskversion", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root     = fs.String("root", ".", "consumer repo root holding .assay-versions")
		releases = fs.String("releases", "", "composition-manifest / materialised-checksums dir (default <root>/releases)")
		home     = fs.String("release-home", deskkit.DefaultReleaseHome, "release home <owner>/<repo> whose checksums.txt derives a composition")
		fetch    = fs.Bool("fetch", false, "allow fetching checksums.txt from the release home (default: local files only)")
		version  = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}

	if *version {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskversion sourceSHA=%s builtAt=%s releaseTag=%s\n",
			sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	// --version / help are the only pure reads; a marker read still honours the
	// kill switch (a disabled desk answers nothing) before doing any work.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	relDir := *releases
	if relDir == "" {
		relDir = filepath.Join(*root, deskkit.ReleasesDir)
	}

	src := deskkit.CompositionSource{ReleasesDir: relDir, ReleaseHome: *home, Announce: stderr}
	if *fetch {
		src.Fetch = fetchFunc
	}
	m := deskkit.ReadMarkerFrom(*root, src)
	fmt.Fprint(stdout, m.Report())
	return m.State.ExitCode()
}
