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
// ACCEPTANCE — two independent halves, BOTH required (the same pair the issue
// names: "Verified stamp + verifyrun witness present"):
//
//  1. THE STAMP. The brief's stream-README row is at Status `verified` or
//     `done`, and its Verified cell carries a dated runner (verifiedCellRe).
//     A brief still at `implemented` fails here — that is the stuck-flip shape.
//
//  2. THE WITNESS. Every Verify row the brief carries has a matching, passing
//     EXECUTION WITNESS in its Evidence (checkWitnesses / checkExitCode, the
//     same audit `verifyrun --check` runs). Filled-but-unwitnessed Evidence
//     fails here. A brief with NO Verify rows has nothing to witness and clears
//     this half on the stamp alone.
//
// Both halves reuse the existing acceptance code — the register Status/Verified
// cells and verifiedCellRe (the stamp), checkWitnesses/checkExitCode (the
// witness) — so the criteria live in ONE place and this command can never drift
// from what the board and the witness audit already mean.
//
// THREE-STATE (common-clause C4). A brief whose file or board row cannot be read
// is could-not-check (exit 2), never rounded to accepted and never to refused:
// the caller must not silently record `verified` for a brief it could not
// evaluate.

import (
	"errors"
	"flag"
	"fmt"
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

Reads ONLY the board and the brief; runs nothing, writes nothing. Exit 0 when the
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

	// Half 1 — the stamp. A brief still at implemented (or any non-closed state),
	// or one at verified/done with no dated runner in the Verified cell, does not
	// present a lint-valid closure. This is the stuck-flip discriminator.
	if br.Status != "verified" && br.Status != "done" {
		fmt.Fprintf(stdout, "%s: NOT accepted — Status is %q, not verified/done — a `verified` sidecar row for a brief the board still calls %q is the stuck-flip mismatch (#1309)\n", key, br.Status, br.Status)
		return verifyclosureExitNotAccepted
	}
	if !verifiedCellRe.MatchString(br.Verified) {
		fmt.Fprintf(stdout, "%s: NOT accepted — Status is %q but the Verified cell %q carries no dated runner stamp (YYYY-MM-DD <runner>)\n", key, br.Status, br.Verified)
		return verifyclosureExitNotAccepted
	}

	// Half 2 — the witness. Every Verify row must carry a matching, passing
	// execution witness. A brief with no Verify rows has nothing to witness and
	// clears this half on the stamp alone.
	art, ok := loadBriefArtifacts(s, num)
	if !ok {
		fmt.Fprintf(stderr, "statusgen verifyclosure: could-not-check — cannot read the brief file for %s under %s\n", key, root)
		return verifyclosureExitCouldNotCheck
	}
	findings := checkWitnesses(art.Verify, art.Evidence)
	if len(findings) == 0 {
		// No Verify rows: nothing to witness. The stamp already passed.
		fmt.Fprintf(stdout, "%s: accepted — Status %q with a stamped Verified cell; no Verify rows to witness\n", key, br.Status)
		return verifyclosureExitAccepted
	}
	if code := checkExitCode(findings); code != verifyrunExitPass {
		var missing []string
		for _, f := range findings {
			if f.State != statePass {
				missing = append(missing, fmt.Sprintf("row %s: %s", f.ID, f.State))
			}
		}
		fmt.Fprintf(stdout, "%s: NOT accepted — Status %q but %d Verify row(s) lack a passing execution witness (%s) — `statusgen verifyrun --brief <path>` writes one\n",
			key, br.Status, len(missing), strings.Join(missing, "; "))
		return verifyclosureExitNotAccepted
	}

	fmt.Fprintf(stdout, "%s: accepted — Status %q, Verified cell stamped, and all %d Verify row(s) carry a passing execution witness\n",
		key, br.Status, len(findings))
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
