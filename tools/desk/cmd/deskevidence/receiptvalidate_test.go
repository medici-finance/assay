package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func rvGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func rvWriteAndAdd(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rvGitIn(t, dir, "add", rel)
}

// briefWithFiles builds a minimal brief-v2 body with a `## Context` `files:` block naming the
// given backtick paths, one bullet each.
func briefWithFiles(paths ...string) string {
	var b strings.Builder
	b.WriteString("---\nbrief: x\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n# Brief\n\n## Context\n\nfiles:\n")
	for _, p := range paths {
		b.WriteString("- **edit** `" + p + "` — thing.\n")
	}
	b.WriteString("\nfacts: none.\n\n## Verify\n\n| # | Command | Expect |\n\n## Evidence\n<!-- appended at verification time -->\n")
	return b.String()
}

// TestReceiptInputsCoverDeliverables is Verify row 13.
func TestReceiptInputsCoverDeliverables(t *testing.T) {
	dir := t.TempDir()
	rvGitIn(t, dir, "init", "-q", "-b", "main")
	rvWriteAndAdd(t, dir, "docs/streams/example-stream/brief-01-x.md", briefWithFiles("foo/bar.go", "foo/baz.go"))
	rvWriteAndAdd(t, dir, "foo/bar.go", "package foo\n")
	rvWriteAndAdd(t, dir, "foo/baz.go", "package foo\n")
	rvGitIn(t, dir, "commit", "-qm", "base")
	sha := rvGitIn(t, dir, "rev-parse", "HEAD")

	t.Run("inputs holding only brief+tool for a brief naming two existing files exits 5 naming the missing path", func(t *testing.T) {
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", SHA: sha, Inputs: map[string]string{"tool": "v1"}}
		err := validateReceiptInputsCoverDeliverables(dir, wr)
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("exit = %v, want Refused; err=%v", deskkit.ExitCodeOf(err), err)
		}
		if !strings.Contains(err.Error(), "foo/bar.go") {
			t.Fatalf("err = %v, want it to name the missing path", err)
		}
	})

	t.Run("one key per file passes", func(t *testing.T) {
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", SHA: sha, Inputs: map[string]string{
			"tool": "v1", "file:foo/bar.go": "h1", "file:foo/baz.go": "h2",
		}}
		if err := validateReceiptInputsCoverDeliverables(dir, wr); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	rvWriteAndAdd(t, dir, "docs/streams/example-stream/brief-02-x.md", briefWithFiles("foo/notyet.go"))
	rvGitIn(t, dir, "commit", "-qm", "brief 02 base")
	sha2 := rvGitIn(t, dir, "rev-parse", "HEAD")

	t.Run("a (planned) path absent at the record's sha is not required", func(t *testing.T) {
		wr := deskkit.WakeReceipt{Brief: "example-stream/02", SHA: sha2, Inputs: map[string]string{"tool": "v1"}}
		if err := validateReceiptInputsCoverDeliverables(dir, wr); err != nil {
			t.Fatalf("planned/absent path must not be required: %v", err)
		}
	})

	rvWriteAndAdd(t, dir, "docs/streams/example-stream/brief-03-x.md", briefWithFiles("foo/"))
	rvGitIn(t, dir, "commit", "-qm", "brief 03 base")
	sha3 := rvGitIn(t, dir, "rev-parse", "HEAD")

	t.Run("a directory entry needs one file: key under it", func(t *testing.T) {
		missing := deskkit.WakeReceipt{Brief: "example-stream/03", SHA: sha3, Inputs: map[string]string{"tool": "v1"}}
		if err := validateReceiptInputsCoverDeliverables(dir, missing); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("directory with no nested file: key: exit = %v, want Refused (err=%v)", deskkit.ExitCodeOf(err), err)
		}
		covered := deskkit.WakeReceipt{Brief: "example-stream/03", SHA: sha3, Inputs: map[string]string{"tool": "v1", "file:foo/bar.go": "h1"}}
		if err := validateReceiptInputsCoverDeliverables(dir, covered); err != nil {
			t.Fatalf("directory covered by a nested file: key must pass: %v", err)
		}
	})

	t.Run("the brief is read at the record's sha, not the working tree", func(t *testing.T) {
		wtDir := t.TempDir()
		rvGitIn(t, wtDir, "init", "-q", "-b", "main")
		rvWriteAndAdd(t, wtDir, "docs/streams/example-stream/brief-01-x.md", briefWithFiles("foo/bar.go"))
		rvWriteAndAdd(t, wtDir, "foo/bar.go", "package foo\n")
		rvGitIn(t, wtDir, "commit", "-qm", "base")
		atSHA := rvGitIn(t, wtDir, "rev-parse", "HEAD")

		// The brief is edited to declare a DIFFERENT file in a LATER commit — the working
		// tree's current copy of the brief now disagrees with the copy at atSHA.
		rvWriteAndAdd(t, wtDir, "docs/streams/example-stream/brief-01-x.md", briefWithFiles("foo/different.go"))
		rvWriteAndAdd(t, wtDir, "foo/different.go", "package foo\n")
		rvGitIn(t, wtDir, "commit", "-qm", "brief edited later")

		// A receipt covering the file declared AT atSHA (foo/bar.go, NOT foo/different.go)
		// must pass when judged against atSHA — proving the read is at the sha, not HEAD.
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", SHA: atSHA, Inputs: map[string]string{"tool": "v1", "file:foo/bar.go": "h1"}}
		if err := validateReceiptInputsCoverDeliverables(wtDir, wr); err != nil {
			t.Fatalf("brief must be read AT sha, not the working tree: %v", err)
		}
	})
}

