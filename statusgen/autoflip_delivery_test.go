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

// evidenceShape is a verify Evidence PR: the brief file, the README row and the
// verify-outcome record, all under docs/streams/, by author.
func evidenceShape(author string) prShape {
	return prShape{Author: author, Files: []string{
		"docs/streams/af/README.md",
		"docs/streams/af/brief-50-walk.md",
		"docs/streams/af/verify-outcomes/50.json",
	}}
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

// verifierRev is the reviewer identity with the roster's verifier App bound.
func verifierRev() reviewerIdentity {
	r := ghReviewer(afReviewer)
	r.EvidenceLanders = []string{afVerifier, "app/verifier-app"}
	return r
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

// TestAutoFlipEvidenceNeedsVerifier: the exclusion is the verifier App's
// identity AND a docs/streams-only diff. Either half missing, the PR is an
// ordinary candidate and its missing App approval refuses the flip.
func TestAutoFlipEvidenceNeedsVerifier(t *testing.T) {
	codeTouching := evidenceShape(afVerifier)
	codeTouching.Files = append(codeTouching.Files, "statusgen/autoflip.go")
	renamedIn := evidenceShape(afVerifier)
	renamedIn.RenamedFrom = []string{"statusgen/autoflip.go"}
	for name, tc := range map[string]struct {
		shape prShape
		rev   reviewerIdentity
	}{
		"author is not the verifier":    {evidenceShape("some-human"), verifierRev()},
		"no verifier bound in roster":   {evidenceShape(afVerifier), ghReviewer(afReviewer)},
		"verifier PR also touches code": {codeTouching, verifierRev()},
		"verifier PR with no file list": {prShape{Author: afVerifier}, verifierRev()},
		"author unknown (empty login)":  {evidenceShape(""), verifierRev()},
		"verifier PR renames code in":   {renamedIn, verifierRev()},
	} {
		t.Run(name, func(t *testing.T) {
			src := walkSource([]int{190, 151},
				map[int]prReviewState{190: humanApproved(afNoAppSHA, 5), 151: mergedApproved(afRealDeliveryHeadSHA, 1)},
				map[int]prShape{190: tc.shape, 151: realDelivery50})
			src.trailerHits = map[string][]int{"af/50": {151}}
			if got := decideWalkRev(t, src, tc.rev); got.Outcome != flipRefused || got.PR != 190 {
				t.Fatalf("want REFUSED naming #190; got %v PR #%d (%s)", got.Outcome, got.PR, got.Reason)
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
	files, from, err := parseFileListing(1, []byte("[\"docs/streams/af/x.md\",\"statusgen/x.go\"]\n[\"docs/streams/af/y.md\",\"\"]\n"))
	if err != nil || strings.Join(files, ",") != "docs/streams/af/x.md,docs/streams/af/y.md" || strings.Join(from, ",") != "statusgen/x.go" {
		t.Errorf("got files=%v from=%v err=%v", files, from, err)
	}
	for _, bad := range []string{"docs/streams/af/x.md\n", "[\"a\"]\n", "[\"\",\"b\"]\n"} {
		if _, _, err := parseFileListing(1, []byte(bad)); err == nil {
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
