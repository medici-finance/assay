package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Lockstep gate — the schema and brieffile.go must not drift.
//
// TestBriefV1SchemaCoverage derives the required-key set, the closed value sets,
// and the risk block DIRECTLY from brieffile.go's own package-level tables and
// asserts the committed schema encodes exactly those. Because both live in the
// same package, the derivation is a compile-time reference to the one source of
// truth: add a field to requiredBriefKeys, or a value to validEffort, without
// updating schemas/brief-v1.json and this test goes red. That is the guarantee the
// contract is machine-readable "with no drift" — a brief that passes the reference
// validator's frontmatter-shape rules passes the schema, and vice versa.
// ---------------------------------------------------------------------------

func TestBriefV1SchemaCoverage(t *testing.T) {
	schema := mustParseEmbeddedSchema(t)
	if problems := schemaCoverageProblems(schema.raw); len(problems) > 0 {
		t.Fatalf("committed schema drifted from brieffile.go's validated contract:\n  - %s",
			strings.Join(problems, "\n  - "))
	}
}

// TestBriefV1SchemaCoverage_RejectsDrift is the negative case: it doctors a fresh
// copy of the schema the way real drift would appear and proves the coverage gate
// GOES RED. Two mutations, matching the two drift shapes the brief's pre-mortem
// names: a closed value set that lost a member, and a required key the validator
// enforces that the schema dropped (the "a field was added to brieffile.go's
// validated set without touching the schema" scenario). Without this, a coverage
// test that never fails is indistinguishable from one that cannot.
func TestBriefV1SchemaCoverage_RejectsDrift(t *testing.T) {
	t.Run("enum-member-dropped", func(t *testing.T) {
		schema := mustParseEmbeddedSchema(t)
		effort := schema.raw["properties"].(map[string]any)["effort"].(map[string]any)
		full := effort["enum"].([]any)
		effort["enum"] = full[:len(full)-1] // drop the last allowed effort value
		if problems := schemaCoverageProblems(schema.raw); len(problems) == 0 {
			t.Fatal("coverage gate did not fire when the effort enum lost a member — the gate is inert")
		}
	})
	t.Run("required-key-dropped", func(t *testing.T) {
		schema := mustParseEmbeddedSchema(t)
		req := schema.raw["required"].([]any)
		schema.raw["required"] = req[:len(req)-1] // schema now requires one fewer key than brieffile.go
		if problems := schemaCoverageProblems(schema.raw); len(problems) == 0 {
			t.Fatal("coverage gate did not fire when a required key the validator enforces was dropped from the schema — the gate is inert")
		}
	})
}

// TestEmbeddedSchemaMatchesCommitted pins the embedded copy byte-identical to the
// canonical repo-root artifact. The embed directive cannot cross the module
// boundary (go.mod is rooted at statusgen/), so schemas/brief-v1.json is mirrored
// under statusgen/schemas/; this test is the parity gate that stops the two
// copies drifting.
func TestEmbeddedSchemaMatchesCommitted(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "schemas", "brief-v1.json"))
	if err != nil {
		t.Fatalf("reading canonical repo-root schema: %v", err)
	}
	if !bytes.Equal(canonical, embeddedBriefV1Schema()) {
		t.Fatal("statusgen/schemas/brief-v1.json is not byte-identical to the canonical schemas/brief-v1.json — re-copy the canonical artifact so `conform --emit-schema` reproduces it exactly")
	}
}

// TestValidatorCoversSchemaKeywords asserts every keyword the committed schema
// actually uses is one the minimal validator implements. A schema that grew a
// keyword the validator silently ignores would validate briefs against a weaker
// contract than the committed artifact advertises — a false-green. This walks the
// whole schema tree and fails on the first unimplemented keyword.
func TestValidatorCoversSchemaKeywords(t *testing.T) {
	schema := mustParseEmbeddedSchema(t)
	var unknown []string
	var walk func(node any)
	walk = func(node any) {
		m, ok := node.(map[string]any)
		if !ok {
			return
		}
		for k, v := range m {
			// Under `properties` and `risk.properties`, the keys are field NAMES,
			// not schema keywords — recurse into their subschemas but do not treat
			// the field name as a keyword.
			if k == "properties" {
				if props, ok := v.(map[string]any); ok {
					for _, sub := range props {
						walk(sub)
					}
				}
				continue
			}
			if !schemaKeywords[k] {
				unknown = append(unknown, k)
			}
			walk(v)
		}
	}
	walk(schema.raw)
	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Fatalf("committed schema uses keyword(s) the validator does not implement: %v — implement them in conform.go or the contract is under-enforced", unique(unknown))
	}
}

