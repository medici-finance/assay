package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `desksourceguard — refuse to compile a desk-tools source tree that is not the pinned commit

  desksourceguard --source <dir> [--repo-root <dir>] [--platform <os-arch>]

  --source     the materialised assay checkout (the directory holding .git),
               as produced by the consumer's desk-tools install action
  --repo-root  the consumer repo root that holds .assay-versions (default ".")
  --platform   override the artifact suffix (default: this binary's os-arch)

Exit: 0 pinned source agrees · 5 a check failed · 6 a precondition was unreadable.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "desksourceguard sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	fs := flag.NewFlagSet("desksourceguard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	var opts options
	fs.StringVar(&opts.source, "source", "", "materialised assay checkout")
	fs.StringVar(&opts.repoRoot, "repo-root", ".", "consumer repo root holding .assay-versions")
	fs.StringVar(&opts.platform, "platform", "", "artifact platform suffix (default: this binary's)")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitUnverifiable
	}
	if opts.source == "" {
		fmt.Fprint(stderr, usage)
		fmt.Fprintln(stderr, "desksourceguard: --source is required — there is nothing to verify without it")
		return deskkit.ExitUnverifiable
	}
	if opts.platform == "" {
		p, err := defaultPlatform()
		if err != nil {
			fmt.Fprintln(stderr, "desksourceguard: "+err.Error())
			return deskkit.ExitCodeOf(err)
		}
		opts.platform = p
	}

	var out strings.Builder
	err := verify(opts, deskkit.SourceSHA, &out)
	if err != nil {
		fmt.Fprintln(stderr, "desksourceguard: "+err.Error())
		return deskkit.ExitCodeOf(err)
	}
	fmt.Fprint(stdout, out.String())
	return deskkit.ExitOK
}
