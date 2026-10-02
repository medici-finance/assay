package main

// The register-transition lane of `statusgen --corroborate` (statusgen/06, §B):
// the ONLINE half of the findings-register state-machine guard.
//
// The offline --lint gate (guttedRegisterFieldsEntries) detects a finding whose
// resolved/affects/ack/parked-until moved in the caution-removing direction since
// the merge-base, and accepts it when the entry carries a MAPPED human name under
// its authorizing key. That gate cannot read the PR, so on its own the key is one
// line an agent can write for itself — or, worse, one already sitting on the entry
// from an earlier, legitimate change: an entry that once carried
// `authorized-by: human:<name>` would authorize every later gutting of it, and the
// stamp lane (which reads only ADDED diff lines) would see no stamp to check.
//
// This lane closes both: it re-derives the SAME transitions (one detector,
// registerFieldTransitions, for both halves) against the PR merge-base and
// requires, per transition category, that at least one human named in that
// category's authorizing key ACTED on the PR — an APPROVED review or an explicit
// approval comment from the mapped account, the same two PR anchors the stamp lane
// uses (corroborateStampsRuled). It fails CLOSED: a register-touching PR whose
// merge-base cannot be resolved, or whose touched finding cannot be parsed, is
// MISSING-CORROBORATION, never a pass.
//
// Whether the PR touches the register at all is decided from the LOCAL tree
// against the resolved merge-base, never from the forge file listing alone: that
// listing stops at 3000 files with no error, so a PR padded with paths sorting
// ahead of docs/streams/findings/ would read as untouched. The listing is only
// unioned in. When the merge-base cannot be resolved the listing is the only
// source left, and it is trusted to say "untouched" only when its length equals
// the forge's own changed_files count; anything else fails closed.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// findingsRegisterDir is the repo-relative home of the per-entry findings files.
const findingsRegisterDir = "docs/streams/findings/"

// registerTransitionResult is the lane's verdict for one finding (or, for a
// fail-closed could-not-evaluate, for the PR as a whole when rel is empty).
type registerTransitionResult struct {
	Rel      string
	Moves    string
	Verdict  verdict
	Evidence string
}

