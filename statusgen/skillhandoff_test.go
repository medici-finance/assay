package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
)

// skillhandoff_test.go — statusgen/21: the author-spec and author-brief skills
// hand changed planned work over through the shipped `statusgen context`
// operation. These tests run the journey the skills describe through the real
// binary and hold the skill text to the verbs, schemas and versions that ship.

const (
	shBriefA02 = "sources/repo-a/docs/streams/alpha/brief-02-reconciliation-report.md"
	shIDA02    = "example:repo-a:alpha:02"
	shRepo     = "../"
)

// shSkills are the files that state the shared context procedure.
var shSkills = []string{
	"plugins/assay/skills/author-spec/SKILL.md",
	"plugins/assay/skills/author-brief/SKILL.md",
	"plugins/assay/skills/worker-desk/SKILL.md",
	"plugins/assay/references/implementation-pickup.md",
}

// shDiff writes a one-hunk unified diff that replaces old lines with new ones,
// with up to three lines of context, as git does.
func shDiff(path string, old, new []byte) []byte {
	a := strings.SplitAfter(string(old), "\n")
	b := strings.SplitAfter(string(new), "\n")
	if a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	if b[len(b)-1] == "" {
		b = b[:len(b)-1]
	}
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	lo, hi := max(0, p-3), min(len(a), len(a)-s+3)
	tail := hi - (len(a) - s)
	var w strings.Builder
	fmt.Fprintf(&w, "--- a/%s\n+++ b/%s\n", path, path)
	fmt.Fprintf(&w, "@@ -%d,%d +%d,%d @@\n", lo+1, hi-lo, lo+1, (p-lo)+(len(b)-s-p)+tail)
	for _, l := range a[lo:p] {
		w.WriteString(" " + l)
	}
	for _, l := range a[p : len(a)-s] {
		w.WriteString("-" + l)
	}
	for _, l := range b[p : len(b)-s] {
		w.WriteString("+" + l)
	}
	for _, l := range a[len(a)-s : hi] {
		w.WriteString(" " + l)
	}
	return []byte(w.String())
}

// shAmend is the brief author's half of the journey: dispose alpha:02 `amend`
// with a candidate patch (or none), and bind it in an amendment manifest.
func shAmend(t *testing.T, dir string, h *contexthandoff.Handoff, withPatch bool) (handoff, manifest, patch string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, shBriefA02))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "Build the reconciliation report.\n",
		"Build the reconciliation report.\nAdd a layout-version column.\n", 1)
	p := shDiff("docs/streams/alpha/brief-02-reconciliation-report.md", raw, []byte(edited))
	cand := &contexthandoff.Patch{BaseRevision: contexthandoff.BlobRevision(raw)[:12], Digest: contexthandoff.PatchDigest(p)}

	var v map[string]any
	if err := json.Unmarshal(h.Canonical(), &v); err != nil {
		t.Fatal(err)
	}
	for _, x := range v["impacts"].([]any) {
		im := x.(map[string]any)
		if im["subject"] == shIDA02 {
			im["disposition"], im["reason"] = "amend", "author: the target changes this brief's task"
		}
	}
	rd := v["readiness"].(map[string]any)
	keep := []any{}
	for _, u := range rd["unresolved"].([]any) {
		if u != shIDA02 {
			keep = append(keep, u)
		}
	}
	rd["unresolved"] = keep
	if withPatch {
		v["amendments"] = append(v["amendments"].([]any), map[string]any{
			"id": "ob-1", "subject": shIDA02, "action": "amend",
			"changes":     []any{map[string]any{"aspect": "task", "before": "one report", "after": "report per layout version"}},
			"replaced_by": []any{}, "candidate": map[string]any{"base_revision": cand.BaseRevision, "digest": cand.Digest},
		})
	}
	hb, _ := json.Marshal(v)
	handoff = ctxWriteTemp(t, "handoff.json", hb)
	patch = ctxWriteTemp(t, "a02.patch", p)
	if !withPatch {
		return handoff, "", patch
	}
	amended, err := contexthandoff.DecodeHandoff(hb)
	if err != nil {
		t.Fatalf("the amended handoff does not validate: %v", err)
	}
	m := contexthandoff.Manifest{
		Schema: contexthandoff.ManifestSchema, DeclaredBy: h.DeclaredBy, Handoff: amended.Digest(),
		Entries: []contexthandoff.Entry{{
			Obligation: "ob-1", Brief: "alpha:02", Action: "amend",
			DeliveryPaths: []string{"docs/streams/alpha/brief-02-reconciliation-report.md"},
			Depends:       []string{}, TestCommands: []string{"make report"},
			ActiveWork: "none observed", Effort: "S",
			Binding: contexthandoff.Binding{Patch: cand},
		}},
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	return handoff, ctxWriteTemp(t, "manifest.json", mb), patch
}

// regression: statusgen/21 Verify row 1 — the spec-to-brief journey: a spec's
// receipt is discovered, the brief author amends with a candidate patch bound
// in a manifest, and check passes; an amend left without its patch fails.
func TestContextSpecToBriefJourney(t *testing.T) {
	bin := buildStatusgenBinary(t)
	dir := ctxCopyJourney(t)
	h, _ := ctxDiscoverReceipt(t, bin, dir)
	if im := shImpact(h, shIDA02); im == nil || im.Disposition != "unresolved" {
		t.Fatalf("discovery left %s as %+v, want an unresolved impact for the brief author", shIDA02, im)
	}

	handoff, manifest, patch := shAmend(t, dir, h, true)
	out, errOut, code := ctxRun(t, bin, "check", "--snapshot", dir, handoff, manifest, patch, "--json")
	var rep contexthandoff.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("check --json: %v\n%s%s", err, out, errOut)
	}
	if code != 0 || rep.Outcome != contexthandoff.OutcomePass {
		t.Fatalf("an amend with a bound candidate patch: exit %d, outcome %s\n%s%s", code, rep.Outcome, out, errOut)
	}

	// The same amend disposition with no amendment obligation is a failure.
	bare, _, _ := shAmend(t, dir, h, false)
	_, errOut, code = ctxRun(t, bin, "check", "--snapshot", dir, bare, patch)
	if code != 1 || !strings.Contains(errOut, "has no amendment obligation") {
		t.Errorf("an amend without a patch: exit %d, want 1 naming the missing obligation\n%s", code, errOut)
	}
}