// ---------------------------------------------------------------------------
// Behavioural agreement — over the real corpus and crafted cases.
// ---------------------------------------------------------------------------

// TestConformAcceptsRepoCorpus runs the schema validator over every real brief in
// the repository and asserts none is rejected. The corpus is the de-facto
// contract the reference validator already accepts in CI; a schema the corpus
// fails is a wrong schema, not a wrong corpus (the descriptive-first rule). This
// is also the two-sided agreement: each file here is one parseBriefFile validates
// green, so "passes brieffile.go ⇒ passes conform" is checked on live data.
func TestConformAcceptsRepoCorpus(t *testing.T) {
	schema := mustParseEmbeddedSchema(t)
	streamsDir := filepath.Join("..", "docs", "streams")
	if _, err := os.Stat(streamsDir); err != nil {
		t.Skipf("no repo corpus at %s: %v", streamsDir, err)
	}
	var checked, briefV1 int
	err := filepath.WalkDir(streamsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !briefNameRe.MatchString(d.Name()) {
			return err
		}
		checked++
		// Only brief-v1 files are in scope; conformFile classifies the rest exempt.
		state, msg := conformFile(path, schema)
		switch state {
		case conformStateClean:
			briefV1++
			// Two-sided: the same file must be one the reference validator parses.
			if _, ok, perr := parseBriefFile(path); perr != nil || !ok {
				t.Errorf("%s: conform accepts but parseBriefFile does not (ok=%v err=%v) — the two surfaces disagree", path, ok, perr)
			}
		case conformStateFailed:
			t.Errorf("conform rejected a real corpus brief (schema is not descriptive of the validated corpus): %s", msg)
		case conformStateVersion, conformStateCouldNot:
			t.Errorf("conform could not check a real corpus brief: %s", msg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking corpus: %v", err)
	}
	if briefV1 == 0 {
		t.Fatalf("scanned %d brief files but none were brief-v1 — the corpus test proved nothing", checked)
	}
	t.Logf("corpus: %d brief-v1 files checked-clean of %d brief files scanned", briefV1, checked)
}

// TestConformRejectsPerRuleViolations exercises each schema rule with a
// single-rule-violating frontmatter and asserts conform rejects it, naming the
// offending field. Each case is a machine-decidable frontmatter-shape rule the
// reference validator also enforces; this is the per-rule fail-first coverage the
// brief requires (a conforming brief passes, a brief violating each rule fails).
func TestConformRejectsPerRuleViolations(t *testing.T) {
	schema := mustParseEmbeddedSchema(t)

	// A minimal conforming brief-v1 frontmatter, as the base every case mutates.
	base := `brief: t/01
title: base
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-30 by fixture
sources: ["fixture"]
`
	// Sanity: the base must itself pass, or every negative below is vacuous.
	if state, msg := conformFrontmatter(t, schema, base); state != conformStateClean {
		t.Fatalf("base frontmatter is not clean (state=%d msg=%q) — fix the base before trusting the negatives", state, msg)
	}

	cases := []struct {
		name     string
		fm       string
		wantWord string // a substring the rejection message must name
	}{
		{"missing-required-risk", strings.Replace(base, "risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n", "", 1), "risk"},
		{"bad-effort", strings.Replace(base, "effort: S", "effort: XL", 1), "effort"},
		{"bad-gate", strings.Replace(base, "gate: model", "gate: maybe", 1), "gate"},
		{"bad-value", base + "value: huge\n", "value"},
		{"bad-domain", base + "domain: swampy\n", "domain"},
		{"bad-exec-tier", base + "exec-tier: turbo\n", "exec-tier"},
		{"bad-blocked-by", base + "blocked-by: coffee\n", "blocked-by"},
		{"bad-measures", base + "measures: nonsense-queue\n", "measures"},
		{"bad-homed-in", base + "homed-in: not-a-repo\n", "homed-in"},
		{"risk-unknown-key", strings.Replace(base, "sensitive-data: no}", "sensitive-data: no, made-up: no}", 1), "made-up"},
		{"risk-non-boolean", strings.Replace(base, "regulatory: no", "regulatory: maybe", 1), "regulatory"},
		{"wave-not-integer", strings.Replace(base, "wave: 0", "wave: soon", 1), "wave"},
		{"title-not-string", strings.Replace(base, "title: base", "title: [a, b]", 1), "title"},
		{"empty-sources", strings.Replace(base, `sources: ["fixture"]`, "sources: []", 1), "sources"},
		{"consumers-bad-item", base + "consumers: [1, 2]\n", "consumers"},
		{"parallel-streams-unknown-key", base + "parallel-streams:\n  - {name: a, files: [\"x/**\"], bogus: 1}\n", "bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, msg := conformFrontmatter(t, schema, tc.fm)
			if state != conformStateFailed {
				t.Fatalf("expected checked-failed, got state=%d msg=%q", state, msg)
			}
			if !strings.Contains(msg, tc.wantWord) {
				t.Fatalf("rejection message should name %q; got %q", tc.wantWord, msg)
			}
		})
	}
}

// TestRunConformEndToEnd drives the subcommand through runConform: a clean tree
// exits 0, a bad-effort tree exits 1, a brief-v2 tree exits 2 (version mismatch,
// fail-closed), and --emit-schema prints the schema $id.
func TestRunConformEndToEnd(t *testing.T) {
	write := func(dir, rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	clean := `---
brief: t/01
title: clean
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-30 by fixture
sources: ["fixture"]
---
# body
`
	t.Run("clean-exit-0", func(t *testing.T) {
		root := t.TempDir()
		write(root, "docs/streams/t/brief-01-clean.md", clean)
		var out, errb bytes.Buffer
		if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitClean {
			t.Fatalf("want exit 0, got %d; out=%s", code, out.String())
		}
		if !strings.Contains(out.String(), "1 checked-clean") {
			t.Fatalf("summary should report 1 checked-clean; got %s", out.String())
		}
	})
	t.Run("bad-effort-exit-1", func(t *testing.T) {
		root := t.TempDir()
		write(root, "docs/streams/t/brief-01-bad.md", strings.Replace(clean, "effort: M", "effort: XL", 1))
		var out, errb bytes.Buffer
		if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitFailed {
			t.Fatalf("want exit 1, got %d; out=%s", code, out.String())
		}
	})
	t.Run("version-mismatch-exit-2", func(t *testing.T) {
		root := t.TempDir()
		write(root, "docs/streams/t/brief-01-v2.md", strings.Replace(clean, "schema: brief-v1", "schema: brief-v2", 1))
		var out, errb bytes.Buffer
		if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitCouldNot {
			t.Fatalf("want exit 2, got %d; out=%s", code, out.String())
		}
		if !strings.Contains(out.String(), "schema-version mismatch") {
			t.Fatalf("expected a version-mismatch report; got %s", out.String())
		}
	})
	t.Run("emit-schema", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := runConform([]string{"--emit-schema"}, &out, &errb); code != conformExitClean {
			t.Fatalf("want exit 0, got %d", code)
		}
		if !bytes.Equal(out.Bytes(), embeddedBriefV1Schema()) {
			t.Fatal("--emit-schema output is not the embedded schema byte-for-byte")
		}
		if !strings.Contains(out.String(), `"$id"`) {
			t.Fatal("emitted schema carries no $id")
		}
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustParseEmbeddedSchema(t *testing.T) *schemaNode {
	t.Helper()
	schema, err := parseSchema(embeddedBriefV1Schema())
	if err != nil {
		t.Fatalf("embedded schema does not parse: %v", err)
	}
	return schema
}

// conformFrontmatter writes a frontmatter body into a temp brief file and returns
// conformFile's classification of it.
func conformFrontmatter(t *testing.T, schema *schemaNode, fm string) (conformState, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "brief-01-x.md")
	if err := os.WriteFile(p, []byte("---\n"+fm+"---\n# body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return conformFile(p, schema)
}

// schemaCoverageProblems returns the ways the schema fails to encode exactly the
// contract brieffile.go's tables define. Empty means lockstep.
func schemaCoverageProblems(raw map[string]any) []string {
	var probs []string
	add := func(f string, a ...any) { probs = append(probs, fmt.Sprintf(f, a...)) }

	props, _ := raw["properties"].(map[string]any)
	if props == nil {
		return []string{"schema has no properties object"}
	}

	// Required keys: exactly requiredBriefKeys plus the opt-in `schema` marker.
	wantRequired := append([]string{}, requiredBriefKeys...)
	wantRequired = append(wantRequired, "schema")
	if got := anyStringList(raw["required"]); !sameSet(got, wantRequired) {
		add("required set %v != brieffile.go required %v", cfSorted(got), cfSorted(wantRequired))
	}

	// schema marker is the current brief-v1 constant.
	if sc, _ := props["schema"].(map[string]any); sc == nil || sc["const"] != briefSchemaCurrent {
		add("properties.schema.const must be %q", briefSchemaCurrent)
	}

	// Top-level unknown-key tolerance must match brieffile.go (which does not
	// reject unknown top-level keys).
	if ap, ok := raw["additionalProperties"].(bool); !ok || ap != true {
		add("top-level additionalProperties must be true (brieffile.go tolerates unknown top-level keys)")
	}

	// Closed value sets, each derived from brieffile.go's own map.
	checkEnum := func(prop string, want map[string]bool) {
		p, _ := props[prop].(map[string]any)
		if p == nil {
			add("properties.%s missing", prop)
			return
		}
		got := anyStringList(p["enum"])
		if !sameSet(got, keysOf(want)) {
			add("properties.%s.enum %v != validator set %v", prop, cfSorted(got), cfSorted(keysOf(want)))
		}
	}
	checkEnum("effort", validEffort)
	checkEnum("gate", validGate)
	checkEnum("value", validValue)
	checkEnum("domain", validDomain)
	checkEnum("exec-tier", validExecTier)
	checkEnum("blocked-by", validBlockedBy)
	checkEnum("measures", validMeasuresQueue)

	// risk block: exactly the four canonical keys, no others, each yes|no.
	risk, _ := props["risk"].(map[string]any)
	if risk == nil {
		add("properties.risk missing")
	} else {
		if got := anyStringList(risk["required"]); !sameSet(got, canonicalRiskKeys) {
			add("risk.required %v != canonicalRiskKeys %v", cfSorted(got), cfSorted(canonicalRiskKeys))
		}
		if ap, ok := risk["additionalProperties"].(bool); !ok || ap != false {
			add("risk.additionalProperties must be false (brieffile.go rejects unknown risk keys)")
		}
		rprops, _ := risk["properties"].(map[string]any)
		if !sameSet(mapKeys(rprops), canonicalRiskKeys) {
			add("risk.properties keys %v != canonicalRiskKeys %v", cfSorted(mapKeys(rprops)), cfSorted(canonicalRiskKeys))
		}
		for _, k := range canonicalRiskKeys {
			if rp, _ := rprops[k].(map[string]any); rp == nil || !sameSet(anyStringList(rp["enum"]), []string{"yes", "no"}) {
				add("risk.properties.%s.enum must be [yes, no]", k)
			}
		}
	}
	return probs
}

// --- tiny set/slice helpers (kept local to avoid touching shared code) ---

func anyStringList(v any) []string {
	out := []string{}
	if list, ok := v.([]any); ok {
		for _, e := range list {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func cfSorted(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return out
}

func sameSet(a, b []string) bool {
	return reflect.DeepEqual(cfSorted(a), cfSorted(b))
}

func unique(s []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range s {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}
