package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// placeholderFile renders a valid placeholder-v1 file body. gate/effort are left
// to derivation unless a non-empty override is given via extra frontmatter lines.
func placeholderFile(stream string, issue int, labels string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nschema: placeholder-v1\nbrief: %s/issue-%d\nissue: %d\nrepo: example-org/tracker\nwave: 0\nstatus: todo\n",
		stream, issue, issue)
	if labels != "" {
		fmt.Fprintf(&b, "labels: [%s]\n", labels)
	}
	for _, e := range extra {
		fmt.Fprintf(&b, "%s\n", e)
	}
	fmt.Fprintf(&b, "---\nSee issue #%d — the issue body is the spec.\n", issue)
	return b.String()
}

func TestDerivePlaceholderGate(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		title  string
		want   string
	}{
		{"plain bug → model", []string{"bug"}, "frontend keydown bubbles to card", "model"},
		{"no labels, no title → model", nil, "", "model"},
		{"security label → human", []string{"bug", "security"}, "x", "human"},
		{"funds label → human", []string{"funds"}, "", "human"},
		{"auth keyword in title → human", []string{"bug"}, "unauthenticated caller — no auth gate", "human"},
		{"funds keyword in title → human", []string{"bug"}, "funds leak on settlement", "human"},
		{"security keyword in title → human", []string{"bug"}, "security-sensitive token path", "human"},
		{"word-boundary: 'authored' is not 'auth' → model", []string{"bug"}, "authored by someone", "model"},
		{"case-insensitive label → human", []string{"SECURITY"}, "", "human"},
		// A product-domain risk word is NOT in the neutral open-core vocabulary,
		// so it does not gate by default until a deployment registers it.
		{"unregistered product word → model", []string{"settlement"}, "settlement algebra drift", "model"},
	}
	for _, c := range cases {
		if got := derivePlaceholderGate(c.labels, c.title); got != c.want {
			t.Errorf("%s: derivePlaceholderGate(%v, %q) = %q, want %q", c.name, c.labels, c.title, got, c.want)
		}
	}
}

// TestPlaceholderGateFromConfigRiskExtra proves a deployment's product-domain
// human-gate is PRESERVED in the shipped tree by wiring it from the config the
// deployment already sets (ASSAY_RISK_PATH_TRIGGERS_EXTRA) — not lost when the
// domain was removed from the compiled-in default. The mechanism is
// domain-agnostic, so this uses a NEUTRAL domain word ("settlement") rather than
// naming any product's; the house's real config value is a product path this
// open-core tree deliberately never spells out. With the trigger configured, a
// matching placeholder derives gate: human; with the config unset, the neutral
// default derives gate: model.
func TestPlaceholderGateFromConfigRiskExtra(t *testing.T) {
	const domain = "settlement" // stands in for a deployment's product risk path

	// Restore the neutral derived matchers afterwards regardless of path, so this
	// test never leaks a configured risk word into other cases.
	t.Cleanup(func() {
		placeholderRiskLabels = buildRiskLabelSet(riskGateLabels)
		placeholderRiskKeywordRe = buildRiskKeywordRe(riskGateKeywords)
	})

	// Config carries a <domain>/ path trigger (the shape the house deployment sets).
	roster := scanExampleRoster()
	roster[scanEnvRiskPathTriggersExtra] = domain + "/,auth/"
	scanWithRoster(t, roster)
	if !scanEffectiveConfig().Configured() {
		t.Fatalf("test roster did not load: %v", scanEffectiveConfig().Problems)
	}
	if got := derivePlaceholderGate([]string{domain}, ""); got != "human" {
		t.Errorf("with %s/ configured, a %s label must derive gate:human, got %q", domain, domain, got)
	}
	if got := derivePlaceholderGate([]string{"bug"}, strings.ToUpper(domain)+" algebra drift"); got != "human" {
		t.Errorf("with %s/ configured, a %s keyword in the title must derive gate:human, got %q", domain, domain, got)
	}

	// Config unset → the neutral default: the product domain is NOT a trigger.
	scanWithNoRoster(t)
	scanEffectiveConfig()
	if got := derivePlaceholderGate([]string{domain}, ""); got != "model" {
		t.Errorf("with no risk config, a %s label must fall back to the neutral default gate:model, got %q", domain, got)
	}
}

// TestRiskGateVocabularyExtension proves the risk vocabulary is a configurable
// set, not a compiled-in literal: a product-domain word is model by default and
// gates human only after a deployment registers it. Globals are saved/restored
// so the extension does not leak into other cases.
func TestRiskGateVocabularyExtension(t *testing.T) {
	const domain = "settlement"
	if got := derivePlaceholderGate([]string{domain}, ""); got != "model" {
		t.Fatalf("open-core default: %q must be model (no product risk baked in), got %q", domain, got)
	}
	labelsSave, keywordsSave := riskGateLabels, riskGateKeywords
	setSave, reSave := placeholderRiskLabels, placeholderRiskKeywordRe
	t.Cleanup(func() {
		riskGateLabels, riskGateKeywords = labelsSave, keywordsSave
		placeholderRiskLabels, placeholderRiskKeywordRe = setSave, reSave
	})
	registerRiskGateVocabulary([]string{domain}, []string{domain})
	if got := derivePlaceholderGate([]string{domain}, ""); got != "human" {
		t.Errorf("after extension: %q label must gate human, got %q", domain, got)
	}
	if got := derivePlaceholderGate([]string{"bug"}, "algebra drift in "+domain); got != "human" {
		t.Errorf("after extension: %q keyword must gate human, got %q", domain, got)
	}
}

