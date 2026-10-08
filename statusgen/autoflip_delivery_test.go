package main

// autoflip_delivery_test.go — #1838: the model autoflip must credit the PR that
// DELIVERED a brief (found by its `Brief:` trailer), not the verify Evidence PR
// that landed the brief's rows, and not a trailer-less authoring PR that merely
// created the brief file.
//
// THE DEFECT CLASS. An approval demanded of a candidate PR by any route other
// than candidateGate, the one door that first sets aside a verify Evidence
// landing. Before #1838 the history walk called approvalAtHead directly, so the
// newest change to the brief file (the verifier's Evidence PR, merged on a human
// approval) refused every PR-landed PASS. TestApprovalOnlyViaCandidateGate
// holds approvalAtHead to that one caller; a second resolver that calls it
// directly would re-open the class.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const afVerifier = "verifier-app[bot]"

// afMerged is a fixed merge time d days after an arbitrary epoch.
func afMerged(d int) time.Time {
	return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, d)
}

// mergedApproved is approvedAt with a merge time.
func mergedApproved(head string, d int) prReviewState {
	st := approvedAt(head)
	st.MergedAt = afMerged(d)
	return st
}

// humanApproved is a merged PR a human (not the reviewer App) approved.
func humanApproved(head string, d int) prReviewState {
	return prReviewState{Merged: true, HeadSHA: head, MergedAt: afMerged(d), Reviews: []ghReview{
		{Author: ghAuthor{Login: "some-human"}, State: "APPROVED", CommitOID: head},
	}}
}

// evidenceShape is a verify Evidence PR: the brief's Evidence section, the
// README row's status cells and the verify-outcome record. Since
// verify-reset/07 the author plays no part; it is kept as a parameter so each
// fixture still says who landed it.
func evidenceShape(author string) prShape {
	return evidenceOnlyShape(author)
}

// authoringShape is a stream-authoring PR: it created the brief file, touched
// one path outside docs/streams/ (so it is not migration-shaped), and carries no
// `Brief:` trailer. On main the walk stopped here as unattributable.
var authoringShape = prShape{Author: "desk-app[bot]", Files: []string{
	"changelog/stream-af.md",
	"docs/streams/af/README.md",
	"docs/streams/af/brief-50-walk.md",
	"docs/streams/af/brief-51-other.md",
}}

// verifierRev is the reviewer identity the delivery fixtures run under. Before
// verify-reset/07 it also bound the verifier App the Evidence exclusion keyed
// on; the exclusion is now by diff shape, so it is the plain reviewer.
func verifierRev() reviewerIdentity {
	return ghReviewer(afReviewer)
}

func decideWalkRev(t *testing.T, src *fakeFlipSource, rev reviewerIdentity) modelFlipResult {
	t.Helper()
	root, streams := loadAFStreams(t)
	s := streams[0]
	return decideModelFlip(root, s, filepath.Join(s.Dir, "brief-50-walk.md"), "af/50", "", src, rev)
}

