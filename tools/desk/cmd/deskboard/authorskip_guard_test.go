package main

// Class guard for #2028 — defect class: "a board classification arm that routes a
// trust-gate-admitted PR out of the review dispatch line on author identity".
//
// The #177 HUMAN-OWNED arm was ONE instance of that class: buildClassifyInput read the PR
// author into a trust predicate (deskkit.TrustedHumanAuthor), and classify used the answer
// to turn a PR the trust gate had already admitted into a no-op row. The fix removes the
// instance; this guard holds the CLASS. It enumerates EVERY site in the board and the
// review reactor (cmd/deskboard, cmd/reviewloop — the two packages that decide whether a
// PR is dispatched) that reads a PR/issue author (`.Author`) or calls a deskkit
// trust/identity predicate (a name containing Trust, Bless, Human, Actor or Author), per
// enclosing function, COUNTED, and requires each to be on the reviewed allow-list below with
// the reason it is not an authorship skip. A new site — a fresh classification arm, or a
// second author check slipped into an already-listed function — fails here by name until a
// reviewer adds it with a reason. The allow-list is also held EXACT: an entry the code no
// longer has fails as stale, so the list cannot rot into a blanket permission.
//
// The trust gate itself (classifyPRFrom / sweepPRsRepo / cmdQueue) is the one sanctioned
// authorship filter: it QUARANTINES an unblessed author, it never skips a trusted one.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// authorshipAllow is the reviewed allow-list: "<pkg>/<file>:<func>:<kind>" → expected count.
// kind is ".Author" (a read of an author field) or "deskkit.<Name>" (a trust/identity call).
var authorshipAllow = map[string]struct {
	n   int
	why string
}{
	// ---- the trust gate: the ONLY authorship filter (quarantine unblessed, never skip trusted) ----
	"deskboard/board.go:classifyPRFrom:.Author": {3, "the `actions` trust gate's TrustedAuthor argument, the " +
		"quarantined externalRow's display field, and the TrailerAbsentAppAnomaly argument — that last one " +
		"ADDS a security gate on an App-authored PR, it never skips dispatch"},
	"deskboard/board.go:classifyPRFrom:deskkit.TrustedAuthor": {1, "the `actions` trust gate (quarantine the unblessed)"},
	"deskboard/board.go:sweepPRsRepo:.Author":                 {2, "the `prs` trust gate's argument and the quarantined externalRow's display field"},
	"deskboard/board.go:sweepPRsRepo:deskkit.TrustedAuthor":   {1, "the `prs` trust gate (quarantine the unblessed)"},
	"deskboard/board.go:cmdQueue:.Author":                     {3, "the issue-queue trust gate's login+id argument and the quarantined externalRow's display field"},
	"deskboard/board.go:cmdQueue:deskkit.TrustedAuthorID":     {1, "the issue-queue trust gate (quarantine the unblessed)"},
	"deskboard/board.go:prBlessed:deskkit.Blessed":            {1, "the trust gate's blessing read for a PR (admits an outsider only on a current blessing)"},
	"deskboard/board.go:issueBlessed:deskkit.Blessed":         {1, "the trust gate's blessing read for an issue"},

	// ---- mapping and display: carry the author, decide nothing ----
	"deskboard/board.go:prBaseFromChange:.Author":                       {2, "maps the typed forge author onto prBase — a copy, no decision"},
	"deskboard/board.go:reviewsFromForge:.Author":                       {1, "maps a REVIEW's author (the verdict's identity, read by isReviewerBot) — not the PR author"},
	"deskboard/board.go:cmdReviews:.Author":                             {1, "the `reviews` debug verb prints each review's author — display only"},
	"deskboard/board.go:renderExternal:.Author":                         {1, "renders the quarantined EXTERNAL / UNBLESSED section — display only"},
	"deskboard/delta_extractors.go:actionsDeltaSet:.Author":             {1, "the quarantined row's delta signature — display only"},
	"deskboard/delta_extractors.go:prsDeltaSet:.Author":                 {1, "the quarantined row's delta signature — display only"},
	"deskboard/delta_extractors.go:queueDeltaSet:.Author":               {1, "the quarantined row's delta signature — display only"},
	"deskboard/board.go:blessAuthorityName:deskkit.BlessAuthorityLogin": {1, "names the blessing authority in the quarantine hint — display only"},

	// ---- the stalled report: who moved last, not whether to review ----
	"deskboard/stalled.go:fetchLastAuthorComment:.Author":           {1, "finds the PR author's last comment for the stalled report — no dispatch decision"},
	"deskboard/stalled.go:fetchLastAuthorComment:deskkit.SameActor": {1, "same — matches the comment to the PR author"},
	"deskboard/stalled.go:sweepStalledPR:.Author":                   {2, "the stalled report's last-actor read — no dispatch decision"},
	"deskboard/stalled.go:sweepStalledPR:deskkit.SameActor":         {1, "same — whether the head push was the author's"},

	// ---- the reactor ----
	"reviewloop/findingcontinuity.go:ReadRecords:deskkit.ActorRole": {1, "types a finding record's reviewer ROLE — not a PR author"},
}