// touchedFindings returns the findings-register entry files (.md directly under
// docs/streams/findings/) a PR touches, read from the structured PR file list —
// both the current and, for a rename, the previous name, so an entry renamed away
// still counts. It reads file NAMES, never diff lines: a removal-only change (an
// `ack:` line deleted, an affects entry dropped) adds no line but is still a touch.
func touchedFindings(files []ghPRFile) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		for _, p := range []string{f.Filename, f.PreviousFilename} {
			if p == "" || seen[p] || !isFindingsRegisterEntry(p) {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// isFindingsRegisterEntry reports whether a repo-relative path is a findings
// register entry file — an .md directly under docs/streams/findings/ (a nested
// file is not one; the register loader never reads it).
func isFindingsRegisterEntry(p string) bool {
	if !strings.HasPrefix(p, findingsRegisterDir) || !strings.HasSuffix(p, ".md") {
		return false
	}
	return !strings.Contains(strings.TrimPrefix(p, findingsRegisterDir), "/")
}

// registerAuthorityNames returns the lower-cased human:<name> names carried under
// the given frontmatter keys of a finding entry, on-behalf-of relays excluded (a
// relay is attribution, never the human's own authorization — the same exclusion
// authorizedByVerifiedHuman applies). Unparseable frontmatter, a missing key or a
// non-scalar value contributes no name: the fail-closed direction, since a
// transition with no named authority can corroborate nothing.
func registerAuthorityNames(raw []byte, keys ...string) []string {
	fm, _, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil
	}
	var kv map[string]any
	if err := yaml.Unmarshal([]byte(fm), &kv); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range keys {
		s, ok := kv[k].(string)
		if !ok {
			continue
		}
		for _, m := range humanStampRe.FindAllStringSubmatch(withoutOnBehalfOfRelays(s), -1) {
			n := strings.ToLower(m[1])
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// corroborateRegisterTransitions judges each transition against the PR's reviews
// and comments. A finding's resolve/affects/ack moves need a human named under
// `authorized-by:`; its park add/extend needs one named under `parked-by:` or
// `authorized-by:` — the same key sets the offline gate accepts, so the two halves
// agree on WHO may authorize and differ only in that this half requires the person
// to have acted on THIS PR. Each category is judged on its own; a finding passes
// only when every category it moved is corroborated.
func corroborateRegisterTransitions(ts []registerTransition, data *ghPRData, repo string, pr int) []registerTransitionResult {
	var out []registerTransitionResult
	for _, tr := range ts {
		type category struct {
			moves []string
			keys  []string
		}
		cats := []category{
			{tr.guts, []string{"authorized-by"}},
			{tr.parkGuts, []string{"parked-by", "authorized-by"}},
		}
		var allMoves, evidence, missing []string
		for _, c := range cats {
			if len(c.moves) == 0 {
				continue
			}
			allMoves = append(allMoves, c.moves...)
			names := registerAuthorityNames(tr.curRaw, c.keys...)
			if len(names) == 0 {
				missing = append(missing, fmt.Sprintf("%s: no human:<name> under %s — nobody authorized it",
					strings.Join(c.moves, "; "), strings.Join(c.keys, "/")))
				continue
			}
			var ok bool
			var why []string
			for _, n := range names {
				res := corroborateStampsRuled([]stamp{{Name: n, File: tr.rel, Unresolved: true}}, data, repo, pr, nil, nil)
				if len(res) == 1 && res[0].Verdict == verdictCorroborated {
					evidence = append(evidence, fmt.Sprintf("human:%s — %s", n, res[0].Evidence))
					ok = true
					break
				}
				if len(res) == 1 {
					why = append(why, fmt.Sprintf("human:%s: %s", n, res[0].Evidence))
				}
			}
			if !ok {
				missing = append(missing, fmt.Sprintf("%s: %s", strings.Join(c.moves, "; "), strings.Join(why, "; ")))
			}
		}
		r := registerTransitionResult{Rel: tr.rel, Moves: strings.Join(allMoves, "; ")}
		if len(missing) > 0 {
			r.Verdict = verdictMissing
			r.Evidence = strings.Join(missing, " | ")
		} else {
			r.Verdict = verdictCorroborated
			r.Evidence = strings.Join(evidence, " | ")
		}
		out = append(out, r)
	}
	return out
}

// localTouchedFindings returns the findings-register entry files that differ
// between base and the working tree at root — renames split into their delete and
// add halves, so both names count. It reads git, not the forge, so it is complete
// however many files the PR changes.
func localTouchedFindings(root, base string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", "--no-renames", "-z",
		base, "--", findingsRegisterDir).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s -- %s: %w", base, findingsRegisterDir, err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if isFindingsRegisterEntry(p) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// registerTransitionLane runs the lane for one PR. root is the checkout (the PR
// head / merge commit in CI), files the PR's forge file list, mergeBase the PR
// merge-base ("" = unresolvable), changedFiles the forge's changed-file count
// (read only when the merge-base is unresolvable and the listing names no
// entry), and fetchData the PR's reviews and comments (called at most once, and
// only when there is a transition to judge). It returns nil when the PR touches
// no findings entry — the lane has nothing to say.
func registerTransitionLane(root, repo string, pr int, files []ghPRFile, mergeBase string,
	changedFiles func() (int, error), fetchData func() (*ghPRData, error)) ([]registerTransitionResult, error) {
	touched := touchedFindings(files)
	if mergeBase == "" {
		if len(touched) > 0 {
			return []registerTransitionResult{{
				Verdict: verdictMissing,
				Moves:   "touches " + strings.Join(touched, ", "),
				Evidence: "the PR merge-base could not be resolved, so the findings-register transitions this PR makes " +
					"cannot be evaluated — fail-closed (statusgen/06 §B); fetch the base branch (fetch-depth: 0) and re-run",
			}}, nil
		}
		// No merge-base, so the forge listing is the only witness that the PR
		// touches no entry — and it is one only when shown complete.
		n, err := changedFiles()
		if err != nil {
			return []registerTransitionResult{{
				Verdict: verdictMissing,
				Moves:   "register touch unknown",
				Evidence: fmt.Sprintf("the PR merge-base could not be resolved and the forge's changed-file count could not be read (%v), "+
					"so a file listing that names no findings entry cannot be shown complete — fail-closed (statusgen/06 §B)", err),
			}}, nil
		}
		if n != len(files) {
			return []registerTransitionResult{{
				Verdict: verdictMissing,
				Moves:   "register touch unknown",
				Evidence: fmt.Sprintf("the PR merge-base could not be resolved and the forge listed %d of the PR's %d changed files, "+
					"so a truncated listing cannot show the PR leaves docs/streams/findings/ untouched — fail-closed (statusgen/06 §B); "+
					"fetch the base branch (fetch-depth: 0) and re-run", len(files), n),
			}}, nil
		}
		return nil, nil
	}
	local, err := localTouchedFindings(root, mergeBase)
	if err != nil {
		return []registerTransitionResult{{
			Verdict: verdictMissing,
			Moves:   "register touch unknown",
			Evidence: fmt.Sprintf("the working tree could not be compared with the PR merge-base (%v), so the findings-register "+
				"transitions this PR makes cannot be evaluated — fail-closed (statusgen/06 §B)", err),
		}}, nil
	}
	touched = unionSorted(touched, local)
	if len(touched) == 0 {
		return nil, nil
	}
	var out []registerTransitionResult
	// A touched entry the detector cannot parse is invisible to it — fail closed
	// rather than read the silence as "no transition".
	for _, rel := range touched {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue // deleted in this PR — the tombstone guard (deletedRegisterFiles) owns deletion
		}
		if _, err := parseFindingFile(raw); err != nil {
			out = append(out, registerTransitionResult{
				Rel:      rel,
				Verdict:  verdictMissing,
				Evidence: fmt.Sprintf("entry does not parse (%v) — its field transitions cannot be evaluated, fail-closed", err),
			})
		}
	}
	ts := registerFieldTransitions(root, mergeBase)
	if len(ts) > 0 {
		data, err := fetchData()
		if err != nil {
			return nil, err
		}
		out = append(out, corroborateRegisterTransitions(ts, data, repo, pr)...)
	}
	return out, nil
}

// unionSorted returns the sorted, de-duplicated union of two path lists.
func unionSorted(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range append(append([]string{}, a...), b...) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// registerTransitionsFail reports whether any lane result fails the run.
func registerTransitionsFail(rs []registerTransitionResult) bool {
	for _, r := range rs {
		if r.Verdict == verdictMissing {
			return true
		}
	}
	return false
}
