package main

// blockerread_test.go — verify-reset/03 review round: the forge issue source is bounded to the
// configured repository set before any credential is minted, maps every forge answer to the
// three states, names its target before first contact, and is the package's only forge
// resolution.

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantAllowedRoster installs a roster whose configured repository set is exactly allowed.
func plantAllowedRoster(t *testing.T, allowed ...string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, r := range allowed {
		entries = append(entries, r+":ci:private")
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=verifier=assay-verifier-app:300000005\n" +
		"ASSAY_ALLOWED_REPOS=" + strings.Join(entries, ",") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// stateForge answers every issue read with one state (or a nil issue), recording which read
// method was called.
type stateForge struct {
	deskkit.Forge
	state    string
	nilIssue bool
	calls    *[]string
}

func (f stateForge) answer(n int) (*deskkit.Issue, error) {
	if f.nilIssue {
		return nil, nil
	}
	return &deskkit.Issue{Number: n, State: f.state}, nil
}

func (f stateForge) GetIssue(_ deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	*f.calls = append(*f.calls, "issue")
	return f.answer(n)
}

func (f stateForge) GetIssueTyped(_ deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	*f.calls = append(*f.calls, "typed:"+string(kind))
	return f.answer(n)
}

// F-blocker-read-coordinate: a reference whose repository is outside the configured set is
// could-not-check and the forge is never resolved for it — even though the forge would have
// answered closed. A repository inside the set is read (positive control), and its target is
// named once, before first contact.
func TestBlockerReadOutsideSetNotRead(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	var resolved []string
	var calls []string
	var notice bytes.Buffer
	src := &forgeIssueSource{
		forgeFor: func(r deskkit.ForgeRepo) (deskkit.Forge, error) {
			resolved = append(resolved, r.Slug())
			return stateForge{state: "closed", calls: &calls}, nil
		},
		notice: &notice,
	}
	for _, raw := range []string{
		"other-owner/private-repo#5",
		"https://unrelated.example/third-owner/secret/issues/9",
		"https://github.com/third-owner/secret/pull/9",
		"example-org/trackers#5",
	} {
		ref, err := deskkit.ParseBlockerRef(raw, "", "")
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		st, why := src.IssueState(ref)
		if st != deskkit.BlockerCouldNotCheck || !strings.Contains(why, "outside the configured repository set") {
			t.Errorf("%s: IssueState = %v (%q), want could-not-check naming the repository set", raw, st, why)
		}
	}
	if len(resolved) != 0 {
		t.Fatalf("forge resolved (a credential minted) for out-of-set repositories: %v", resolved)
	}
	if notice.Len() != 0 {
		t.Errorf("an out-of-set repository was announced as a read target: %q", notice.String())
	}

	// Through the plan: a held record pointing outside the set is held, never dispatched.
	same := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}
	rec := heldReceipt(holdTS)
	rec.BlockerRef = "other-owner/private-repo#5"
	if d, r, why := holdCase(t, same, src, rec); d != dispCouldNotCheck || strings.Contains(why, "blocker closed") {
		t.Errorf("plan over an out-of-set blocker: got %v %q (%q); want could-not-check", d, r, why)
	}
	if len(resolved) != 0 {
		t.Fatalf("the plan resolved a forge for an out-of-set repository: %v", resolved)
	}

	// Positive control: the configured repository is read, announced once.
	for i := 0; i < 2; i++ {
		ref, _ := deskkit.ParseBlockerRef("example-org/tracker#5", "", "")
		if st, why := src.IssueState(ref); st != deskkit.BlockerClosed {
			t.Errorf("in-set read: %v (%q), want closed", st, why)
		}
	}
	if len(resolved) != 1 || resolved[0] != "example-org/tracker" {
		t.Errorf("resolved = %v, want one resolution for example-org/tracker", resolved)
	}
	if got := strings.Count(notice.String(), "example-org/tracker"); got != 1 {
		t.Errorf("target notice = %q, want example-org/tracker named exactly once", notice.String())
	}
}

// F-unknown-state-unpinned / COR-2410-3: every forge answer maps to exactly one of the three
// states; only "closed"/"merged" can release a hold. A pull-request URL is read as a change, the
// rest as an issue.
func TestForgeIssueSourceStates(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	cases := []struct {
		raw, state string
		nilIssue   bool
		want       deskkit.BlockerState
		call       string
	}{
		{"example-org/tracker#1", "open", false, deskkit.BlockerOpen, "issue"},
		{"example-org/tracker#1", "OPENED", false, deskkit.BlockerOpen, "issue"},
		{"example-org/tracker#1", "closed", false, deskkit.BlockerClosed, "issue"},
		{"https://github.com/example-org/tracker/issues/1", "closed", false, deskkit.BlockerClosed, "issue"},
		{"https://github.com/example-org/tracker/pull/1", "merged", false, deskkit.BlockerClosed, "typed:change"},
		{"https://github.com/example-org/tracker/pull/1", "open", false, deskkit.BlockerOpen, "typed:change"},
		{"example-org/tracker#1", "locked", false, deskkit.BlockerCouldNotCheck, "issue"},
		{"example-org/tracker#1", "", false, deskkit.BlockerCouldNotCheck, "issue"},
		{"https://github.com/example-org/tracker/pull/1", "draft", false, deskkit.BlockerCouldNotCheck, "typed:change"},
		{"example-org/tracker#1", "", true, deskkit.BlockerCouldNotCheck, "issue"},
	}
	for _, c := range cases {
		var calls []string
		src := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
			return stateForge{state: c.state, nilIssue: c.nilIssue, calls: &calls}, nil
		}}
		ref, err := deskkit.ParseBlockerRef(c.raw, "", "")
		if err != nil {
			t.Fatalf("%s: %v", c.raw, err)
		}
		st, why := src.IssueState(ref)
		if st != c.want {
			t.Errorf("%s state %q nil=%v: %v (%q), want %v", c.raw, c.state, c.nilIssue, st, why, c.want)
		}
		if len(calls) != 1 || calls[0] != c.call {
			t.Errorf("%s: read calls = %v, want [%s]", c.raw, calls, c.call)
		}
	}
}

