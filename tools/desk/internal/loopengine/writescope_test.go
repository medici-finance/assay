package loopengine

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestDeriveWriteScopes_FromContextFiles covers the default derivation from a Context `files:`
// list: repo-relative prefixes, a sibling `../<repo>/…` entry, and glob trimming — the
// normalization the spec §4.1.1 requires.
func TestDeriveWriteScopes_FromContextFiles(t *testing.T) {
	brief := "---\nbrief: x/01\ngate: model\n---\n\n## Context\n\nfiles:\n" +
		"- `../assay/tools/desk/` (home) — the surface\n" +
		"- `docs/streams/*/brief-*.md` — the scope source\n" +
		"- `spec/brief-v1.md` — a file\n\n## Task\ndo\n"
	got := DeriveWriteScopes(brief)
	if !got.Derivable {
		t.Fatalf("expected derivable, got could-not-derive")
	}
	want := []WriteScope{
		{Repo: "assay", Prefix: "tools/desk/"},
		{Repo: "", Prefix: "docs/streams/"}, // glob trimmed back to the dir prefix
		{Repo: "", Prefix: "spec/brief-v1.md"},
	}
	if !reflect.DeepEqual(got.Scopes, want) {
		t.Fatalf("scopes:\n got %+v\nwant %+v", got.Scopes, want)
	}
}

// TestDeriveWriteScopes_InlineFilesForm covers the single-line `files: cmd/fanoutloop/` shape
// that the existing board fixtures use.
func TestDeriveWriteScopes_InlineFilesForm(t *testing.T) {
	brief := "---\nbrief: x/01\n---\n\n## Context\nfiles: cmd/fanoutloop/\nout-of-repo files: `~/.claude/x`\n\n## Task\ndo\n"
	got := DeriveWriteScopes(brief)
	if !got.Derivable || len(got.Scopes) != 1 || got.Scopes[0].Prefix != "cmd/fanoutloop/" {
		t.Fatalf("inline files form not derived: %+v", got)
	}
}

// TestDeriveWriteScopes_OverrideReplaces covers the `write-scopes:` frontmatter override
// REPLACING the derived set (spec §3.2 / §4.1.1), in both block and inline flow forms.
func TestDeriveWriteScopes_OverrideReplaces(t *testing.T) {
	block := "---\nbrief: x/01\nwrite-scopes:\n  - tools/desk/internal/\n  - ../assay/cmd/\n---\n\n## Context\nfiles: something/else/\n\n## Task\ndo\n"
	got := DeriveWriteScopes(block)
	want := []WriteScope{{Repo: "", Prefix: "tools/desk/internal/"}, {Repo: "assay", Prefix: "cmd/"}}
	if !got.Derivable || !reflect.DeepEqual(got.Scopes, want) {
		t.Fatalf("override (block) not honored:\n got %+v\nwant %+v", got.Scopes, want)
	}
	// The override REPLACES, never merges: the Context `files:` entry must not appear.
	for _, s := range got.Scopes {
		if strings.HasPrefix(s.Prefix, "something") {
			t.Fatalf("override did not replace the derived set: %+v", got.Scopes)
		}
	}
	inline := "---\nbrief: x/01\nwrite-scopes: [a/b/, c/d/]\n---\n\n## Context\nfiles: e/f/\n\n## Task\ndo\n"
	gi := DeriveWriteScopes(inline)
	wi := []WriteScope{{Prefix: "a/b/"}, {Prefix: "c/d/"}}
	if !gi.Derivable || !reflect.DeepEqual(gi.Scopes, wi) {
		t.Fatalf("override (inline flow) not honored:\n got %+v\nwant %+v", gi.Scopes, wi)
	}
}

// TestDeriveWriteScopes_CouldNotDerive is the three-state honesty guard: a brief with no
// override and no parseable `files:` list is could-not-derive, NEVER an empty-but-clear set.
func TestDeriveWriteScopes_CouldNotDerive(t *testing.T) {
	for name, brief := range map[string]string{
		"no files line":     "---\nbrief: x/01\n---\n\n## Context\n\nprose only, no files line\n\n## Task\ndo\n",
		"no frontmatter":    "## Context\n\nprose only\n",
		"empty files block": "---\nbrief: x/01\n---\n\n## Context\nfiles:\n\n## Task\ndo\n",
	} {
		got := DeriveWriteScopes(brief)
		if got.Derivable {
			t.Fatalf("%s: expected could-not-derive, got derivable=%+v", name, got)
		}
	}
}

