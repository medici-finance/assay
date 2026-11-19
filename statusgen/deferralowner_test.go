package main

// deferralowner_test.go — a deferral notice statusgen PRINTS must name the brief that
// actually owns the deferred work (#7202).
//
// The GitLab auto-flip could-not-check told every operator the merge-request read
// "lands with the conformance round trip (forge-neutral/10)". Brief 10 has no statusgen
// task at all; the auto-flip reads move onto `deskread` in forge-neutral/18 (its
// consumers line takes statusgen/autoflip.go as fixed-here). An operator who followed
// the notice to brief 10 found nothing to wait for. The notice is the only place the
// deferral is visible at run time, so a wrong owner there is a wrong answer, not a typo.
//
// The class: a string literal in non-test statusgen code that DEFERS work to a
// forge-neutral brief — a deferral cue (lands, until, yet, deferred, pending) followed,
// in the same clause, by `(forge-neutral/<NN>)` — where that brief's frontmatter
// `consumers:` block does not route the citing file to itself (`fixed-here`, or
// `follow-up forge-neutral/<NN>` naming itself). A brief that does not claim the file
// cannot be the one that lands the deferred read there.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const deferralStreamsRoot = "../docs/streams"

// deferralStream is the stream whose deferral citations the guard resolves: the forge
// seam's own, the one a statusgen forge-read deferral names.
const deferralStream = "forge-neutral"

// deferralCiteRE is a deferral cue followed, within one clause (no ; : . — or
// parenthesis between), by a parenthesised forge-neutral brief citation. A provenance
// cite such as "emit the report (forge-neutral/07): …" carries no cue before it and
// is not a deferral.
var deferralCiteRE = regexp.MustCompile(`\b(?:lands|until|yet|deferred|pending)\b[^;:.—()]*\((` + deferralStream + `)/([0-9]{2})\)`)

// deferralCite is one brief citation found in a string literal.
type deferralCite struct {
	file   string // statusgen source file, e.g. autoflip.go
	line   int
	stream string
	num    string
}

// deferralCitesIn returns every parenthesised brief citation in the string literals of
// one Go source text.
func deferralCitesIn(t *testing.T, name string, src []byte) []deferralCite {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var out []deferralCite
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, uerr := strconv.Unquote(lit.Value)
		if uerr != nil {
			return true
		}
		for _, m := range deferralCiteRE.FindAllStringSubmatch(s, -1) {
			out = append(out, deferralCite{
				file: filepath.Base(name), line: fset.Position(lit.Pos()).Line,
				stream: m[1], num: m[2],
			})
		}
		return true
	})
	return out
}

// briefClaimsFile reports whether the brief text's frontmatter `consumers:` block holds
// an entry that names statusgen/<file> before its first ": " and routes it to this
// brief: `fixed-here`, or `follow-up <self>` where self is the brief's own id.
func briefClaimsFile(brief, file, self string) bool {
	in := false
	for _, ln := range strings.Split(brief, "\n") {
		if ln == "---" && in {
			return false
		}
		if strings.HasPrefix(ln, "consumers:") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if ln != "" && !strings.HasPrefix(ln, " ") {
			return false // the next top-level key ends the block
		}
		entry := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "-")), `"`)
		head, tail, ok := strings.Cut(entry, ": ")
		if !ok {
			continue
		}
		tail = strings.TrimSpace(tail)
		if strings.Contains(head, "statusgen/"+file) &&
			(strings.HasPrefix(tail, "fixed-here") || strings.HasPrefix(tail, "follow-up "+self+" ")) {
			return true
		}
	}
	return false
}