// TestAutoFlipSkipsEvidenceLanding — shape A, delivery in the file history. The
// newest PR touching the brief is the verifier's Evidence PR, merged on a human
// approval; the older PR is the App-approved delivery. It must flip, crediting
// the delivery. On main it was REFUSED on the Evidence PR.
func TestAutoFlipSkipsEvidenceLanding(t *testing.T) {
	src := walkSource([]int{190, 151},
		map[int]prReviewState{
			190: humanApproved(afNoAppSHA, 5),
			151: mergedApproved(afRealDeliveryHeadSHA, 1),
		},
		map[int]prShape{190: evidenceShape(afVerifier), 151: realDelivery50})
	got := decideWalkRev(t, src, verifierRev())
	if got.Outcome != flipDone || got.PR != 151 || got.SHA != afRealDeliveryHeadSHA {
		t.Fatalf("want flipDone crediting #151 @ %s; got %v PR #%d (%s)", afRealDeliveryHeadSHA, got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipEvidenceByTrailer — shape A, the usual case: the delivery never
// touched the brief file, so only its `Brief:` trailer finds it.
func TestAutoFlipEvidenceByTrailer(t *testing.T) {
	src := walkSource([]int{190},
		map[int]prReviewState{
			190: humanApproved(afNoAppSHA, 5),
			151: mergedApproved(afRealDeliveryHeadSHA, 1),
		},
		map[int]prShape{190: evidenceShape(afVerifier), 151: realDelivery50})
	src.trailerHits = map[string][]int{"af/50": {151}}
	got := decideWalkRev(t, src, verifierRev())
	if got.Outcome != flipDone || got.PR != 151 {
		t.Fatalf("want flipDone crediting the trailer-resolved #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerPastAuthoring — shape B: the file history holds only the
// Evidence PR and the approved, trailer-less authoring PR that created the brief.
// On main the walk stopped at the authoring PR (COULD-NOT-CHECK). The delivery's
// own trailer settles attribution, whether or not merge times are known.
func TestAutoFlipTrailerPastAuthoring(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(fmt.Sprintf("mergedAt known=%v", known), func(t *testing.T) {
			states := map[int]prReviewState{
				190: humanApproved(afNoAppSHA, 9),
				129: mergedApproved(afHeadSHA, 0),
				151: mergedApproved(afRealDeliveryHeadSHA, 3),
			}
			if !known {
				for n, st := range states {
					st.MergedAt = time.Time{}
					states[n] = st
				}
			}
			src := walkSource([]int{190, 129},
				states, map[int]prShape{190: evidenceShape(afVerifier), 129: authoringShape, 151: realDelivery50})
			src.trailerHits = map[string][]int{"af/50": {151}}
			got := decideWalkRev(t, src, verifierRev())
			if got.Outcome != flipDone || got.PR != 151 {
				t.Fatalf("want flipDone crediting #151 past the authoring PR; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
}

// TestAutoFlipTrailerUnapproved: a trailer-resolved delivery must itself carry
// the App approval at its merged head, or the brief is refused naming it.
func TestAutoFlipTrailerUnapproved(t *testing.T) {
	src := walkSource([]int{190},
		map[int]prReviewState{190: humanApproved(afNoAppSHA, 5), 151: humanApproved(afRealDeliveryHeadSHA, 1)},
		map[int]prShape{190: evidenceShape(afVerifier), 151: realDelivery50})
	src.trailerHits = map[string][]int{"af/50": {151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipRefused || got.PR != 151 {
		t.Fatalf("want REFUSED naming the unapproved delivery #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerOlderUnapproved: when a brief has two delivering PRs, the
// older one must be approved too; crediting the newer never excuses it.
func TestAutoFlipTrailerOlderUnapproved(t *testing.T) {
	src := walkSource(nil,
		map[int]prReviewState{151: humanApproved(afRealDeliveryHeadSHA, 1), 153: mergedApproved(afHeadSHA, 4)},
		map[int]prShape{151: realDelivery50, 153: realDelivery50})
	src.trailerHits = map[string][]int{"af/50": {153, 151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipRefused || got.PR != 151 {
		t.Fatalf("want REFUSED naming the unapproved #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerNewestCredited: of two approved deliveries, the stamp
// cites the newest by merge time, whatever order the search returned.
func TestAutoFlipTrailerNewestCredited(t *testing.T) {
	src := walkSource(nil,
		map[int]prReviewState{151: mergedApproved(afRealDeliveryHeadSHA, 6), 153: mergedApproved(afHeadSHA, 2)},
		map[int]prShape{151: realDelivery50, 153: realDelivery50})
	src.trailerHits = map[string][]int{"af/50": {153, 151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipDone || got.PR != 151 {
		t.Fatalf("want flipDone crediting the newest delivery #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipGuardNewerUnapproved: with a trailer-resolved delivery in hand,
// an unapproved non-Evidence PR that changed the brief file AFTER the delivery
// still refuses the flip, as it did on main.
func TestAutoFlipGuardNewerUnapproved(t *testing.T) {
	src := walkSource([]int{195, 190},
		map[int]prReviewState{
			195: humanApproved(afNoAppSHA, 7),
			190: humanApproved(afMergedPR, 5),
			151: mergedApproved(afRealDeliveryHeadSHA, 1),
		},
		map[int]prShape{
			195: {Files: []string{"docs/streams/af/brief-50-walk.md"}, BriefTrailers: 1, Briefs: []string{"af/50"}},
			190: evidenceShape(afVerifier),
			151: realDelivery50,
		})
	src.trailerHits = map[string][]int{"af/50": {151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipRefused || got.PR != 195 {
		t.Fatalf("want REFUSED naming the unapproved later #195; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipGuardStopsAtDelivery: a change to the brief file that merged
// BEFORE the delivery is outside the guard, approved or not.
func TestAutoFlipGuardStopsAtDelivery(t *testing.T) {
	src := walkSource([]int{190, 129},
		map[int]prReviewState{
			190: humanApproved(afNoAppSHA, 9),
			129: humanApproved(afHeadSHA, 0),
			151: mergedApproved(afRealDeliveryHeadSHA, 3),
		},
		map[int]prShape{190: evidenceShape(afVerifier), 129: authoringShape, 151: realDelivery50})
	src.trailerHits = map[string][]int{"af/50": {151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipDone || got.PR != 151 {
		t.Fatalf("want flipDone crediting #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerMultiBrief: a search hit whose body carries two `Brief:`
// trailers cannot be attributed on a guess — could-not-check naming it.
func TestAutoFlipTrailerMultiBrief(t *testing.T) {
	src := walkSource(nil,
		map[int]prReviewState{196: mergedApproved(afHeadSHA, 2)},
		map[int]prShape{196: {Files: []string{"internal/x/x.go"}, BriefTrailers: 2, Briefs: []string{"af/50", "af/51"}}})
	src.trailerHits = map[string][]int{"af/50": {196}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipUnchecked || got.PR != 196 {
		t.Fatalf("want COULD-NOT-CHECK naming #196; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerFalseHit: the search is only a prefilter. A hit that
// carries no real `Brief:` trailer (prose quoting one) or names another brief
// is never credited, and the walk decides as it did on main.
func TestAutoFlipTrailerFalseHit(t *testing.T) {
	src := walkSource([]int{151},
		map[int]prReviewState{
			197: mergedApproved(afHeadSHA, 8),
			198: mergedApproved(afMergedPR, 8),
			151: mergedApproved(afRealDeliveryHeadSHA, 1),
		},
		map[int]prShape{
			197: {Files: []string{"internal/x/x.go"}},
			198: {Files: []string{"internal/x/x.go"}, BriefTrailers: 1, Briefs: []string{"af/500"}},
			151: realDelivery50,
		})
	src.trailerHits = map[string][]int{"af/50": {197, 198}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipDone || got.PR != 151 {
		t.Fatalf("want flipDone crediting #151 from the walk; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipTrailerSearchFails: a search that cannot be read falls back to
// the walk alone and says so on any non-flip; it never becomes a flip.
func TestAutoFlipTrailerSearchFails(t *testing.T) {
	src := walkSource([]int{190},
		map[int]prReviewState{190: humanApproved(afNoAppSHA, 5)},
		map[int]prShape{190: evidenceShape(afVerifier)})
	src.trailerErr = errors.New("HTTP 502")
	got := decideWalkRev(t, src, verifierRev())
	if got.Outcome != flipUnchecked {
		t.Fatalf("want COULD-NOT-CHECK; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
	if !strings.Contains(got.Reason, "HTTP 502") || !strings.Contains(got.Reason, "#190") {
		t.Errorf("reason must carry the search error and the walked Evidence PR; got %q", got.Reason)
	}
}

// editedFixture is the review's S1 shape. The brief file's history holds #160,
// approved by a human only, trailer-less, touching code and the brief file:
// main refuses on it. #170 is App-approved, merged later, touches an unrelated
// file, and carries `Brief: af/50` in its body only.
func editedFixture() *fakeFlipSource {
	src := walkSource([]int{160},
		map[int]prReviewState{
			160: humanApproved(afNoAppSHA, 2),
			170: mergedApproved(afHeadSHA, 4),
		},
		map[int]prShape{
			160: {Files: []string{"internal/x/x.go", "docs/streams/af/brief-50-walk.md"}},
			170: {Files: []string{"internal/y/y.go"}, BriefTrailers: 1, Briefs: []string{"af/50"}},
		})
	src.trailerHits = map[string][]int{"af/50": {170}}
	return src
}

// TestAutoFlipTrailerEditedBody: a trailer the PR did not merge with was never
// reviewed. A credited hit whose body was edited after it merged, or whose
// edit time cannot be ordered against its merge, is COULD-NOT-CHECK naming it.
// It is not credited and does not end the guard over #160.
func TestAutoFlipTrailerEditedBody(t *testing.T) {
	for name, set := range map[string]func(*fakeFlipSource){
		"edited after merge": func(f *fakeFlipSource) {
			f.bodyEdits = map[int]time.Time{170: afMerged(4).Add(time.Hour)}
		},
		"edit time unreadable": func(f *fakeFlipSource) {
			f.bodyEditErrs = map[int]error{170: errors.New("HTTP 502")}
		},
		"edited, merge time unknown": func(f *fakeFlipSource) {
			st := f.states[170]
			st.MergedAt = time.Time{}
			f.states[170] = st
			f.bodyEdits = map[int]time.Time{170: afMerged(1)}
		},
		"edited after merge, unapproved": func(f *fakeFlipSource) {
			f.states[170] = humanApproved(afHeadSHA, 4)
			f.bodyEdits = map[int]time.Time{170: afMerged(5)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			src := editedFixture()
			set(src)
			got := decideWalkRev(t, src, verifierRev())
			if got.Outcome != flipUnchecked || got.PR != 170 {
				t.Fatalf("want COULD-NOT-CHECK naming #170; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
}

// TestAutoFlipTrailerEditedPreMerge: a body last edited at or before the merge
// is the body the PR merged with. It credits as before, and main's refusal on
// the older unapproved #160 is outside the guard.
func TestAutoFlipTrailerEditedPreMerge(t *testing.T) {
	for _, edit := range []time.Time{afMerged(4).Add(-time.Hour), afMerged(4)} {
		src := editedFixture()
		src.bodyEdits = map[int]time.Time{170: edit}
		if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipDone || got.PR != 170 {
			t.Fatalf("edit %s: want flipDone crediting #170; got %v PR #%d (%s)", edit, got.Outcome, got.PR, got.Reason)
		}
	}
}

// TestAutoFlipTrailerSideBranch: a trailer hit that merged into a branch other
// than the default is not a delivery. Its code may never have reached the
// default branch. It is walked past, whether or not it is approved, and named
// on the result. A hit merged into the default branch is credited.
func TestAutoFlipTrailerSideBranch(t *testing.T) {
	onBranch := func(base string) prShape {
		s := realDelivery50
		s.BaseRef, s.DefaultBranch = base, "main"
		return s
	}
	for name, st := range map[string]prReviewState{
		"approved at head": mergedApproved(afRealDeliveryHeadSHA, 1),
		"not approved":     humanApproved(afRealDeliveryHeadSHA, 1),
	} {
		t.Run(name, func(t *testing.T) {
			src := walkSource([]int{190},
				map[int]prReviewState{190: humanApproved(afNoAppSHA, 5), 151: st},
				map[int]prShape{190: evidenceShape(afVerifier), 151: onBranch("feat/side")})
			src.trailerHits = map[string][]int{"af/50": {151}}
			got := decideWalkRev(t, src, verifierRev())
			if got.Outcome != flipUnchecked || got.PR == 151 || !strings.Contains(got.Reason, "#151") {
				t.Fatalf("want COULD-NOT-CHECK not crediting, but naming, side-branch #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
	src := walkSource([]int{190},
		map[int]prReviewState{190: humanApproved(afNoAppSHA, 5), 151: mergedApproved(afRealDeliveryHeadSHA, 1)},
		map[int]prShape{190: evidenceShape(afVerifier), 151: onBranch("main")})
	src.trailerHits = map[string][]int{"af/50": {151}}
	if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipDone || got.PR != 151 {
		t.Fatalf("default-branch hit: want flipDone crediting #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
	}
}

// TestAutoFlipGuardLogOrder: the guard judges every change to the brief file
// that merged after the delivery, wherever git log lists it. Log order is
// commit order, not merge order, so a change merged later (#195, day 7) can
// be listed after an older change (#129) or after the delivery itself (#151).
func TestAutoFlipGuardLogOrder(t *testing.T) {
	for name, commits := range map[string][]int{
		"pre-delivery change listed first": {129, 195},
		"delivery listed first":            {151, 195},
	} {
		t.Run(name, func(t *testing.T) {
			src := walkSource(commits,
				map[int]prReviewState{
					129: mergedApproved(afHeadSHA, 0),
					151: mergedApproved(afRealDeliveryHeadSHA, 3),
					195: humanApproved(afNoAppSHA, 7),
				},
				map[int]prShape{
					129: authoringShape,
					151: realDelivery50,
					195: {Files: []string{"internal/x/x.go", "docs/streams/af/brief-50-walk.md"}},
				})
			src.trailerHits = map[string][]int{"af/50": {151}}
			if got := decideWalkRev(t, src, verifierRev()); got.Outcome != flipRefused || got.PR != 195 {
				t.Fatalf("want REFUSED naming the later-merged unapproved #195; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
}

// TestAutoFlipSummaryBase: a PR record missing its base or default branch is
// an error, never a match of "" to "".
func TestAutoFlipSummaryBase(t *testing.T) {
	var v ghPRSummaryJSON
	v.Base.Ref, v.Base.Repo.DefaultBranch = "feat/side", "main"
	if b, d, err := summaryBase(1, v); err != nil || b != "feat/side" || d != "main" {
		t.Errorf("got %q %q %v", b, d, err)
	}
	for _, unset := range []func(*ghPRSummaryJSON){
		func(v *ghPRSummaryJSON) { v.Base.Ref = "" },
		func(v *ghPRSummaryJSON) { v.Base.Repo.DefaultBranch = "" },
	} {
		w := v
		unset(&w)
		if _, _, err := summaryBase(1, w); err == nil {
			t.Errorf("want an error for base %q default %q", w.Base.Ref, w.Base.Repo.DefaultBranch)
		}
	}
}

// TestAutoFlipParseLastEdited: null is never edited. A missing pull request or
// an unparseable time is an error, never a pass.
func TestAutoFlipParseLastEdited(t *testing.T) {
	at, err := parseLastEditedAt(1, []byte(`{"data":{"repository":{"pullRequest":{"lastEditedAt":"2026-09-25T20:13:32Z"}}}}`))
	if err != nil || !at.Equal(time.Date(2026, 9, 25, 20, 13, 32, 0, time.UTC)) {
		t.Errorf("edited: got %v, %v", at, err)
	}
	if at, err := parseLastEditedAt(1, []byte(`{"data":{"repository":{"pullRequest":{"lastEditedAt":null}}}}`)); err != nil || !at.IsZero() {
		t.Errorf("never edited: got %v, %v", at, err)
	}
	for _, bad := range []string{
		`{"data":{"repository":{"pullRequest":null}}}`,
		`{"data":{"repository":null}}`,
		`{"data":{"repository":{"pullRequest":{"lastEditedAt":"yesterday"}}}}`,
		`not json`,
	} {
		if _, err := parseLastEditedAt(1, []byte(bad)); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

// TestAutoFlipParseFileListing: a rename's old path is kept; a malformed line
// is an error, never skipped.
func TestAutoFlipParseFileListing(t *testing.T) {
	files, from, patches, err := parseFileListing(1, []byte("[\"docs/streams/af/x.md\",\"statusgen/x.go\"]\n[\"docs/streams/af/y.md\",\"\",\"@@ -1 +1 @@\\n-a\\n+b\"]\n[\"z.bin\",\"\",null]\n"))
	if err != nil || strings.Join(files, ",") != "docs/streams/af/x.md,docs/streams/af/y.md,z.bin" || strings.Join(from, ",") != "statusgen/x.go" {
		t.Errorf("got files=%v from=%v err=%v", files, from, err)
	}
	if len(patches) != 1 || patches["docs/streams/af/y.md"] != "@@ -1 +1 @@\n-a\n+b" {
		t.Errorf("want one patch (y.md); a null or absent patch is no entry; got %q", patches)
	}
	for _, bad := range []string{"docs/streams/af/x.md\n", "[\"a\"]\n", "[\"\",\"b\"]\n", "[null,\"\"]\n", "[\"a\",null]\n", "[\"a\",\"\",\"p\",\"x\"]\n"} {
		if _, _, _, err := parseFileListing(1, []byte(bad)); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

// ---- the class guard ------------------------------------------------------------

// approvalGateName is the only function allowed to reference approvalAtHead.
const approvalGateName = "candidateGate"

// scanApprovalCallers parses every non-test Go file directly in dir and returns
// each reference to approvalAtHead (a call or a function value), keyed
// `<file>:<enclosing func>`. The declaration itself is not a reference.
func scanApprovalCallers(dir string) (map[string]int, error) {
	refs := map[string]int{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			return nil, perr
		}
		for _, decl := range f.Decls {
			fn := "<package>"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				fn = fd.Name.Name
				if fn == "approvalAtHead" && fd.Recv == nil {
					if fd.Body != nil {
						ast.Inspect(fd.Body, func(n ast.Node) bool {
							if id, ok := n.(*ast.Ident); ok && id.Name == "approvalAtHead" {
								refs[name+":"+fn]++ // recursion is a reference too
							}
							return true
						})
					}
					continue
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "approvalAtHead" {
					refs[name+":"+fn]++
				}
				return true
			})
		}
	}
	return refs, nil
}

// approvalRefProblems names every reference outside candidateGate, and fails
// when the scan found no reference at all (a guard looking in the wrong place).
func approvalRefProblems(refs map[string]int) []string {
	if len(refs) == 0 {
		return []string{"found no reference to approvalAtHead — this guard is looking in the wrong place"}
	}
	var problems []string
	for k, n := range refs {
		if !strings.HasSuffix(k, ":"+approvalGateName) {
			problems = append(problems, fmt.Sprintf("%s references approvalAtHead %d time(s) — every candidate's "+
				"approval must go through %s, which sets aside a verify Evidence landing first (#1838)", k, n, approvalGateName))
		}
	}
	sort.Strings(problems)
	return problems
}

// TestApprovalOnlyViaCandidateGate is the class guard over this package.
func TestApprovalOnlyViaCandidateGate(t *testing.T) {
	refs, err := scanApprovalCallers(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range approvalRefProblems(refs) {
		t.Error(p)
	}
}

// TestApprovalGuardCatchesPlant is the guard's own control: a planted second
// caller and a function-value reference are both reported, the gate is not, and
// an empty tree fails loud rather than passing.
func TestApprovalGuardCatchesPlant(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("gate.go", "package main\n\nfunc approvalAtHead(n int) int { return n }\n\n"+
		"func candidateGate(n int) int { return approvalAtHead(n) }\n")
	write("planted.go", "package main\n\nfunc resolveOther(n int) int { return approvalAtHead(n) }\n\n"+
		"var sneaky = approvalAtHead\n")
	refs, err := scanApprovalCallers(dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(approvalRefProblems(refs), "\n")
	for _, want := range []string{"planted.go:resolveOther", "planted.go:<package>"} {
		if !strings.Contains(joined, want) {
			t.Errorf("planted reference %s not reported; got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "gate.go:candidateGate") {
		t.Errorf("the gate itself was reported; got:\n%s", joined)
	}
	if p := approvalRefProblems(map[string]int{}); len(p) != 1 || !strings.Contains(p[0], "wrong place") {
		t.Errorf("an empty scan must fail loud; got %v", p)
	}
}

// ---- verify-reset/07: diff shape, files: overlap, visible refusals ------------------

// afBriefHead is brief af/50 at an Evidence PR's head: lines 13-18 are the
// `## Evidence` section (heading on 13, `## Review` on 19).
var afBriefHead = strings.Join([]string{
	"---", "brief: af/50", "---", "", "# Brief 50", "", // 1-6
	"## Verify", "", "| # | Command | Expect |", "|---|---|---|", "| 1 | `true` | exit 0 |", "", // 7-12
	"## Evidence", "", "| # | Result |", "|---|---|", "| 1 | pass |", "", // 13-18
	"## Review", "Gate: model", "", // 19-21
}, "\n")

// afBriefEvidencePatch appends the Evidence table: new lines 15-17.
const afBriefEvidencePatch = "@@ -13,3 +13,6 @@\n ## Evidence\n \n+| # | Result |\n+|---|---|\n+| 1 | pass |\n \n"

// afReadmeHead is the af stream README at the Evidence PR's head.
var afReadmeHead = strings.Join([]string{
	"# af", "",
	"| # | Brief | Status | Verified | Reviewed |",
	"|---|---|---|---|---|",
	"| 49 | [other](brief-49-other.md) | done | 2026-09-01 v | 2026-09-02 r |",
	"| 50 | [walk](brief-50-walk.md) | verified | 2026-09-06 verifier | — |",
	"",
}, "\n")

// afReadmeStatusPatch moves row 50's status and Verified cells only.
const afReadmeStatusPatch = "@@ -6 +6 @@\n-| 50 | [walk](brief-50-walk.md) | implemented | — | — |\n+| 50 | [walk](brief-50-walk.md) | verified | 2026-09-06 verifier | — |\n"

// evidenceOnlyShape is a verify Evidence PR by diff shape: the brief's
// `## Evidence` section, the README row's status cells and a verify-outcome
// record. It names af/50 in a `Brief:` trailer, so before verify-reset/07 it
// was a delivery candidate whenever its author was not the verifier App.
func evidenceOnlyShape(author string) prShape {
	return prShape{
		Author:        author,
		BriefTrailers: 1, Briefs: []string{"af/50"},
		BaseRef: "main", DefaultBranch: "main",
		Files: []string{
			"docs/streams/af/README.md",
			"docs/streams/af/brief-50-walk.md",
			"docs/streams/verify-outcomes/af/50.json",
		},
		Patches: map[string]string{
			"docs/streams/af/README.md":               afReadmeStatusPatch,
			"docs/streams/af/brief-50-walk.md":        afBriefEvidencePatch,
			"docs/streams/verify-outcomes/af/50.json": "@@ -0,0 +1 @@\n+{}\n",
		},
		Heads: map[string]string{
			"docs/streams/af/README.md":        afReadmeHead,
			"docs/streams/af/brief-50-walk.md": afBriefHead,
		},
	}
}

// TestAutoflipIgnoresEvidenceOnlyPR — verify-reset/07 Task 1. A newer PR whose
// diff is Evidence-only (whoever authored it, and carrying `Brief: af/50`) is
// never the delivering PR and is asked for no approval; the older, App-approved
// delivery is credited. Every way out of the Evidence-only shape makes the PR an
// ordinary candidate again, whose missing App approval refuses the flip.
func TestAutoflipIgnoresEvidenceOnlyPR(t *testing.T) {
	run := func(t *testing.T, ev prShape) modelFlipResult {
		t.Helper()
		src := walkSource([]int{195, 151},
			map[int]prReviewState{195: humanApproved(afNoAppSHA, 5), 151: mergedApproved(afRealDeliveryHeadSHA, 1)},
			map[int]prShape{195: ev, 151: realDelivery50})
		src.trailerHits = map[string][]int{"af/50": {195, 151}}
		return decideWalkRev(t, src, ghReviewer(afReviewer))
	}
	statusOnly := prShape{Author: "some-human", Files: []string{"STATUS.md"}, Patches: map[string]string{"STATUS.md": "@@ -1 +1 @@\n-a\n+b\n"}}
	for name, ev := range map[string]prShape{
		"evidence-only, human author":    evidenceOnlyShape("some-human"),
		"evidence-only, verifier author": evidenceOnlyShape(afVerifier),
		"evidence-only, no author read":  evidenceOnlyShape(""),
		"status-only":                    statusOnly,
	} {
		t.Run(name, func(t *testing.T) {
			got := run(t, ev)
			if got.Outcome != flipDone || got.PR != 151 || got.SHA != afRealDeliveryHeadSHA {
				t.Fatalf("want flipDone crediting the delivery #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}

	edit := func(f func(*prShape)) prShape { s := evidenceOnlyShape("some-human"); f(&s); return s }
	for name, ev := range map[string]prShape{
		"touches code": edit(func(s *prShape) {
			s.Files = append(s.Files, "statusgen/autoflip.go")
			s.Patches["statusgen/autoflip.go"] = "@@ -1 +1 @@\n-a\n+b\n"
		}),
		"renames code in": edit(func(s *prShape) { s.RenamedFrom = []string{"statusgen/autoflip.go"} }),
		"no file list":    {Author: "some-human", Briefs: []string{"af/50"}, BriefTrailers: 1},
		"brief hunk in Verify": edit(func(s *prShape) {
			s.Patches["docs/streams/af/brief-50-walk.md"] = "@@ -11 +11 @@\n-| 1 | `false` | exit 0 |\n+| 1 | `true` | exit 0 |\n"
		}),
		"brief hunk adds a section heading": edit(func(s *prShape) {
			s.Patches["docs/streams/af/brief-50-walk.md"] = "@@ -13,3 +13,4 @@\n ## Evidence\n \n+## Review\n \n"
		}),
		"brief hunk deletes the Review heading": edit(func(s *prShape) {
			s.Patches["docs/streams/af/brief-50-walk.md"] = "@@ -18,3 +18,2 @@\n \n-## Review\n Gate: model\n"
		}),
		"README edits a brief title": edit(func(s *prShape) {
			s.Patches["docs/streams/af/README.md"] = "@@ -6 +6 @@\n-| 50 | [old](brief-50-walk.md) | verified | 2026-09-06 verifier | — |\n+| 50 | [walk](brief-50-walk.md) | verified | 2026-09-06 verifier | — |\n"
		}),
		"README adds a row": edit(func(s *prShape) {
			s.Patches["docs/streams/af/README.md"] = "@@ -5,0 +6 @@\n+| 50 | [walk](brief-50-walk.md) | verified | 2026-09-06 verifier | — |\n"
		}),
		"README edits prose": edit(func(s *prShape) {
			s.Patches["docs/streams/af/README.md"] = "@@ -1 +1 @@\n-# old\n+# af\n"
		}),
		"brief patch not reported": edit(func(s *prShape) { delete(s.Patches, "docs/streams/af/brief-50-walk.md") }),
		"brief head not read":      edit(func(s *prShape) { delete(s.Heads, "docs/streams/af/brief-50-walk.md") }),
		"README head not read":     edit(func(s *prShape) { delete(s.Heads, "docs/streams/af/README.md") }),
		"other docs/streams file": edit(func(s *prShape) {
			s.Files = append(s.Files, "docs/streams/af/design.md")
			s.Patches["docs/streams/af/design.md"] = "@@ -1 +1 @@\n-a\n+b\n"
		}),
	} {
		t.Run("not evidence-only: "+name, func(t *testing.T) {
			if got := run(t, ev); got.Outcome != flipRefused || got.PR != 195 {
				t.Fatalf("want REFUSED naming the unapproved #195; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
			}
		})
	}
}

// decideWalkFiles runs decideModelFlipFiles for af/50 with a `files:`
// declaration.
func decideWalkFiles(t *testing.T, src *fakeFlipSource, files ...string) modelFlipResult {
	t.Helper()
	root, streams := loadAFStreams(t)
	s := streams[0]
	bf := briefFiles{entries: files, declared: files != nil}
	return decideModelFlipFiles(root, s, filepath.Join(s.Dir, "brief-50-walk.md"), "af/50", "", bf, src, ghReviewer(afReviewer))
}

// TestAutoflipDeliveringByFilesOverlap — verify-reset/07 Task 2. Of the PRs
// credited to af/50, the delivering PR is the newest one whose diff touches a
// path in the brief's `files:`, never a newer credited PR that touches none.
// When no PR touches them the result is COULD-NOT-CHECK and nothing flips.
func TestAutoflipDeliveringByFilesOverlap(t *testing.T) {
	other := prShape{Files: []string{"internal/other/x.go"}, BriefTrailers: 1, Briefs: []string{"af/50"}}
	states := func(newer prReviewState) map[int]prReviewState {
		return map[int]prReviewState{151: mergedApproved(afRealDeliveryHeadSHA, 1), 152: newer}
	}
	shapes := map[int]prShape{151: realDelivery50, 152: other}
	declared := []string{"./internal/walk/walk.go", "changelog/<slug>.md", "docs/streams/af/README.md"}

	t.Run("trailer: the PR touching files: is credited over a newer one", func(t *testing.T) {
		src := walkSource(nil, states(mergedApproved(afHeadSHA, 4)), shapes)
		src.trailerHits = map[string][]int{"af/50": {152, 151}}
		got := decideWalkFiles(t, src, declared...)
		if got.Outcome != flipDone || got.PR != 151 || got.SHA != afRealDeliveryHeadSHA {
			t.Fatalf("want flipDone crediting #151 (touches internal/walk/walk.go) over the newer #152; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
		}
	})
	t.Run("history walk: a credited PR touching none is walked past", func(t *testing.T) {
		src := walkSource([]int{152, 151}, states(mergedApproved(afHeadSHA, 4)), shapes)
		got := decideWalkFiles(t, src, declared...)
		if got.Outcome != flipDone || got.PR != 151 {
			t.Fatalf("want flipDone crediting #151; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
		}
	})
	t.Run("no PR touches files: is COULD-NOT-CHECK", func(t *testing.T) {
		src := walkSource([]int{152, 151}, states(mergedApproved(afHeadSHA, 4)), shapes)
		src.trailerHits = map[string][]int{"af/50": {152, 151}}
		got := decideWalkFiles(t, src, "internal/nothing/")
		if got.Outcome != flipUnchecked || !strings.Contains(got.Reason, "no PR touches the brief's files") {
			t.Fatalf("want COULD-NOT-CHECK naming that no PR touches the brief's files; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
		}
	})
	t.Run("files: naming only another repo is COULD-NOT-CHECK", func(t *testing.T) {
		src := walkSource(nil, states(mergedApproved(afHeadSHA, 4)), shapes)
		src.trailerHits = map[string][]int{"af/50": {152, 151}}
		got := decideWalkFiles(t, src, "../elsewhere/internal/walk/walk.go")
		if got.Outcome != flipUnchecked || !strings.Contains(got.Reason, "no PR touches the brief's files") ||
			!strings.Contains(got.Reason, "../elsewhere/internal/walk/walk.go") {
			t.Fatalf("want COULD-NOT-CHECK naming the unusable entry; got %v (%s)", got.Outcome, got.Reason)
		}
	})
	t.Run("a non-overlapping credited PR still needs its approval", func(t *testing.T) {
		src := walkSource(nil, states(humanApproved(afHeadSHA, 4)), shapes)
		src.trailerHits = map[string][]int{"af/50": {152, 151}}
		if got := decideWalkFiles(t, src, declared...); got.Outcome != flipRefused || got.PR != 152 {
			t.Fatalf("want REFUSED naming the unapproved #152; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
		}
	})
	t.Run("no files: declared keeps the newest credited PR", func(t *testing.T) {
		src := walkSource(nil, states(mergedApproved(afHeadSHA, 4)), shapes)
		src.trailerHits = map[string][]int{"af/50": {152, 151}}
		if got := decideWalkFiles(t, src); got.Outcome != flipDone || got.PR != 152 {
			t.Fatalf("want flipDone crediting the newest #152; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
		}
	})
	t.Run("autoFlipModel reads the brief's files: line", func(t *testing.T) {
		for files, want := range map[string]flipOutcome{"`internal/example/`": flipDone, "`internal/nothing/`": flipUnchecked} {
			root, _ := loadAFStreams(t)
			p := filepath.Join(root, "docs", "streams", "af", "brief-01-model-approved-at-head.md")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			withCtx := strings.Replace(string(b), "## Verify (executable)", "## Context\n\nfiles: "+files+"\n\n## Verify (executable)", 1)
			if err := os.WriteFile(p, []byte(withCtx), 0o644); err != nil {
				t.Fatal(err)
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			results, err := autoFlipModel(root, streams, afSource(), ghReviewer(afReviewer), afNow, true)
			if err != nil {
				t.Fatal(err)
			}
			if got := afResult(t, results, "af/01"); got.Outcome != want {
				t.Errorf("files: %s — af/01 outcome %v (%s), want %v", files, got.Outcome, got.Reason, want)
			}
		}
	})
}
