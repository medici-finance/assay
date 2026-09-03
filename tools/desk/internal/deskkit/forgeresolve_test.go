package deskkit

// forgeresolve_test.go — the executable half of the ForgeFor contract (forgeresolve.go's
// header). Every negative path here is chosen because the failure mode it guards against
// is a SILENT one: a wrong forge, a wrong identity, or a wrong "clean" would otherwise
// surface as a write to the wrong place performed as the wrong actor, discovered only
// after the fact.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// --- resolution -----------------------------------------------------------------------

func TestForgeForUnconfiguredRepoRefuses(t *testing.T) {
	withRoster(t, goldenRoster()) // no ASSAY_REPO_FORGES entry anywhere in this roster

	prevHost := originRemoteHost
	originRemoteHost = func() (string, error) { return "", fmt.Errorf("no remote in this fixture") }
	t.Cleanup(func() { originRemoteHost = prevHost })

	repo := ForgeRepo{Owner: "example-org", Name: "unconfigured-repo"}
	f, err := ForgeFor(repo, "reviewer")
	if err == nil {
		t.Fatalf("an unconfigured repo silently resolved to a forge: %#v", f)
	}
	if f != nil {
		t.Fatalf("ForgeFor returned a non-nil Forge alongside an error: %#v", f)
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable) — an unresolved forge is could-not-check, never a "+
			"guessed default", got, ExitUnverifiable)
	}
	for _, want := range []string{repo.Slug(), EnvRepoForges} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so an operator cannot act on it: %v", want, err)
		}
	}
}

func TestForgeForResolvesFromRepoConfig(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	roster := goldenRoster()
	roster[EnvRepoForges] = repo.Slug() + "=gitlab"
	withRoster(t, roster)

	// The remote-host fallback must never even be consulted when repo-config already
	// answered — proved by making it panic if reached.
	prevHost := originRemoteHost
	originRemoteHost = func() (string, error) {
		t.Fatal("remote-host fallback consulted despite a repo-config entry")
		return "", nil
	}
	t.Cleanup(func() { originRemoteHost = prevHost })

	credDir := t.TempDir()
	path := filepath.Join(credDir, "gitlab-reviewer.token")
	if err := os.WriteFile(path, []byte("gitlab-pat-stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigHome, credDir)

	f, err := ForgeFor(repo, "reviewer")
	if err != nil {
		t.Fatalf("ForgeFor: %v", err)
	}
	if _, ok := f.(*GitLabForge); !ok {
		t.Fatalf("ForgeFor returned %T, want *GitLabForge", f)
	}
}

// --- no caller-suppliable forge ---------------------------------------------------------

func TestForgeForRejectsCallerSuppliedForge(t *testing.T) {
	rt := reflect.TypeOf(ForgeFor)
	if rt == nil || rt.Kind() != reflect.Func {
		t.Fatal("ForgeFor is not a function value")
	}
	kindType := reflect.TypeOf(ForgeGitHub)
	for i := 0; i < rt.NumIn(); i++ {
		if rt.In(i) == kindType {
			t.Fatalf("ForgeFor parameter %d has type ForgeKind — a caller could dictate the forge", i)
		}
	}

	// AST-scan every non-test source file in this package: no EXPORTED function may take a
	// ForgeKind parameter, or a parameter literally named "forge" — the two shapes by which
	// a forge selector could sneak back onto a caller-reachable surface.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("cannot read package directory: %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", name, perr)
		}
		checked++
		ast.Inspect(f, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if !ok || !fd.Name.IsExported() || fd.Type.Params == nil {
				return true
			}
			for _, p := range fd.Type.Params.List {
				if ident, ok := p.Type.(*ast.Ident); ok && ident.Name == "ForgeKind" {
					t.Errorf("%s: exported func %s takes a ForgeKind parameter — a caller could supply "+
						"the forge", fset.Position(fd.Pos()), fd.Name.Name)
				}
				for _, pn := range p.Names {
					if strings.EqualFold(pn.Name, "forge") {
						t.Errorf("%s: exported func %s has a parameter literally named %q",
							fset.Position(fd.Pos()), fd.Name.Name, pn.Name)
					}
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source files were scanned — this check would pass vacuously")
	}
}

// --- custody -----------------------------------------------------------------------------

func TestForgeForMissingTokenRefuses(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "custody-repo"}
	roster := goldenRoster()
	roster[EnvRepoForges] = repo.Slug() + "=github"
	withRoster(t, roster)

	t.Run("file_absent", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-such-token")
		stubMinter(t, missing, "", nil)
		f, err := ForgeFor(repo, "reviewer")
		if err == nil {
			t.Fatalf("a missing token file was accepted: %#v", f)
		}
		if f != nil {
			t.Fatal("ForgeFor returned a non-nil Forge alongside an error")
		}
		if got := ExitCodeOf(err); got != ExitRefused {
			t.Fatalf("exit = %d, want %d (refused) — a missing custody file is a deployment "+
				"precondition, not a could-not-check", got, ExitRefused)
		}
	})

	t.Run("insecure_mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reviewer-token-1")
		if err := os.WriteFile(path, []byte("tok"), 0o644); err != nil {
			t.Fatal(err)
		}
		stubMinter(t, path, "", nil)
		f, err := ForgeFor(repo, "reviewer")
		if err == nil {
			t.Fatalf("a 0644 token file was accepted: %#v", f)
		}
		if f != nil {
			t.Fatal("ForgeFor returned a non-nil Forge alongside an error")
		}
		if got := ExitCodeOf(err); got != ExitRefused {
			t.Fatalf("exit = %d, want %d (refused)", got, ExitRefused)
		}
		if !strings.Contains(err.Error(), "600") {
			t.Errorf("refusal does not name the remedy (chmod 600): %v", err)
		}
	})

	t.Run("no_ambient_fallback", func(t *testing.T) {
		// A failing mint must never fall through to reading an ambient gh-CLI credential —
		// proved by asserting the refusal names the role/repo the mint failed for, which is
		// only possible if the failure was surfaced rather than swallowed into a silent
		// ambient read.
		stubMinter(t, "", "", fmt.Errorf("no installation id for role"))
		_, err := ForgeFor(repo, "reviewer")
		if err == nil {
			t.Fatal("a failing mint returned a Forge")
		}
		if got := ExitCodeOf(err); got != ExitRefused {
			t.Fatalf("exit = %d, want %d (refused)", got, ExitRefused)
		}
	})
}