// errForge fails every read with a transport error.
type errForge struct{ deskkit.Forge }

func (errForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	return nil, errors.New("dial tcp: connection refused")
}

// A failed forge resolution is remembered for the pass: the credential is not re-minted per
// reference, and every later read of that repository stays could-not-check.
func TestForgeIssueSourceCachesFailure(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	n := 0
	src := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
		n++
		return nil, deskkit.Unverifiable("no token", nil)
	}}
	for i := 0; i < 3; i++ {
		ref, _ := deskkit.ParseBlockerRef("#7", "example-org", "tracker")
		if st, _ := src.IssueState(ref); st != deskkit.BlockerCouldNotCheck {
			t.Errorf("read %d: %v, want could-not-check", i, st)
		}
	}
	if n != 1 {
		t.Errorf("forge resolved %d times, want 1", n)
	}
	ok := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return errForge{}, nil }}
	ref, _ := deskkit.ParseBlockerRef("#7", "example-org", "tracker")
	if st, _ := ok.IssueState(ref); st != deskkit.BlockerCouldNotCheck {
		t.Errorf("transport error: %v, want could-not-check", st)
	}
}

// Class guard (F-blocker-read-coordinate): resolveForge is the ONLY forge resolution in this
// package's non-test code, and only issueSourceFn — the allowlisted forgeIssueSource — uses it.
// A second deskkit.ForgeFor / deskkit.ResolveForge caller would be a forge read whose coordinate
// nothing bounds; it fails here.
func TestForgeForSingleCaller(t *testing.T) {
	if bad := forgeCallers(t, "."); len(bad) != 0 {
		t.Fatalf("forge resolution outside resolveForge, or resolveForge used outside issueSourceFn:\n  %s",
			strings.Join(bad, "\n  "))
	}
}

// forgeCallers returns every violation of the single-resolution rule in dir's non-test files.
func forgeCallers(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var bad []string
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range af.Decls {
			encl := "package scope"
			var declName *ast.Ident
			if fd, ok := decl.(*ast.FuncDecl); ok {
				encl, declName = fd.Name.Name, fd.Name
			} else if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
				for _, s := range gd.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Names) == 1 {
						encl = "var " + vs.Names[0].Name
					}
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if id, ok := x.X.(*ast.Ident); ok && id.Name == "deskkit" &&
						(x.Sel.Name == "ForgeFor" || x.Sel.Name == "ResolveForge") {
						seen++
						if encl != "resolveForge" {
							bad = append(bad, fset.Position(x.Pos()).String()+": deskkit."+x.Sel.Name+" in "+encl)
						}
					}
				case *ast.Ident:
					if x.Name == "resolveForge" && x != declName && encl != "var issueSourceFn" {
						bad = append(bad, fset.Position(x.Pos()).String()+": resolveForge used in "+encl)
					}
				}
				return true
			})
		}
	}
	if seen == 0 {
		bad = append(bad, "no deskkit.ForgeFor call found at all — the guard is reading the wrong files")
	}
	return bad
}