func TestDerivePlaceholderEffort(t *testing.T) {
	// Effort is always M (unknown-until-triaged) regardless of labels/title.
	for _, labels := range [][]string{nil, {"bug"}, {"security", "funds"}} {
		if got := derivePlaceholderEffort(labels, "anything at all"); got != "M" {
			t.Errorf("derivePlaceholderEffort(%v) = %q, want M", labels, got)
		}
	}
}

func TestParsePlaceholderFileValid(t *testing.T) {
	dir := t.TempDir()

	// Derived gate model (bug only).
	ph, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-300.md", placeholderFile("issue-loop", 300, "bug")))
	if err != nil || !ok {
		t.Fatalf("valid placeholder: ok=%v err=%v", ok, err)
	}
	if ph.Issue != 300 || ph.Repo != "example-org/tracker" {
		t.Errorf("wrong issue/repo: %+v", ph)
	}
	if ph.Gate != "model" || ph.Effort != "M" || ph.Status != "todo" || ph.Wave != 0 {
		t.Errorf("derivation/defaults wrong: gate=%q effort=%q status=%q wave=%d", ph.Gate, ph.Effort, ph.Status, ph.Wave)
	}

	// Derived gate human (security label).
	ph2, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-301.md", placeholderFile("issue-loop", 301, "bug, security")))
	if err != nil || !ok {
		t.Fatalf("security placeholder: ok=%v err=%v", ok, err)
	}
	if ph2.Gate != "human" {
		t.Errorf("security label should derive gate:human, got %q", ph2.Gate)
	}

	// Explicit gate override wins over derivation.
	ph3, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-302.md", placeholderFile("issue-loop", 302, "security", "gate: model", "effort: L")))
	if err != nil || !ok {
		t.Fatalf("override placeholder: ok=%v err=%v", ok, err)
	}
	if ph3.Gate != "model" {
		t.Errorf("explicit gate:model must override the risk-derived human, got %q", ph3.Gate)
	}
	if ph3.Effort != "L" {
		t.Errorf("explicit effort:L must override the M default, got %q", ph3.Effort)
	}
}

func TestParsePlaceholderFileExemptAndMalformed(t *testing.T) {
	dir := t.TempDir()

	// A brief-v1 (or any non-placeholder) schema is EXEMPT: (nil, false, nil).
	briefish := "---\nschema: brief-v1\nbrief: x/01\n---\nbody\n"
	if ph, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-10.md", briefish)); ph != nil || ok || err != nil {
		t.Errorf("non-placeholder schema must be exempt (nil,false,nil); got (%v,%v,%v)", ph, ok, err)
	}
	// No frontmatter → exempt.
	if ph, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-11.md", "just prose, no frontmatter\n")); ph != nil || ok || err != nil {
		t.Errorf("no-frontmatter must be exempt; got (%v,%v,%v)", ph, ok, err)
	}
	// Opted-in but missing a required field (repo) → error.
	noRepo := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-12\nissue: 12\n---\nbody\n"
	if _, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-12.md", noRepo)); ok || err == nil {
		t.Errorf("missing repo must error; got ok=%v err=%v", ok, err)
	} else if !strings.Contains(err.Error(), "repo") {
		t.Errorf("error should name the missing repo field, got: %v", err)
	}
}

// placeholderRepo copies goodrepo and drops the given placeholder files into the
// alpha stream directory, returning the loaded+attached streams and the root.
func placeholderRepo(t *testing.T, files map[string]string) ([]*Stream, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		writeTemp(t, filepath.Join(root, "docs/streams/alpha"), name, body)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	return streams, root
}

// --- close-candidate tests (D4) ---

// TestCloseCandidateParsesAndRenders — a placeholder carrying a valid
// close-candidate: verdict parses the field and renders a [close:...] marker on
// its Next-up row.
func TestCloseCandidateParsesAndRenders(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "issue-820.md",
		placeholderFile("alpha", 820, "bug", "close-candidate: WONTFIX"))
	ph, ok, err := parsePlaceholderFile(path)
	if err != nil || !ok {
		t.Fatalf("valid close-candidate placeholder: ok=%v err=%v", ok, err)
	}
	if ph.CloseCandidate != "WONTFIX" {
		t.Errorf("close-candidate = %q, want WONTFIX", ph.CloseCandidate)
	}
	if got := ph.toBrief().Title; !strings.Contains(got, "[close:WONTFIX]") {
		t.Errorf("Next-up title = %q, want a [close:WONTFIX] marker", got)
	}
}

