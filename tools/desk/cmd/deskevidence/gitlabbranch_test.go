package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabbranch_test.go — #6610: on GitLab the branch lane of the board-match hold has no
// CompareRefs. A branch tip that descends from the landing home's HEAD and differs from it
// only in Evidence paths is read through the GitLab adapter's exact descendant read and
// judged like a GitHub compare; every other shape stays could-not-check.

// glBranch serves, over the real GitLab adapter, the tip commit of every ref, a merge base
// and a compare between the home's HEAD and the tip, and (through glFiles) file reads.
type glBranch struct {
	files     *glFiles
	tip       string
	mergeBase string
	diffs     []map[string]any
	timeout   bool
	compares  int
}

func (s *glBranch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	esc := r.URL.EscapedPath()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(esc, "/repository/commits/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.tip})
	case strings.HasSuffix(esc, "/repository/merge_base"):
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.mergeBase})
	case strings.HasSuffix(esc, "/repository/compare"):
		s.compares++
		_ = json.NewEncoder(w).Encode(map[string]any{"commits": []any{}, "diffs": s.diffs, "compare_timeout": s.timeout})
	default:
		s.files.ServeHTTP(w, r)
	}
}

func glDiff(oldPath, newPath string, added, renamed, deleted bool) map[string]any {
	return map[string]any{"old_path": oldPath, "new_path": newPath, "new_file": added, "renamed_file": renamed, "deleted_file": deleted}
}

// regression: #6610
func TestGitLabEvidenceBranch(t *testing.T) {
	const target = "docs/streams/sdlc/README.md"
	const brief = "docs/streams/sdlc/brief-18.md"
	const record = "docs/streams/verify-outcomes/sdlc/18.json"
	tip := strings.Repeat("cd", 20)
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	setup := func(t *testing.T, mb string, diffs ...map[string]any) (deskkit.Forge, *glBranch, string, string) {
		t.Helper()
		root, head := freshHome(t)
		if mb == "" {
			mb = head
		}
		f := &fakeForge{}
		f.setFile(brief, briefOf("human", "\n## Evidence\n\n| 1 | pass | ran |\n"))
		s := &glBranch{files: &glFiles{f: f}, tip: tip, mergeBase: mb, diffs: diffs}
		srv := httptest.NewServer(s)
		t.Cleanup(srv.Close)
		gl := deskkit.OutboundChecked(&deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}, "verifier")
		return gl, s, root, head
	}
	t.Run("Evidence-only descendant lands", func(t *testing.T) {
		fg, s, root, _ := setup(t, "", glDiff(brief, brief, false, false, false), glDiff(record, record, true, false, false), glDiff(target, target, false, false, false))
		edits, err := decisionGateBranchBoard(fg, fr, root, "main", target)
		if err != nil {
			t.Fatalf("got %v; want the Evidence-only branch admitted", err)
		}
		if s.compares != 1 || len(edits) != 1 || edits[0].Path != brief || !strings.Contains(string(edits[0].Content), "## Evidence") {
			t.Fatalf("compares %d, edits %+v; want the changed brief read at the tip for the judge", s.compares, edits)
		}
	})
	refuse := []struct {
		name, want string
		mb         string
		timeout    bool
		diffs      []map[string]any
	}{
		{name: "a non-Evidence path", want: "not an Evidence path", diffs: []map[string]any{glDiff(brief, brief, false, false, false), glDiff("tools/x.go", "tools/x.go", false, false, false)}},
		{name: "a board README on the branch", want: "not an Evidence path", diffs: []map[string]any{glDiff("docs/streams/other/README.md", "docs/streams/other/README.md", false, false, false)}},
		{name: "a removed brief", want: "as removed", diffs: []map[string]any{glDiff(brief, brief, false, false, true)}},
		{name: "a renamed brief", want: "as renamed", diffs: []map[string]any{glDiff("docs/streams/sdlc/brief-17.md", brief, false, true, false)}},
		{name: "a dot-dot path", want: "not an Evidence path", diffs: []map[string]any{glDiff("docs/streams/../x/brief-1.md", "docs/streams/../x/brief-1.md", false, false, false)}},
		{name: "an unclean brief path", want: "not an Evidence path", diffs: []map[string]any{glDiff("docs/streams/sdlc/./brief-18.md", "docs/streams/sdlc/./brief-18.md", false, false, false)}},
		{name: "a diverged home", want: "not an ancestor", mb: strings.Repeat("ef", 20), diffs: []map[string]any{glDiff(brief, brief, false, false, false)}},
		{name: "a timed-out compare", want: "compare_timeout", timeout: true, diffs: []map[string]any{glDiff(brief, brief, false, false, false)}},
	}
	for _, c := range refuse {
		t.Run(c.name, func(t *testing.T) {
			fg, s, root, _ := setup(t, c.mb, c.diffs...)
			s.timeout = c.timeout
			edits, err := decisionGateBranchBoard(fg, fr, root, "main", target)
			if err == nil || !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got edits %+v, err %v; want could-not-check naming %q", edits, err, c.want)
			}
		})
	}
	t.Run("a listing at the cap", func(t *testing.T) {
		var diffs []map[string]any
		for i := 0; i < decisionGateCompareFileCap; i++ {
			p := "docs/streams/verify-outcomes/sdlc/" + strings.Repeat("a", i%7+1) + string(rune('a'+i%26)) + ".json"
			diffs = append(diffs, glDiff(p, p, true, false, false))
		}
		fg, _, root, _ := setup(t, "", diffs...)
		if _, err := decisionGateBranchBoard(fg, fr, root, "main", target); err == nil || !strings.Contains(err.Error(), "cannot be shown complete") {
			t.Fatalf("got %v; want could-not-check at the listing cap", err)
		}
	})
}
