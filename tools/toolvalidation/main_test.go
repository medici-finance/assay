package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// healthyReport builds a muhar-format healthy report whose per-mutation lines
// use muhar's exact `  %-16s %s` layout, so the parser is exercised against the
// real on-the-wire shape rather than a convenient stand-in.
func healthyReport(caught, notCaught, couldNotMutate []string) string {
	var b strings.Builder
	b.WriteString("Harness healthy: baseline GREEN, positive control CAUGHT.\n")
	for _, n := range caught {
		b.WriteString("  " + pad16("CAUGHT") + " " + n + "\n")
	}
	for _, n := range notCaught {
		b.WriteString("  " + pad16("NOT_CAUGHT") + " " + n + "\n")
	}
	for _, n := range couldNotMutate {
		b.WriteString("  " + pad16("COULD_NOT_MUTATE") + " " + n + "  (mutation could not be planted)\n")
	}
	b.WriteString("Totals: X caught, Y NOT CAUGHT, Z could-not-mutate.\n")
	return b.String()
}

func pad16(s string) string {
	for len(s) < 16 {
		s += " "
	}
	return s
}

func brokenReport(reason string) string {
	return "HARNESS BROKEN — run discarded.\n  " + reason + "\n"
}

// specJSON writes a minimal muhar spec with the given control name and mutation
// names.
func specJSON(control string, mutations ...string) string {
	type m struct {
		Name string `json:"name"`
	}
	var ms []m
	for _, n := range mutations {
		ms = append(ms, m{Name: n})
	}
	obj := map[string]any{
		"test":      "go test ./...",
		"control":   map[string]string{"name": control},
		"mutations": ms,
	}
	raw, _ := json.MarshalIndent(obj, "", "  ")
	return string(raw)
}