func shImpact(h *contexthandoff.Handoff, subject string) *contexthandoff.Impact {
	for i := range h.Impacts {
		if h.Impacts[i].Subject == subject {
			return &h.Impacts[i]
		}
	}
	return nil
}

var (
	shVerbRE    = regexp.MustCompile("`statusgen context ([a-z][a-z-]*)")
	shSchemaRE  = regexp.MustCompile(`\b((?:context-handoff|amendment-manifest)-v[0-9]+)\b`)
	shVersionRE = regexp.MustCompile(`\bv(1\.0\.[0-9]+)\b`)
)

// shUnshipped returns each context verb, schema or release a skill text cites
// that this build does not ship.
func shUnshipped(t *testing.T, text string) []string {
	t.Helper()
	verbs := map[string]bool{}
	for _, c := range newContextCmd().Commands() {
		verbs[c.Name()] = true
	}
	changelog, err := os.ReadFile(filepath.Join(shRepo, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, m := range shVerbRE.FindAllStringSubmatch(text, -1) {
		if !verbs[m[1]] {
			bad = append(bad, "verb "+m[1])
		}
	}
	for _, m := range shSchemaRE.FindAllStringSubmatch(text, -1) {
		if _, err := os.Stat(filepath.Join(shRepo, "schemas", m[1]+".json")); err != nil {
			bad = append(bad, "schema "+m[1])
		}
	}
	for _, m := range shVersionRE.FindAllStringSubmatch(text, -1) {
		if !strings.Contains(string(changelog), "\n## v"+m[1]+" ") {
			bad = append(bad, "release v"+m[1])
		}
	}
	return bad
}

// regression: statusgen/21 Verify row 2 — a skill that names a verb, schema or
// release statusgen does not ship sends authors to an operation that is not
// there; each must also keep the manual bootstrap for an unsupported install.
func TestSkillsCiteShippedContextVerbs(t *testing.T) {
	for _, f := range shSkills {
		b, err := os.ReadFile(filepath.Join(shRepo, f))
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !shVerbRE.MatchString(text) {
			t.Errorf("%s names no `statusgen context` verb", f)
		}
		if bad := shUnshipped(t, text); len(bad) != 0 {
			t.Errorf("%s cites what does not ship: %v", f, bad)
		}
		if !strings.Contains(strings.ToLower(text), "manual bootstrap") {
			t.Errorf("%s drops the manual bootstrap for an install without the verb", f)
		}
	}
	for _, f := range shSkills[:2] {
		b, _ := os.ReadFile(filepath.Join(shRepo, f))
		for _, w := range []string{"v1.0.55", "v1.0.63", contexthandoff.HandoffSchema, contexthandoff.HandoffSchemaV3} {
			if !strings.Contains(string(b), w) {
				t.Errorf("%s does not state the supported version %q", f, w)
			}
		}
	}
	// The check itself must see a citation that does not ship.
	bogus := "Run `statusgen context reconcile` and write context-handoff-v9 (v1.0.999)."
	if bad := shUnshipped(t, bogus); len(bad) != 3 {
		t.Errorf("unshipped citations found %v, want the verb, the schema and the release", bad)
	}
}

// regression: statusgen/21 Verify row 3 — a product-bound handoff that drops
// the selection revision or folds product into architecture coverage loses
// what the work was selected against.
func TestSkillsProductTargetHandoff(t *testing.T) {
	bin := buildStatusgenBinary(t)
	dir := ctxCopyJourney(t)
	ctxEditFile(t, dir, ctxBriefA04, "\n---\n", "\noutcome: REQ-nightly-export\noutcome-revision: r1\n---\n")
	ctxProductTargets(t, dir, "r1", "r2")
	h, _ := ctxDiscoverReceipt(t, bin, dir)
	if h.Schema != contexthandoff.HandoffSchemaV3 {
		t.Fatalf("a product-bound handoff is %s, want %s", h.Schema, contexthandoff.HandoffSchemaV3)
	}
	b, _ := ctxProductOf(t, h, ctxIDA04)
	if ctxStr(b.Selected) != "r1" {
		t.Errorf("binding %+v, want the selection revision r1 kept", b)
	}
	if shImpact(h, ctxIDA04) == nil {
		t.Errorf("the architecture impact on %s is missing beside its product binding", ctxIDA04)
	}
	for _, p := range h.Readiness.Prerequisites {
		if p == contexthandoff.ArchitectureOnly {
			t.Errorf("a product-assessed receipt still states %q", p)
		}
	}
	for _, f := range shSkills[:2] {
		raw, _ := os.ReadFile(filepath.Join(shRepo, f))
		text := string(raw)
		for _, w := range []string{"outcome-revision", "first selected against", "product and architecture bindings separate"} {
			if !strings.Contains(strings.ToLower(text), strings.ToLower(w)) {
				t.Errorf("%s does not carry %q for a product-bound handoff", f, w)
			}
		}
	}
}