// checkDeferralOwner returns the reason one citation fails, or "" when the cited brief
// owns the citing file. ok=false means the stream is not carried in this repo.
func checkDeferralOwner(t *testing.T, c deferralCite) (reason string, ok bool) {
	t.Helper()
	dir := filepath.Join(deferralStreamsRoot, c.stream)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", false
	}
	matches, err := filepath.Glob(filepath.Join(dir, "brief-"+c.num+"-*.md"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(matches) != 1 {
		return "cites " + c.stream + "/" + c.num + ", which resolves to " + strconv.Itoa(len(matches)) + " brief files (want exactly 1)", true
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}
	if !briefClaimsFile(string(body), c.file, c.stream+"/"+c.num) {
		return "defers to " + c.stream + "/" + c.num + ", but " + filepath.Base(matches[0]) +
			" has no consumers entry routing statusgen/" + c.file + " to itself (fixed-here or follow-up " + c.stream + "/" + c.num + ")", true
	}
	return "", true
}

// regression: #7202
func TestDeferralNoticesNameOwningBrief(t *testing.T) {
	if st, err := os.Stat(deferralStreamsRoot); err != nil || !st.IsDir() {
		t.Fatalf("could-not-check: %s is not readable from the statusgen module (%v) — the owner of each cited brief cannot be resolved", deferralStreamsRoot, err)
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, rerr := os.ReadFile(name)
		if rerr != nil {
			t.Fatalf("read %s: %v", name, rerr)
		}
		for _, c := range deferralCitesIn(t, name, src) {
			reason, carried := checkDeferralOwner(t, c)
			if !carried {
				t.Fatalf("could-not-check: %s:%d defers to %s/%s, but %s/%s is not readable", c.file, c.line, c.stream, c.num, deferralStreamsRoot, c.stream)
			}
			checked++
			if reason != "" {
				t.Errorf("%s:%d %s", c.file, c.line, reason)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("could-not-check: no forge-neutral deferral found in any statusgen string literal — the scan matched nothing, so it proves nothing (the GitLab auto-flip notice is one)")
	}
}

// regression: #7202
func TestGitLabFlipNoticeNamesForgeNeutral18(t *testing.T) {
	msg := errGitLabFlipReadUnavailable.Error()
	if strings.Contains(msg, "forge-neutral/10") {
		t.Errorf("GitLab auto-flip notice still names forge-neutral/10, which carries no statusgen read: %q", msg)
	}
	if !strings.Contains(msg, "(forge-neutral/18)") {
		t.Errorf("GitLab auto-flip notice does not name forge-neutral/18, the brief that moves auto-flip's reads onto deskread: %q", msg)
	}
}

// TestDeferralOwnerScanFlagsAPlant is the guard's positive control: a planted notice
// naming a brief that does not own the file must be flagged, so a matcher that silently
// stops matching fails here instead of reporting clean.
func TestDeferralOwnerScanFlagsAPlant(t *testing.T) {
	src := []byte("package main\n\nvar planted = \"x lands with the conformance round trip (forge-neutral/10)\"\n")
	cites := deferralCitesIn(t, "autoflip.go", src)
	if len(cites) != 1 {
		t.Fatalf("planted citation not found by the scan: %+v", cites)
	}
	reason, carried := checkDeferralOwner(t, cites[0])
	if !carried {
		t.Fatalf("could-not-check: forge-neutral is not carried under %s", deferralStreamsRoot)
	}
	if reason == "" {
		t.Fatalf("planted forge-neutral/10 citation in autoflip.go passed the owner check")
	}
	prov := []byte("package main\n\nvar p = \"emit the report (forge-neutral/10): until done\"\n")
	if got := deferralCitesIn(t, "autoflip.go", prov); len(got) != 0 {
		t.Fatalf("a provenance cite with no deferral cue before it was read as a deferral: %+v", got)
	}
	src18 := []byte("package main\n\nvar ok = \"x lands with (forge-neutral/18)\"\n")
	if r, _ := checkDeferralOwner(t, deferralCitesIn(t, "autoflip.go", src18)[0]); r != "" {
		t.Fatalf("forge-neutral/18 citation in autoflip.go was refused: %s", r)
	}
}
