package main

// verifyclosure.go — `statusgen verifyclosure`: does this tree present a
// lint-valid `verified` closure for one brief?
//
// THE HOLE IT CLOSES. verify-desk appends `docs/streams/verify-outcomes.jsonl`
// with `"outcome":"verified"` on a PASS run — the row the Change-Failure-Rate
// denominator counts. But a sidecar row is written from the desk's own view of
// the run, not from the tree's board state. When Evidence is filled while the
// stream table STAYS at `implemented` (the Verified cell still `—`, because the
// board was never flipped, or a witness the flip needs is absent), the sidecar
// still records `verified`. `verifyloop` then buckets the mismatch as a
// STUCK-FLIP (verified sidecar + filled Evidence + status still implemented),
// which review correctly refuses to merge.
//
// This subcommand answers, for ONE brief and ONE tree, the exact question the
// sidecar-writer should have asked BEFORE recording `verified`: would a
// `verified` closure be lint-valid here? It is a READ-ONLY board question — it
// runs nothing, writes nothing — so unlike `verifyrun` it has no business
// executing Verify rows and is never part of `--lint`.
//
// ACCEPTANCE is owned by checkSelectedClosure, the same per-brief validator
// brief --check-verified uses: verified/done status, dated stamp, non-empty
// applicable Verify rows, passing current witnesses, a human stamp for gate:human
// rows, and current fail-first evidence for risk-bearing rows. The fail-first
// policy is also the one corpus lint calls, with its own transition scope.
// Attribution and landing authorization are separate caller responsibilities.
//
// THREE-STATE (common-clause C4). A brief whose file or board row cannot be read
// is could-not-check (exit 2), never rounded to accepted and never to refused:
// the caller must not silently record `verified` for a brief it could not
// evaluate.

import (
	"errors"
	"flag"
	"fmt"
	"github.com/medici-finance/assay/statusgen/evaluation"
	"os"
	"strings"
)

const (
	// verifyclosureExitAccepted: the tree presents a lint-valid `verified`
	// closure for the brief — stamp present AND every Verify row witnessed.
	verifyclosureExitAccepted = 0
	// verifyclosureExitNotAccepted: a well-formed tree that does NOT accept a
	// `verified` closure (the stuck-flip shape: no stamp, or an unwitnessed /
	// failing Verify row). This is the signal to REFUSE a `verified` sidecar row.
	verifyclosureExitNotAccepted = 1
	// verifyclosureExitCouldNotCheck: the brief/board could not be evaluated
	// (unreadable root, brief not on any board, usage error). Never rounded to
	// accepted or refused.
	verifyclosureExitCouldNotCheck = 2
)

const verifyclosureUsage = `statusgen verifyclosure — is a lint-valid ` + "`verified`" + ` closure present for one brief?

Usage:
  statusgen verifyclosure --brief <stream>/<NN> [--root <dir>]

Reads the board, brief and local Git provenance; runs no Verify commands and writes nothing. Exit 0 when the
tree accepts a ` + "`verified`" + ` closure for the brief (the Verified stamp is present AND
every Verify row carries a passing execution witness), exit 1 when it does not
(the stuck-flip shape a ` + "`verified`" + ` sidecar row must not record), exit 2 when the
brief or board could not be read (could-not-check).

Flags:
  --brief <stream>/<NN>   the brief key, e.g. desk-tools/14 (required)
  --root <dir>            repo root to read (default: the cwd)
`

// runVerifyclosure is the `statusgen verifyclosure` entry point. args excludes
// the "verifyclosure" token main() already stripped.
func runVerifyclosure(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("verifyclosure", flag.ContinueOnError)
	defer transitionFlags(fs)()
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	briefKey := fs.String("brief", "", "brief key <stream>/<NN> to evaluate")
	rootDir := fs.String("root", "", "repo root to read (default: cwd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, verifyclosureUsage)
			return verifyclosureExitAccepted
		}
		fmt.Fprint(stderr, verifyclosureUsage)
		return verifyclosureExitCouldNotCheck
	}

	key := strings.TrimSpace(*briefKey)
	if key == "" {
		fmt.Fprint(stderr, verifyclosureUsage)
		return verifyclosureExitCouldNotCheck
	}
	stream, num, ok := splitBriefKey(key)
	if !ok {
		fmt.Fprintf(stderr, "statusgen verifyclosure: --brief must be <stream>/<NN>, got %q\n", key)
		return verifyclosureExitCouldNotCheck
	}

	root := *rootDir
	if root == "" {
		root = "."
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		// The board could not be read at all — could-not-check, never a verdict
		// on a tree we could not evaluate.
		fmt.Fprintf(stderr, "statusgen verifyclosure: could-not-check — cannot read the board at %s: %v\n", root, err)
		return verifyclosureExitCouldNotCheck
	}

	s, br := findBriefRow(streams, stream, num)
	if s == nil || br == nil {
		fmt.Fprintf(stderr, "statusgen verifyclosure: could-not-check — brief %s is not on any stream board under %s (nothing records its Status)\n", key, root)
		return verifyclosureExitCouldNotCheck
	}

	art, ok := loadBriefArtifacts(s, num)
	if !ok {
		fmt.Fprintf(stderr, "statusgen verifyclosure: could-not-check — cannot read the brief file for %s under %s\n", key, root)
		return verifyclosureExitCouldNotCheck
	}
	findings, err := checkSelectedClosure(root, key, br.Status, br.Verified, art)
	if err != nil {
		var structural *evaluation.ClosureStructureError
		if errors.As(err, &structural) {
			fmt.Fprintf(stdout, "%s: NOT accepted — %s\n", key, structural.Detail)
		} else {
			fmt.Fprintf(stdout, "%s: NOT accepted — %v\n", key, err)
		}
		return verifyclosureExitNotAccepted
	}

	passed, supplemental := 0, 0
	for _, f := range findings {
		if f.State == statePass {
			passed++
		} else if f.State == stateAuthorAdded {
			supplemental++
		}
	}
	fmt.Fprintf(stdout, "%s: accepted — Status %q, Verified cell stamped, %d passing acceptance row(s), %d author-added row(s) with no acceptance credit\n",
		key, br.Status, passed, supplemental)
	return verifyclosureExitAccepted
}

// splitBriefKey splits a "<stream>/<NN>" brief key into its stream name and
// brief number. ok is false for anything not shaped like exactly one "/"
// separating two non-empty halves.
func splitBriefKey(key string) (stream, num string, ok bool) {
	i := strings.LastIndex(key, "/")
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	stream = strings.TrimSpace(key[:i])
	num = strings.TrimSpace(key[i+1:])
	if stream == "" || num == "" {
		return "", "", false
	}
	return stream, num, true
}

// findBriefRow locates the stream + register row for a brief key. It matches on
// the stream's canonical Name (the frontmatter `stream:` value the sidecar key is
// built from) and, as a fallback, the stream directory's base name, so a board
// whose Name and directory differ still resolves. Returns (nil, nil) when no such
// row exists.
func findBriefRow(streams []*Stream, stream, num string) (*Stream, *Brief) {
	for _, s := range streams {
		if s.Name != stream && baseName(s.Dir) != stream {
			continue
		}
		for i := range s.Briefs {
			if s.Briefs[i].Num == num {
				return s, &s.Briefs[i]
			}
		}
	}
	return nil, nil
}

// baseName returns the final path element of dir, matching the stream directory's
// own name without pulling in path/filepath at every call site.
func baseName(dir string) string {
	dir = strings.TrimRight(dir, "/")
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		return dir[i+1:]
	}
	return dir
}
