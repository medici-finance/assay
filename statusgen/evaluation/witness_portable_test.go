package evaluation

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// witness_portable_test.go — statusgen/36 Verify row 9. The digest is computed
// from git object ids with the repository's own attributes applied, so the
// same content checked out with LF and with CRLF digests the same, from the
// committed listing and from the working tree. The test drives a real local
// git (no network, no global or system configuration) with the same reads the
// statusgen port makes (ls-tree, check-attr --source, hash-object
// --stdin-paths); the evaluation library itself starts no process.

func portableGit(t *testing.T, dir string, stdin []byte, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.attributesFile=" + os.DevNull, "-c", "core.quotePath=false"}, args...)...)
	cmd.Dir = dir
	home := goquiet.Home(t) // a scratch HOME with go telemetry off (TestGoPackagesQuietHome)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_ATTR_NOSYSTEM=1",
		"GIT_NO_REPLACE_OBJECTS=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-10-25T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-25T00:00:00Z",
		"GIT_TERMINAL_PROMPT=0")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return out
}

// portableContent reads one checkout the way the statusgen port does and
// returns the content computed from its committed listing and from its
// working tree, plus the working tree's dirty paths.
func portableContent(t *testing.T, dir string, spec WitnessSpec) (committed, worktree WitnessContent, dirty []string) {
	t.Helper()
	var listing []TreeBlob
	for _, e := range strings.Split(strings.TrimRight(string(portableGit(t, dir, nil, "ls-tree", "-r", "-z", "--full-tree", "HEAD")), "\x00"), "\x00") {
		meta, name, ok := strings.Cut(e, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			t.Fatalf("ls-tree entry %q", e)
		}
		listing = append(listing, TreeBlob{Mode: f[0], OID: f[2], Path: name})
	}
	var in bytes.Buffer
	var paths bytes.Buffer
	for _, e := range listing {
		in.WriteString(e.Path + "\x00")
		paths.WriteString(e.Path + "\n")
	}
	fields := strings.Split(strings.TrimRight(string(portableGit(t, dir, in.Bytes(), "check-attr", "-a", "-z", "--stdin", "--source=HEAD")), "\x00"), "\x00")
	attrs := map[string][]string{}
	for i := 0; i+2 < len(fields); i += 3 {
		attrs[fields[i]] = append(attrs[fields[i]], fields[i+1]+": "+fields[i+2])
	}
	ids := strings.Fields(string(portableGit(t, dir, paths.Bytes(), "hash-object", "--stdin-paths")))
	if len(ids) != len(listing) {
		t.Fatalf("hash-object returned %d ids for %d paths", len(ids), len(listing))
	}
	wt := map[string]string{}
	for i, e := range listing {
		wt[e.Path] = ids[i]
	}
	wl, dirty := WorktreeListing(listing, wt)
	return ComputeWitnessContent(spec, listing, attrs), ComputeWitnessContent(spec, wl, attrs), dirty
}

func sameContent(a, b WitnessContent) bool {
	ea, _, _ := contentKey(a)
	eb, _, _ := contentKey(b)
	return ea == eb
}

// contentKey renders a content's digest-bearing fields in one
// fixed order for comparison.
func contentKey(c WitnessContent) (string, int, int) {
	var b strings.Builder
	b.WriteString(c.Scope + "|" + c.TreeDigest + "|")
	for _, m := range []map[string]string{c.Inputs, c.Implicit, c.Attributes} {
		for _, k := range sortedKeysOf(m) {
			b.WriteString(k + "=" + m[k] + ";")
		}
		b.WriteString("|")
	}
	return b.String(), len(c.Inputs), len(c.Implicit)
}

