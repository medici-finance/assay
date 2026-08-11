package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// stampKey identifies a specific cell in a stream README brief table.
type stampKey struct {
	stream string
	brief  string
	cell   string // "Verified" or "Reviewed"
}

// ambiguousStampName is the sentinel name used for a "human:" occurrence that
// the done-gate's hasHumanReviewer (brieffile.go) accepts but humanStampRe
// cannot parse into a name — e.g. "human: alice" (a space between the colon and
// the name splits it into two whitespace fields, so hasHumanReviewer's
// strings.Fields + HasPrefix(tok, "human:") is satisfied by the bare "human:"
// token alone). humanStampRe requires a name character immediately after the
// colon with no separating whitespace, so it does not see this at all — a
// one-character edit on a PR branch previously satisfied the done-gate while
// staying invisible to this control (assay-toolkit review, PR #324: "the gate
// can be made to accept a stamp the human did not write"). It cannot collide
// with a real captured name, since humanStampRe's capture group is
// [0-9A-Za-z_]+ (non-empty, no NUL).
const ambiguousStampName = "\x00ambiguous-human-token"

// looseHumanTokenRe finds a bare "human:" token — the same left-boundary rule
// as humanStampRe (a consumed non-identifier character or start-of-string),
// but requiring the colon to be followed by whitespace or end-of-string
// rather than a name character. This is exactly the token shape that
// hasHumanReviewer treats as satisfying "names a human": a whitespace-
// separated field equal to "human:" is enough for it, independent of whether
// a name follows in a later field. Mutually exclusive with humanStampRe:
// wherever humanStampRe matches, the colon is followed by a name character,
// not whitespace/end, so the two never double-count the same occurrence.
var looseHumanTokenRe = regexp.MustCompile(`(?:^|[^0-9A-Za-z_-])human:(?:\s|$)`)

// collectStamps runs both humanStampRe (the parseable "human:<name>" form)
// and looseHumanTokenRe (the ambiguous "human:" form that a whitespace split
// still counts as a stamp) over a cell value, and records both into dst under
// the given key. A stamp this gate cannot cleanly parse is recorded as a gain
// candidate rather than silently ignored — see ambiguousStampName.
func collectStamps(dst map[stampKey]map[string]bool, key stampKey, cellValue string) {
	for _, m := range humanStampRe.FindAllStringSubmatch(cellValue, -1) {
		if dst[key] == nil {
			dst[key] = map[string]bool{}
		}
		dst[key][strings.ToLower(m[1])] = true
	}
	if looseHumanTokenRe.MatchString(cellValue) {
		if dst[key] == nil {
			dst[key] = map[string]bool{}
		}
		dst[key][ambiguousStampName] = true
	}
}

func humanStampProblems(root string, streams []*Stream) (problems, notices []string) {
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		notices = append(notices, "human-stamp guard: .git absent — cannot resolve base; running degraded (no gain detection on this run)")
		return nil, notices
	}

	// Arming condition (A): git-derived. The rule is armed on branches
	// where HEAD != merge-base(HEAD, origin/main), and disarmed on main
	// where HEAD == merge-base (verify-gate-close and status-regen both
	// run on main and must not fire).
	base, resolved := registerLandedBase(root)
	if !resolved {
		// origin/main cannot be resolved (shallow clone, local, no remote).
		// Fail-open: emit a NOTICE and return no problems.
		notices = append(notices, "human-stamp guard is running degraded: origin/main could not be resolved, so stamps gained on a branch cannot be detected in this run (working-tree stamps against an uncommitted base). If this is CI, fetch origin/main before the lint step.")
		return nil, notices
	}

	// Resolve HEAD SHA to compare against the merge-base.
	headOut, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		// Cannot determine HEAD — emit NOTICE, don't silently skip.
		notices = append(notices, "human-stamp guard: cannot resolve HEAD; running degraded (no gain detection on this run)")
		return nil, notices
	}
	headSHA := strings.TrimSpace(string(headOut))
	if headSHA == base {
		// HEAD is the merge-base: we are on main. Rule is disarmed.
		return nil, nil
	}

	// Build stamp name sets for each stream README at the merge-base, then
	// compare against the working tree. Reuses splitFrontmatter +
	// parseBriefTable for the base parse — any divergence between the base
	// parser and the working-tree parser (extractStamps) is a false positive
	// or a miss.
	for _, s := range streams {
		rel := relPath(s.Dir) + "/README.md"
		baseRaw, baseErr := exec.Command("git", "-C", root, "show", base+":"+rel).Output()
		if baseErr != nil {
			// File did not exist at the merge-base. This could be a new stream,
			// or a renamed stream. For a new stream, any stamps are gains.
			// For renames, try git rename detection.
			if oldPath := detectRename(root, base, rel); oldPath != "" {
				baseRaw, baseErr = exec.Command("git", "-C", root, "show", base+":"+oldPath).Output()
			}
			if baseErr != nil {
				// Truly new: every stamp in this file is a gain.
				curStamps := extractStamps(s)
				problems = append(problems, checkGainedStamps(s, nil, curStamps)...)
				continue
			}
		}

		// Parse the base README to extract per-row stamp name sets.
		baseStamps := parseReadmeStamps(baseRaw)
		curStamps := extractStamps(s)
		problems = append(problems, checkGainedStamps(s, baseStamps, curStamps)...)
	}

	sort.Strings(problems)
	return problems, notices
}

