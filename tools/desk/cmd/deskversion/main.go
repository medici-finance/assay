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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// helpLong is the root help's prose. The flags are generated from the command tree
// (desktools-v2/47); this is what the tree cannot say on its own.
const helpLong = `deskversion — the adopter version marker.

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
  5  known-inconsistent  — records disagree (the report names which and how);
                           also a command line deskversion cannot parse
  6  could-not-determine — no/unreadable pin, no umbrella line, or unreadable
                           composition manifest (never "assume latest")`

// fetchFunc is the release-home fetcher; a package variable so tests swap in an
// offline fake and the production binary never needs the network in a test.
var fetchFunc deskkit.Fetcher = deskkit.HTTPFetch

func main() {
	os.Exit(deskkit.PerfRun("deskversion", nil, os.Args[1:], func(a []string) int {
		return execute(a, os.Stdout, os.Stderr, startup)
	}))
}

// startup is the process startup a marker read runs first; help and version never reach it.
// false means the verb is not activated (exit 6).
func startup(errw io.Writer) bool {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(errw)
	return deskkit.CheckVerbActivation(errw)
}

// run is one invocation without the process startup — the shape the package's tests drive.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, nil)
}

// markerOptions are the typed values the command tree hands the marker read.
type markerOptions struct {
	root, releases, releaseHome string
	fetch                       bool
}

// markerFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var markerFn = readMarker

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version and -version print. Install and upgrade checks read it.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskversion sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskversion declares: flags and defaults only, no environment
// variable and no configuration file.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.String, Flag: "root", Default: ".", Usage: "consumer repo root holding .assay-versions"},
		{Key: "releases", Kind: cli.String, Flag: "releases", Default: "", Usage: "composition-manifest / materialised-checksums `dir` (default <root>/releases)"},
		{Key: "release-home", Kind: cli.String, Flag: "release-home", Default: deskkit.DefaultReleaseHome, Usage: "release home `owner/repo` whose checksums.txt derives a composition"},
		{Key: "fetch", Kind: cli.Bool, Flag: "fetch", Default: false, Usage: "allow fetching checksums.txt from the release home (default: local files only)"},
	}
}

// newTree builds deskversion's command tree on the given streams; pre is as for execute.
func newTree(stdout, stderr io.Writer, pre func(io.Writer) bool) *cobra.Command {
	root := cli.NewRoot("deskversion", "the adopter version marker")
	root.Long = helpLong
	set := cli.Declare(root, bindings()...)
	// deskversion takes no positional argument; a stray word is refused before anything runs.
	root.Args = func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown argument %q", args[0])
		}
		return nil
	}
	root.RunE = func(*cobra.Command, []string) error {
		v, err := set.Resolve(cli.Inputs{})
		if err != nil {
			fmt.Fprintf(stderr, "deskversion: refused: %v\n", err)
			return exitErr(deskkit.ExitRefused)
		}
		if onResolve != nil {
			onResolve(v)
		}
		if pre != nil && !pre(stderr) {
			return exitErr(deskkit.ExitUnverifiable)
		}
		return codeErr(markerFn(markerOptions{
			root: v.String("root"), releases: v.String("releases"),
			releaseHome: v.String("release-home"), fetch: v.Bool("fetch"),
		}, stdout, stderr))
	}
	return root
}

// execute runs one invocation against a fresh command tree. pre is the process startup (roster
// class, config echo, activation check); it runs after the command line parses and never for
// help or version.
func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) bool) int {
	build := func() *cobra.Command { return newTree(stdout, stderr, pre) }
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A command line deskversion cannot parse is refused (5), as the flag set refused it.
		UsageExit: deskkit.ExitRefused,
		// The Go flag package accepted -root, -releases, -release-home, -fetch and -version;
		// they stay accepted.
		GoFlagCompat: true,
	})
}

// readMarker is the marker read itself. It honours the kill switch (a disabled desk answers
// nothing) before doing any work, and its report and exit code are the reader's, unchanged.
func readMarker(o markerOptions, stdout, stderr io.Writer) int {
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	relDir := o.releases
	if relDir == "" {
		relDir = filepath.Join(o.root, deskkit.ReleasesDir)
	}

	src := deskkit.CompositionSource{ReleasesDir: relDir, ReleaseHome: o.releaseHome, Announce: stderr}
	if o.fetch {
		src.Fetch = fetchFunc
	}
	m := deskkit.ReadMarkerFrom(o.root, src)
	fmt.Fprint(stdout, m.Report())
	return m.State.ExitCode()
}
