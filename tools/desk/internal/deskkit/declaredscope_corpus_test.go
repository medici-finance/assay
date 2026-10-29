package deskkit

// Authoring-time lint for the Context `files:` declaration grammar (#3697).
//
// BriefDeclaredFiles is fail-closed: a brief whose `files:` block wraps a
// declaration across prose, hides a path behind an em dash, or ends an item
// with an unsupported tail yields NO declared scope, and every consumer of
// that scope (dispatch input keys, scope checks) refuses the brief. Until this
// test existed, such a brief was only discovered when a consumer refused it,
// long after it was authored.
//
// The repo's corpus is not yet clean, so this is a RATCHET, not a gate on
// zero: docs/streams/declared-scope-refused.txt lists every brief the parser
// refuses today. The list lives beside the corpus it describes, under
// docs/streams, so this shipping package carries no stream path of its own.
// The test fails when
//
//   - a brief NOT on that list is refused (a new or newly broken
//     declaration: fix the `files:` block, never add it to the list), or
//   - a brief ON the list now parses or no longer exists (remove the entry,
//     so the list only ever shrinks).
//
// The fix for a refused brief is to rewrite its Context `files:` block in
// the grammar BriefDeclaredFiles reads; the parser is deliberately not
// loosened to accept prose.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// briefScopeBaseline is the ratchet list, relative to the repo root.
const briefScopeBaseline = "docs/streams/declared-scope-refused.txt"

// scopeDrift is the difference between the refused briefs found under a
// repo root and the ratchet list.
type scopeDrift struct {
	New     []string // refused, not on the list: path + parser error
	Stale   []string // on the list, but now parses or is gone
	Refused int      // briefs refused in total
	Scanned int      // briefs scanned in total
}

// readScopeBaseline reads the ratchet list: one repo-relative brief path per
// line; blank lines and `#` comments are ignored.
func readScopeBaseline(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if out[line] {
			return nil, fmt.Errorf("%s: duplicate entry %s", path, line)
		}
		out[line] = true
	}
	return out, sc.Err()
}

// refusedScopeDrift runs BriefDeclaredFiles over every
// docs/streams/*/brief-*.md under root and compares the refusals with the
// baseline set.
func refusedScopeDrift(root string, baseline map[string]bool) (scopeDrift, error) {
	var d scopeDrift
	briefs, err := filepath.Glob(filepath.Join(root, "docs", "streams", "*", "brief-*.md"))
	if err != nil {
		return d, err
	}
	sort.Strings(briefs)
	refused := map[string]bool{}
	for _, f := range briefs {
		b, err := os.ReadFile(f)
		if err != nil {
			return d, err
		}
		rel, err := filepath.Rel(root, f)
		if err != nil {
			return d, err
		}
		rel = filepath.ToSlash(rel)
		d.Scanned++
		if _, perr := BriefDeclaredFiles(string(b)); perr != nil {
			d.Refused++
			refused[rel] = true
			if !baseline[rel] {
				d.New = append(d.New, rel+": "+perr.Error())
			}
		}
	}
	for rel := range baseline {
		if !refused[rel] {
			d.Stale = append(d.Stale, rel)
		}
	}
	sort.Strings(d.Stale)
	return d, nil
}

// regression: #3697
func TestBriefScopeRatchet(t *testing.T) {
	root := repoRootFromHere(t)
	baseline, err := readScopeBaseline(filepath.Join(root, filepath.FromSlash(briefScopeBaseline)))
	if err != nil {
		t.Fatalf("read ratchet list: %v", err)
	}
	d, err := refusedScopeDrift(root, baseline)
	if err != nil {
		t.Fatalf("scan briefs: %v", err)
	}
	// Three states: a scan that saw nothing is broken, not clean.
	if d.Scanned == 0 {
		t.Fatalf("could-not-check: no docs/streams/*/brief-*.md under %s", root)
	}
	for _, n := range d.New {
		t.Errorf("Context files: declaration refused (rewrite the files: block in the declaration grammar; do not add it to %s): %s", briefScopeBaseline, n)
	}
	for _, s := range d.Stale {
		t.Errorf("%s lists %s, which now parses or is gone: remove the entry so the list shrinks", briefScopeBaseline, s)
	}
	t.Logf("scanned %d briefs, %d refused, %d on the ratchet list", d.Scanned, d.Refused, len(baseline))
}

// regression: #3697
func TestBriefScopeRatchetPlanted(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, files string) {
		t.Helper()
		body := "# Brief\n\n## Context\n\n" + files + "\n## Deliverables\n\n- x\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The defect class: a declaration wrapped into prose that names a
	// second path after an Oxford comma.
	write("brief-01-wrapped.md", "files:\n- `tools/a.go`, `tools/b.go`, and `tools/c.go` are\n  edited together.\n")
	// A clean declaration that the ratchet list still (wrongly) carries.
	write("brief-02-clean.md", "files:\n- `tools/a.go`\n- `tools/b.go` — the reader\n")
	// A refused brief that is already on the list stays quiet.
	write("brief-03-known.md", "files:\n- edit `tools/a.go` and then\n  `tools/b.go`, and `tools/c.go`\n")

	baseline := map[string]bool{
		"docs/streams/example/brief-02-clean.md": true,
		"docs/streams/example/brief-03-known.md": true,
		"docs/streams/example/brief-09-gone.md":  true,
	}
	d, err := refusedScopeDrift(root, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if d.Scanned != 3 || d.Refused != 2 {
		t.Fatalf("scanned=%d refused=%d, want 3 and 2 (new=%v)", d.Scanned, d.Refused, d.New)
	}
	if len(d.New) != 1 || !strings.HasPrefix(d.New[0], "docs/streams/example/brief-01-wrapped.md: ") {
		t.Errorf("new = %v, want only brief-01-wrapped.md", d.New)
	}
	wantStale := []string{"docs/streams/example/brief-02-clean.md", "docs/streams/example/brief-09-gone.md"}
	if fmt.Sprint(d.Stale) != fmt.Sprint(wantStale) {
		t.Errorf("stale = %v, want %v", d.Stale, wantStale)
	}
}