// writeDeclaredSpecs materialises every declared control's spec file under root
// with two synthetic mutations each (named "<key> A" / "<key> B"), so a test can
// run the real declaredControls against a hermetic tree. Returns the per-key
// mutation names for report construction.
func writeDeclaredSpecs(t *testing.T, root string) map[string][2]string {
	t.Helper()
	names := map[string][2]string{}
	for _, c := range declaredControls {
		a := c.ReportKey + " A"
		bn := c.ReportKey + " B"
		names[c.ReportKey] = [2]string{a, bn}
		p := filepath.Join(root, filepath.FromSlash(c.Spec))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(specJSON("CONTROL "+c.ReportKey, a, bn)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return names
}

// writeAllHealthyReports writes a present, healthy report for every declared
// control (deskmerge split across its three shards), so the baseline is a
// COMPLETE pack. Individual tests then perturb one file.
func writeAllHealthyReports(t *testing.T, dir string, names map[string][2]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range declaredControls {
		nm := names[c.ReportKey]
		if c.Shards > 0 {
			// shard 0 carries A, shard 1 carries B, shard 2 is empty — union is {A,B}.
			write(t, dir, c.ReportKey+".0.report", healthyReport([]string{nm[0]}, nil, nil))
			write(t, dir, c.ReportKey+".1.report", healthyReport([]string{nm[1]}, nil, nil))
			write(t, dir, c.ReportKey+".2.report", healthyReport(nil, nil, nil))
			continue
		}
		write(t, dir, c.ReportKey+".report", healthyReport([]string{nm[0], nm[1]}, nil, nil))
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- the three tests the brief's Verify table names ----

func TestMissingReportIsOmittedAndExitsThree(t *testing.T) {
	root := t.TempDir()
	names := writeDeclaredSpecs(t, root)
	reports := filepath.Join(t.TempDir(), "reports")
	writeAllHealthyReports(t, reports, names)

	// With every report present, the pack is complete and exits 0.
	if code := runPack(t, root, reports, t.TempDir()); code != 0 {
		t.Fatalf("complete set: want exit 0, got %d", code)
	}

	// Remove one declared control's report; the pack must name it in `omitted`
	// and exit 3 — never a quietly smaller pack.
	victim := "reviewloop.report"
	if err := os.Remove(filepath.Join(reports, victim)); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	code := runPack(t, root, reports, out)
	if code != 3 {
		t.Fatalf("missing report: want exit 3, got %d", code)
	}
	p := readJSONPack(t, out)
	if p.Complete {
		t.Fatal("pack marked complete despite a missing report")
	}
	if !omittedHasSpec(p, "tools/desk/cmd/reviewloop/mutations.json") {
		t.Fatalf("omitted does not name the missing control's spec: %+v", p.Omitted)
	}
}

func TestHarnessBrokenIsNotAPass(t *testing.T) {
	root := t.TempDir()
	names := writeDeclaredSpecs(t, root)
	reports := filepath.Join(t.TempDir(), "reports")
	writeAllHealthyReports(t, reports, names)

	// Replace bodycheck's report with a HARNESS BROKEN discard (muhar exit 2).
	write(t, reports, "bodycheck.report", brokenReport("positive control was NOT CAUGHT — the harness is broken; discard the run"))

	out := t.TempDir()
	code := runPack(t, root, reports, out)
	if code != 3 {
		t.Fatalf("harness broken: want exit 3 (incomplete), got %d", code)
	}
	p := readJSONPack(t, out)

	g := gateBySpec(t, p, "tools/desk/internal/deskkit/mutations.json")
	if g.Harness != "broken" {
		t.Fatalf("gate harness = %q, want broken", g.Harness)
	}
	if len(g.Mutations) == 0 {
		t.Fatal("broken gate rendered no mutation rows; every declared control must appear as could-not-check")
	}
	for _, m := range g.Mutations {
		if m.Verdict != vCouldNotCheck {
			t.Fatalf("broken-harness mutation %q rendered %q; must be could-not-check, never a pass or fail", m.Name, m.Verdict)
		}
	}
	// Distinguishable from a caught mutation in the output.
	md := readMarkdown(t, out, p.Tag)
	if !strings.Contains(md, "could-not-check") {
		t.Fatal("markdown does not surface could-not-check for the broken gate")
	}
	if !omittedHasSpec(p, "tools/desk/internal/deskkit/mutations.json") {
		t.Fatal("broken gate not named in omitted")
	}
}

func TestDeclaredSetDriftIsReportedBothWays(t *testing.T) {
	root := t.TempDir()
	writeDeclaredSpecs(t, root)

	// Direction 1: a declared spec with no file — remove one declared spec.
	gone := filepath.Join(root, filepath.FromSlash("tools/desk/cmd/deskclose/mutations.json"))
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	// Direction 2: a spec file in no declaration — add one the set never names.
	extra := filepath.Join(root, filepath.FromSlash("tools/desk/cmd/extra/mutations.json"))
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte(specJSON("c", "x")), 0o644); err != nil {
		t.Fatal(err)
	}

	d := assembleDrift(root)
	if !contains(d.DeclaredSpecsWithNoFile, "tools/desk/cmd/deskclose/mutations.json") {
		t.Fatalf("declared-but-no-file not reported: %+v", d.DeclaredSpecsWithNoFile)
	}
	if !contains(d.SpecFilesNotDeclared, "tools/desk/cmd/extra/mutations.json") {
		t.Fatalf("on-disk-but-not-declared not reported: %+v", d.SpecFilesNotDeclared)
	}

	// Each is a distinct report line in the markdown.
	p, err := assemble(root, t.TempDir(), "v0.0.0-test", "2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	md := p.renderMarkdown()
	if !strings.Contains(md, "Declared here but no file on disk") ||
		!strings.Contains(md, "On disk but not declared") {
		t.Fatal("markdown does not carry both drift directions as distinct lines")
	}
}

// ---- supporting coverage ----

func TestCompleteExitsZeroAndWritesBothFormats(t *testing.T) {
	root := t.TempDir()
	names := writeDeclaredSpecs(t, root)
	reports := filepath.Join(t.TempDir(), "reports")
	writeAllHealthyReports(t, reports, names)

	out := t.TempDir()
	if code := runPack(t, root, reports, out); code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "tool-validation-v0.0.0-test.md")); err != nil {
		t.Fatalf("md not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "tool-validation-v0.0.0-test.json")); err != nil {
		t.Fatalf("json not written: %v", err)
	}
	p := readJSONPack(t, out)
	if !p.Complete || p.Declared != len(declaredControls) {
		t.Fatalf("pack not complete/declared: complete=%v declared=%d", p.Complete, p.Declared)
	}
	// Every mutation caught (all healthy).
	for _, g := range p.Gates {
		for _, m := range g.Mutations {
			if m.Verdict != vCaught {
				t.Fatalf("gate %q mutation %q = %q, want caught", g.Gate, m.Name, m.Verdict)
			}
		}
	}
}

func TestCouldNotMutateIsDistinctFromCouldNotCheck(t *testing.T) {
	// A HEALTHY report whose one mutation could-not-mutate is a different fact
	// from a BROKEN report (could-not-check): the pack must keep them apart.
	rep := healthyReport(nil, nil, []string{"guard X"})
	o := parseReport(rep)
	if o.Broken {
		t.Fatal("healthy-with-could-not-mutate misread as broken")
	}
	if v := matchVerdict("guard X", o.Verdicts); v != vCouldNotMut {
		t.Fatalf("verdict = %q, want could-not-mutate", v)
	}
	if v := matchVerdict("guard X", parseReport(brokenReport("boom")).Verdicts); v != vCouldNotCheck {
		t.Fatalf("broken report verdict = %q, want could-not-check", v)
	}
}

func TestNameWithParenthesesNotTruncated(t *testing.T) {
	// A could-not-mutate line carries a `  (err)` suffix; a name that itself
	// ends in parentheses must still match, and must not be confused with the
	// error suffix.
	name := "the same actor may propose AND confirm (the two-role property collapses)"
	rep := healthyReport(nil, nil, []string{name})
	o := parseReport(rep)
	if v := matchVerdict(name, o.Verdicts); v != vCouldNotMut {
		t.Fatalf("verdict = %q, want could-not-mutate for a name ending in parens", v)
	}
}

func TestMissingShardMakesGateIncomplete(t *testing.T) {
	root := t.TempDir()
	names := writeDeclaredSpecs(t, root)
	reports := filepath.Join(t.TempDir(), "reports")
	writeAllHealthyReports(t, reports, names)
	// Drop one deskmerge shard: a partial shard set covers only part of the spec.
	if err := os.Remove(filepath.Join(reports, "deskmerge.1.report")); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if code := runPack(t, root, reports, out); code != 3 {
		t.Fatalf("missing shard: want exit 3, got %d", code)
	}
	p := readJSONPack(t, out)
	if !omittedHasSpec(p, "tools/desk/cmd/deskmerge/mutations.json") {
		t.Fatalf("missing shard did not omit the deskmerge gate: %+v", p.Omitted)
	}
}

func TestHeaderCarriesNonClaimRegister(t *testing.T) {
	// The generated header must state it is not an audit opinion and uses the
	// "does not" non-claim register — Verify item 10 greps source for this.
	if !strings.Contains(packHeader, "audit opinion") || !strings.Contains(packHeader, "does not") {
		t.Fatal("packHeader missing the non-claim register")
	}
}

// ---- helpers ----

func runPack(t *testing.T, root, reports, out string) int {
	t.Helper()
	var so, se bytes.Buffer
	return run([]string{"-root", root, "-reports", reports, "-tag", "v0.0.0-test", "-out", out, "-date", "2026-09-01"}, &so, &se)
}

func readJSONPack(t *testing.T, out string) *pack {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(out, "tool-validation-v0.0.0-test.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p pack
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func readMarkdown(t *testing.T, out, tag string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(out, "tool-validation-"+tag+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func gateBySpec(t *testing.T, p *pack, spec string) gateRecord {
	t.Helper()
	for _, g := range p.Gates {
		if g.Spec == spec {
			return g
		}
	}
	t.Fatalf("no gate for spec %s", spec)
	return gateRecord{}
}

func omittedHasSpec(p *pack, spec string) bool {
	for _, o := range p.Omitted {
		if o.Spec == spec {
			return true
		}
	}
	return false
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
