package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This file holds the source-level guards on the fail-closed properties. They exist
// because the behavioural tests can only reach branches that EXIST; the defect class in
// #228/#406 is a branch that returns an installation ID it did not get from
// GitHub, and nothing stops the next edit from adding a fifth such branch. A source guard
// covers the branch that has not been written yet.
//
// Provenance: four of resolveInstallID's failure branches — the
// request-build error, the transport error, the unparseable body and the absent/zero id —
// each accepted `return "148002199", nil` with `go test ./cmd/deskevidence/ -count=1`
// still reporting ok. The behavioural table in TestLookupFailureFailsClosed now reaches
// all of them; this closes the same class structurally.

// parseGitHubGo parses github.go into an AST, or fails the test.
func parseGitHubGo(t *testing.T) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "github.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse github.go: %v", err)
	}
	return fset, f
}

func findFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("func %s not found in github.go — it was renamed or removed; this guard is now blind", name)
	return nil
}

// TestResolveInstallIDHasNoSilentFallback pins the SHAPE of every return in
// resolveInstallID: an installation ID may only be returned from one of the three
// sanctioned sources, and every refusal must return the empty string.
//
// Concretely, each `return` in that function must be exactly one of:
//
//	return "", <a non-nil error>          — a refusal, carrying no ID
//	return <v|cached|id>, nil             — the env override, the cache, or GitHub's answer
//
// `return "148002199", nil` — the mutation that survived the entire suite on four separate
// branches — is neither, and fails here regardless of WHICH branch it is seated in,
// including a branch added after this test was written. That is the property the
// behavioural table cannot have: it can only script failures the code already models.
//
// If a legitimate fourth source of an installation ID is ever added, this test must be
// edited to name it. That edit is the point: it makes "where can an ID come from" a
// decision someone takes deliberately, in a diff, rather than a line that slips in.
func TestResolveInstallIDHasNoSilentFallback(t *testing.T) {
	fset, file := parseGitHubGo(t)
	fn := findFunc(t, file, "resolveInstallID")

	// The only expressions allowed to carry an ID out of the function alongside a nil
	// error. Each corresponds to a documented source: VERIFIER_INSTALL_ID, the process
	// cache, and the parsed lookup response.
	allowedSuccess := map[string]string{
		"v":      "the VERIFIER_INSTALL_ID override",
		"cached": "the process-lifetime install cache",
		"id":     "the id parsed out of GitHub's lookup response",
	}

	var successes, refusals int
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		pos := fset.Position(ret.Pos())
		if len(ret.Results) != 2 {
			t.Errorf("%s: return has %d results, want 2 (id, error)", pos, len(ret.Results))
			return true
		}

		errIsNil := false
		if idn, ok := ret.Results[1].(*ast.Ident); ok && idn.Name == "nil" {
			errIsNil = true
		}

		if errIsNil {
			successes++
			idn, ok := ret.Results[0].(*ast.Ident)
			if !ok {
				t.Errorf("%s: returns an installation ID with a nil error from an expression that is not "+
					"one of the sanctioned sources (%v) — an ID the tool did not get from GitHub or from "+
					"the explicit override is a silent fallback (#228)", pos, sourceNames(allowedSuccess))
				return true
			}
			if _, ok := allowedSuccess[idn.Name]; !ok {
				t.Errorf("%s: returns %q with a nil error; the only sanctioned sources are %v",
					pos, idn.Name, sourceNames(allowedSuccess))
			}
			return true
		}

		refusals++
		lit, ok := ret.Results[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || lit.Value != `""` {
			t.Errorf("%s: a refusal (non-nil error) must return the empty string as the ID, "+
				"so a caller that ignores the error cannot mint against a fabricated installation", pos)
		}
		return true
	})

	// Pin the counts too. Dropping a refusal branch (rather than mutating it) is the other
	// way this property dies, and it would otherwise leave every remaining return valid.
	if successes != len(allowedSuccess) {
		t.Errorf("resolveInstallID has %d success returns, want %d (one per sanctioned source: %v) — "+
			"if a source was added or removed, update allowedSuccess deliberately",
			successes, len(allowedSuccess), sourceNames(allowedSuccess))
	}
	if refusals < 6 {
		t.Errorf("resolveInstallID has %d refusal returns, want at least 6 (request-build, transport, "+
			"404, non-2xx, unparseable body, empty/zero id) — a failure branch was deleted, not fixed",
			refusals)
	}
}

func sourceNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+" ("+v+")")
	}
	return out
}