// rvBlockerForge is the minimal fake Forge TestReceiptBlockerRef needs: GetIssue/GetIssueTyped
// for issue/PR refs, RunStatus for a workflow-run ref.
type rvBlockerForge struct {
	deskkit.Forge
	known map[string]bool // "owner/name#N" or "owner/name#run:ID" -> exists
	err   error           // forced read error for every lookup
}

func (f *rvBlockerForge) notFound() error {
	return deskkit.Unverifiable("not found", &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "x"})
}

func (f *rvBlockerForge) GetIssue(repo deskkit.ForgeRepo, number int) (*deskkit.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := fmt.Sprintf("%s/%s#%d", repo.Owner, repo.Name, number)
	if f.known[key] {
		return &deskkit.Issue{Number: number}, nil
	}
	return nil, f.notFound()
}

func (f *rvBlockerForge) GetIssueTyped(repo deskkit.ForgeRepo, number int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	return f.GetIssue(repo, number)
}

func (f *rvBlockerForge) RunStatus(repo deskkit.ForgeRepo, run deskkit.RunRef) (*deskkit.RunState, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := fmt.Sprintf("%s/%s#run:%s", repo.Owner, repo.Name, run.ID)
	if f.known[key] {
		return &deskkit.RunState{Status: "completed"}, nil
	}
	return nil, f.notFound()
}

// TestReceiptBlockerRef is Verify row 14.
func TestReceiptBlockerRef(t *testing.T) {
	t.Run("free text, an action sentence and an empty value each exit 5", func(t *testing.T) {
		f := &rvBlockerForge{known: map[string]bool{}}
		for _, raw := range []string{"desk to file", "action: fix the widget", ""} {
			err := validateReceiptBlockerRef(f, "example-org", "example", raw)
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("blocker_ref %q: exit = %v, want Refused (err=%v)", raw, deskkit.ExitCodeOf(err), err)
			}
		}
	})

	t.Run("a bare #N, owner/repo#N and an issue URL pass when the stubbed forge has them", func(t *testing.T) {
		f := &rvBlockerForge{known: map[string]bool{
			"example-org/example#1546": true,
			"other-org/other#12":       true,
		}}
		cases := []string{
			"#1546",
			"other-org/other#12",
			"https://github.com/other-org/other/issues/12",
		}
		for _, raw := range cases {
			if err := validateReceiptBlockerRef(f, "example-org", "example", raw); err != nil {
				t.Fatalf("blocker_ref %q: want nil, got %v", raw, err)
			}
		}
	})

	t.Run("a well-formed #999999 the forge does not have exits 5", func(t *testing.T) {
		f := &rvBlockerForge{known: map[string]bool{}}
		err := validateReceiptBlockerRef(f, "example-org", "example", "#999999")
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("exit = %v, want Refused (err=%v)", deskkit.ExitCodeOf(err), err)
		}
	})

	t.Run("a forge read error is could-not-check (exit 6)", func(t *testing.T) {
		f := &rvBlockerForge{err: deskkit.Unverifiable("transport error", nil)}
		err := validateReceiptBlockerRef(f, "example-org", "example", "#1546")
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %v, want Unverifiable (err=%v)", deskkit.ExitCodeOf(err), err)
		}
	})
}

