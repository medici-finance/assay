package main

// The release-time spec header check, ported from tools/release/check-spec-header-version.sh
// (now an adapter onto `deskchangelog check-spec-header`): the canonical spec's
// 'Describes reference implementation:' line must name the tag the release is cutting.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const specHeaderMarker = "Describes reference implementation:"

var (
	releaseTagRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	versionTokRe = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
)

func runSpecHeader(tag, file string, stdout, stderr io.Writer) int {
	if !releaseTagRe.MatchString(tag) {
		fmt.Fprintf(stderr, "FAIL usage: tag must be vMAJOR.MINOR.PATCH (got '%s')\n", tag)
		return 2
	}
	b, err := os.ReadFile(file)
	if err != nil {
		if st, serr := os.Stat(file); serr == nil && st.IsDir() {
			// A readable directory passes the script's readability test and then holds no
			// header line.
			b = nil
		} else {
			fmt.Fprintf(stderr, "FAIL could-not-check: cannot read %s\n", file)
			return 2
		}
	}
	line := ""
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.Contains(ln, specHeaderMarker) {
			line = ln
			break
		}
	}
	if line == "" {
		fmt.Fprintf(stderr, "FAIL could-not-check: %s has no 'Describes reference implementation:' line\n", file)
		return 2
	}
	have := versionTokRe.FindString(line)
	if have == "" {
		fmt.Fprintf(stderr, "FAIL could-not-check: could not parse a vX.Y.Z token out of: %s\n", line)
		return 2
	}
	if have != tag {
		fmt.Fprintf(stderr, "FAIL %s names statusgen %s, but this release is cutting %s — run tools/release/stamp-spec-header-version.sh %s (the release does this on dispatch), or update the 'Describes reference implementation:' line in %s to %s, before releasing (brief-13 task 4, assay#1192)\n", file, have, tag, tag, file, tag)
		return 1
	}
	fmt.Fprintf(stdout, "ok   %s names statusgen %s, matches %s\n", file, have, tag)
	return 0
}

func specHeaderCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "check-spec-header <vX.Y.Z> [spec-file]",
		Short: "check the spec header names the release tag being cut",
		Long: `Checks that [spec-file] (default spec/brief-v1.md, relative to the working
directory) carries a 'Describes reference implementation:' line naming <vX.Y.Z>,
the tag the release is cutting. Fails closed: an unreadable file, a missing line or
an unparseable version is could-not-check, never a pass.

Exit codes: 0 match · 1 the file names a different version · 2 usage, a malformed
tag, or could-not-check.`,
		Args: argsBetween(1, 2),
		RunE: func(_ *cobra.Command, a []string) error {
			file := "spec/brief-v1.md"
			if len(a) > 1 && a[1] != "" {
				file = a[1]
			}
			return codeErr(runSpecHeader(a[0], file, stdout, stderr))
		},
	}
}