func TestGitHubCustodyMinterHookIsHonored(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "hook-repo"}
	roster := goldenRoster()
	roster[EnvRepoForges] = repo.Slug() + "=github"
	withRoster(t, roster)

	var gotRole string
	var gotRepo ForgeRepo
	SetGitHubCustodyMinter(func(role string, r ForgeRepo) (string, string, error) {
		gotRole, gotRepo = role, r
		return "hook-minted-token", "https://fake.example.invalid", nil
	})
	t.Cleanup(func() { SetGitHubCustodyMinter(nil) })

	f, err := ForgeFor(repo, "worker")
	if err != nil {
		t.Fatalf("ForgeFor: %v", err)
	}
	gh, ok := f.(*GitHubForge)
	if !ok {
		t.Fatalf("ForgeFor returned %T, want *GitHubForge", f)
	}
	if gh.Token != "hook-minted-token" || gh.BaseURL != "https://fake.example.invalid" {
		t.Fatalf("GitHubForge = %+v, want the installed hook's token/baseURL", gh)
	}
	if gotRole != "worker" || gotRepo != repo {
		t.Fatalf("hook called with role=%q repo=%+v, want worker/%+v", gotRole, gotRepo, repo)
	}
}

// --- refusal semantics: an unsupported operation is could-not-check, never a raw request -

// noRequestTransport fails the test if any request reaches it — the proof that an
// unsupported operation refuses BEFORE attempting a call, never after a failed one.
type noRequestTransport struct{ t *testing.T }

func (rt noRequestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.t.Fatalf("a request reached the transport for an operation that should have refused before any "+
		"call: %s %s", r.Method, r.URL)
	return nil, fmt.Errorf("unreachable")
}

