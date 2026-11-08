package main

// evidenceverdict.go — `statusgen evidenceverdict`: may this run's verdict be
// appended to this brief's Evidence?
//
// A re-run of a brief that is already `verified` or `done` can come back BLOCKED
// or could-not-check for reasons outside the brief (a runner image, a missing
// tool). Appended to Evidence, that run becomes the latest recorded verdict, and
// the protected-closure lint (protectedClosureProblems → flipLatestPass) then
// refuses the tree: "the latest recorded verdict is VERIFY: BLOCKED, not PASS".
// The lint is right. This subcommand lets the landing verb refuse the append
// BEFORE it is written, so a verdict the lint will refuse is never landable.
//
// The verdict is read with latestVerdictIndex over flipVerdictTokens — the same
// selector flipLatestPass uses — so "the latest recorded verdict" means the same
// thing here and in the lint. The status is the brief's board row.
//
// Decision (one brief, one tree). The rule is a whitelist: on a verified or
// done brief, every verdict other than PASS and FAIL is refused — BLOCKED,
// NEEDS-CONTEXT, PARTIAL and HELD alike, exactly the verdicts the lint refuses.
//
//   - the brief's status cannot be resolved → could-not-check (exit 2): no
//     brief file at --brief-path under --root, a file name whose brief id does
//     not parse, or a stream board with no row for that id. None of these is
//     "the status is not verified", so none is rounded to landable;
//   - status neither verified nor done → landable (an implemented brief's
//     BLOCKED run lands as it always has);
//   - the latest verdict is PASS or FAIL → landable (a fresh PASS, or a
//     reproducing FAIL, which is a real regression signal);
//   - the latest verdict is unchanged by this landing (same word, same count
//     as --before) → landable: this landing records no new verdict;
//   - otherwise → NOT landable (exit 1): any verdict other than PASS or
//     FAIL belongs in the outcome record only.
//
// --verdict asks the same question for a verdict word with no Evidence text
// (the outcome-only record's check in deskevidence): exit 1 means an Evidence
// append of that verdict would be refused, so its outcome record may land
// without one.
//
// THREE-STATE (common-clause C4): an unreadable board, file or Evidence, and a
// status that cannot be resolved, is could-not-check (exit 2), never rounded to
// either answer. boardStatusForBriefPath returns an error, never a "not found"
// flag, so no caller can read an unresolved status as a status.

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	evidenceverdictExitLandable      = 0
	evidenceverdictExitNotLandable   = 1
	evidenceverdictExitCouldNotCheck = 2
)

const evidenceverdictUsage = `statusgen evidenceverdict — may this run's verdict be appended to a brief's Evidence?

Usage:
  statusgen evidenceverdict --brief-path <repo-relative path> (--after <file> | --verdict <WORD>) [--before <file>] [--root <dir>]

Reads the board and the given brief text; runs nothing and writes nothing. Exit 1 when the
brief's board status is verified or done and the run's verdict (the latest recorded verdict
in --after's Evidence, or --verdict) is any verdict other than PASS or FAIL: such a run
belongs in the outcome record only. Exit 0 otherwise. Exit 2 when the board or the brief
text cannot be read, or the brief's board status cannot be resolved (no brief file at the
path, an id that does not parse, no board row for it).

Flags:
  --brief-path <path>  the brief file, relative to --root (required)
  --after <file>       the brief file as it would land
  --verdict <WORD>     the run's verdict word, in place of --after
  --before <file>      the brief file as it stands before this landing (optional)
  --root <dir>         repo root to read (default: the cwd)
`

