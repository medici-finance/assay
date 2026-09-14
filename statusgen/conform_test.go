package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
	if problems := schemaCoverageProblems(schema.raw, briefSchemaCurrent); len(problems) > 0 {
		t.Fatalf("committed schema drifted from brieffile.go's validated contract:\n  - %s",
			strings.Join(problems, "\n  - "))
	}
}

// TestBriefV2SchemaCoverage holds the brief-v2 contract in the same lockstep with
// brieffile.go/briefv2.go that TestBriefV1SchemaCoverage holds brief-v1 in: the
// shared brief-v1 surface (required keys, closed value sets, the risk block) is
// derived from the same tables, and the v2-specific surface — the marker const,
// the hierarchical `brief:` pattern, the uuid v4 `id:` pattern, the version floor,
// and the edge-type taxonomy on `gates`/`feathers` — is derived from briefv2.go's
// own regexps and table. Add an edge type to validEdgeTypes, or change uuidV4Re,
// without updating schemas/brief-v2.json and this goes red.
func TestBriefV2SchemaCoverage(t *testing.T) {
	schema := mustParseEmbeddedSchemaV2(t)
	problems := schemaCoverageProblems(schema.raw, briefSchemaV2)
	problems = append(problems, schemaV2CoverageProblems(schema.raw)...)
	if len(problems) > 0 {
		t.Fatalf("committed brief-v2 schema drifted from the validated contract:\n  - %s",
			strings.Join(problems, "\n  - "))
	}
}