// rvBriefHashForge is the minimal fake Forge TestReceiptBriefHashAsLanded needs: ReadFile at a
// ref.
type rvBriefHashForge struct {
	deskkit.Forge
	content map[string][]byte // "branch:path" -> content
	err     error
}

func (f *rvBriefHashForge) ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := in.Ref + ":" + in.File
	c, ok := f.content[key]
	if !ok {
		return nil, deskkit.Unverifiable("not found", &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: in.File})
	}
	return &deskkit.FileContent{Content: c, Exists: true}, nil
}

// TestReceiptBriefHashAsLanded is Verify row 15.
func TestReceiptBriefHashAsLanded(t *testing.T) {
	dir := t.TempDir()
	rvGitIn(t, dir, "init", "-q", "-b", "main")
	rvWriteAndAdd(t, dir, "docs/streams/example-stream/brief-01-x.md", briefWithFiles("foo/bar.go"))
	rvGitIn(t, dir, "commit", "-qm", "base")

	preEvidence := []byte(briefWithFiles("foo/bar.go"))
	asLanded := []byte(briefWithFiles("foo/bar.go") + "\n<!-- evidence appended by this landing -->\n")

	t.Run("a brief revision equal to the pre-Evidence copy exits 5, naming the path and both hashes", func(t *testing.T) {
		fg := &rvBriefHashForge{content: map[string][]byte{"main:docs/streams/example-stream/brief-01-x.md": asLanded}}
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", Inputs: map[string]string{
			"file:docs/streams/example-stream/brief-01-x.md": deskkit.Sha256Hex(preEvidence),
		}}
		err := validateReceiptBriefHashAsLanded(fg, deskkit.ForgeRepo{Owner: "example-org", Name: "example"}, "main", dir, wr)
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("exit = %v, want Refused (err=%v)", deskkit.ExitCodeOf(err), err)
		}
		if !strings.Contains(err.Error(), "docs/streams/example-stream/brief-01-x.md") {
			t.Fatalf("err = %v, want it to name the brief path", err)
		}
	})

	t.Run("the hash of the target branch's copy, which carries the Evidence append, passes", func(t *testing.T) {
		fg := &rvBriefHashForge{content: map[string][]byte{"main:docs/streams/example-stream/brief-01-x.md": asLanded}}
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", Inputs: map[string]string{
			"file:docs/streams/example-stream/brief-01-x.md": deskkit.Sha256Hex(asLanded),
		}}
		if err := validateReceiptBriefHashAsLanded(fg, deskkit.ForgeRepo{Owner: "example-org", Name: "example"}, "main", dir, wr); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("a branch copy that cannot be read exits 6", func(t *testing.T) {
		fg := &rvBriefHashForge{err: deskkit.Unverifiable("transport error", nil)}
		wr := deskkit.WakeReceipt{Brief: "example-stream/01", Inputs: map[string]string{
			"file:docs/streams/example-stream/brief-01-x.md": deskkit.Sha256Hex(asLanded),
		}}
		err := validateReceiptBriefHashAsLanded(fg, deskkit.ForgeRepo{Owner: "example-org", Name: "example"}, "main", dir, wr)
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %v, want Unverifiable (err=%v)", deskkit.ExitCodeOf(err), err)
		}
	})

	t.Run("reader side: RootRevisionReader.Revision over the merged tree returns the as-landed hash", func(t *testing.T) {
		// Pins that the EXISTING reader (unchanged by #882) already hashes the working tree's
		// current copy — the same "as it lands" value the writer now compares against — so a
		// receipt the writer accepts holds (does not fire relevant-input-changed) right after
		// landing. Write the as-landed content into the SAME local tree the writer judged.
		rvWriteAndAdd(t, dir, "docs/streams/example-stream/brief-01-x.md", string(asLanded))
		rvGitIn(t, dir, "commit", "-qm", "evidence appended")

		reader := deskkit.NewRootRevisionReader(dir, "v-test")
		got, ok := reader.Revision("file:docs/streams/example-stream/brief-01-x.md")
		if !ok {
			t.Fatal("RootRevisionReader could not resolve the brief revision")
		}
		want := deskkit.Sha256Hex(asLanded)
		if got != want {
			t.Fatalf("RootRevisionReader.Revision = %q, want the as-landed hash %q", got, want)
		}
	})
}
