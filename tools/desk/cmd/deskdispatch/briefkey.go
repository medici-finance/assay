package main

// briefkey.go — the brief an item KEY names, read for the human-decision gate when no --brief was
// given (#6173).
//
// THE DEFECT. The gate read the brief's `gate: human` only through --brief. A brief-shaped key
// (`<repo>--<stream>--<NN>`, `<stream>/<NN>`, alias-prefixed) dispatched without --brief was never
// read, so a gated brief printed "decision-gate SKIPPED: item is not human-gated" and dispatched
// with no decision issue in front of the human.
//
// THE RULE. With no --brief and no --gate-human, a key that reduces to a valid `<stream>/<NN>`
// (briefIDFromItem, then deskkit.SplitBriefKey) names exactly one file,
// docs/streams/<stream>/brief-<NN>-*.md, looked up under --root then --claim-root, the order
// resolveBrief uses. No file, more than one, or an unreadable one is REFUSED before the claim: a
// skip there would be a could-not-check reported as "not human-gated". A key that is not
// brief-shaped (issue-<N>, a review key, item-1) names no brief and changes nothing.
//
// THE SCOPE. The derived brief feeds the human gate ONLY: the gate detection, the recorded-ruling
// read, the decision tool's argument and the pre-claim decision hold (plan.gateOpts). Every other
// reader of o.brief (deliverable resolution, exec-tier check, write scopes, the prompt's
// specification line, the dispatch record, worker attestation) keeps reading --brief alone, so
// their behaviour on a key-only dispatch is what it was.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// keyBriefFn is the seam the gate resolves the key-named brief through; tests whose fixture
// checkouts carry no board relax it (briefkey_test.go).
var keyBriefFn = briefFromItemKey

// briefFromItemKey resolves the brief o.item names. It returns ("", "", nil) when the key is not
// brief-shaped. path is absolute; shown is the --root-relative spelling when the file is under
// --root, else the absolute path (the briefShown rule resolveBrief applies).
func briefFromItemKey(o dispatchOpts) (path, shown string, err error) {
	id := briefIDFromItem(o.item)
	if id == "" {
		return "", "", nil
	}
	stream, num, serr := deskkit.SplitBriefKey(id)
	if serr != nil {
		return "", "", nil
	}
	rel := filepath.Join("docs", "streams", stream)
	prefix := "brief-" + num + "-"
	var tried []string
	seen := map[string]bool{}
	for _, base := range []string{o.root, o.claimRoot} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		if abs, aerr := filepath.Abs(base); aerr == nil {
			base = abs
		}
		if seen[base] {
			continue
		}
		seen[base] = true
		dir := filepath.Join(base, rel)
		tried = append(tried, filepath.Join(dir, prefix+"*.md"))
		entries, _ := os.ReadDir(dir)
		var matches []string
		for _, e := range entries {
			if n := e.Name(); !e.IsDir() && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, ".md") {
				matches = append(matches, filepath.Join(dir, n))
			}
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
		default:
			return "", "", briefKeyRefusal(o.item, fmt.Sprintf("more than one brief matches (%s)", strings.Join(matches, ", ")))
		}
		p := matches[0]
		if _, rerr := os.ReadFile(p); rerr != nil {
			return "", "", briefKeyRefusal(o.item, fmt.Sprintf("%s cannot be read (%v)", p, rerr))
		}
		shown = p
		if base == absOr(o.root) {
			shown = filepath.ToSlash(filepath.Join(rel, filepath.Base(p)))
		}
		return p, shown, nil
	}
	return "", "", briefKeyRefusal(o.item, "no file matches; tried "+strings.Join(tried, ", then "))
}

// briefKeyRefusal is the pre-claim refusal for a brief-shaped key whose brief cannot be read.
func briefKeyRefusal(item, why string) error {
	return deskkit.Refused(fmt.Sprintf(
		"step %s: could-not-check: item key %q names a brief but none resolved (%s); the human gate was not "+
			"read, so nothing is dispatched on it. Nothing was claimed. Pass --brief <path>, or fix the key.",
		stepClaimAcquire, item, why))
}

// absOr returns p made absolute, or p unchanged when that fails.
func absOr(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// gateOpts is o as the human-gate steps read it: with the key-named brief standing in for an empty
// --brief. Every other step keeps o.
func (p dispatchPlan) gateOpts(o dispatchOpts) dispatchOpts {
	if p.keyBrief != "" {
		o.brief, o.briefShown = p.keyBrief, p.keyBriefShown
	}
	return o
}