// TestHTTPClientIsBoundedNotDefault guards the value the PR argues for at length and that
// nothing else could see: httpClient must carry a Timeout.
//
// It cannot be behavioural. setupFake replaces httpClient wholesale with a client wired to
// the test server, so the production declaration is never exercised by any test in the
// package — reverting it to http.DefaultClient left the entire suite green
// (#406).
//
// What the timeout buys: every request runs inside the shared ~/.config/assay/
// audit.lock, so an unbounded hang here does not stall this tool, it stalls deskpost and
// deskrelease too. The 60s acquire deadline bounds the waiters; this bounds the holder.
func TestHTTPClientIsBoundedNotDefault(t *testing.T) {
	fset, file := parseGitHubGo(t)

	var spec *ast.ValueSpec
	ast.Inspect(file, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, name := range vs.Names {
			if name.Name == "httpClient" {
				spec = vs
			}
		}
		return true
	})
	if spec == nil || len(spec.Values) != 1 {
		t.Fatalf("httpClient var declaration not found in github.go with exactly one value")
	}
	pos := fset.Position(spec.Pos())

	// Want: &http.Client{Timeout: <non-zero>}.
	unary, ok := spec.Values[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		t.Fatalf("%s: httpClient must be its own &http.Client with a Timeout, not a shared client "+
			"(http.DefaultClient has no Timeout and a hang there pins the shared audit lock)", pos)
	}
	lit, ok := unary.X.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("%s: httpClient must be an http.Client composite literal carrying a Timeout", pos)
	}
	var timeout ast.Expr
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Timeout" {
			timeout = kv.Value
		}
	}
	if timeout == nil {
		t.Fatalf("%s: httpClient has no Timeout field — an unbounded request inside the audit flock "+
			"starves every other desk tool on the same lock", pos)
	}
	if b, ok := timeout.(*ast.BasicLit); ok && b.Value == "0" {
		t.Fatalf("%s: httpClient.Timeout is 0, which is the same as no timeout", pos)
	}
}

// TestNoParallelTestsInPackage is the guard the previous review asked for by implication:
// the concurrency tests mutate package-level state (lockWait, apiBaseURL, httpClient,
// stdout, stderr, installCache) through setupFake, none of which is safe to share. Today
// no test calls t.Parallel, so those mutations are safe; a future one added without
// noticing would race them, and the symptom would be an intermittently green suite — the
// worst possible failure mode for a package whose whole job is proving guards fire.
func TestNoParallelTestsInPackage(t *testing.T) {
	entries, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no _test.go files found — this guard is looking in the wrong directory")
	}
	// Assembled at run time so this guard's own source does not contain the literal it
	// searches for — otherwise it reports itself and can never pass.
	needle := "t." + "Parallel("
	for _, name := range entries {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, needle) {
				t.Errorf("%s:%d parallelises a test; this package's tests share mutable package state "+
					"(lockWait, apiBaseURL, httpClient, stdout, stderr, installCache via setupFake). "+
					"Make that state per-test before parallelising anything here.", name, i+1)
			}
		}
	}
}

// --- App-credential search path (#794) -------------------------------------------

// TestVerifierKeyOffSearchPathRefused is the POSITIVE CONTROL for deskevidence's key
// resolution, and the deskevidence half of #794. desktoken honouring
// ASSAY_CONFIG_HOME is not the whole fix: this tool mints its OWN verifier token, and
// while it read the key from a hardcoded ~/.config/assay it kept failing in a fresh shell
// on a deployment that provisions elsewhere.
//
// The control is a key that is real and readable but sits in a directory nothing
// searches. It must be REFUSED (exit 6), and the refusal must name every directory
// searched plus both knobs — a credential resolver that cannot find its source has to say
// where it looked. It must NOT name the stray directory, which it never searched.
func TestVerifierKeyOffSearchPathRefused(t *testing.T) {
	setupFake(t)
	home, _ := os.UserHomeDir()
	t.Setenv("VERIFIER_PEM", "")
	t.Setenv(deskkit.EnvConfigHome, "")
	t.Setenv("VERIFIER_APP_ID", "999999")

	stray := filepath.Join(home, "elsewhere")
	if err := os.MkdirAll(stray, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stray, "verifier-app.pem"), verifierPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	// Remove the key setupFake planted on the default search path, so the ONLY key on
	// this machine is the off-path one.
	if err := os.Remove(filepath.Join(home, ".config", "assay", "verifier-app.pem")); err != nil {
		t.Fatalf("remove default pem: %v", err)
	}

	_, err := mintVerifierToken("example-org", "tracker")
	if err == nil {
		t.Fatal("a key off the search path must NOT mint — the resolver has to fail closed")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("off-path key err code = %d, want 6 (unverifiable)", deskkit.ExitCodeOf(err))
	}
	for _, want := range []string{
		"cannot read verifier App key",
		filepath.Join(home, ".config", "assay"),
		deskkit.EnvConfigHome,
		"VERIFIER_PEM",
		"#794",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal should name %q; got: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), stray) {
		t.Errorf("refusal names a directory it never searched (%s): %v", stray, err)
	}
}

// TestVerifierKeyFoundOnSearchPathHead is the paired GREEN run: the same tool, the same
// cleared VERIFIER_PEM, with the provisioning directory at the head of the search path.
// Without this pair, the refusal above proves only that the tool can fail.
func TestVerifierKeyFoundOnSearchPathHead(t *testing.T) {
	setupFake(t)
	home, _ := os.UserHomeDir()
	t.Setenv("VERIFIER_PEM", "")
	t.Setenv("VERIFIER_APP_ID", "999999")

	provisioned := filepath.Join(home, "vault", "provisioned")
	if err := os.MkdirAll(provisioned, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(provisioned, "verifier-app.pem"), verifierPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	t.Setenv(deskkit.EnvConfigHome, provisioned)
	if err := os.Remove(filepath.Join(home, ".config", "assay", "verifier-app.pem")); err != nil {
		t.Fatalf("remove default pem: %v", err)
	}

	if _, err := mintVerifierToken("example-org", "tracker"); err != nil {
		t.Fatalf("mint from the head of the search path: %v", err)
	}
}
