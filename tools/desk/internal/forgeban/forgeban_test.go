package forgeban

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forgeban_test.go — the checker's own evidence.
//
// The reviewer's standing question about a ban like this is "is its detection provably
// resistant to a re-introduced call site in a spelling the test does not match?". The only
// honest answer is a corpus of spellings, each written out and each proven to be CAUGHT —
// which is what TestScannerCatchesEveryLaunchSpelling is. Every case there is a real evasion
// somebody would reach for: an import alias, a dot import, a named constant, an absolute
// path, a local wrapper, a method value, an argv assembled in a slice.
//
// The complement matters just as much. A ban that fires on the WORD `gh` gets suppressed the
// first time it flags a guard's own vocabulary, and a suppressed ban checks nothing;
// TestScannerIgnoresNonLaunchingMentions is that half.

// writeFixture writes one synthetic Go file into dir and returns dir.
func writeFixture(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", name, err)
	}
}

// scanOne scans a temp dir holding a single fixture file and returns its findings.
func scanOne(t *testing.T, src string) []Finding {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "fixture.go", src)
	f, err := Scan(dir)
	if err != nil {
		t.Fatalf("scanning fixture: %v", err)
	}
	return f
}

func invocations(fs []Finding) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Kind == KindInvocation {
			out = append(out, f)
		}
	}
	return out
}

// TestScannerCatchesEveryLaunchSpelling is the fail-first evidence for the ban: each case is
// source in which a forge CLI IS invoked, and the scanner must say so. A case that stopped
// being caught would fail here rather than in production, where it would look like a clean
// tree.
func TestScannerCatchesEveryLaunchSpelling(t *testing.T) {
	cases := []struct {
		name string
		src  string
		bin  string
	}{
		{
			name: "direct_exec_command",
			src: `package x
import "os/exec"
func f() { _ = exec.Command("gh", "pr", "list") }`,
			bin: "gh",
		},
		{
			name: "exec_command_context",
			src: `package x
import ("context"; "os/exec")
func f(ctx context.Context) { _ = exec.CommandContext(ctx, "glab", "mr", "list") }`,
			bin: "glab",
		},
		// The four cases below spell the binary as an ABSOLUTE PATH on purpose. Layer 2
		// matches a forge-CLI name exactly, so "/usr/local/bin/gh" is invisible to it —
		// which means only layer 1 can catch these, and each therefore tests the structural
		// half in isolation. Written as bare names they would pass on layer 2 alone and the
		// case would silently stop testing what it says it tests.
		{
			name: "aliased_os_exec_import",
			src: `package x
import xc "os/exec"
func f() { _ = xc.Command("/usr/local/bin/gh", "issue", "list") }`,
			bin: "gh",
		},
		{
			name: "dot_imported_os_exec",
			src: `package x
import . "os/exec"
func f() { _ = Command("/usr/local/bin/gh", "issue", "list") }`,
			bin: "gh",
		},
		{
			name: "command_context_resolved_structurally",
			src: `package x
import ("context"; "os/exec")
func f(ctx context.Context) { _ = exec.CommandContext(ctx, "/usr/local/bin/glab", "mr", "list") }`,
			bin: "glab",
		},
		{
			name: "argv0_behind_a_named_constant",
			src: `package x
import "os/exec"
const forgeBin = "/usr/local/bin/gh"
func f() { _ = exec.Command(forgeBin, "pr", "view") }`,
			bin: "gh",
		},
		{
			name: "absolute_path_to_the_binary",
			src: `package x
import "os/exec"
func f() { _ = exec.Command("/opt/homebrew/bin/gh", "pr", "view") }`,
			bin: "gh",
		},
		{
			name: "through_a_local_wrapper",
			src: `package x
func runCmd(name string, args ...string) {}
func f() { runCmd("gh", "pr", "ready", "1") }`,
			bin: "gh",
		},
		{
			name: "through_a_wrapper_with_a_leading_dir_argument",
			src: `package x
func runCmdIn(dir, name string, args ...string) {}
func f() { runCmdIn("", "gh", "pr", "view", "1") }`,
			bin: "gh",
		},
		{
			name: "through_a_method_value_field",
			src: `package x
type s struct{ run func(args ...string) (string, error) }
func (v *s) f() { _, _ = v.run("gh", "api", "-X", "DELETE", "repos/o/r/git/refs/x") }`,
			bin: "gh",
		},
		{
			name: "argv_assembled_in_a_slice_then_splatted",
			src: `package x
import "os/exec"
func f() {
	argv := []string{"gh", "api", "repos/o/r"}
	_ = exec.Command(argv[0], argv[1:]...)
}`,
			bin: "gh",
		},
		{
			name: "lookpath_of_a_forge_cli",
			src: `package x
import "os/exec"
func f() { _, _ = exec.LookPath("gh") }`,
			bin: "gh",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := invocations(scanOne(t, tc.src))
			if len(got) == 0 {
				t.Fatalf("the ban did NOT catch this spelling — it would land clean:\n%s", tc.src)
			}
			for _, f := range got {
				if f.Bin != tc.bin {
					t.Fatalf("caught, but named the wrong binary: got %q want %q (%s)", f.Bin, tc.bin, f)
				}
			}
			t.Logf("caught: %s", got[0])
		})
	}
}

