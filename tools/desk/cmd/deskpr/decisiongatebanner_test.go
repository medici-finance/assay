package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisiongatebanner_test.go — the gate: human PR-body banner (lifecycle-v1 §4.5).

func bannerRoot(t *testing.T, frontmatter string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "docs", "streams", "sdlc", "brief-18-binding.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("---\nbrief: sdlc/18\ntitle: t\n"+frontmatter+"---\n\n# Brief\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDecisionGateBannerInsertedNamingIssueAndState(t *testing.T) {
	cases := []struct {
		name, fm string
		want     []string
	}{
		{"unruled", "gate: human\ndecision-issue: 41\n",
			[]string{decisionGateBannerMarker + " NOT RULED", "Decision issue: #41", "Ruling: none recorded"}},
		{"ruling linked", "gate: human\ndecision-issue: 41\nruling: https://github.com/example-org/tracker/issues/41#issuecomment-9001\n",
			[]string{decisionGateBannerMarker + " RULING LINKED", "Decision issue: #41", "`ruling:` link is recorded", "checked at the status transition, not here"}},
		{"no issue filed", "gate: human\n",
			[]string{decisionGateBannerMarker + " NOT RULED", "Decision issue: none filed yet"}},
		{"quoted gate", "gate: \"human\"\ndecision-issue: 41 # filed\n",
			[]string{decisionGateBannerMarker + " NOT RULED", "Decision issue: #41"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := bannerRoot(t, c.fm)
			body := []byte("Does the thing.\n\nBrief: sdlc/18\n")
			out, added := withDecisionGateBanner(body, root, "/")
			if !added {
				t.Fatalf("no banner added for a gate: human brief")
			}
			s := string(out)
			for _, w := range append(c.want, "`sdlc/18`", "informs; it blocks nothing") {
				if !strings.Contains(s, w) {
					t.Fatalf("banner missing %q:\n%s", w, s)
				}
			}
			if !strings.HasSuffix(s, string(body)) {
				t.Fatalf("the caller's body was altered, not prefixed:\n%s", s)
			}
			// The trailer still parses as the one Brief: trailer after insertion.
			trs, err := deskkit.ParseTrailers(out)
			if err != nil || len(trs) != 1 || trs[0].Kind != deskkit.TrailerBrief {
				t.Fatalf("trailers after banner = %v, %v", trs, err)
			}
			// Idempotent: a body already carrying the banner is left alone.
			again, added2 := withDecisionGateBanner(out, root, "/")
			if added2 || string(again) != s {
				t.Fatalf("a second pass stacked another banner")
			}
		})
	}
}

func TestDecisionGateBannerLeavesOtherBodiesAlone(t *testing.T) {
	cases := []struct{ name, fm, body string }{
		{"gate model", "gate: model\ndecision-issue: 41\n", "x\nBrief: sdlc/18\n"},
		{"no gate key", "", "x\nBrief: sdlc/18\n"},
		{"issue trailer", "gate: human\n", "x\nIssue: #12\n"},
		{"authors trailer", "gate: human\n", "x\nAuthors: sdlc/18\n"},
		{"brief not found", "gate: human\n", "x\nBrief: sdlc/19\n"},
		{"no trailer", "gate: human\n", "x\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := bannerRoot(t, c.fm)
			out, added := withDecisionGateBanner([]byte(c.body), root, "/")
			if added || string(out) != c.body {
				t.Fatalf("body changed:\n%s", out)
			}
		})
	}
}

// TestDecisionGateBannerRelativeRoot: a relative --root resolves against the work dir,
// the same way requireTrailer resolves it.
func TestDecisionGateBannerRelativeRoot(t *testing.T) {
	root := bannerRoot(t, "gate: human\n")
	_, added := withDecisionGateBanner([]byte("x\nBrief: sdlc/18\n"), ".", root)
	if !added {
		t.Fatal("relative --root did not resolve against the work dir")
	}
}

// TestCreateInsertsDecisionGateBanner: `deskpr create` for a PR delivering a gate: human
// brief succeeds and sends the body WITH the banner — it informs and never refuses.
func TestCreateInsertsDecisionGateBanner(t *testing.T) {
	work := newBaseFixture(t)
	writeFile(t, filepath.Join(work, "docs", "streams", "fixture", "brief-01-test.md"),
		"---\nschema: brief-v1\nbrief: fixture/01\ntitle: fixture brief\ngate: human\ndecision-issue: 41\n---\n\nFixture brief.\n")
	mustGit(t, work, "add", "docs")
	mustGit(t, work, "commit", "-m", "gate the fixture brief")
	withEnv(t, work)
	stderr := withStderrCapture(t)

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0 (the banner blocks nothing); stderr: %s", rc, stderr.String())
	}
	if curForge.created == nil {
		t.Fatal("no draft change created")
	}
	b := curForge.created.Body
	if !strings.HasPrefix(b, "> **"+decisionGateBannerMarker+" NOT RULED.**") || !strings.Contains(b, "Decision issue: #41") {
		t.Fatalf("created body does not open with the banner:\n%s", b)
	}
	if !strings.Contains(stderr.String(), "added the gate: human decision banner") {
		t.Fatalf("no notice that the banner was added; stderr: %s", stderr.String())
	}
}

// TestCreateNoBannerForModelGate: the default fixture brief carries no gate: human, and
// its created body is exactly what the caller supplied (plus the forge-side trailer).
func TestCreateNoBannerForModelGate(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}
	if curForge.created == nil || strings.Contains(curForge.created.Body, decisionGateBannerMarker) {
		t.Fatalf("banner on a brief that is not gate: human: %+v", curForge.created)
	}
}
