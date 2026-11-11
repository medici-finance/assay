package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/forgeban"
)

// forge_surface_test.go — the two enforcement tests that make "the forge surface is closed"
// a checked property rather than a convention.
//
//	TestNoForgeCLIShellout   — nothing under tools/desk invokes a forge CLI.
//	TestForgeNoPassthrough   — nothing on the interface, or on either backend, accepts an
//	                           arbitrary endpoint.
//
// They are DELIBERATELY DIFFERENT MECHANISMS on the same property, which is what the brief's
// single-point-of-failure note asks for. The ban reads SOURCE (an AST walk plus a
// position-aware literal scan); the passthrough test reads TYPES (reflection over the method
// sets, plus the interface's own parameter names). A passthrough that slipped the source scan
// — a raw-request method reachable through an exported helper, say — still has to exist as a
// method, and the reflection layer sees methods, not spellings. Neither is a subset of the
// other, and neither is weakened by weakening the other.

// --- The shell-exec ban ------------------------------------------------------------------

// deskTreeRoot is the tools/desk module root as seen from this package's directory. It is a
// relative literal rather than a discovered path because Scan FAILS CLOSED on a tree with no
// shipped Go source, so a wrong root is an error, never a silent pass.
const deskTreeRoot = "../.."
const liftedGitTreeRoot = "../../../../gitcore"

// loopAdminTreeRoot is the loopadmin module, home of the one process runner
// (loopadmin/process, moved from internal/cellprocess, #5492).
const loopAdminTreeRoot = "../../../../loopadmin"

// auditRoot is one tree the process-launch audit scans; Prefix is prepended to
// each finding's File so register keys name the tree they come from.
type auditRoot struct{ Dir, Prefix, What string }

// processLaunchAuditRoots is every tree the process-launch audit scans. Dropping
// a root makes its registered exec sites stale and its new ones invisible.
var processLaunchAuditRoots = []auditRoot{
	{deskTreeRoot, "", "the desk tree"},
	{liftedGitTreeRoot, "gitcore/", "lifted Git source"},
	{loopAdminTreeRoot, "loopadmin/", "the loopadmin module"},
}