// regression: #3658
func TestWitnessDigestPortable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required: %v", err)
	}
	lf := t.TempDir()
	files := map[string]string{
		".gitattributes":       "*.txt text eol=crlf\n",
		"go.work":              "go 1.25\n",
		"tools/x/go.mod":       "module example.test/x\n\ngo 1.25\n",
		"tools/x/go.sum":       "",
		"tools/x/pkg/a.go":     "package pkg\n\nfunc A() int { return 1 }\n",
		"tools/x/pkg/data.txt": "line one\nline two\n",
		"README.md":            "readme\n",
	}
	for p, c := range files {
		full := filepath.Join(lf, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	portableGit(t, lf, nil, "init", "-q", "-b", "main")
	portableGit(t, lf, nil, "-c", "core.autocrlf=false", "add", "-A")
	portableGit(t, lf, nil, "-c", "core.autocrlf=false", "commit", "-q", "-m", "fixture")

	parent := t.TempDir()
	portableGit(t, parent, nil, "-c", "core.autocrlf=true", "clone", "-q", "--no-local", lf, "crlf")
	crlf := filepath.Join(parent, "crlf")
	portableGit(t, crlf, nil, "config", "core.autocrlf", "true")
	portableGit(t, crlf, nil, "rm", "-q", "--cached", "-r", ".")
	portableGit(t, crlf, nil, "reset", "-q", "--hard")

	// The controls are real: the CRLF checkout holds CRLF bytes, and its raw
	// bytes hash differently from the committed blob.
	for _, p := range []string{"tools/x/pkg/data.txt", "tools/x/pkg/a.go"} {
		raw, err := os.ReadFile(filepath.Join(crlf, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, []byte("\r\n")) {
			t.Fatalf("fixture: %s was not checked out with CRLF: %q", p, raw)
		}
		noFilter := strings.TrimSpace(string(portableGit(t, crlf, nil, "hash-object", "--no-filters", "--", p)))
		filtered := strings.TrimSpace(string(portableGit(t, crlf, nil, "hash-object", "--", p)))
		if noFilter == filtered {
			t.Fatalf("fixture: %s hashes the same with and without filters, so the CRLF control proves nothing", p)
		}
	}

	for _, spec := range []WitnessSpec{
		{Declared: []string{"tools/x/pkg/"}},
		{Declared: []string{"tools/x/pkg/data.txt"}},
		{WholeTree: true},
	} {
		name := strings.Join(spec.Declared, ",")
		if spec.WholeTree {
			name = "whole-tree"
		}
		lfC, lfW, lfDirty := portableContent(t, lf, spec)
		crC, crW, crDirty := portableContent(t, crlf, spec)
		if len(lfDirty) != 0 || len(crDirty) != 0 {
			t.Errorf("%s: a clean checkout reports dirty paths: LF %v, CRLF %v", name, lfDirty, crDirty)
		}
		for label, c := range map[string]WitnessContent{"LF working tree": lfW, "CRLF committed listing": crC, "CRLF working tree": crW} {
			if !sameContent(lfC, c) {
				got, _, _ := contentKey(c)
				want, _, _ := contentKey(lfC)
				t.Errorf("%s: the %s digests differently from the LF committed listing:\n got %s\nwant %s", name, label, got, want)
			}
		}
		if !spec.WholeTree && len(lfC.Inputs) == 0 {
			t.Errorf("%s: the content names no input", name)
		}
		if spec.WholeTree && lfC.TreeDigest == "" {
			t.Errorf("%s: no whole-tree digest", name)
		}
	}

	// A real edit in the CRLF working tree is seen, so the equality above is not
	// a digest that ignores content.
	edited := filepath.Join(crlf, "tools", "x", "pkg", "data.txt")
	if err := os.WriteFile(edited, []byte("line one\r\nline 2\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	crC, crW, dirty := portableContent(t, crlf, WitnessSpec{Declared: []string{"tools/x/pkg/"}})
	if sameContent(crC, crW) || len(dirty) != 1 || dirty[0] != "tools/x/pkg/data.txt" {
		t.Errorf("an edited CRLF working tree: dirty %v, content unchanged %v", dirty, sameContent(crC, crW))
	}

	// A backslash path is refused, never normalised silently.
	for _, p := range []string{`tools\x\pkg\a.go`, `tools/x\pkg/a.go`} {
		if witnessPathOK(p) == nil {
			t.Errorf("witnessPathOK(%q) accepted a backslash path", p)
		}
		if entries, found, problem := ParseInputsDeclaration("inputs: `" + p + "`\n"); found || problem == "" || len(entries) != 0 {
			t.Errorf("ParseInputsDeclaration(%q) = %v %v %q, want unparseable", p, entries, found, problem)
		}
		if lp, ok := literalPath("", p); ok {
			t.Errorf("literalPath(%q) = %q, want no candidate", p, lp)
		}
	}
	fx := newV2fx(t)
	rec := fx.defaultRecord(fx.row)
	canonical, _, err := EncodeWitnessRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(canonical), `"tools/x/pkg/a.go"`, `"tools\\x\\pkg\\a.go"`, 1)
	if _, _, err := ParseWitnessRecord([]byte(bad)); err == nil || !strings.Contains(err.Error(), "slash-separated") {
		t.Errorf("a record keyed by a backslash path: err %v, want a slash-separated refusal", err)
	}
	rec.Inputs[`tools\x\pkg\b.go`] = rec.Inputs["tools/x/pkg/a.go"]
	if _, _, err := EncodeWitnessRecord(rec); err == nil {
		t.Error("EncodeWitnessRecord accepted a backslash input key")
	}

	// The working-tree overlay is independent of the order git answers in.
	committed := []TreeBlob{{"100644", "aa", "b"}, {"100644", "bb", "a"}}
	l1, d1 := WorktreeListing(committed, map[string]string{"a": "cc", "c": "dd"})
	l2, d2 := WorktreeListing([]TreeBlob{committed[1], committed[0]}, map[string]string{"c": "dd", "a": "cc"})
	if TreeDigest(l1) != TreeDigest(l2) || strings.Join(d1, ",") != "a,c" || strings.Join(d2, ",") != "a,c" {
		t.Errorf("WorktreeListing depends on order: %v %v", d1, d2)
	}
}
