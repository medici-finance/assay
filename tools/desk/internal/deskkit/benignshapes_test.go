package deskkit

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Tests for the benign-shape registry (benignshapes.go). The registry's Admitted and Bounds
// name corpus fixtures or `case:` entries; the cases live here, generated at test time from
// a fixed seed or split at a seam, so no source line carries a credential-shaped literal.

// benignRand returns n characters of alphabet from a fixed-seed stream; one stream per
// case name, so adding a case never reshuffles another.
func benignRand(name, alphabet string, n int) string {
	var seed int64 = 61
	for i := 0; i < len(name); i++ {
		seed = seed*31 + int64(name[i])
	}
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b)
}

const (
	benignLowerHex = "0123456789abcdef"
	benignUpperHex = "0123456789ABCDEF"
	benignBase62   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// benignAdmitCases are the in-test examples an entry admits: clean under the production
// registry, refused once the entry is removed.
func benignAdmitCases() map[string]string {
	return map[string]string{
		"git-sha-40":         "merged at " + benignRand("git-sha-40", benignLowerHex, 40),
		"git-sha-64":         "object " + benignRand("git-sha-64", benignLowerHex, 64),
		"go-subtest-name":    "run BenchmarkSupersede" + "MarkerCitation/ok/go here",
		"diffstat-bar":       "docs/a.md | 52 " + strings.Repeat("+", 52),
		"digest-key-sha":     "ARG BASE_DIGEST_HEX=" + benignRand("digest-key-sha", benignLowerHex, 64),
		"caps-key-test-name": "make test TEST=TestBenignEntries" + "AreBoundedByRefusals",
		"pgp-recipient":      "creation_rules:\n  - pgp: " + benignRand("pgp-recipient", benignUpperHex, 40) + "\n",
	}
}

// benignBoundCases are the in-test refusals that bound an entry: refused by the high-entropy
// loop under the production registry. Each is the entry's own shape one step outside it.
func benignBoundCases() map[string]string {
	return map[string]string{
		"hex-41":               "merged at " + benignRand("hex-41", benignLowerHex, 41),
		"hex-upper-40":         "merged at " + benignRand("hex-upper-40", benignUpperHex, 40),
		"mixed-case-base62":    "value " + benignRand("mixed-case-base62", benignBase62, 40),
		"assign-opaque-value":  "x=" + benignRand("assign-opaque-value", benignBase62, 40),
		"equals-then-payload":  strings.Repeat("=", 20) + benignRand("equals-then-payload", benignBase62, 20),
		"subtest-token-tail":   "TestSupersedeMarker/" + benignRand("subtest-token-tail", benignBase62, 40),
		"plus-then-payload":    strings.Repeat("+", 20) + benignRand("plus-then-payload", benignBase62, 40),
		"credential-key-sha":   "SIGNING_SEC" + "RET_SHA=" + benignRand("credential-key-sha", benignLowerHex, 64),
		"credential-key-ident": "DB_PA" + "SS=TestBenignEntries" + "AreBoundedByRefusals",
		"upper-hex-unnamed":    "value " + benignRand("upper-hex-unnamed", benignUpperHex, 40),
	}
}

// literalArmFixtures are the corpus fixtures the literal credential arms decide on their
// own. None carries a run of the class that an entry admits, so no entry can honestly name
// it; TestBenignRegistryComplete proves that for each (a negative one is clean with an
// empty registry, a positive one refuses under a literal arm).
var literalArmFixtures = []string{
	"neg-bodycheck-own-marker-literals.txt",
	"neg-k8s-secret-template-in-fence.txt",
	"neg-review-cites-sops-material.txt",
	"neg-url-userinfo-placeholders.txt",
	"pos-github-and-jwt-tokens.txt",
	"pos-k8s-secret-literal-in-fence.txt",
	"pos-pem-private-key.txt",
	"pos-review-quotes-sops-envelope.txt",
	"pos-review-quotes-sops-footer.txt",
	"pos-secret-hidden-in-a-sops-document.txt",
	"pos-sops-envelope-without-document.txt",
	"pos-sops-imitation.txt",
	"pos-unknown-armor-label.txt",
	"pos-url-userinfo-credential.txt",
}

// benignNamedFixtures is every corpus fixture the registry or literalArmFixtures names:
// the derived set the corpus coverage test requires to exist.
func benignNamedFixtures() map[string]string {
	out := map[string]string{}
	for _, e := range benignShapes {
		for _, n := range append(append([]string(nil), e.Admitted...), e.Bounds...) {
			if strings.HasSuffix(n, ".txt") {
				out[n] = "registry entry " + e.ID
			}
		}
	}
	for _, n := range literalArmFixtures {
		out[n] = "a literal credential arm"
	}
	return out
}

// benignInputs resolves names (fixture files or `case:` keys) to scan inputs.
func benignInputs(t *testing.T, names []string, cases map[string]string) map[string]string {
	t.Helper()
	payloads := map[string]string{}
	for _, a := range loadCorpus(t) {
		payloads[a.file] = a.payload
	}
	out := map[string]string{}
	for _, n := range names {
		if k, ok := strings.CutPrefix(n, "case:"); ok {
			c, found := cases[k]
			if !found {
				t.Fatalf("%s names no in-test case", n)
			}
			out[n] = c
			continue
		}
		p, found := payloads[n]
		if !found {
			t.Fatalf("%s names no corpus fixture", n)
		}
		out[n] = p
	}
	return out
}

// forcedOpen returns e with its admission forced to true for every run.
func forcedOpen(e benignShape) benignShape {
	e.run, e.line, e.spans = nil, nil, nil
	switch e.Context {
	case ctxRun:
		e.run = func(string) bool { return true }
	case ctxLine:
		e.line = func(string, int, int) bool { return true }
	case ctxArtifact:
		e.spans = func(raw string) []span { return []span{{0, len(raw)}} }
	}
	return e
}

// scanRule is the rule a scan with reg refuses content under, or "" when it is clean.
func scanRule(content string, reg []benignShape) string {
	err := scanSurface("benign test", []byte(content), false, reg)
	if err == nil {
		return ""
	}
	var f *ScanFinding
	if errors.As(err, &f) && f != nil {
		return f.Rule
	}
	return "unexplained"
}

func TestBenignRegistryComplete(t *testing.T) {
	admit, bound := benignAdmitCases(), benignBoundCases()
	seen := map[string]bool{}
	for i, e := range benignShapes {
		if e.ID == "" || seen[e.ID] {
			t.Errorf("entry %d: ID %q is empty or not unique", i, e.ID)
		}
		seen[e.ID] = true
		funcs := 0
		for _, set := range []bool{e.run != nil, e.line != nil, e.spans != nil} {
			if set {
				funcs++
			}
		}
		want := map[string]bool{ctxRun: e.run != nil, ctxLine: e.line != nil, ctxArtifact: e.spans != nil}
		if set, known := want[e.Context]; !known || !set || funcs != 1 {
			t.Errorf("%s: Context %q needs exactly its one admission function (have %d)", e.ID, e.Context, funcs)
		}
		if e.Doc == "" || e.Issue == "" || strings.ContainsAny(e.Doc+e.Issue, "|<>\n") {
			t.Errorf("%s: Doc and Issue must be one non-empty line free of | < >", e.ID)
		}
		if len(e.Admitted) == 0 || len(e.Bounds) == 0 {
			t.Errorf("%s: needs at least one Admitted and one Bounds", e.ID)
			continue
		}
		for _, n := range e.Admitted {
			if strings.HasPrefix(n, "pos-") {
				t.Errorf("%s: Admitted names positive fixture %s", e.ID, n)
			}
		}
		for _, n := range e.Bounds {
			if strings.HasPrefix(n, "neg-") {
				t.Errorf("%s: Bounds names negative fixture %s", e.ID, n)
			}
		}

		// Admitted examples are clean, and the entry is what admits one of them.
		without := append(append([]benignShape(nil), benignShapes[:i]...), benignShapes[i+1:]...)
		needed := false
		for n, in := range benignInputs(t, e.Admitted, admit) {
			if r := scanRule(in, benignShapes); r != "" {
				t.Errorf("%s: Admitted %s refuses (%s) under the production registry", e.ID, n, r)
			}
			if scanRule(in, without) != "" {
				needed = true
			}
		}
		if !needed {
			t.Errorf("%s: no Admitted example refuses without the entry, so none shows what it admits", e.ID)
		}
		// Bounds refuse in the high-entropy loop.
		for n, in := range benignInputs(t, e.Bounds, bound) {
			if r := scanRule(in, benignShapes); r != "high-entropy-run" {
				t.Errorf("%s: Bounds %s scans as %q; want a high-entropy-run refusal", e.ID, n, r)
			}
		}
	}

	// Every negative fixture is named by an entry or is literal-arm-only, and the
	// literal-arm list is honest.
	named := benignNamedFixtures()
	empty := []benignShape{}
	for _, a := range loadCorpus(t) {
		if a.expect == "clean" {
			if _, ok := named[a.file]; !ok {
				t.Errorf("negative fixture %s is named by no registry entry", a.file)
			}
		}
	}
	lit := benignInputs(t, literalArmFixtures, nil)
	for _, n := range literalArmFixtures {
		r := scanRule(lit[n], empty)
		switch {
		case strings.HasPrefix(n, "neg-") && r != "":
			t.Errorf("%s is listed literal-arm-only but needs an entry (empty registry: %s)", n, r)
		case strings.HasPrefix(n, "pos-") && (r == "" || r == "high-entropy-run"):
			t.Errorf("%s is listed literal-arm-only but scans as %q", n, r)
		}
	}
}

func TestBenignEntriesBounded(t *testing.T) {
	bound := benignBoundCases()
	for i, e := range benignShapes {
		reg := append([]benignShape(nil), benignShapes...)
		reg[i] = forcedOpen(e)
		flipped := []string{}
		for n, in := range benignInputs(t, e.Bounds, bound) {
			if scanRule(in, benignShapes) != "" && scanRule(in, reg) == "" {
				flipped = append(flipped, n)
			}
		}
		if len(flipped) == 0 {
			t.Errorf("%s: forcing the entry open flips none of its Bounds %v; nothing bounds it", e.ID, e.Bounds)
			continue
		}
		sort.Strings(flipped)
		t.Logf("%s: forced open admits %v", e.ID, flipped)
	}
}

func TestBenignKeepsLiteralArms(t *testing.T) {
	open := make([]benignShape, len(benignShapes))
	for i, e := range benignShapes {
		open[i] = forcedOpen(e)
	}
	checked := 0
	for _, a := range loadCorpus(t) {
		if a.expect != "refused" {
			continue
		}
		want := scanRule(a.payload, benignShapes)
		if want == "" || want == "high-entropy-run" {
			continue
		}
		checked++
		if got := scanRule(a.payload, open); got != want {
			t.Errorf("%s: literal arm %s became %q with every entry forced open", a.file, want, got)
		}
	}
	if checked < 5 {
		t.Fatalf("only %d positive fixtures refuse under a literal arm; the check proves little", checked)
	}
}

func TestBenignRefusalText(t *testing.T) {
	in := "value " + benignRand("refusal-text", benignBase62, 48)
	err := ScanSurfaceSecrets("benign test", []byte(in))
	if err == nil {
		t.Fatal("a random 48-char run scanned clean; the refusal text cannot be checked")
	}
	msg := err.Error()
	for _, e := range benignShapes {
		if !strings.Contains(msg, e.Doc) {
			t.Errorf("refusal message lacks %s's Doc %q", e.ID, e.Doc)
		}
	}
	if !strings.Contains(msg, "--explain") {
		t.Error("refusal message does not name --explain")
	}
	if !strings.Contains(msg, "high-entropy run (possible secret)") {
		t.Error("refusal message lost its rule wording")
	}
}

func TestBenignDocsParity(t *testing.T) {
	const begin, end = "<!-- benign-shapes:begin -->", "<!-- benign-shapes:end -->"
	path := filepath.Join("..", "..", "README.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(raw)
	if strings.Count(s, begin) != 1 || strings.Count(s, end) != 1 {
		t.Fatalf("%s must carry exactly one %s and one %s", path, begin, end)
	}
	_, after, _ := strings.Cut(s, begin)
	got, _, _ := strings.Cut(after, end)
	if want := renderBenignTable(benignShapes); got != want {
		t.Errorf("%s's benign-shapes table differs from the registry.\nREADME:   %s\nregistry: %s",
			path, got, want)
	}
}