// authorshipPredicate matches the deskkit trust/identity surface: today's predicates
// (TrustedAuthor, TrustedAuthorID, TrustedPublicAuthor, TrustedHumanAuthor, IsTrustedHumanLogin,
// Blessed, ItemTrustedEvents, IsBlessAuthority*, SameActor, ActorRole, …) and any new one named
// in the same vocabulary.
var authorshipPredicate = regexp.MustCompile(`Trust|Bless|Human|Actor|Author`)

// authorshipSites parses one Go source and adds every authorship site it holds, keyed
// "<label>:<func>:<kind>", to into.
func authorshipSites(label string, src []byte, into map[string]int) error {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, label, src, 0)
	if err != nil {
		return err
	}
	for _, d := range af.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "Author" {
				into[label+":"+fd.Name.Name+":.Author"]++
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "deskkit" && authorshipPredicate.MatchString(sel.Sel.Name) {
				into[label+":"+fd.Name.Name+":deskkit."+sel.Sel.Name]++
			}
			return true
		})
	}
	return nil
}

// unlistedAuthorshipSites returns one message per site the allow-list does not cover (a new
// key, or more occurrences than reviewed).
func unlistedAuthorshipSites(found map[string]int) []string {
	var bad []string
	for k, n := range found {
		a, ok := authorshipAllow[k]
		switch {
		case !ok:
			bad = append(bad, fmt.Sprintf("%s ×%d — NEW authorship site", k, n))
		case n > a.n:
			bad = append(bad, fmt.Sprintf("%s ×%d — %d more than the reviewed %d", k, n, n-a.n, a.n))
		}
	}
	sort.Strings(bad)
	return bad
}

// TestAuthorSkipClassGuard — every authorship site in the dispatch-deciding packages is a
// reviewed one; no classification arm may skip a trust-gate-admitted PR on who wrote it.
func TestAuthorSkipClassGuard(t *testing.T) {
	found := map[string]int{}
	for _, dir := range []string{".", "../reviewloop"} {
		pkg := filepath.Base(dir)
		if dir == "." {
			pkg = "deskboard"
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no Go sources under %s — the guard would pass vacuously", dir)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := authorshipSites(pkg+"/"+filepath.Base(f), src, found); err != nil {
				t.Fatalf("parsing %s: %v", f, err)
			}
		}
	}
	for _, msg := range unlistedAuthorshipSites(found) {
		t.Errorf("%s: a site that reads the author or calls a trust predicate is a candidate "+
			"authorship skip (#2028's defect class — the #177 HUMAN-OWNED arm). If it only "+
			"quarantines the unblessed, maps or displays, add it to authorshipAllow WITH the "+
			"reason; if it routes an admitted PR out of dispatch, remove it", msg)
	}
	for k, a := range authorshipAllow {
		if found[k] < a.n {
			t.Errorf("stale allow-list entry %s: reviewed ×%d, found ×%d — tighten the entry", k, a.n, found[k])
		}
	}
}

// TestAuthorGuardPositiveCtl — the guard's positive control: the retired #177 shape (an
// author read fed to TrustedHumanAuthor in buildClassifyInput) and a second author check
// slipped into the already-listed classifyPRFrom must BOTH be flagged.
func TestAuthorGuardPositiveCtl(t *testing.T) {
	const planted = `package main

func buildClassifyInput(p prBase) classifyInput {
	return classifyInput{authorTrustedHuman: deskkit.TrustedHumanAuthor(p.Author.Login)}
}

func classifyPRFrom(p prBase) prOutcome {
	authorTrusted := deskkit.TrustedAuthor(p.Author.Login)
	_ = authorTrusted
	_ = externalRow{Author: p.Author.Login}
	_ = deskkit.TrailerAbsentAppAnomaly(p.Author.Login, nil)
	if deskkit.IsTrustedHumanLogin(p.Author.Login) {
		return prOutcome{}
	}
	return prOutcome{}
}
`
	found := map[string]int{}
	if err := authorshipSites("deskboard/board.go", []byte(planted), found); err != nil {
		t.Fatal(err)
	}
	bad := strings.Join(unlistedAuthorshipSites(found), "\n")
	for _, want := range []string{
		"deskboard/board.go:buildClassifyInput:.Author",
		"deskboard/board.go:buildClassifyInput:deskkit.TrustedHumanAuthor",
		"deskboard/board.go:classifyPRFrom:.Author ×4",
		"deskboard/board.go:classifyPRFrom:deskkit.IsTrustedHumanLogin",
	} {
		if !strings.Contains(bad, want) {
			t.Errorf("the guard did not flag the planted site %q; it flagged:\n%s", want, bad)
		}
	}
}
