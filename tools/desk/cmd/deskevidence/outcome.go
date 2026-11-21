package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var outcomeGuardFn = guardVerifiedOutcomes
var closureCheckFn = checkVerifiedClosure

// Check only added records: historical successes must not prevent recording a
// later failure. Reuse the canonical brief parser and witness validator.
func guardVerifiedOutcomes(root, landedRoot, target string, before, after []byte, fg deskkit.Forge, repo deskkit.ForgeRepo, branch string, options childOptions) error {
	if !isVerifyOutcomesSidecar(target) {
		return nil
	}
	seen := map[string]bool{}
	retained := map[string]int{}
	for _, line := range bytes.Split(before, []byte("\n")) {
		retained[string(line)]++
	}
	for _, line := range bytes.Split(after, []byte("\n")) {
		if retained[string(line)] > 0 {
			retained[string(line)]--
			continue
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Brief   string `json:"brief"`
			Outcome string `json:"outcome"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return deskkit.Refused("invalid verify-outcomes JSON record")
		}
		if evaluation.ParseVerificationOutcome(row.Outcome) != evaluation.OutcomeVerified || seen[row.Brief] {
			continue
		}
		seen[row.Brief] = true
		file, err := closureCheckFn(landedRoot, row.Brief, options)
		if err != nil {
			return err
		}
		// A locally staged closure is insufficient for this single-file write.
		// Both the board row and its evidence must already be on the target branch.
		for _, rel := range []string{file, path.Join(path.Dir(file), "README.md")} {
			local, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return deskkit.Unverifiable("cannot read closure source "+rel, err)
			}
			remote, err := fg.ReadFile(repo, deskkit.ReadFileInput{File: rel, Ref: branch})
			if err != nil {
				return err
			}
			if kerr := plainFileRead(remote, repo, rel, branch, "the verified outcome's landed closure", true); kerr != nil {
				return kerr
			}
			if !remote.Exists || !bytes.Equal(local, remote.Content) {
				return deskkit.Refused("verified outcome requires closure already landed on target branch: " + rel + " differs; refresh --root after landing the status and Evidence")
			}
		}
	}
	if len(seen) > 0 {
		problems, err := statusgenLintFn(landedRoot, options)
		if err != nil {
			return err
		}
		own, foreign := splitClosureProblems(problems, seen)
		for _, p := range foreign {
			fmt.Fprintf(stderr, "deskevidence: notice: %s — names a brief this record does not close; it is that brief's to fix, not this outcome's to block\n", p)
		}
		if len(own) > 0 {
			return deskkit.Refused("verified outcome requires a lint-valid closure tree:\n" + strings.Join(own, "\n"))
		}
	}
	return nil
}

// problemKeyRe is the `<stream>/<NN>` brief key statusgen leads a brief's PROBLEM with.
var problemKeyRe = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)/0*([0-9]+[a-z]?)$`)

// briefKey normalises a `<stream>/<NN>` key (leading zeros dropped, so a board's
// "07" and a file's "7" agree), or returns "" when k is not one. The one parser
// the closure gate and the landing and flip diffs (lintdiff.go) attribute by.
func briefKey(k string) string {
	m := problemKeyRe.FindStringSubmatch(strings.TrimSpace(k))
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// problemBriefKey is the normalised brief key a PROBLEM line leads with, and the
// text after it; key is "" when the leading segment is not a brief key (a path,
// a stream, a register, a summary line).
func problemBriefKey(p string) (key, rest string) {
	lead, rest, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(p, "PROBLEM:")), ":")
	return briefKey(lead), strings.TrimSpace(rest)
}

// splitClosureProblems separates the lint PROBLEMs that name a brief this record
// does not close (foreign) from the rest (own). The closure gate refuses on own
// only, the same scoping the landing and flip gates apply through their
// before/after diff (#4823). It fails closed: a PROBLEM naming a closed brief,
// and any PROBLEM whose leading segment is not a parseable brief key (a path, a
// stream, a register, a summary line), stays own and still refuses.
func splitClosureProblems(problems []string, closed map[string]bool) (own, foreign []string) {
	closedKeys := map[string]bool{}
	for k := range closed {
		if key := briefKey(k); key != "" {
			closedKeys[key] = true
		}
	}
	for _, p := range problems {
		key, _ := problemBriefKey(p)
		if key == "" || closedKeys[key] {
			own = append(own, p)
			continue
		}
		foreign = append(foreign, p)
	}
	return own, foreign
}

func checkVerifiedClosure(root, key string, options childOptions) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := statusgenAudit(ctx, auditBrief, root, options, "--json", "--check-verified", "--check-attribution", key)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return "", deskkit.Refused("verified outcome refused: " + strings.TrimSpace(stderr.String()))
		}
		return "", deskkit.Unverifiable("cannot validate verified closure for "+key+": "+strings.TrimSpace(stderr.String()), err)
	}
	var info struct {
		Key  string `json:"key"`
		File string `json:"file"`
	}
	if err := json.Unmarshal(out, &info); err != nil || info.Key != key {
		return "", deskkit.Unverifiable("invalid statusgen brief response for "+key, err)
	}
	file := info.File
	if !strings.HasPrefix(file, "docs/streams/") || path.Clean(file) != file || strings.Contains(file, "\\") {
		return "", deskkit.Unverifiable(fmt.Sprintf("invalid closure source path %q", file), nil)
	}
	return file, nil
}