// scanAudit scans every root and returns the prefixed findings. A root that
// cannot be scanned is could-not-check, never a clean tree.
func scanAudit(roots []auditRoot) ([]forgeban.Finding, error) {
	var findings []forgeban.Finding
	for _, root := range roots {
		found, err := forgeban.Scan(root.Dir)
		if err != nil {
			return nil, fmt.Errorf("the ban could not scan %s: %w — this is could-not-check, NOT a clean tree", root.What, err)
		}
		for _, finding := range found {
			finding.File = root.Prefix + finding.File
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

// TestNoForgeCLIShellout is the CI-gating half of the closed surface: no shipped source under
// tools/desk invokes `gh`, `glab`, or any other forge CLI.
//
// Findings are reconciled against the two in-tree registers (internal/forgeban/allowlist.go)
// rather than asserted to be zero. That is not a softening — the registers are Go source with
// a ratchet on their length, so a NEW call site cannot land without a reviewed diff, and a
// MIGRATED one cannot be quietly left listed. What it buys is that this gate reports the
// truth about the tree today instead of being switched off until the day the tree is perfect.
func TestNoForgeCLIShellout(t *testing.T) {
	findings, err := scanAudit(processLaunchAuditRoots)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("the ban's scan produced no findings at all, not even the known unresolved exec sites — " +
			"a scanner that sees nothing certifies everything; re-point deskTreeRoot")
	}
	rep := forgeban.Check(findings)
	if !rep.OK() {
		t.Fatalf("the forge surface is not closed:\n%s", rep.Explain())
	}
	t.Logf("forge-CLI ban: %d shipped call site(s) still invoke a forge CLI, across %d registered declarations "+
		"each carrying an exit condition (ratchet ceiling %d); %d exec site(s) carry a non-constant argv[0] and "+
		"are registered as could-not-check", rep.Allowed, forgeban.Ceiling(), forgeban.Ceiling(), rep.Unresolved)
}

// movedRunnerKey is the register key of the one process runner's exec site.
const movedRunnerKey = "loopadmin/process/run.go::runObserved::<unresolved>"

// regression: #5492 — moving the process runner out of tools/desk
// must not take its exec site out of the process-launch audit's sight.
func TestProcessLaunchAuditCoversLoopAdmin(t *testing.T) {
	covered := false
	for _, root := range processLaunchAuditRoots {
		covered = covered || (root.Dir == loopAdminTreeRoot && root.Prefix == "loopadmin/")
	}
	if !covered {
		t.Fatalf("the process-launch audit does not scan %s", loopAdminTreeRoot)
	}

	// The moved runner's exec site is found, and found registered.
	findings, err := scanAudit(processLaunchAuditRoots)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		found = found || f.Key() == movedRunnerKey
	}
	if !found {
		t.Fatalf("the audit did not find the moved runner's exec site %s", movedRunnerKey)
	}
	rep := forgeban.Check(findings)
	for _, f := range rep.UnregisteredUnresolved {
		if strings.HasPrefix(f.File, "loopadmin/") {
			t.Fatalf("loopadmin exec site %s is not registered", f.Key())
		}
	}
	for _, key := range append(append([]string{}, rep.StaleUnresolved...), rep.StaleAllowed...) {
		if key == movedRunnerKey {
			t.Fatalf("the moved runner's register entry %s is stale: the audit lost sight of it", key)
		}
	}

	// An unregistered exec site planted in a loopadmin fixture is reported.
	planted := t.TempDir()
	if err := os.MkdirAll(filepath.Join(planted, "process"), 0o700); err != nil {
		t.Fatal(err)
	}
	src := "package process\n\nimport \"os/exec\"\n\nfunc plantedLaunch(name string) error { return exec.Command(name).Run() }\n"
	if err := os.WriteFile(filepath.Join(planted, "process", "planted.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	var withPlant []auditRoot
	for _, root := range processLaunchAuditRoots {
		withPlant = append(withPlant, root)
		if root.Dir == loopAdminTreeRoot {
			withPlant = append(withPlant, auditRoot{planted, root.Prefix, "a planted loopadmin fixture"})
		}
	}
	findings, err = scanAudit(withPlant)
	if err != nil {
		t.Fatal(err)
	}
	rep = forgeban.Check(findings)
	caught := false
	for _, f := range rep.UnregisteredUnresolved {
		caught = caught || f.File == "loopadmin/process/planted.go"
	}
	if !caught || rep.OK() {
		t.Fatalf("the audit did not fail on an unregistered exec site planted under loopadmin/: %+v", rep.UnregisteredUnresolved)
	}
}

// --- No passthrough ----------------------------------------------------------------------

// passthroughNames are method names that mean "hand me an endpoint". They are matched
// EXACTLY (case-insensitively), never as substrings: `GetPullRequest` contains "Request" and
// is not a passthrough, and a check that flagged it would be relaxed until it stopped
// flagging anything.
var passthroughNames = map[string]bool{
	"do": true, "raw": true, "api": true, "apirequest": true, "request": true,
	"call": true, "exec": true, "query": true, "graphql": true, "send": true,
	"invoke": true, "roundtrip": true, "fetch": true, "get": true, "post": true,
	"put": true, "patch": true, "delete": true, "head": true, "options": true,
}

// passthroughPrefixes catch the compound spellings of the same thing (`RawRequest`,
// `DoGraphQL`, `APICall`) without the substring problem: they only fire at the START of a
// name, where a generic verb means the method is generic.
var passthroughPrefixes = []string{"Raw", "Do", "API", "Api", "Graph"}

// passthroughSuffixes catch a method that returns or takes a location rather than naming an
// operation (`CallEndpoint`, `RequestURL`).
var passthroughSuffixes = []string{"Endpoint", "URL", "Uri", "Route"}

func isPassthroughName(name string) (string, bool) {
	if passthroughNames[strings.ToLower(name)] {
		return "the name is a generic verb, not an operation", true
	}
	for _, p := range passthroughPrefixes {
		if len(name) > len(p) && strings.HasPrefix(name, p) && name[len(p)] >= 'A' && name[len(p)] <= 'Z' {
			return "the name begins with the generic verb " + p, true
		}
	}
	for _, s := range passthroughSuffixes {
		if name != s && strings.HasSuffix(name, s) {
			return "the name ends in " + s + ", which names a location rather than an operation", true
		}
	}
	return "", false
}

// endpointParamNames are parameter names that mean the CALLER supplies the address. A method
// taking one of these is a passthrough however typed its signature looks.
//
// `ref` is deliberately NOT here: DeleteRef takes a ref path, which is a coordinate INSIDE
// the named repo, not an address of an endpoint — and it is the one such parameter on the
// interface, validated by ValidateRefPath before a request exists. The subtest below proves
// that validation refuses a ref that would traverse out of the namespace, which is what keeps
// the exception honest.
var endpointParamNames = map[string]bool{
	"path": true, "endpoint": true, "url": true, "uri": true, "route": true,
	"apipath": true, "query": true, "rawquery": true, "method": true, "verb": true,
}

// TestForgeNoPassthrough is the shape check: the enumerated surface is the ONLY surface.
func TestForgeNoPassthrough(t *testing.T) {
	iface := forgeInterfaceMethods()
	if len(iface) == 0 {
		t.Fatal("reflection read no methods off Forge — a shape check over an empty method set proves nothing")
	}

	t.Run("no_generic_method_on_the_interface", func(t *testing.T) {
		for _, m := range iface {
			if why, bad := isPassthroughName(m); bad {
				t.Errorf("Forge.%s is an arbitrary-request method: %s. The interface is frozen at the "+
					"operations a shipping tool consumes; a generic verb re-opens the whole API behind one name", m, why)
			}
		}
	})

	// The layer that catches what a name check cannot: a method may be called anything and
	// still be a passthrough if it takes the address as an argument. Parameter names are not
	// available through reflection, so this reads the interface declaration itself.
	t.Run("no_method_takes_an_endpoint_argument", func(t *testing.T) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "forge.go", nil, 0)
		if err != nil {
			t.Fatalf("parsing forge.go: %v", err)
		}
		var checked int
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Forge" {
				return true
			}
			it, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}
			for _, fld := range it.Methods.List {
				fn, ok := fld.Type.(*ast.FuncType)
				if !ok || len(fld.Names) == 0 {
					continue
				}
				checked++
				for _, p := range fn.Params.List {
					for _, nm := range p.Names {
						if endpointParamNames[strings.ToLower(nm.Name)] {
							t.Errorf("Forge.%s takes a parameter named %q — a caller-supplied address makes the "+
								"method a passthrough whatever it is called", fld.Names[0].Name, nm.Name)
						}
					}
				}
			}
			return false
		})
		if checked == 0 {
			t.Fatal("the Forge interface declaration was not found in forge.go — this check read nothing")
		}
		if checked != len(iface) {
			t.Fatalf("the declaration walk saw %d methods but reflection sees %d — the walk is missing some",
				checked, len(iface))
		}
	})

	// THE SECOND LAYER, and the one the brief's defense-in-depth note is really about. A
	// passthrough does not have to be on the interface to be reachable: an exported
	// `func (g *GitHubForge) API(path string)` would satisfy every check above and still hand
	// any caller the whole API. So each backend's EXPORTED method set must equal the
	// interface's exactly — no extra exported method, in any spelling.
	t.Run("neither_backend_exports_a_method_outside_the_interface", func(t *testing.T) {
		want := map[string]bool{}
		for _, m := range iface {
			want[m] = true
		}
		for _, backend := range []any{(*GitHubForge)(nil), (*GitLabForge)(nil)} {
			rt := reflect.TypeOf(backend)
			name := rt.Elem().Name()
			var extra []string
			for i := 0; i < rt.NumMethod(); i++ {
				m := rt.Method(i).Name
				if !want[m] {
					extra = append(extra, m)
				}
			}
			sort.Strings(extra)
			if len(extra) > 0 {
				t.Errorf("%s exports %d method(s) outside the frozen Forge surface: %s — an exported method a "+
					"caller can reach without the interface is an escape hatch, whether or not it is called one",
					name, len(extra), strings.Join(extra, ", "))
			}
			// And the backend must implement the whole interface, or "equal" would be
			// satisfiable by a backend that simply has fewer methods.
			for m := range want {
				if _, ok := rt.MethodByName(m); !ok {
					t.Errorf("%s does not implement Forge.%s", name, m)
				}
			}
		}
	})

	// The interface's method set must equal what the stream's committed inventory tabulates.
	// A reflection check against a list restated inside this test would report agreement with
	// itself; the inventory is an independently maintained document, so the two can disagree.
	t.Run("method_set_equals_the_committed_inventory", func(t *testing.T) {
		const repoRoot = "../../../.."
		inv, sources, err := committedInventoryMethods(repoRoot)
		if err != nil {
			if os.IsNotExist(err) {
				t.Skip("could-not-check: this checkout carries no stream registers to reconcile against")
			}
			t.Fatalf("the stream registers are present but unreadable: %v — could-not-check, not a pass", err)
		}
		if len(inv) == 0 {
			t.Skip("could-not-check: no committed inventory table naming Forge operations was found")
		}
		var missing []string
		for _, m := range iface {
			if !inv[m] {
				missing = append(missing, m)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Fatalf("the committed inventory (%s) tabulates no row for: %s — an operation nobody wrote down is "+
				"an operation nobody reviewed", strings.Join(sources, ", "), strings.Join(missing, ", "))
		}
		t.Logf("the frozen surface is %d operations, every one tabulated in %s",
			len(iface), strings.Join(sources, ", "))
	})

	// DeleteRef is the one operation whose argument is path-shaped, so it is the one that has
	// to earn its place. These are the refusals that keep it from being `gh api -X DELETE` in
	// a typed coat.
	t.Run("the_one_path_shaped_argument_is_validated", func(t *testing.T) {
		refused := []struct{ ref, why string }{
			{"", "empty"},
			{"heads", "un-namespaced — a bare component is not a ref path"},
			{"heads/../../branches/main/protection", "traverses out of the ref namespace"},
			{"heads/..", "traverses out of the ref namespace"},
			{"heads/x..y", "a component containing \"..\" — git refuses it, and so must this"},
			{"heads/x?per_page=100", "carries a query string"},
			{"heads/x#frag", "carries a fragment"},
			{"heads/%2e%2e", "carries a percent escape that could decode to a traversal"},
			{"heads/-x", "a component beginning with a dash reads as an option"},
			{"heads/.hidden", "a component beginning with a dot"},
			{"heads/x.lock", "a .lock component"},
			{"heads/x y", "a space"},
			{"heads/x\ny", "a newline"},
			{"heads/x@{1}", "a reflog sequence"},
			{"heads//x", "an empty component"},
			{"/heads/x", "a leading separator"},
			{"heads/x/", "a trailing separator"},
		}
		for _, c := range refused {
			if _, err := ValidateRefPath(c.ref); err == nil {
				t.Errorf("ValidateRefPath(%q) was ACCEPTED — it should be refused: %s", c.ref, c.why)
			}
		}
		accepted := map[string]string{
			"heads/topic":            "heads/topic",
			"refs/heads/topic":       "heads/topic",
			"dispatch/repo--str--01": "dispatch/repo--str--01",
			"tags/v1.2.3":            "tags/v1.2.3",
			"heads/feat/x":           "heads/feat/x",
		}
		for in, want := range accepted {
			got, err := ValidateRefPath(in)
			if err != nil {
				t.Errorf("ValidateRefPath(%q) was refused: %v", in, err)
				continue
			}
			if got != want {
				t.Errorf("ValidateRefPath(%q) = %q, want %q", in, got, want)
			}
		}
	})
}