func runEvidenceverdict(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("evidenceverdict", flag.ContinueOnError)
	defer transitionFlags(fs)()
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	briefPath := fs.String("brief-path", "", "repo-relative brief file")
	afterFile := fs.String("after", "", "the brief file as it would land")
	verdictWord := fs.String("verdict", "", "the run's verdict word")
	beforeFile := fs.String("before", "", "the brief file before this landing")
	rootDir := fs.String("root", "", "repo root to read (default: cwd)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, evidenceverdictUsage)
			return evidenceverdictExitLandable
		}
		fmt.Fprint(stderr, evidenceverdictUsage)
		return evidenceverdictExitCouldNotCheck
	}
	rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(*briefPath)))
	if *briefPath == "" || (*afterFile == "") == (strings.TrimSpace(*verdictWord) == "") || fs.NArg() > 0 {
		fmt.Fprint(stderr, evidenceverdictUsage)
		return evidenceverdictExitCouldNotCheck
	}
	root := *rootDir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen evidenceverdict: could-not-check — cannot resolve %s: %v\n", root, err)
		return evidenceverdictExitCouldNotCheck
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen evidenceverdict: could-not-check — cannot read the board at %s: %v\n", root, err)
		return evidenceverdictExitCouldNotCheck
	}
	key, status, err := boardStatusForBriefPath(streams, absRoot, rel)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen evidenceverdict: could-not-check — %v\n", err)
		return evidenceverdictExitCouldNotCheck
	}
	if status != "verified" && status != "done" {
		fmt.Fprintf(stdout, "%s: landable — status %q\n", key, status)
		return evidenceverdictExitLandable
	}

	word := strings.ToUpper(strings.TrimSpace(*verdictWord))
	if *afterFile != "" {
		w, count, rerr := evidenceVerdictOfFile(*afterFile)
		if rerr != nil {
			fmt.Fprintf(stderr, "statusgen evidenceverdict: could-not-check — %s: %v\n", *afterFile, rerr)
			return evidenceverdictExitCouldNotCheck
		}
		if w == "" {
			fmt.Fprintf(stdout, "%s: landable — Evidence records no verdict\n", key)
			return evidenceverdictExitLandable
		}
		if *beforeFile != "" {
			bw, bcount, berr := evidenceVerdictOfFile(*beforeFile)
			if berr != nil {
				fmt.Fprintf(stderr, "statusgen evidenceverdict: could-not-check — %s: %v\n", *beforeFile, berr)
				return evidenceverdictExitCouldNotCheck
			}
			if bw == w && bcount == count {
				fmt.Fprintf(stdout, "%s: landable — this landing records no new verdict\n", key)
				return evidenceverdictExitLandable
			}
		}
		word = w
	}
	if word == "PASS" || word == "FAIL" {
		fmt.Fprintf(stdout, "%s: landable — status %q, verdict VERIFY: %s\n", key, status, word)
		return evidenceverdictExitLandable
	}
	fmt.Fprintf(stdout, "%s: NOT landable — status %q, this run's verdict is VERIFY: %s; a re-run of a verified or done brief whose verdict is any verdict other than PASS or FAIL is recorded in the outcome record only\n",
		key, status, word)
	return evidenceverdictExitNotLandable
}

// boardStatusForBriefPath finds the board row whose brief file is rel (relative
// to absRoot) and returns its key and lower-cased Status. Every way the status
// can go unresolved is an error, so a caller cannot round it to a status.
func boardStatusForBriefPath(streams []*Stream, absRoot, rel string) (key, status string, err error) {
	for _, s := range streams {
		for _, p := range briefFilePaths(s) {
			r, rerr := filepath.Rel(realPath(absRoot), realPath(p))
			if rerr != nil || filepath.ToSlash(r) != rel {
				continue
			}
			_, num, ok := expectedBriefID(p)
			if !ok {
				return "", "", fmt.Errorf("%s: the brief id does not parse from its file name, so its board status cannot be resolved", rel)
			}
			for _, b := range s.Briefs {
				if b.Num == num {
					return s.Name + "/" + num, strings.ToLower(strings.TrimSpace(b.Status)), nil
				}
			}
			return "", "", fmt.Errorf("%s: stream %s has no board row for brief %s, so its status cannot be resolved", rel, s.Name, num)
		}
	}
	return "", "", fmt.Errorf("%s: no brief file at this path under %s, so its board status cannot be resolved", rel, absRoot)
}

// realPath is the absolute, symlink-resolved form of p (p's absolute form when
// it cannot be resolved), so a root spelled through a symlink still matches.
func realPath(p string) string {
	if ap, err := filepath.Abs(p); err == nil {
		p = ap
	}
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}

// evidenceVerdictOfFile reads a brief file and returns its Evidence section's
// latest recorded verdict and verdict count, as latestEvidenceVerdict reads them.
func evidenceVerdictOfFile(path string) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	f.Close()
	return latestEvidenceVerdict(extractEvidence(briefBody(path)))
}