// mapTree is an in-memory OutcomeTree: path -> content; a directory is any prefix.
type mapTree map[string]string

func (m mapTree) PathKind(rel string) (string, error) {
	if _, ok := m[rel]; ok {
		return "blob", nil
	}
	for k := range m {
		if strings.HasPrefix(k, strings.TrimSuffix(rel, "/")+"/") {
			return "tree", nil
		}
	}
	return "", nil
}

func (m mapTree) ReadFile(rel string) ([]byte, error) {
	if b, ok := m[rel]; ok {
		return []byte(b), nil
	}
	return nil, errors.New("absent: " + rel)
}

func (m mapTree) ListFiles(rel string) ([]string, error) {
	var out []string
	for k := range m {
		if strings.HasPrefix(k, strings.TrimSuffix(rel, "/")+"/") {
			out = append(out, k)
		}
	}
	return out, nil
}

// COR-2410-1 / F-bare-ref-no-repo, writer half: a record exactly as the writer lands it — a
// bare #100 blocker_ref, no repo key from the caller — holds while #100 is open and wakes when it
// closes. Before the writer stamped the landing repository, both read "names no repository".
func TestWriterBareRefReadsBlocker(t *testing.T) {
	brief := "# x\n\n## Context\n\nfiles:\n- `pkg/x.go`\n\n## Verify\n"
	tree := mapTree{"docs/streams/example-stream/brief-01-x.md": brief, "pkg/x.go": "package x\n"}
	line := `{"ts":"` + holdTS + `","brief":"example-stream/01","outcome":"verify-fail","sha":"0000abc",` +
		`"blocker_kind":"implementation","blocker_ref":"#100"}`
	out, err := deskkit.BuildOutcomeRecord([]byte(line), deskkit.OutcomeReceiptInput{
		BriefPath: "docs/streams/example-stream/brief-01-x.md", LandedBrief: []byte(brief),
		ToolVersion: "dev", Repo: "example-org/tracker",
	}, tree)
	if err != nil {
		t.Fatalf("BuildOutcomeRecord: %v", err)
	}
	rec, ok := deskkit.ParseWakeReceipt(out)
	if !ok || !rec.Complete() {
		t.Fatalf("writer output is not a complete receipt: %s", out)
	}
	same := fakeWake{revs: rec.Inputs}
	d, r, _ := holdCase(t, same, refIssues{"example-org/tracker#100": deskkit.BlockerOpen}, rec)
	if d != dispWaitReceipt {
		t.Errorf("open blocker: got %v %q; want WAIT", d, r)
	}
	d, r, why := holdCase(t, same, refIssues{"example-org/tracker#100": deskkit.BlockerClosed}, rec)
	if d != dispDispatch || !strings.Contains(why, "blocker closed") {
		t.Errorf("closed blocker: got %v %q (%q); want DISPATCH on the closed blocker", d, r, why)
	}
}

// COR-2410-1 / F-bare-ref-no-repo, planner half: a record with no repo key (written before the
// writer stamped one) resolves its bare #N in the single-root plan's own repository — the
// checkout's origin — through cmdPlan; with no origin it stays could-not-check, never guessed.
func TestPlanBareRefUsesRootRepo(t *testing.T) {
	old := issueSourceFn
	t.Cleanup(func() { issueSourceFn = old })
	issueSourceFn = func() deskkit.IssueStateSource {
		return refIssues{"example-org/tracker#100": deskkit.BlockerOpen}
	}
	plan := func(withOrigin bool) string {
		root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no")})
		content := "package x\n"
		if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "pkg", "x.go"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := heldReceipt(holdTS)
		rec.Repo = ""
		rec.Inputs = map[string]string{"file:pkg/x.go": deskkit.Sha256Hex([]byte(content))}
		writeReceipts(t, root, rec)
		if withOrigin {
			gitT(t, root, "init", "-q")
			gitT(t, root, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
		}
		var perr error
		out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
		if perr != nil {
			t.Fatalf("cmdPlan: %v", perr)
		}
		return out
	}
	if out := plan(true); !strings.Contains(out, "   WAIT example-stream/01 — blocker #100 open") {
		t.Errorf("with an origin: no WAIT naming #100:\n%s", out)
	}
	if out := plan(false); strings.Contains(out, "   WAIT ") || !strings.Contains(out, "could-not-check=1") {
		t.Errorf("with no origin: want could-not-check, no WAIT:\n%s", out)
	}
}
