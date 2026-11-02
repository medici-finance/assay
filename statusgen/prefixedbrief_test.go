package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
)

// ms04Tree copies the brief-v4 fixture tree into a fresh temp root, applies
// overlays (path relative to docs/streams/demo4 -> new content, or a func
// editing the current content), and returns the root.
func ms04Tree(t *testing.T, edits map[string]func(string) string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/milestone-support/04/tree")); err != nil {
		t.Fatal(err)
	}
	for name, edit := range edits {
		p := filepath.Join(root, "docs", "streams", "demo4", name)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(edit(string(raw))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func ms04Check(t *testing.T, root string) (problems, notices []string, streams []*Stream) {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, notices = checkBriefFiles(streams, streams)
	return problems, notices, streams
}

func ms04Replace(old, new string) func(string) string {
	return func(s string) string {
		if strings.Count(s, old) != 1 {
			panic("fixture anchor not unique: " + old)
		}
		return strings.Replace(s, old, new, 1)
	}
}

// legacyReaderCaps is a reader that advertises only the shared legacy syntax
// registry (briefdoc.SchemaVersions — the set desk-tools reads).
func legacyReaderCaps() readercaps.Advertisement {
	return readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: "legacy-reader", Version: "test",
		Operations: []readercaps.Operation{{Operation: "syntax", Schemas: briefdoc.SchemaVersions(), Semantics: []string{}}}}
}

func hasHold(e Eligibility, ref string, state EligibilityState) bool {
	for _, h := range e.Holds {
		if h.Ref == ref && h.State == state {
			return true
		}
	}
	return false
}

// regression: milestone-support/04
func TestPrefixedBriefSchema(t *testing.T) {
	t.Run("valid prefixed tree parses, binds and holds", func(t *testing.T) {
		root := ms04Tree(t, nil)
		problems, notices, streams := ms04Check(t, root)
		if len(problems) != 0 {
			t.Fatalf("the valid brief-v4 fixture tree must lint clean; problems:\n%s", strings.Join(problems, "\n"))
		}
		bf, ok, err := parseBriefFile(filepath.Join(root, "docs/streams/demo4/brief-02-prefixed.md"))
		if err != nil || !ok {
			t.Fatalf("parse brief-02: ok=%v err=%v", ok, err)
		}
		if bf.Schema != briefSchemaV4 || bf.TypedID != "assay:assay:demo4:b-02" || bf.Brief != "assay:assay:demo4:02" {
			t.Errorf("prefixed id: schema=%q typed=%q compat=%q", bf.Schema, bf.TypedID, bf.Brief)
		}
		if bf.Version != 1 || bf.ID != "00000000-0000-4000-8000-000000000402" {
			t.Errorf("version/id: %d %q", bf.Version, bf.ID)
		}
		if len(bf.Depends) != 1 || bf.Depends[0] != "demo4/01" {
			t.Errorf("prefixed depends must normalize to the one logical key demo4/01; got %v", bf.Depends)
		}
		if len(bf.RequiresMilestones) != 1 || bf.RequiresMilestones[0] != "demo4:m-01@1" {
			t.Errorf("requires-milestones: %v", bf.RequiresMilestones)
		}
		// Unversioned selector binds to the snapshot: no unknown notice for 02.
		if hasProblem(notices, "brief-02-prefixed.md", "is unknown") {
			t.Errorf("an unversioned selector must bind to the selected snapshot; notices:\n%s", strings.Join(notices, "\n"))
		}
		// Pinned unavailable revision: unknown, never substituted.
		if !hasProblem(notices, "brief-03-pinned.md", `"demo4:b-01@1" is unknown`, "never substituted") {
			t.Errorf("an unavailable pinned revision must be reported unknown; notices:\n%s", strings.Join(notices, "\n"))
		}
		if !hasProblem(notices, "brief-02-prefixed.md", "requires-milestones demo4:m-01@1", "held could-not-check") {
			t.Errorf("a milestone start requirement must be announced as held; notices:\n%s", strings.Join(notices, "\n"))
		}
		elig := eligibilityForStreams(streams)
		if e := elig["demo4/02"]; e.Verdict != VerdictHeld || !hasHold(e, "demo4:m-01@1", StateCouldNotCheck) {
			t.Errorf("brief 02 must be HELD could-not-check on its milestone requirement; got %+v", e)
		}
		if e := elig["demo4/03"]; e.Verdict != VerdictHeld || !hasHold(e, "demo4:b-01@1", StateCouldNotCheck) {
			t.Errorf("brief 03 must be HELD on its unknown pinned revision; got %+v", e)
		}
	})

	t.Run("pinned available revision binds", func(t *testing.T) {
		root := ms04Tree(t, map[string]func(string) string{
			"brief-03-pinned.md": ms04Replace(`depends: ["demo4:b-01@1"]`, `depends: ["demo4:b-01@2"]`),
		})
		problems, notices, streams := ms04Check(t, root)
		if len(problems) != 0 || hasProblem(notices, "brief-03-pinned.md", "is unknown") {
			t.Fatalf("an exact available revision must bind; problems=%v notices=%v", problems, notices)
		}
		if e := eligibilityForStreams(streams)["demo4/03"]; hasHold(e, "demo4:b-01@2", StateCouldNotCheck) {
			t.Errorf("a bound pinned selector must not add a could-not-check hold; got %+v", e)
		}
	})

	t.Run("milestone gate held only while declared", func(t *testing.T) {
		root := ms04Tree(t, map[string]func(string) string{
			"brief-02-prefixed.md": ms04Replace("requires-milestones: [\"demo4:m-01@1\"]\n", ""),
		})
		_, _, streams := ms04Check(t, root)
		if e := eligibilityForStreams(streams)["demo4/02"]; hasHold(e, "demo4:m-01@1", StateCouldNotCheck) {
			t.Errorf("without requires-milestones there is no milestone hold; got %+v", e)
		}
	})

	refusals := []struct {
		name, file, old, new string
		want                 []string
	}{
		{"legacy and prefixed alias read once", "brief-02-prefixed.md", `depends: ["demo4:b-01"]`, `depends: ["demo4/01", "demo4:b-01"]`, []string{"read once"}},
		{"milestone in depends", "brief-02-prefixed.md", `depends: ["demo4:b-01"]`, `depends: ["demo4:m-01@1"]`, []string{"names a milestone", "requires-milestones"}},
		{"duplicate uuid", "brief-03-pinned.md", "id: 00000000-0000-4000-8000-000000000403", "id: 00000000-0000-4000-8000-000000000401", []string{"is already used by"}},
		{"version required", "brief-02-prefixed.md", "version: 1\n", "", []string{"version is required under brief-v4"}},
		{"id required", "brief-02-prefixed.md", "id: 00000000-0000-4000-8000-000000000402\n", "", []string{"id is required under brief-v4"}},
		{"unprefixed definition id", "brief-02-prefixed.md", "brief: assay:assay:demo4:b-02", "brief: assay:assay:demo4:02", []string{"colon-only definition id"}},
		{"revision in definition id", "brief-02-prefixed.md", "brief: assay:assay:demo4:b-02", "brief: assay:assay:demo4:b-02@1", []string{"colon-only definition id"}},
		{"envelope required", "brief-02-prefixed.md", "reader-requirements:\n  schema: reader-requirements-v1\n  required:\n    - operation: syntax\n      schemas: [brief-v4]\n      semantics: []\n", "", []string{"reader-requirements is required under brief-v4"}},
		{"envelope that holds back no legacy reader", "brief-02-prefixed.md", "required:\n    - operation: syntax\n      schemas: [brief-v4]\n      semantics: []\n", "required: []\n", []string{"reader-requirements:", "must require syntax support for brief-v4"}},
		{"envelope naming only a legacy schema", "brief-02-prefixed.md", "schemas: [brief-v4]", "schemas: [brief-v3]", []string{"reader-requirements:", "must require syntax support for brief-v4"}},
		{"slash milestone", "brief-02-prefixed.md", `["demo4:m-01@1"]`, `["demo4/m-01@1"]`, []string{"requires-milestones"}},
		{"unversioned milestone", "brief-02-prefixed.md", `["demo4:m-01@1"]`, `["demo4:m-01"]`, []string{"requires-milestones", "not an exact versioned milestone"}},
		{"unknown milestone root", "brief-02-prefixed.md", `["demo4:m-01@1"]`, `["other:demo4:m-01@1"]`, []string{"does not resolve"}},
		{"conflicting milestone versions", "brief-02-prefixed.md", `["demo4:m-01@1"]`, `["demo4:m-01@1", "assay:demo4:m-01@2"]`, []string{"conflicting revision requirements"}},
		{"legacy schema carrying requires-milestones", "brief-01-base.md", "version: 2\n", "version: 2\nrequires-milestones: [\"demo4:m-01@1\"]\n", []string{"requires-milestones needs schema brief-v4", "silently drop"}},
		{"legacy schema carrying prefixed id", "brief-01-base.md", "brief: assay:assay:demo4:01", "brief: assay:assay:demo4:b-01", []string{"only schema brief-v4 defines"}},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			root := ms04Tree(t, map[string]func(string) string{tc.file: ms04Replace(tc.old, tc.new)})
			problems, _, _ := ms04Check(t, root)
			if !hasProblem(problems, append([]string{tc.file}, tc.want...)...) {
				t.Errorf("want a PROBLEM on %s naming %q; problems:\n%s", tc.file, tc.want, strings.Join(problems, "\n"))
			}
		})
	}

	// regression: milestone-support/04-followup F4 — the same-root rule for
	// depends/unblocks had no test: a qualified edge naming another registered
	// root must be refused, and the same qualified form naming this root must not.
	t.Run("cross-root brief edge refuses", func(t *testing.T) {
		withOther := func(t *testing.T, spelling string) []string {
			t.Helper()
			root := ms04Tree(t, map[string]func(string) string{
				"brief-02-prefixed.md": ms04Replace(`depends: ["demo4:b-01"]`, `depends: ["`+spelling+`"]`),
			})
			reg := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
			raw, err := os.ReadFile(reg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(reg, append(raw, []byte("  other: {cell: assay, repo: medici-finance/other}\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			problems, _, _ := ms04Check(t, root)
			return problems
		}
		if p := withOther(t, "other:demo4:b-01"); !hasProblem(p, "brief-02-prefixed.md", "depends", "names brief work in another root (assay:other)", "gates:/feathers:") {
			t.Errorf("a depends edge naming another registered root must be refused as cross-root; problems:\n%s", strings.Join(p, "\n"))
		}
		if p := withOther(t, "assay:demo4:b-01"); hasProblem(p, "brief-02-prefixed.md", "another root") {
			t.Errorf("a qualified edge naming THIS root must not be refused as cross-root; problems:\n%s", strings.Join(p, "\n"))
		}
	})

	// regression: milestone-support/04-followup F3 — a failure to resolve the
	// brief's own root used to be discarded and reported as a cross-root edge.
	t.Run("own-root resolve failure names its cause", func(t *testing.T) {
		root := ms04Tree(t, map[string]func(string) string{
			"brief-02-prefixed.md": func(s string) string {
				s = ms04Replace("brief: assay:assay:demo4:b-02", "brief: assay:ghost:demo4:b-02")(s)
				return ms04Replace(`depends: ["demo4:b-01"]`, `depends: ["assay:demo4:b-01"]`)(s)
			},
		})
		problems, _, _ := ms04Check(t, root)
		if !hasProblem(problems, "brief-02-prefixed.md", "depends", "assay:demo4:b-01", "unknown owning root assay:ghost") {
			t.Errorf("the own-root resolve error must be surfaced; problems:\n%s", strings.Join(problems, "\n"))
		}
		if hasProblem(problems, "brief-02-prefixed.md", "another root") {
			t.Errorf("a failure that is not cross-root must not be reported as cross-root; problems:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("mismatched schema contract and legacy reader refuse", func(t *testing.T) {
		raw, err := os.ReadFile("testdata/milestone-support/04/tree/docs/streams/demo4/brief-02-prefixed.md")
		if err != nil {
			t.Fatal(err)
		}
		schemas, err := parseEmbeddedBriefSchemas()
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		v4 := filepath.Join(dir, "v4.md")
		if err := os.WriteFile(v4, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		if st, msg := conformFile(v4, schemas); st != conformStateClean {
			t.Errorf("the brief-v4 fixture must conform to the embedded brief-v4 contract: %v %s", st, msg)
		}
		// A legacy-shaped record (unprefixed id, no uuid) declared brief-v4 must
		// fail the brief-v4 contract; the reverse mismatch (a prefixed record
		// declared brief-v2) is the parser PROBLEM in the refusal table above.
		for name, edit := range map[string][2]string{
			"unprefixed id": {"brief: assay:assay:demo4:b-02", "brief: assay:assay:demo4:02"},
			"no uuid":       {"id: 00000000-0000-4000-8000-000000000402\n", ""},
			"no version":    {"version: 1\n", ""},
			"no envelope":   {"reader-requirements:\n  schema: reader-requirements-v1\n  required:\n    - operation: syntax\n      schemas: [brief-v4]\n      semantics: []\n", ""},
		} {
			mis := filepath.Join(dir, "mis.md")
			if err := os.WriteFile(mis, []byte(strings.Replace(string(raw), edit[0], edit[1], 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			if st, _ := conformFile(mis, schemas); st != conformStateFailed {
				t.Errorf("%s: a legacy-shaped record declared brief-v4 must fail the brief-v4 contract; got %v", name, st)
			}
		}
		if r := record.Check(raw, statusgenReaderCaps()); r.State != readercaps.Supported {
			t.Errorf("statusgen must be Supported for its own brief-v4 record: %+v", r)
		}
		if r := record.Check(raw, legacyReaderCaps()); r.State != readercaps.Unsupported {
			t.Errorf("a legacy (v1-v3) reader must be Unsupported for a brief-v4 record: %+v", r)
		}
	})
}

// TestBriefV4SchemaCoverage holds schemas/brief-v4.json in lockstep with the
// brief-v4 parser: the shared surface through schemaCoverageProblems (with the
// two extra required keys), and the definition-id and requires-milestones
// patterns behaviourally against parseBriefV4Keys — the schema may be looser
// than the validator, never stricter, and never accepts the legacy unprefixed
// definition id the marker exists to distinguish.
func TestBriefV4SchemaCoverage(t *testing.T) {
	node, err := parseSchema(embeddedSchemaBytes(embeddedBriefSchemaNames[briefSchemaV4]))
	if err != nil {
		t.Fatal(err)
	}
	raw := node.raw
	var problems []string
	for _, p := range schemaCoverageProblems(raw, briefSchemaV4) {
		if !strings.HasPrefix(p, "required set ") {
			problems = append(problems, p)
		}
	}
	want := append(append([]string{}, requiredBriefKeys...), "schema", "version", "id", "reader-requirements")
	if got := anyStringList(raw["required"]); !sameSet(got, want) {
		problems = append(problems, "required set does not equal brief-v1 required + schema + version + id + reader-requirements")
	}
	props, _ := raw["properties"].(map[string]any)
	pattern := func(prop string, items bool) *regexp.Regexp {
		p, _ := props[prop].(map[string]any)
		if items {
			p, _ = p["items"].(map[string]any)
		}
		s, _ := p["pattern"].(string)
		re, err := regexp.Compile(s)
		if s == "" || err != nil {
			t.Fatalf("properties.%s pattern missing or invalid: %q %v", prop, s, err)
		}
		return re
	}
	parses := func(brief string, ms []any) bool {
		data := map[string]any{"version": 1, "reader-requirements": map[string]any{
			"schema":   readercaps.RequirementsSchema,
			"required": []any{map[string]any{"operation": "syntax", "schemas": []any{briefSchemaV4}, "semantics": []any{}}},
		}}
		if ms != nil {
			data["requires-milestones"] = ms
		}
		bad := false
		parseBriefV4Keys(data, &BriefFile{Brief: brief, ID: "00000000-0000-4000-8000-000000000404"}, func(string, ...any) { bad = true })
		return !bad
	}
	briefRe := pattern("brief", false)
	for _, id := range []string{"assay:assay:demo4:b-02", "example:app:svc:b-07b", "c:r:s:b-1234"} {
		if !parses(id, nil) {
			t.Fatalf("fixture id %q must parse", id)
		}
		if !briefRe.MatchString(id) {
			problems = append(problems, "brief pattern rejects parser-accepted "+id)
		}
	}
	for _, id := range []string{"assay:assay:demo4:02", "demo4/02", "demo4:b-02", "assay:assay:demo4:b-02@1"} {
		if parses(id, nil) || briefRe.MatchString(id) {
			problems = append(problems, "legacy or selector-carrying definition id accepted: "+id)
		}
	}
	msRe := pattern("requires-milestones", true)
	for _, m := range []string{"demo4:m-01@1", "assay:demo4:m-10@2", "assay:assay:demo4:m-123@30"} {
		if !parses("assay:assay:demo4:b-02", []any{m}) {
			t.Fatalf("fixture milestone %q must parse", m)
		}
		if !msRe.MatchString(m) {
			problems = append(problems, "requires-milestones pattern rejects parser-accepted "+m)
		}
	}
	if len(problems) > 0 {
		t.Fatalf("committed brief-v4 schema drifted from the validated contract:\n  - %s", strings.Join(problems, "\n  - "))
	}
}