func TestUnsupportedOperationIsCouldNotCheck(t *testing.T) {
	g := &GitLabForge{
		Token:   "x",
		BaseURL: "https://gitlab.example.invalid",
		Client:  &http.Client{Transport: noRequestTransport{t: t}},
	}
	repo := ForgeRepo{Owner: "o", Name: "n"}
	err := g.DeleteRef(repo, "dispatch/repo--str--01")
	if err == nil {
		t.Fatal("an operation outside the mapped namespace was accepted")
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable) — an unsupported operation is could-not-check, "+
			"never a refusal or a silent success", got, ExitUnverifiable)
	}
	for _, want := range []string{"gitlab", "deleteref", "dispatch/repo--str--01"} {
		if !strings.Contains(strings.ToLower(err.Error()), want) {
			t.Errorf("refusal does not name %q, so a caller cannot tell what gap it hit: %v", want, err)
		}
	}
}

// --- single construction site ------------------------------------------------------------

// compositeLitTypeName returns the bare type name of a composite literal's type
// expression, unwrapping a package-qualified form (pkg.Type{...}) to its selector name so
// a caller in another package (deskkit.GitHubForge{...}) is caught the same as a
// same-package literal.
func compositeLitTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

func TestForgeSingleConstructionSite(t *testing.T) {
	root := deskTreeRoot // "../.." — defined in forge_surface_test.go, this package
	var offenders []string
	sawResolverSite := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			switch info.Name() {
			case "testdata", ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", path, perr)
		}
		isResolverFile := filepath.Base(path) == "forgeresolve.go"
		ast.Inspect(f, func(n ast.Node) bool {
			var typeName string
			switch expr := n.(type) {
			case *ast.CompositeLit:
				typeName = compositeLitTypeName(expr.Type)
			case *ast.UnaryExpr:
				if expr.Op == token.AND {
					if cl, ok := expr.X.(*ast.CompositeLit); ok {
						typeName = compositeLitTypeName(cl.Type)
					}
				}
			}
			if typeName != "GitHubForge" && typeName != "GitLabForge" {
				return true
			}
			if isResolverFile {
				sawResolverSite[typeName] = true
			} else {
				offenders = append(offenders, fmt.Sprintf("%s: %s{...}", fset.Position(n.Pos()), typeName))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("could not walk %s: %v — this is could-not-check, not a clean tree", root, err)
	}
	if !sawResolverSite["GitHubForge"] || !sawResolverSite["GitLabForge"] {
		t.Fatalf("forgeresolve.go does not construct both backends itself (saw %v) — this test would be "+
			"vacuous if the resolver's own construction site did not exist", sawResolverSite)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("a backend is constructed outside forgeresolve.go:\n%s", strings.Join(offenders, "\n"))
	}
}

// --- roster key registration --------------------------------------------------------------

func TestRosterKnownKeySet(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/tracker=github,example-org/gitlab-pilot=gitlab"
	withRoster(t, roster)

	cfg := EffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("a roster carrying %s was refused (an unregistered key fails the roster closed): %v",
			EnvRepoForges, cfg.Problems)
	}
	if got := cfg.RepoForges["example-org/tracker"]; got != "github" {
		t.Fatalf("RepoForges[%q] = %q, want %q", "example-org/tracker", got, "github")
	}
	if got := cfg.RepoForges["example-org/gitlab-pilot"]; got != "gitlab" {
		t.Fatalf("RepoForges[%q] = %q, want %q", "example-org/gitlab-pilot", got, "gitlab")
	}
}

func TestRepoForgesRejectsBareBasenameAndBadForge(t *testing.T) {
	cases := []string{
		"tracker=github",          // bare basename, unlike ASSAY_REPO_ALIASES this must refuse
		"example-org/tracker=svn", // unrecognised forge
		"example-org/tracker",     // no '='
		"example-org/tracker=",    // empty forge
	}
	for _, entry := range cases {
		roster := goldenRoster()
		roster[EnvRepoForges] = entry
		home := t.TempDir()
		dir := filepath.Join(home, ".config", "assay")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for k, v := range roster {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
		}
		if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		ReloadConfig()
		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Errorf("entry %q was accepted — want the whole roster refused", entry)
		}
	}
	ReloadConfig()
}