// TestBriefV2SchemaCoverage_RejectsDrift is the v2 negative control: it doctors a
// fresh copy of the v2 schema the way real drift would appear and proves the
// coverage gate GOES RED, so a passing TestBriefV2SchemaCoverage is evidence and
// not a tautology.
func TestBriefV2SchemaCoverage_RejectsDrift(t *testing.T) {
	t.Run("edge-type-dropped", func(t *testing.T) {
		schema := mustParseEmbeddedSchemaV2(t)
		gates := schema.raw["properties"].(map[string]any)["gates"].(map[string]any)
		items := gates["items"].(map[string]any)
		typ := items["properties"].(map[string]any)["type"].(map[string]any)
		full := typ["enum"].([]any)
		typ["enum"] = full[:len(full)-1] // drop an edge type briefv2.go still accepts
		if problems := schemaV2CoverageProblems(schema.raw); len(problems) == 0 {
			t.Fatal("coverage gate did not fire when the gates edge-type enum lost a member — the gate is inert")
		}
	})
	t.Run("marker-const-wrong", func(t *testing.T) {
		schema := mustParseEmbeddedSchemaV2(t)
		schema.raw["properties"].(map[string]any)["schema"].(map[string]any)["const"] = briefSchemaCurrent
		if problems := schemaCoverageProblems(schema.raw, briefSchemaV2); len(problems) == 0 {
			t.Fatal("coverage gate did not fire when the v2 schema declared the brief-v1 marker — the gate is inert")
		}
	})
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
		if problems := schemaCoverageProblems(schema.raw, briefSchemaCurrent); len(problems) == 0 {
			t.Fatal("coverage gate did not fire when the effort enum lost a member — the gate is inert")
		}
	})
	t.Run("required-key-dropped", func(t *testing.T) {
		schema := mustParseEmbeddedSchema(t)
		req := schema.raw["required"].([]any)
		schema.raw["required"] = req[:len(req)-1] // schema now requires one fewer key than brieffile.go
		if problems := schemaCoverageProblems(schema.raw, briefSchemaCurrent); len(problems) == 0 {
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
	for _, marker := range embeddedBriefSchemaMarkers() {
		name := embeddedBriefSchemaNames[marker]
		base := filepath.Base(name)
		canonical, err := os.ReadFile(filepath.Join("..", "schemas", base))
		if err != nil {
			t.Fatalf("reading canonical repo-root schema %s: %v", base, err)
		}
		if !bytes.Equal(canonical, embeddedSchemaBytes(name)) {
			t.Errorf("statusgen/%s is not byte-identical to the canonical schemas/%s — re-copy the canonical artifact so `conform --emit-schema --schema %s` reproduces it exactly", name, base, marker)
		}
	}
}

// TestEveryRecognizedBriefSchemaHasAContract is the gap this fix closed, pinned
// as a check: brieffile.go RECOGNIZES a set of brief-schema markers
// (recognizedBriefSchemas — what `--lint` will parse), and conform must embed a
// contract for every one of them. When the two sets diverge, a tree `--lint`
// reads happily reports could-not-check for every brief and exits 2, which is
// exactly what a migrated tree did before brief-v2 was embedded. Add a brief-v3
// to recognizedBriefSchemas without authoring schemas/brief-v3.json and this goes
// red in this repo's own CI, not in an adopter's flag-day PR.
func TestEveryRecognizedBriefSchemaHasAContract(t *testing.T) {
	for marker := range recognizedBriefSchemas {
		if !strings.HasPrefix(marker, briefSchemaFamilyPrefix) {
			continue // a non-brief document kind is not conform's contract surface
		}
		if _, ok := embeddedBriefSchemaNames[marker]; !ok {
			t.Errorf("brieffile.go recognizes %q but conform embeds no contract for it — every tree on that schema would report could-not-check and exit 2; author schemas/%s.json and register it in embeddedBriefSchemaNames", marker, marker)
		}
	}
}

// TestValidatorCoversSchemaKeywords asserts every keyword the committed schema
// actually uses is one the minimal validator implements. A schema that grew a
// keyword the validator silently ignores would validate briefs against a weaker
// contract than the committed artifact advertises — a false-green. This walks the
// whole schema tree and fails on the first unimplemented keyword.
func TestValidatorCoversSchemaKeywords(t *testing.T) {
	for _, marker := range embeddedBriefSchemaMarkers() {
		t.Run(marker, func(t *testing.T) {
			schema, err := parseSchema(embeddedSchemaBytes(embeddedBriefSchemaNames[marker]))
			if err != nil {
				t.Fatalf("embedded %s schema does not parse: %v", marker, err)
			}
			assertValidatorCoversKeywords(t, schema)
		})
	}
}

func assertValidatorCoversKeywords(t *testing.T, schema *schemaNode) {
	t.Helper()
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

// TestValidatorMinimumIsEnforced is the negative control for the one keyword this
// change added to the minimal validator. A keyword listed in schemaKeywords but
// not actually implemented would pass TestValidatorCoversSchemaKeywords while
// enforcing nothing — the exact false-green that test exists to prevent, one level
// down. `version: 0` is the real case: briefv2.go PROBLEMs it, so conform must too.
func TestValidatorMinimumIsEnforced(t *testing.T) {
	node := map[string]any{"type": "integer", "minimum": 1}
	if v := validateNode(node, 0, "version"); len(v) == 0 {
		t.Fatal("minimum is in schemaKeywords but validateNode did not reject a value below it — the keyword is declared and inert")
	}
	if v := validateNode(node, 1, "version"); len(v) != 0 {
		t.Fatalf("minimum rejected a value at the bound: %v", v)
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
	schemas := mustParseEmbeddedSchemas(t)
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
		state, msg := conformFile(path, schemas)
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
	schemas := mustParseEmbeddedSchemas(t)

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
	if state, msg := conformFrontmatter(t, schemas, base); state != conformStateClean {
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
			state, msg := conformFrontmatter(t, schemas, tc.fm)
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
		// brief-v3 is the marker no embedded contract describes. (Before brief-v2
		// was embedded this case was spelled brief-v2 — the three-state behaviour
		// is unchanged, it is the boundary that moved.)
		root := t.TempDir()
		write(root, "docs/streams/t/brief-01-v3.md", strings.Replace(clean, "schema: brief-v1", "schema: brief-v3", 1))
		var out, errb bytes.Buffer
		if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitCouldNot {
			t.Fatalf("want exit 2, got %d; out=%s", code, out.String())
		}
		if !strings.Contains(out.String(), "schema-version mismatch") {
			t.Fatalf("expected a version-mismatch report; got %s", out.String())
		}
		if !strings.Contains(out.String(), "brief-v1, brief-v2") {
			t.Fatalf("the mismatch message must name the contracts this binary DOES embed; got %s", out.String())
		}
		if !strings.Contains(out.String(), "1 schema-version mismatch") {
			t.Fatalf("summary should count the mismatch; got %s", out.String())
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
	t.Run("emit-schema-brief-v2", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := runConform([]string{"--emit-schema", "--schema", briefSchemaV2}, &out, &errb); code != conformExitClean {
			t.Fatalf("want exit 0, got %d; stderr=%s", code, errb.String())
		}
		if !bytes.Equal(out.Bytes(), embeddedBriefV2Schema()) {
			t.Fatal("--emit-schema --schema brief-v2 output is not the embedded brief-v2 schema byte-for-byte")
		}
	})
	t.Run("emit-schema-unknown-version-refuses", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := runConform([]string{"--emit-schema", "--schema", "brief-v3"}, &out, &errb); code != conformExitUsageError {
			t.Fatalf("want exit 2 (usage), got %d", code)
		}
		if out.Len() != 0 {
			t.Fatalf("a refused --emit-schema must print no schema; got %q", out.String())
		}
	})
}

// ---------------------------------------------------------------------------
// brief-v2 — the migrated tree the released binary could not check.
// ---------------------------------------------------------------------------

// TestConformValidatesMigratedTree is the regression the fix exists for, driven
// end to end through the REAL migration rather than a hand-written v2 fixture:
// migrate a brief-v1 tree with `statusgen migrate brief-v1-to-v2`, then run
// `conform` over the result. What migrate emits is exactly what conform must
// accept — a hand-written fixture could drift from the migration's actual output
// and this test would still pass.
//
// Fail-first: against the pre-fix binary (brief-v1 the only embedded contract)
// this reds with exit 2 and "schema marker \"brief-v2\" is newer than this
// binary's brief-v1 contract" for the migrated brief — the adopter's flag-day PR
// going red, reproduced in a unit test.
func TestConformValidatesMigratedTree(t *testing.T) {
	root := migrateFixtureTree(t, true)
	var mout, merr bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &mout, &merr); code != migrateExitOK {
		t.Fatalf("migrate exit=%d, want 0; stderr=%s", code, merr.String())
	}
	migrated, err := os.ReadFile(filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migrated), "schema: brief-v2") {
		t.Fatalf("fixture precondition failed — the tree is not on brief-v2:\n%s", migrated)
	}

	var out, errb bytes.Buffer
	code := runConform([]string{"--root", root}, &out, &errb)
	if code != conformExitClean {
		t.Fatalf("conform over a MIGRATED tree: exit=%d, want 0 (this is the bug in the issue — a migrated tree could not be checked)\nstdout:\n%s\nstderr:\n%s",
			code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "1 checked-clean") {
		t.Fatalf("summary should report the migrated brief as checked-clean; got %s", out.String())
	}
	if strings.Contains(out.String(), "could-not-check:") {
		t.Fatalf("a migrated tree must produce no could-not-check line; got %s", out.String())
	}
}

// TestConformMixedV1V2Tree pins the per-FILE dispatch: a tree mid-migration holds
// both schema versions, and each file is validated against the contract it
// declares — not against one contract chosen for the whole run.
func TestConformMixedV1V2Tree(t *testing.T) {
	root := migrateFixtureTree(t, true)
	// A second stream left on brief-v1, added AFTER the fixture so the migration
	// below rewrites only the first one.
	v1 := "---\nbrief: legacy/01\ntitle: still v1\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\nauthored: 2026-09-09 by fixture\nsources: [\"fixture\"]\n---\n# body\n"
	dir := filepath.Join(root, "docs", "streams", "legacy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-legacy.md"), []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	// Migrate ONLY the svc brief by hand-rewriting it the way the migration does,
	// so the tree really is mixed (running the migration would move both).
	svc := filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md")
	raw, err := os.ReadFile(svc)
	if err != nil {
		t.Fatal(err)
	}
	v2 := strings.Replace(string(raw), "schema: brief-v1", "schema: brief-v2", 1)
	v2 = strings.Replace(v2, "brief: svc/01", "brief: example:app:svc:01", 1)
	v2 = strings.Replace(v2, "sources: [\"note\"]", "sources: [\"note\"]\nversion: 2\nid: 3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d", 1)
	if err := os.WriteFile(svc, []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitClean {
		t.Fatalf("mixed v1/v2 tree: exit=%d, want 0\nstdout:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "2 checked-clean") {
		t.Fatalf("both the v1 and the v2 brief should be checked-clean; got %s", out.String())
	}
}

// TestConformRejectsBriefV2FieldViolations proves the brief-v2 contract actually
// CONSTRAINS rather than waving v2 files through. Each case violates exactly one
// v2 rule the reference validator also enforces; a schema that merely recognised
// the marker would pass every one of them.
func TestConformRejectsBriefV2FieldViolations(t *testing.T) {
	schemas := mustParseEmbeddedSchemas(t)

	base := `brief: example:app:svc:01
title: base
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d
authored: 2026-09-09 by fixture
sources: ["fixture"]
`
	if state, msg := conformFrontmatter(t, schemas, base); state != conformStateClean {
		t.Fatalf("base brief-v2 frontmatter is not clean (state=%d msg=%q) — fix the base before trusting the negatives", state, msg)
	}

	cases := []struct {
		name     string
		fm       string
		wantWord string
	}{
		{"v1-brief-id-form", strings.Replace(base, "brief: example:app:svc:01", "brief: svc/01", 1), "brief"},
		{"brief-id-too-few-segments", strings.Replace(base, "brief: example:app:svc:01", "brief: app:svc:01", 1), "brief"},
		{"id-not-a-uuid", strings.Replace(base, "id: 3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d", "id: not-a-uuid", 1), "id"},
		{"id-uuid-wrong-version-nibble", strings.Replace(base, "id: 3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d", "id: 3f1b2c4d-5e6f-1a7b-8c9d-0e1f2a3b4c5d", 1), "id"},
		{"version-below-one", strings.Replace(base, "version: 1", "version: 0", 1), "version"},
		{"version-not-integer", strings.Replace(base, "version: 1", "version: two", 1), "version"},
		{"gates-unknown-edge-type", base + "gates:\n  - {on: \"svc/02\", type: telepathy, reason: \"because\"}\n", "type"},
		{"gates-missing-reason", base + "gates:\n  - {on: \"svc/02\", type: ordering-gate}\n", "reason"},
		{"gates-unknown-key", base + "gates:\n  - {on: \"svc/02\", type: ordering-gate, reason: r, bogus: 1}\n", "bogus"},
		{"feathers-unknown-edge-type", base + "feathers:\n  - {ref: \"rec:svc/02\", type: vibes}\n", "type"},
		{"verify-unknown-key", base + "verify:\n  - {id: v1, target: cluster, bogus: 1}\n", "bogus"},
		{"supersedes-not-strings", base + "supersedes: [1, 2]\n", "supersedes"},
		// The brief-v1 rules brief-v2 inherits unchanged — a v2 file must not be a
		// hole where the shared contract stops applying.
		{"bad-effort", strings.Replace(base, "effort: S", "effort: XL", 1), "effort"},
		{"missing-required-risk", strings.Replace(base, "risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n", "", 1), "risk"},
		{"empty-sources", strings.Replace(base, `sources: ["fixture"]`, "sources: []", 1), "sources"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, msg := conformFrontmatter(t, schemas, tc.fm)
			if state != conformStateFailed {
				t.Fatalf("expected checked-failed, got state=%d msg=%q", state, msg)
			}
			if !strings.Contains(msg, tc.wantWord) {
				t.Fatalf("rejection message should name %q; got %q", tc.wantWord, msg)
			}
		})
	}
}

// TestConformBriefV2FieldErrorIsExit1 carries the same point up to the process
// boundary: a v2 field error is checked-FAILED (exit 1), never rounded into the
// could-not-check lane the version mismatch uses. The three states stay distinct.
func TestConformBriefV2FieldErrorIsExit1(t *testing.T) {
	root := migrateFixtureTree(t, true)
	var mout, merr bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &mout, &merr); code != migrateExitOK {
		t.Fatalf("migrate exit=%d; stderr=%s", code, merr.String())
	}
	p := filepath.Join(root, "docs", "streams", "svc", "brief-01-a.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), "effort: M", "effort: XL", 1)
	if broken == string(raw) {
		t.Fatal("fixture precondition failed — effort line not found to break")
	}
	if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := runConform([]string{"--root", root}, &out, &errb); code != conformExitFailed {
		t.Fatalf("a brief-v2 field error must be exit 1 (checked-failed), got %d; out=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "checked-failed") || !strings.Contains(out.String(), "effort") {
		t.Fatalf("the failure must name the offending field; got %s", out.String())
	}
	if strings.Contains(out.String(), "could-not-check:") {
		t.Fatalf("a field error must not be reported in the could-not-check lane; got %s", out.String())
	}
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

func mustParseEmbeddedSchemaV2(t *testing.T) *schemaNode {
	t.Helper()
	schema, err := parseSchema(embeddedBriefV2Schema())
	if err != nil {
		t.Fatalf("embedded brief-v2 schema does not parse: %v", err)
	}
	return schema
}

func mustParseEmbeddedSchemas(t *testing.T) map[string]*schemaNode {
	t.Helper()
	schemas, err := parseEmbeddedBriefSchemas()
	if err != nil {
		t.Fatalf("embedded schemas do not parse: %v", err)
	}
	return schemas
}

// conformFrontmatter writes a frontmatter body into a temp brief file and returns
// conformFile's classification of it against the full embedded contract set.
func conformFrontmatter(t *testing.T, schemas map[string]*schemaNode, fm string) (conformState, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "brief-01-x.md")
	if err := os.WriteFile(p, []byte("---\n"+fm+"---\n# body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return conformFile(p, schemas)
}

// schemaCoverageProblems returns the ways the schema fails to encode exactly the
// contract brieffile.go's tables define, for the brief-schema version wantMarker.
// Empty means lockstep. Everything checked here is the surface brief-v1 and
// brief-v2 SHARE — brief-v2 adds keys, removes none — so one function holds both
// in lockstep with the same tables.
func schemaCoverageProblems(raw map[string]any, wantMarker string) []string {
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

	// schema marker is this contract's own version constant.
	if sc, _ := props["schema"].(map[string]any); sc == nil || sc["const"] != wantMarker {
		add("properties.schema.const must be %q", wantMarker)
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

// schemaV2CoverageProblems returns the ways the brief-v2 schema fails to encode
// the v2-SPECIFIC surface briefv2.go defines: the hierarchical `brief:` shape,
// the uuid v4 `id:` shape, the `version:` floor, and the edge-type taxonomy on
// both reserved edge fields. Each expectation is derived from briefv2.go's own
// regexp or table, not restated, so the two cannot drift silently.
func schemaV2CoverageProblems(raw map[string]any) []string {
	var probs []string
	add := func(f string, a ...any) { probs = append(probs, fmt.Sprintf(f, a...)) }

	props, _ := raw["properties"].(map[string]any)
	if props == nil {
		return []string{"schema has no properties object"}
	}

	// brief: the four-segment hierarchical id. Derived behaviourally from
	// parseBriefV2ID — the schema pattern must accept exactly what it accepts.
	briefProp, _ := props["brief"].(map[string]any)
	if briefProp == nil {
		add("properties.brief missing")
	} else {
		pat, _ := briefProp["pattern"].(string)
		if pat == "" {
			add("properties.brief has no pattern — a v2 id shape must be pinned")
		} else {
			re, err := regexp.Compile(pat)
			if err != nil {
				add("properties.brief pattern does not compile: %v", err)
			} else {
				for _, id := range []string{"assay:assay:derived-board:01", "example:app:svc:07b"} {
					if _, _, _, _, ok := parseBriefV2ID(id); ok && !re.MatchString(id) {
						add("properties.brief pattern rejects %q, which parseBriefV2ID accepts — the schema is stricter than the validator", id)
					}
				}
				for _, id := range []string{"svc/01", "a:b:c", "a:b:c:d:e", "a::c:d"} {
					if _, _, _, _, ok := parseBriefV2ID(id); !ok && re.MatchString(id) {
						add("properties.brief pattern accepts %q, which parseBriefV2ID rejects — the schema is looser than the validator", id)
					}
				}
			}
		}
	}

	// id: the uuid v4 shape briefv2.go's uuidV4Re pins (plus the empty string,
	// which the validator skips exactly as it skips an absent id).
	idProp, _ := props["id"].(map[string]any)
	if idProp == nil {
		add("properties.id missing")
	} else {
		pat, _ := idProp["pattern"].(string)
		re, err := regexp.Compile(pat)
		switch {
		case pat == "":
			add("properties.id has no pattern — the uuid v4 shape must be pinned")
		case err != nil:
			add("properties.id pattern does not compile: %v", err)
		default:
			for _, s := range []string{"", "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"} {
				if s != "" && !uuidV4Re.MatchString(s) {
					continue
				}
				if !re.MatchString(s) {
					add("properties.id pattern rejects %q, which the reference validator accepts", s)
				}
			}
			for _, s := range []string{"not-a-uuid", "3f1b2c4d-5e6f-1a7b-8c9d-0e1f2a3b4c5d"} {
				if !uuidV4Re.MatchString(s) && re.MatchString(s) {
					add("properties.id pattern accepts %q, which uuidV4Re rejects", s)
				}
			}
		}
	}

	// version: an integer with the >= 1 floor checkBriefV2Semantics enforces.
	verProp, _ := props["version"].(map[string]any)
	if verProp == nil {
		add("properties.version missing")
	} else {
		if verProp["type"] != "integer" {
			add("properties.version.type must be integer")
		}
		if toInt(verProp["minimum"]) != 1 {
			add("properties.version.minimum must be 1 (checkBriefV2Semantics PROBLEMs version < 1)")
		}
	}

	// gates/feathers: the edge-type taxonomy, derived from validEdgeTypes.
	for _, field := range []string{"gates", "feathers"} {
		p, _ := props[field].(map[string]any)
		if p == nil {
			add("properties.%s missing", field)
			continue
		}
		items, _ := p["items"].(map[string]any)
		if items == nil {
			add("properties.%s.items missing", field)
			continue
		}
		iprops, _ := items["properties"].(map[string]any)
		typ, _ := iprops["type"].(map[string]any)
		if typ == nil {
			add("properties.%s.items.properties.type missing", field)
			continue
		}
		if got := anyStringList(typ["enum"]); !sameSet(got, keysOf(validEdgeTypes)) {
			add("properties.%s edge-type enum %v != briefv2.go validEdgeTypes %v", field, cfSorted(got), cfSorted(keysOf(validEdgeTypes)))
		}
		if ap, ok := items["additionalProperties"].(bool); !ok || ap != false {
			add("properties.%s.items.additionalProperties must be false (edgeList rejects unknown keys)", field)
		}
	}

	// verify: the {id, target} closed row record.
	vp, _ := props["verify"].(map[string]any)
	if vp == nil {
		add("properties.verify missing")
	} else {
		items, _ := vp["items"].(map[string]any)
		if items == nil {
			add("properties.verify.items missing")
		} else {
			if ap, ok := items["additionalProperties"].(bool); !ok || ap != false {
				add("properties.verify.items.additionalProperties must be false (verifyRowMetaList rejects unknown keys)")
			}
			if !sameSet(mapKeys(mapOf(items["properties"])), []string{"id", "target"}) {
				add("properties.verify.items.properties must be exactly id and target")
			}
		}
	}
	return probs
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
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