// TestOverlap_SharedPrefix proves the component-wise prefix overlap: a shared directory
// ancestor overlaps and reports the SHORTER (shared) prefix; a sibling-repo scope only
// overlaps a same-repo scope; partial component names do NOT overlap.
func TestOverlap_SharedPrefix(t *testing.T) {
	a := WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "internal/loopengine/engine.go"}}}
	b := WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "internal/loopengine/"}}}
	if got := a.Overlap(b); !reflect.DeepEqual(got, []string{"internal/loopengine/"}) {
		t.Fatalf("shared-prefix overlap: got %v want [internal/loopengine/]", got)
	}
	// A partial component name must NOT be read as a prefix (internal/loop vs internal/loopengine).
	c := WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "internal/loop"}}}
	if got := b.Overlap(c); len(got) != 0 {
		t.Fatalf("partial-component false overlap: got %v want none", got)
	}
	// Repo must match: a sibling-repo scope never overlaps a same-repo one on the same path.
	sib := WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Repo: "assay", Prefix: "internal/loopengine/"}}}
	if got := b.Overlap(sib); len(got) != 0 {
		t.Fatalf("cross-repo false overlap: got %v want none", got)
	}
	if got := sib.Overlap(WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Repo: "assay", Prefix: "internal/"}}}); !reflect.DeepEqual(got, []string{"../assay/internal/"}) {
		t.Fatalf("sibling-repo overlap: got %v want [../assay/internal/]", got)
	}
}

// TestOverlap_CouldNotDeriveNeverClear proves a could-not-derive set NEVER reports overlaps as
// clear: Overlap returns nil (there is nothing to compare), and it is the WARNING layer, not
// Overlap, that surfaces the could-not-derive state.
func TestOverlap_CouldNotDeriveNeverClear(t *testing.T) {
	unknown := WriteScopeSet{Derivable: false}
	known := WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "a/"}}}
	if got := unknown.Overlap(known); got != nil {
		t.Fatalf("could-not-derive Overlap must be nil, got %v", got)
	}
	if got := known.Overlap(unknown); got != nil {
		t.Fatalf("Overlap against could-not-derive must be nil, got %v", got)
	}
}

// TestWriteOverlapWarnings covers the warning-line assembly: overlap lines name both ids and
// the shared prefix; a could-not-derive candidate is reported (never rounded to clear); a
// disjoint candidate produces nothing; self-comparison is skipped.
func TestWriteOverlapWarnings(t *testing.T) {
	cand := Item{ID: "eng-a/09", WriteScopes: WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "internal/svc/"}}}}
	inflight := Item{ID: "eng-b/11", WriteScopes: WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "internal/svc/handler.go"}}}}
	disjoint := Item{ID: "other/01", WriteScopes: WriteScopeSet{Derivable: true, Scopes: []WriteScope{{Prefix: "docs/site/"}}}}
	unknown := Item{ID: "mystery/02", WriteScopes: WriteScopeSet{Derivable: false}}

	got := WriteOverlapWarnings([]Item{cand, disjoint, unknown}, []Item{inflight})
	wantOverlap := "WRITE-OVERLAP: eng-a/09 ~ eng-b/11 on internal/svc/"
	if !contains(got, wantOverlap) {
		t.Fatalf("missing overlap line %q in %v", wantOverlap, got)
	}
	if !contains(got, "mystery/02: scopes: could-not-derive") {
		t.Fatalf("could-not-derive not reported: %v", got)
	}
	for _, l := range got {
		if strings.Contains(l, "other/01") {
			t.Fatalf("disjoint candidate produced a line: %q", l)
		}
	}

	// Self-comparison is skipped: an item cannot overlap itself.
	self := WriteOverlapWarnings([]Item{cand}, []Item{cand})
	for _, l := range self {
		if strings.HasPrefix(l, "WRITE-OVERLAP") {
			t.Fatalf("self-overlap produced a warning: %q", l)
		}
	}
}

// TestInFlightClaimScopes_ReadsLocalRefs proves the offline in-flight reader: a local
// refs/dispatch claim resolves to the named brief under the root and carries its derived
// scopes, and a non-git root degrades to nil (advisory, never a failure).
func TestInFlightClaimScopes_ReadsLocalRefs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	writeBrief(t, root, "beta", "02", "files:\n- `internal/loopengine/`\n")

	// Non-git root: nil (advisory).
	if got := InFlightClaimScopes(root); got != nil {
		t.Fatalf("non-git root should yield nil in-flight items, got %v", got)
	}

	gitInit(t, root)
	gitRun(t, root, "update-ref", "refs/dispatch/repo--beta--02", "HEAD")

	got := InFlightClaimScopes(root)
	if len(got) != 1 || got[0].ID != "beta/02" {
		t.Fatalf("in-flight claim not resolved: %+v", got)
	}
	if !got[0].WriteScopes.Derivable || got[0].WriteScopes.Scopes[0].Prefix != "internal/loopengine/" {
		t.Fatalf("in-flight scopes not derived: %+v", got[0].WriteScopes)
	}
}

// --- test helpers -----------------------------------------------------------------------------

func writeBrief(t *testing.T, root, stream, num, filesBlock string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nbrief: " + stream + "/" + num + "\ngate: model\n---\n\n## Context\n\n" + filesBlock + "\n## Task\ndo\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-"+num+"-x.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	full := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