// TestCloseCandidateLintRejectsUnknown — an unrecognised close-candidate: verdict
// is a lint PROBLEM (the enum is closed).
func TestCloseCandidateLintRejectsUnknown(t *testing.T) {
	streams, _ := placeholderRepo(t, map[string]string{
		"issue-821.md": placeholderFile("alpha", 821, "bug", "close-candidate: BOGUS"),
	})
	problems, _ := checkPlaceholderFiles(streams)
	if !hasProblem(problems, "invalid close-candidate") {
		t.Errorf("want a problem for an unknown close-candidate value, got: %v", problems)
	}
}

// TestCloseCandidateValidAccepted — each of the four sanctioned verdicts parses
// and lints clean.
func TestCloseCandidateValidAccepted(t *testing.T) {
	for _, v := range []string{"FIXED-NOT-CLOSED", "WONTFIX", "DUPLICATE", "STALE"} {
		streams, _ := placeholderRepo(t, map[string]string{
			"issue-822.md": placeholderFile("alpha", 822, "bug", "close-candidate: "+v),
		})
		problems, _ := checkPlaceholderFiles(streams)
		if hasProblem(problems, "close-candidate") {
			t.Errorf("verdict %q should lint clean, got: %v", v, problems)
		}
	}
}

// TestPlaceholderNextUpFirstClass — a placeholder attaches as a synthetic Brief,
// appears as a Next-up pick with its derived gate surfaced, and is claim-aware.
func TestPlaceholderNextUpFirstClass(t *testing.T) {
	streams, _ := placeholderRepo(t, map[string]string{
		"issue-77.md": placeholderFile("alpha", 77, "bug, security"), // → gate human
	})

	var alpha *Stream
	for _, s := range streams {
		if s.Name == "alpha" {
			alpha = s
		}
	}
	if alpha == nil || len(alpha.Placeholders) != 1 {
		t.Fatalf("expected 1 placeholder on alpha, got %+v", alpha)
	}

	// Appears in Next-up with the derived gate in its title.
	picks := nextUp(streams, nil, nil).Picks
	var found *Pick
	for i := range picks {
		if picks[i].Brief.Num == "issue-77" {
			found = &picks[i]
		}
	}
	if found == nil {
		t.Fatalf("placeholder issue-77 not in Next-up: %+v", picks)
	}
	if !strings.Contains(found.Brief.Title, "gate:human") {
		t.Errorf("derived gate not surfaced on the Next-up row: %q", found.Brief.Title)
	}

	// Claim-awareness: an open fix/issue-77 branch excludes it.
	claimed := claimedPlaceholders(streams, []string{"fix/issue-77-auth-gate", "main"})
	if !claimed["alpha/issue-77"] {
		t.Fatalf("fix/issue-77 branch should claim alpha/issue-77, got %v", claimed)
	}
	for _, p := range nextUp(streams, claimed, nil).Picks {
		if p.Brief.Num == "issue-77" {
			t.Fatal("a claimed placeholder must be excluded from Next-up")
		}
	}
}

// TestPlaceholderExemptFromBriefLint — a placeholder trips NONE of the brief-v1
// gates (Verify-table, Evidence/attribution, brief-file schema), and a well-formed
// placeholder is not flagged by its own checker.
func TestPlaceholderExemptFromBriefLint(t *testing.T) {
	streams, _ := placeholderRepo(t, map[string]string{
		"issue-77.md": placeholderFile("alpha", 77, "bug, security"),
	})

	briefProblems, _ := checkBriefFiles(streams)
	if hasProblem(briefProblems, "issue-77") {
		t.Errorf("placeholder tripped checkBriefFiles: %v", briefProblems)
	}
	if hasProblem(verifySectionProblems(streams), "issue-77") {
		t.Error("placeholder tripped the Verify-table structure lint (it has no Verify by design)")
	}
	if hasProblem(attributionProblems(streams), "issue-77") {
		t.Error("placeholder tripped the attribution/Evidence lint")
	}
	if p, _ := checkPlaceholderFiles(streams); len(p) != 0 {
		t.Errorf("a valid placeholder must not be flagged by checkPlaceholderFiles: %v", p)
	}
}

// TestCheckPlaceholderFilesProblems — the reduced-schema checker catches a
// filename/frontmatter mismatch and an invalid gate override.
func TestCheckPlaceholderFilesProblems(t *testing.T) {
	// brief id does not match the filename-derived id.
	badID := "---\nschema: placeholder-v1\nbrief: alpha/issue-999\nissue: 55\nrepo: o/r\n---\nbody\n"
	// issue number in frontmatter disagrees with the filename.
	streams, _ := placeholderRepo(t, map[string]string{
		"issue-55.md": badID,
	})
	problems, _ := checkPlaceholderFiles(streams)
	if !hasProblem(problems, "issue-55.md", "filename-derived id") {
		t.Errorf("want a brief-id mismatch problem, got: %v", problems)
	}
}

// TestPlaceholderLintGreen — the full run(--lint) path stays exit 0 with a
// placeholder present (Verify item 3 in miniature).
func TestPlaceholderLintGreen(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, filepath.Join(root, "docs/streams/alpha"), "issue-88.md", placeholderFile("alpha", 88, "bug"))
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint with a placeholder present exited %d, want 0", code)
	}
}