// relPath returns the repo-relative path for a stream directory.
func relPath(dir string) string {
	// The dir is typically an absolute path like /repo/docs/streams/name.
	// Extract the docs/streams/name portion.
	if idx := strings.Index(dir, "docs/streams/"); idx >= 0 {
		return dir[idx:]
	}
	return dir
}

// detectRename returns the old path if the given file was renamed from an
// existing path at the merge-base (detected via git --find-renames).
// Returns "" if the file is truly new (not a rename).
func detectRename(root, base, newPath string) string {
	cmd := exec.Command("git", "-C", root, "diff", "--find-renames", "--name-status", base, "HEAD", "--", newPath)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// git diff --name-status with rename detection produces lines like:
	// R100\told/path/README.md\tnew/path/README.md
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "R") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				return parts[1] // old path
			}
		}
	}
	return ""
}

// extractStamps reads the working-tree stream README (already parsed into
// s.Briefs) and returns a map from stampKey to the set of normalized stamp
// names found in that cell. The name set supports cells with multiple
// human:<name> stamps; comparison against the base set catches stamp-swap
// and stamp-append evasions that a key-existence-only check would miss.
func extractStamps(s *Stream) map[stampKey]map[string]bool {
	result := map[stampKey]map[string]bool{}
	for _, b := range s.Briefs {
		for _, cell := range []struct{ label, value string }{
			{"Verified", b.Verified},
			{"Reviewed", b.Reviewed},
		} {
			if cell.value == "" {
				continue
			}
			collectStamps(result, stampKey{s.Name, b.Num, cell.label}, cell.value)
		}
	}
	return result
}

// parseReadmeStamps parses the base version of a README and returns a map of
// stamp name sets per stampKey. Reuses splitFrontmatter + parseBriefTable
// rather than maintaining a separate parser — any divergence between the two
// is a false positive or a miss, and both defects occurred in the original
// hand-rolled parser (wrong table selected via first-|---| latch; stream name
// captured from prose containing "stream:" rather than frontmatter).
func parseReadmeStamps(raw []byte) map[stampKey]map[string]bool {
	result := map[stampKey]map[string]bool{}
	fmRaw, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return result
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return result
	}
	briefs, err := parseBriefTable(body)
	if err != nil {
		return result
	}
	for _, b := range briefs {
		for _, cell := range []struct{ label, value string }{
			{"Verified", b.Verified},
			{"Reviewed", b.Reviewed},
		} {
			if cell.value == "" {
				continue
			}
			collectStamps(result, stampKey{fm.Stream, b.Num, cell.label}, cell.value)
		}
	}
	return result
}

// checkGainedStamps compares current working-tree stamp name sets against
// base stamp name sets. A PROBLEM is emitted for each stamp name present in
// the working tree that was NOT present at the merge-base for the same
// (stream, brief, cell). Comparing name sets rather than key existence
// catches stamp-swap (reviewer case 4b: human:alice → human:mallory) and
// stamp-append (4c: a second stamp added to an already-stamped cell), both of
// which a key-existence check misses.
//
// RESIDUAL — reviewer case 4 is NOT closed: re-dating an existing stamp
// (same name, new date) leaves the name set unchanged and stays silent, so a
// stale sign-off can still be made to look fresh on the PR path. The unit of
// comparison is deliberately the name and not the raw cell text, because a
// text comparison fires on ordinary re-flow (TestHumanStampCaseB_Reflow-
// DoesNotFire pins that as must-not-fire) and a false-positive class is what
// made the first cut of this gate unadoptable. TestHumanStamp_ReDateSameName-
// DoesNotFire pins the residual so it cannot regress silently.
func checkGainedStamps(s *Stream, baseStamps, curStamps map[stampKey]map[string]bool) []string {
	var problems []string
	for key, curNames := range curStamps {
		baseNames := baseStamps[key]
		for name := range curNames {
			if baseNames[name] {
				continue // this name was already present at the base
			}
			if name == ambiguousStampName {
				problems = append(problems, fmt.Sprintf(
					"%s/brief-%s: %s cell gained an ambiguous human: token "+
						"(a bare \"human:\" with no name character immediately "+
						"after the colon, e.g. \"human: name\" with a separating "+
						"space) — this satisfies brieffile.go's hasHumanReviewer "+
						"done-gate check (a whitespace-split \"human:\" token) but "+
						"is not a stamp verify-gate-close.yml writes and cannot be "+
						"parsed to a name. Treated as an unauthorized human-stamp "+
						"gain: write the canonical form human:<name> with no space.",
					s.Name, key.brief, key.cell))
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s/brief-%s: %s cell gained human:%s — "+
					"verify-gate-close.yml is the sole permitted writer of "+
					"human:<name> sign-off stamps in stream-README "+
					"Verified/Reviewed cells, running only when the "+
					"allowlisted human reviewer closes a verify-gate issue in "+
					"the writer workflow's home repo. This stamp was "+
					"added via a PR branch or other path the --lint gate "+
					"checks and is rejected. The control is structural "+
					"(branch-vs-main): the gate constrains WHERE a stamp "+
					"may be written, not who may write it.",
				s.Name, key.brief, key.cell, name))
		}
	}
	return problems
}
