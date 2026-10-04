package clicontract

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// repoRoot is the checkout root relative to this package.
const repoRoot = "../../../.."

func realTree(t *testing.T) (fs.FS, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(RegistryPath)))
	if err != nil {
		t.Skipf("could-not-check: %s is not in this file set (%v); this is not a pass", RegistryPath, err)
	}
	return os.DirFS(repoRoot), raw
}

func decode(t *testing.T, raw []byte) *Registry {
	t.Helper()
	reg, err := DecodeRegistry(raw)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// TestCLIInventory: the discovered maintained-entrypoint set equals the routed set, every
// row has an owning brief wired between the reference and the final gate, and the test
// names this brief's own Verify rows select are real tests.
func TestCLIInventory(t *testing.T) {
	fsys, raw := realTree(t)
	reg := decode(t, raw)

	found, err := Discover(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if problems := Validate(fsys, reg); len(problems) > 0 {
		t.Fatalf("%d routing problem(s):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}

	routed, excludedRows, byRule := 0, 0, 0
	owners := map[string]bool{}
	kinds := map[string]int{}
	for _, e := range reg.Entrypoints {
		if e.State == StateExcluded {
			excludedRows++
			continue
		}
		routed++
		owners[e.Owner] = true
		kinds[e.Kind]++
	}
	for _, d := range found {
		for _, r := range reg.ExclusionRules {
			if MatchPattern(r.Pattern, d.Path) {
				byRule++
				break
			}
		}
	}
	if routed == 0 || kinds["go"] == 0 || kinds["script"] == 0 {
		t.Fatalf("routed=%d go=%d script=%d: an inventory without both kinds proves nothing", routed, kinds["go"], kinds["script"])
	}
	if got, want := len(found), routed+excludedRows+byRule; got != want {
		t.Fatalf("discovered %d entrypoints, but rows+exclusions account for %d", got, want)
	}
	// The set must reach beyond tools/desk/cmd: standalone modules and operator scripts.
	for _, p := range []string{"statusgen", "qualgen", "tools/skillslint", "plugins/assay/scripts/assay-inbox.sh"} {
		if !hasRow(reg, p) {
			t.Errorf("inventory has no row for %s", p)
		}
	}
	t.Logf("discovered %d entrypoints: %d routed rows (%d go, %d script) across %d owners, %d excluded rows, %d excluded by rule",
		len(found), routed, kinds["go"], kinds["script"], len(owners), excludedRows, byRule)

	checkOwnTestNames(t, fsys)
}

func hasRow(reg *Registry, p string) bool {
	for _, e := range reg.Entrypoints {
		if e.Path == p {
			return true
		}
	}
	return false
}

var testNameRE = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)

// checkOwnTestNames asserts every test named in desktools-v2/15's Verify section is
// declared in the packages that brief delivers, so no row selects an empty set.
func checkOwnTestNames(t *testing.T, fsys fs.FS) {
	t.Helper()
	b, err := LoadBrief(fsys, "desktools-v2/15")
	if err != nil || b == nil {
		t.Fatalf("desktools-v2/15 brief not found: %v", err)
	}
	i := strings.Index(b.Text, "## Verify")
	j := strings.Index(b.Text, "## Pre-mortem")
	if i < 0 || j < i {
		t.Fatal("desktools-v2/15 has no Verify section")
	}
	names := map[string]bool{}
	for _, n := range testNameRE.FindAllString(b.Text[i:j], -1) {
		names[n] = true
	}
	if len(names) == 0 {
		t.Fatal("desktools-v2/15 Verify rows name no tests")
	}
	declared := map[string]bool{}
	for _, dir := range []string{"tools/desk/internal/cli", "tools/desk/internal/clicontract"} {
		for n := range declaredTests(fsys, dir) {
			declared[n] = true
		}
	}
	for n := range names {
		if !declared[n] {
			t.Errorf("desktools-v2/15 Verify names %s, which no test file declares", n)
		}
	}
}

var testFuncRE = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(t \*testing\.T\)`)

func declaredTests(fsys fs.FS, dir string) map[string]bool {
	out := map[string]bool{}
	entries, _ := fs.ReadDir(fsys, dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range testFuncRE.FindAllStringSubmatch(string(src), -1) {
			out[m[1]] = true
		}
	}
	return out
}

// TestCLIRoutingMutations plants each routing defect into an in-memory copy of the real
// tree and registry and requires Validate to report it for the planted reason. The
// unmutated baseline must be clean, or a case could pass on a pre-existing problem.
func TestCLIRoutingMutations(t *testing.T) {
	base, raw := realTree(t)
	if p := Validate(base, decode(t, raw)); len(p) > 0 {
		t.Fatalf("baseline is not clean, so no mutation result means anything:\n  %s", strings.Join(p, "\n  "))
	}
	ref := decode(t, raw)
	victim := rowFor(t, ref, "tools/desk/cmd/deskack")
	victimOwner := victim.Owner
	final, err := LoadBrief(base, ref.FinalGate)
	if err != nil || final == nil {
		t.Fatalf("final gate brief: %v", err)
	}
	child, err := LoadBrief(base, victimOwner)
	if err != nil || child == nil {
		t.Fatalf("owner brief %s: %v", victimOwner, err)
	}
	complexOwner := rowFor(t, ref, "statusgen").Owner

	type mutation struct {
		name  string
		files map[string]string
		edit  func(*Registry)
		want  string
	}
	cases := []mutation{
		{name: "added-desk-command",
			files: map[string]string{"tools/desk/cmd/zzplanted/main.go": "package main\n\nfunc main() {}\n"},
			want:  "unrouted entrypoint tools/desk/cmd/zzplanted"},
		{name: "added-module-main",
			files: map[string]string{"zzmodule/cmd/zztool/main.go": "package main\n\nfunc main() {}\n"},
			want:  "unrouted entrypoint zzmodule/cmd/zztool"},
		{name: "added-script",
			files: map[string]string{"tools/zzplanted.sh": "#!/bin/sh\necho planted\n"},
			want:  "unrouted entrypoint tools/zzplanted.sh"},
		{name: "added-extensionless-launcher",
			files: map[string]string{"scripts/zzlauncher": "#!/usr/bin/env bash\nexec true\n"},
			want:  "unrouted entrypoint scripts/zzlauncher"},
		{name: "omitted-row",
			edit: func(r *Registry) { dropRow(r, "tools/desk/cmd/deskack") },
			want: "unrouted entrypoint tools/desk/cmd/deskack"},
		{name: "orphan-owner",
			edit: func(r *Registry) { rowFor(t, r, "tools/desk/cmd/deskack").Owner = "desktools-v2/99" },
			want: "orphan owner desktools-v2/99"},
		{name: "unowned-row",
			edit: func(r *Registry) { rowFor(t, r, "tools/desk/cmd/deskack").Owner = "" },
			want: "row tools/desk/cmd/deskack has no owner"},
		{name: "missing-final-gate-dependency",
			files: map[string]string{final.File: dropDepend(t, final.Text, victimOwner)},
			want:  fmt.Sprintf("final gate %s does not depend on %s", ref.FinalGate, victimOwner)},
		{name: "child-skips-reference",
			files: map[string]string{child.File: dropDepend(t, child.Text, ref.Reference)},
			want:  fmt.Sprintf("owner %s does not depend on %s", victimOwner, ref.Reference)},
		{name: "six-simple-rows-one-owner",
			edit: func(r *Registry) {
				n := 0
				for i := range r.Entrypoints {
					e := &r.Entrypoints[i]
					if e.Complexity == "simple" && e.State == StatePending && n < MaxSimple+1 {
						e.Owner = victimOwner
						n++
					}
				}
			},
			want: "owner " + victimOwner + " over budget"},
		{name: "complex-row-shares-owner",
			edit: func(r *Registry) { rowFor(t, r, "tools/desk/cmd/deskack").Owner = complexOwner },
			want: "owner " + complexOwner + " over budget"},
		{name: "shipped-command-excluded-by-rule",
			edit: func(r *Registry) {
				dropRow(r, "tools/desk/cmd/deskack")
				r.ExclusionRules = append(r.ExclusionRules, Rule{Class: "fixture", Pattern: "tools/desk/cmd/deskack", Reason: "planted"})
			},
			want: "shipped entrypoint tools/desk/cmd/deskack is excluded"},
		{name: "shipped-command-excluded-row",
			edit: func(r *Registry) {
				e := rowFor(t, r, "tools/desk/cmd/deskack")
				e.State, e.Class, e.Reason, e.Owner = StateExcluded, "demo", "planted", ""
			},
			want: "shipped entrypoint tools/desk/cmd/deskack is an excluded row"},
		{name: "stale-row",
			edit: func(r *Registry) {
				e := *rowFor(t, r, "tools/desk/cmd/deskack")
				e.Path = "tools/desk/cmd/zzgone"
				r.Entrypoints = append(r.Entrypoints, e)
			},
			want: "stale row tools/desk/cmd/zzgone"},
		{name: "duplicate-row",
			edit: func(r *Registry) { r.Entrypoints = append(r.Entrypoints, *rowFor(t, r, "tools/desk/cmd/deskack")) },
			want: "duplicate row tools/desk/cmd/deskack"},
		{name: "stale-exclusion-rule",
			edit: func(r *Registry) {
				r.ExclusionRules = append(r.ExclusionRules, Rule{Class: "test", Pattern: "zznone/**", Reason: "planted"})
			},
			want: `stale exclusion rule "zznone/**"`},
		{name: "exclusion-rule-without-reason",
			edit: func(r *Registry) { r.ExclusionRules[0].Reason = " " },
			want: "has no reason"},
		{name: "imports-only-migration",
			edit: func(r *Registry) { rowFor(t, r, "tools/desk/cmd/deskack").State = StateMigrated },
			want: "migrated row tools/desk/cmd/deskack imports neither"},
		{name: "migrated-script-without-target",
			edit: func(r *Registry) { rowFor(t, r, "plugins/assay/scripts/pdfingest.sh").State = StateMigrated },
			want: "migrated script plugins/assay/scripts/pdfingest.sh must name"},
		{name: "empty-test-selector",
			files: map[string]string{child.File: child.Text + "\n| 99 | check | `cd tools/desk && go test -count=1 -run '' ./x` | exit 0 |\n"},
			want:  "owner " + victimOwner + " has an empty or non-test -run selector"},
		{name: "owner-without-cli-test",
			// Keep valid anchored selectors, but select only non-CLI tests. This must
			// fail the owner coverage check, not the malformed-selector check above.
			files: map[string]string{child.File: strings.ReplaceAll(child.Text, "TestCLI", "TestOther")},
			want:  "owner " + victimOwner + " selects no TestCLI test in its Verify rows"},
	}
	if len(cases) == 0 {
		t.Fatal("no mutation cases")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg := decode(t, raw)
			if c.edit != nil {
				c.edit(reg)
			}
			var fsys fs.FS = base
			if c.files != nil {
				fsys = newOverlay(base, c.files)
			}
			problems := Validate(fsys, reg)
			for _, p := range problems {
				if strings.Contains(p, c.want) {
					return
				}
			}
			t.Fatalf("planted %s; want a problem containing %q, got:\n  %s", c.name, c.want, strings.Join(problems, "\n  "))
		})
	}
}

func rowFor(t *testing.T, r *Registry, p string) *Entry {
	t.Helper()
	for i := range r.Entrypoints {
		if r.Entrypoints[i].Path == p {
			return &r.Entrypoints[i]
		}
	}
	t.Fatalf("registry has no row %s", p)
	return nil
}

func dropRow(r *Registry, p string) {
	out := r.Entrypoints[:0]
	for _, e := range r.Entrypoints {
		if e.Path != p {
			out = append(out, e)
		}
	}
	r.Entrypoints = out
}

// dropDepend removes one id from the frontmatter depends list of a brief text.
func dropDepend(t *testing.T, text, id string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^depends:.*$`)
	line := re.FindString(text)
	if line == "" || !strings.Contains(line, `"`+id+`"`) {
		t.Fatalf("brief does not list %s in depends: %q", id, line)
	}
	var kept []string
	for _, d := range frontmatterList(text, "depends") {
		if d != id {
			kept = append(kept, `"`+d+`"`)
		}
	}
	return strings.Replace(text, line, "depends: ["+strings.Join(kept, ", ")+"]", 1)
}

// Contract tests every migration child must declare in each migrated package.
var ownerTests = []string{"TestCLIHelpOffline", "TestCLIConfigFlow", "TestCLILegacyForms"}

// TestCLIOwnerMigrated is the per-child completion check: with CLI_OWNER=<stream>/<NN>,
// every row that brief owns is migrated (or retired), the registry still validates, and
// each migrated package — the row's own, or the Go entrypoint a migrated script names —
// declares the contract tests and passes them when run in its owning module. Unset, it
// skips — could-not-check, never a pass; child Verify rows grep for PASS.
func TestCLIOwnerMigrated(t *testing.T) {
	owner := os.Getenv("CLI_OWNER")
	if owner == "" {
		t.Skip("could-not-check: CLI_OWNER is unset; a child's Verify row sets it to that child's id")
	}
	fsys, raw := realTree(t)
	reg := decode(t, raw)
	if problems := Validate(fsys, reg); len(problems) > 0 {
		t.Fatalf("registry does not validate:\n  %s", strings.Join(problems, "\n  "))
	}
	rows := map[string]*Entry{}
	for i := range reg.Entrypoints {
		rows[reg.Entrypoints[i].Path] = &reg.Entrypoints[i]
	}
	n := 0
	pkgs := map[string]bool{}
	for _, e := range reg.Entrypoints {
		if e.Owner != owner {
			continue
		}
		n++
		switch e.State {
		case StateRetired:
			continue
		case StateMigrated:
		default:
			t.Errorf("%s is %s, not migrated or retired", e.Path, e.State)
			continue
		}
		pkg := e.Path
		if e.Kind == "script" {
			pkg = e.MigratedTo
		}
		declared := declaredTests(fsys, pkg)
		for _, name := range ownerTests {
			if !declared[name] {
				t.Errorf("%s: package %s declares no %s", e.Path, pkg, name)
			}
		}
		pkgs[pkg] = true
	}
	if n == 0 {
		t.Fatalf("owner %s routes no rows", owner)
	}
	if t.Failed() {
		return
	}
	sorted := make([]string, 0, len(pkgs))
	for p := range pkgs {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	for _, pkg := range sorted {
		runOwnerTests(t, pkg)
	}
}

// runOwnerTests runs the contract tests of one migrated package inside its owning
// module and requires each to PASS by name; a declared-but-skipped test is a failure.
func runOwnerTests(t *testing.T, pkg string) {
	t.Helper()
	mod, rel, ok := moduleOf(pkg)
	if !ok {
		t.Errorf("package %s has no go.mod above it in the checkout", pkg)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	target := "./" + rel
	if rel == "" {
		target = "."
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v",
		"-run", "^("+strings.Join(ownerTests, "|")+")$", target)
	cmd.Dir = filepath.Join(repoRoot, filepath.FromSlash(mod))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test %s in %s: %v\n%s", target, mod, err, out)
		return
	}
	for _, name := range ownerTests {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("package %s: %s did not PASS\n%s", pkg, name, out)
		}
	}
}

// moduleOf returns the nearest directory at or above pkg that holds a go.mod, and pkg's
// path relative to it.
func moduleOf(pkg string) (mod, rel string, ok bool) {
	for dir := pkg; ; dir = path.Dir(dir) {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(dir), "go.mod")); err == nil {
			r := strings.TrimPrefix(strings.TrimPrefix(pkg, dir), "/")
			return dir, r, true
		}
		if dir == "." || dir == "/" {
			return "", "", false
		}
	}
}