// TestScannerIgnoresNonLaunchingMentions is the other half. Each case NAMES a forge CLI in a
// position that cannot launch one — the vocabulary of a guard that reasons about the name.
// A ban that flagged these would be turned off, and a ban that is off checks nothing.
func TestScannerIgnoresNonLaunchingMentions(t *testing.T) {
	cases := []struct{ name, src string }{
		{
			name: "map_key_in_an_allow_list",
			src: `package x
var readOnlyBinaries = map[string]bool{"gh": true, "statusgen": true}`,
		},
		{
			name: "switch_case_dispatching_on_the_name",
			src: `package x
func guard(bin string) int { switch bin { case "gh": return 1 }; return 0 }`,
		},
		{
			name: "equality_comparison_in_a_parser",
			src: `package x
func ok(f []string) bool { return len(f) >= 2 && f[0] != "gh" }`,
		},
		{
			name: "index_expression_into_a_table",
			src: `package x
var verbs = map[string]int{}
func n() int { return verbs["gh"] }`,
		},
		{
			name: "prose_only_reference_in_a_comment",
			src: `package x
// This tool no longer shells gh or glab; it reaches the forge through the interface.
func f() {}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := invocations(scanOne(t, tc.src)); len(got) != 0 {
				t.Fatalf("a non-launching mention was flagged as an invocation — this is the false positive that "+
					"gets a ban suppressed: %v", got)
			}
		})
	}
}

// TestScannerReportsUnresolvedRatherThanClean pins the three-state rule at the checker's own
// boundary: an exec site whose argv[0] it cannot resolve is reported as UNRESOLVED, not
// silently omitted. Omission would read as "checked, nothing there".
func TestScannerReportsUnresolvedRatherThanClean(t *testing.T) {
	got := scanOne(t, `package x
import "os/exec"
func f(bin string, args []string) { _ = exec.Command(bin, args...) }`)
	if len(got) != 1 || got[0].Kind != KindUnresolved {
		t.Fatalf("a non-constant argv[0] must be reported as unresolved, got %v", got)
	}
	if len(invocations(got)) != 0 {
		t.Fatal("an unresolved argv[0] must not be reported as an observed invocation")
	}
}

// TestScannerIgnoresTestFilesAndTestdata — a fixture spelling a banned argv is the ban's own
// evidence. If the scanner read _test.go, this very file would fail the gate.
func TestScannerIgnoresTestFilesAndTestdata(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "real.go", "package x\nfunc f() {}\n")
	writeFixture(t, dir, "real_test.go", `package x
import "os/exec"
func TestX() { _ = exec.Command("gh", "pr", "list") }`)
	if err := os.Mkdir(filepath.Join(dir, "testdata"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(dir, "testdata"), "golden.go", `package x
import "os/exec"
func g() { _ = exec.Command("gh", "api", "x") }`)

	got, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("test and testdata sources must not be scanned, got %v", got)
	}
}

// TestScanFailsClosedOnAnEmptyTree — a scan that read nothing must ERROR, not return a clean
// empty slice. A checker whose "pass" is indistinguishable from "never ran" is the exact
// false-clean this package exists to prevent, and a mis-pointed root is how it happens.
func TestScanFailsClosedOnAnEmptyTree(t *testing.T) {
	if _, err := Scan(t.TempDir()); err == nil {
		t.Fatal("a scan that found no shipped Go source returned success — it would certify anything")
	}
	if _, err := Scan(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("a scan of a missing root returned success")
	}
}

// TestCheckSeparatesThePermitFromTheLedger proves Check's two classes do not bleed into each
// other: a permit row can never clear an unresolved site, and a ledger row can never clear an
// observed invocation. If they did, one register would silently widen the other.
func TestCheckSeparatesThePermitFromTheLedger(t *testing.T) {
	inv := Finding{File: "a.go", Func: "f", Bin: "gh", Kind: KindInvocation}
	unr := Finding{File: "a.go", Func: "f", Kind: KindUnresolved}

	rep := Check([]Finding{inv, unr})
	if len(rep.Unallowed) != 1 || len(rep.UnregisteredUnresolved) != 1 {
		t.Fatalf("both classes must surface unregistered: %+v", rep)
	}
	if rep.Explain() == "" {
		t.Fatal("a failing report must explain itself")
	}
	if rep.OK() {
		t.Fatal("a report with unregistered findings must not read as OK")
	}

	// A register row that matches nothing must surface as STALE. Without this, a permit
	// outlives the call site it was written for and quietly becomes a place to park a name —
	// and the ratchet, which counts rows, stops measuring anything real.
	staleRep := Check(nil)
	if len(staleRep.StaleAllowed) != len(AllowedInvocations) {
		t.Fatalf("scanning a tree with no findings must report every permit row stale: got %d of %d",
			len(staleRep.StaleAllowed), len(AllowedInvocations))
	}
	if len(staleRep.StaleUnresolved) != len(UnresolvedArgv) {
		t.Fatalf("scanning a tree with no findings must report every ledger row stale: got %d of %d",
			len(staleRep.StaleUnresolved), len(UnresolvedArgv))
	}
	if staleRep.OK() {
		t.Fatal("a report carrying stale rows must not read as OK")
	}
}

// TestRegistersAreWellFormed — the registers themselves. Duplicate keys would make a stale
// row undetectable; a mismatched ceiling would make the ratchet decorative; a row with no
// reason is a name parked in a list.
func TestRegistersAreWellFormed(t *testing.T) {
	for _, reg := range []struct {
		name string
		rows []Allowance
	}{
		{"AllowedInvocations", AllowedInvocations},
		{"UnresolvedArgv", UnresolvedArgv},
	} {
		seen := map[string]bool{}
		for _, a := range reg.rows {
			if seen[a.Key] {
				t.Errorf("%s carries a duplicate key %q — a duplicated row can never be detected as stale", reg.name, a.Key)
			}
			seen[a.Key] = true
			if strings.TrimSpace(a.Reason) == "" {
				t.Errorf("%s row %q carries no reason", reg.name, a.Key)
			}
			if !strings.Contains(a.Key, "::") {
				t.Errorf("%s row %q is not a Finding key (<file>::<decl>::<bin>)", reg.name, a.Key)
			}
		}
	}
	if len(AllowedInvocations) != allowedInvocationCeiling {
		t.Fatalf("the ratchet is out of step: %d permit rows against a ceiling of %d.\n"+
			"If a call site was migrated, lower allowedInvocationCeiling to %d in the same change — a ratchet "+
			"that does not lock the gain in is not a ratchet.\nIf one was added, that is a deliberate widening "+
			"of the forge surface and belongs in a reviewed diff.",
			len(AllowedInvocations), allowedInvocationCeiling, len(AllowedInvocations))
	}
	// Every permit row must state what has to happen before it can be retired. A permit with
	// no exit condition is a decision to keep the CLI, which is not what this list is for.
	for _, a := range AllowedInvocations {
		if !strings.Contains(a.Reason, "TODO(forge-surface)") {
			t.Errorf("permit row %q carries no TODO(forge-surface) exit condition", a.Key)
		}
	}
}
