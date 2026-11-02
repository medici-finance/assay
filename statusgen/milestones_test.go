package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
	"gopkg.in/yaml.v3"
)

func milestoneFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "milestone-support", "03", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func milestoneEntry(name, content string) MilestoneEntry {
	return MilestoneEntry{Name: name, Content: []byte(content)}
}

// milestoneTestSource is the explicit loader input the host adapter would build:
// owning root assay:assay (with historical repository alias "old"), a second root
// assay:other, and a repository view holding the cited source and evidence.
func milestoneTestSource(t *testing.T, entries ...MilestoneEntry) MilestoneSource {
	t.Helper()
	reg, err := workref.NewRegistry([]workref.Root{
		{Cell: "assay", Repo: "assay", Aliases: []string{"old"}},
		{Cell: "assay", Repo: "other"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return MilestoneSource{
		Stream:   "delivery",
		Owner:    &workref.Context{Cell: "assay", Repo: "assay"},
		Registry: reg,
		Entries:  entries,
		Repo: fstest.MapFS{
			"docs/spec.md":            {Data: []byte("spec")},
			"docs/evidence/c01.md":    {Data: []byte("c01")},
			"docs/evidence/runner.md": {Data: []byte("runner")},
		},
	}
}

func loadMilestoneTest(t *testing.T, in MilestoneSource) *MilestonePlan {
	t.Helper()
	plan := loadMilestonePlan(in)
	resolveMilestonePrerequisites([]*MilestonePlan{plan})
	return plan
}

func milestoneReasons(plan *MilestonePlan) string {
	var b strings.Builder
	for _, d := range plan.Diagnostics {
		b.WriteString("refused " + d.Path + ": " + d.Reason + "\n")
	}
	for _, u := range plan.Unknowns {
		b.WriteString("unknown " + u.Path + ": " + u.Reason + "\n")
	}
	return b.String()
}

func milestoneRefs(plan *MilestonePlan) []string {
	var out []string
	for _, r := range plan.Records {
		out = append(out, r.Ref())
	}
	return out
}

// regression: milestone-support/03
func TestMilestoneRecords(t *testing.T) {
	canonical := milestoneFixture(t, "canonical.md")
	compatible := milestoneFixture(t, "compatible.md")

	t.Run("canonical and compatible records load; version comes from keys", func(t *testing.T) {
		// The -v2 filename suffix is only a locator: contract-version 1 governs.
		plan := loadMilestoneTest(t, milestoneTestSource(t,
			milestoneEntry("m-03-build-less-brittle-v2.md", canonical),
			milestoneEntry("m-01-runner.md", compatible),
			milestoneEntry("README.md", "index"),
		))
		if plan.Coverage != MilestoneCoverageKnown || len(plan.Diagnostics)+len(plan.Unknowns) != 0 {
			t.Fatalf("well-formed plan not known:\n%s", milestoneReasons(plan))
		}
		got := strings.Join(milestoneRefs(plan), ",")
		if got != "assay:assay:delivery:m-01@1,assay:assay:delivery:m-03@1" {
			t.Fatalf("records = %s, want m-01@1 (slash draft key normalized) and m-03@1 (version from contract-version, not the -v2 filename)", got)
		}
		m03 := plan.Records[1]
		if m03.Path != "milestones/m-03-build-less-brittle-v2.md" || m03.Title != "Build changes are less brittle" {
			t.Fatalf("locator/title lost: %+v", m03)
		}
		if len(m03.Contributes) != 2 || m03.Contributes[0].Revision != 0 || m03.Contributes[1].Revision != 2 ||
			m03.Contributes[1].LogicalKey != "assay:assay:delivery:b-12" {
			t.Fatalf("exact optional brief revisions not preserved: %+v", m03.Contributes)
		}
		if len(m03.RequiresMilestones) != 1 || m03.RequiresMilestones[0].Revision != 1 ||
			m03.RequiresMilestones[0].Identity.String() != "assay:assay:delivery:m-01" {
			t.Fatalf("prerequisite version not exact: %+v", m03.RequiresMilestones)
		}
		if len(m03.Criteria) != 2 || m03.Criteria[0].ID != "C01" || strings.Join(m03.Criteria[0].Contributors, ",") != "assay:assay:delivery:b-11" ||
			strings.Join(m03.Criteria[0].Evidence, ",") != "docs/evidence/c01.md" {
			t.Fatalf("coverage not mapped: %+v", m03.Criteria)
		}
	})

	t.Run("distinct versions of one identity are both retained", func(t *testing.T) {
		v2 := strings.Replace(canonical, "contract-version: 1", "contract-version: 2", 1)
		plan := loadMilestoneTest(t, milestoneTestSource(t,
			milestoneEntry("m-03-build-less-brittle.md", canonical),
			milestoneEntry("m-03-build-less-brittle-v2.md", v2),
			milestoneEntry("m-01-runner.md", compatible),
		))
		if got := strings.Join(milestoneRefs(plan), ","); got != "assay:assay:delivery:m-01@1,assay:assay:delivery:m-03@1,assay:assay:delivery:m-03@2" {
			t.Fatalf("records = %s\n%s", got, milestoneReasons(plan))
		}
	})

	t.Run("duplicate version, including through a repository alias, refuses both copies only", func(t *testing.T) {
		aliased := strings.Replace(canonical, "milestone: assay:assay:delivery:m-03", "milestone: assay:old:delivery:m-03", 1)
		plan := loadMilestoneTest(t, milestoneTestSource(t,
			milestoneEntry("m-03-a.md", canonical),
			milestoneEntry("m-03-b.md", aliased),
			milestoneEntry("m-01-runner.md", compatible),
		))
		if got := strings.Join(milestoneRefs(plan), ","); got != "assay:assay:delivery:m-01@1" {
			t.Fatalf("records = %s; the duplicated version must be refused without hiding m-01\n%s", got, milestoneReasons(plan))
		}
		reasons := milestoneReasons(plan)
		if plan.Coverage != MilestoneCoverageUnknown || !strings.Contains(reasons, "milestones/m-03-a.md") ||
			!strings.Contains(reasons, "milestones/m-03-b.md") || !strings.Contains(reasons, "duplicate record for assay:assay:delivery:m-03@1") {
			t.Fatalf("duplicate not diagnosed with both locations:\n%s", reasons)
		}
	})

	refused := func(t *testing.T, name, record, want string) {
		t.Helper()
		plan := loadMilestoneTest(t, milestoneTestSource(t,
			milestoneEntry("m-03-x.md", record),
			milestoneEntry("m-01-runner.md", compatible),
		))
		reasons := milestoneReasons(plan)
		if got := strings.Join(milestoneRefs(plan), ","); got != "assay:assay:delivery:m-01@1" || plan.Coverage != MilestoneCoverageUnknown ||
			!strings.Contains(reasons, want) {
			t.Fatalf("%s: records=%s coverage=%s, want refusal containing %q:\n%s", name, got, plan.Coverage, want, reasons)
		}
	}
	for _, c := range []struct{ name, old, new, want string }{
		{"owner stream", "milestone: assay:assay:delivery:m-03", "milestone: assay:assay:elsewhere:m-03", "not this record's location"},
		{"owner repository", "milestone: assay:assay:delivery:m-03", "milestone: assay:other:delivery:m-03", "not this record's location"},
		{"unknown root", "milestone: assay:assay:delivery:m-03", "milestone: assay:nowhere:delivery:m-03", "unknown owning root"},
		{"unknown key", "owner: build maintainers\n", "owner: build maintainers\nwave: 2\n", `unknown key "wave"`},
		{"missing key", "owner: build maintainers\n", "", `missing required key "owner"`},
		{"version in key", "milestone: assay:assay:delivery:m-03", "milestone: assay:assay:delivery:m-03@1", "carries a version selector"},
		{"non-integer version", "contract-version: 1", `contract-version: "1"`, "contract-version must be a positive integer"},
		{"unpadded id", "milestone: assay:assay:delivery:m-03", "milestone: assay:assay:delivery:m-3", "padded positive number"},
		{"wrong-kind contribution", `"assay:assay:delivery:b-11", `, `"assay:assay:delivery:m-04@1", `, "wrong work kind"},
		{"wrong-kind prerequisite", `requires-milestones: ["delivery:m-01@1"]`, `requires-milestones: ["delivery:b-01"]`, "wrong work kind"},
		{"duplicate contribution", `"assay:assay:delivery:b-12@2"]`, `"assay:assay:delivery:b-11@3"]`, "names assay:assay:delivery:b-11 twice"},
		{"wave-label title", `title: "Build changes are less brittle"`, `title: "m-03"`, "title must state a useful outcome"},
		{"missing body section", "## Acceptance authority", "## Authority", `"Acceptance authority" is missing`},
		{"bad criterion id", "| C02 | Failing", "| X2 | Failing", `criterion ID "X2"`},
		{"coverage of undefined criterion", "| C02 | `assay:assay:delivery:b-12` |", "| C09 | `assay:assay:delivery:b-12` |", `undefined criterion "C09"`},
		{"coverage contributor not declared", "| C02 | `assay:assay:delivery:b-12` |", "| C02 | `assay:assay:delivery:b-13` |", "not in contributes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if strings.Count(canonical, c.old) != 1 {
				t.Fatalf("fixture anchor %q not unique", c.old)
			}
			refused(t, c.name, strings.Replace(canonical, c.old, c.new, 1), c.want)
		})
	}

	t.Run("committed schema agrees with the loader on frontmatter shape", func(t *testing.T) {
		raw, err := os.ReadFile("../schemas/milestone-v1.json")
		if err != nil {
			t.Fatal(err)
		}
		schema, err := parseSchema(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			name, doc string
			ok        bool
		}{
			{"canonical", canonical, true},
			{"compatible", compatible, true},
			{"unknown key", strings.Replace(canonical, "owner: build maintainers\n", "owner: build maintainers\nwave: 2\n", 1), false},
			{"missing key", strings.Replace(canonical, "owner: build maintainers\n", "", 1), false},
			{"string version", strings.Replace(canonical, "contract-version: 1", `contract-version: "1"`, 1), false},
			{"versioned key", strings.Replace(canonical, "delivery:m-03\n", "delivery:m-03@1\n", 1), false},
			{"unversioned prerequisite", strings.Replace(canonical, "delivery:m-01@1", "delivery:m-01", 1), false},
		} {
			block, _, err := briefdoc.Split(c.doc)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := yaml.Unmarshal([]byte(block), &value); err != nil {
				t.Fatal(err)
			}
			if got := len(validateValue(schema, value, "")) == 0; got != c.ok {
				t.Fatalf("%s: schema accepted=%v, want %v", c.name, got, c.ok)
			}
			entries := []MilestoneEntry{milestoneEntry("m-09-record.md", c.doc)}
			if c.doc != compatible {
				entries = append(entries, milestoneEntry("m-01-runner.md", compatible))
			}
			plan := loadMilestoneTest(t, milestoneTestSource(t, entries...))
			loaded := len(plan.Records) == len(entries)
			if loaded != c.ok {
				t.Fatalf("%s: loader loaded=%v, want %v:\n%s", c.name, loaded, c.ok, milestoneReasons(plan))
			}
		}
	})

	t.Run("production loader reads opted-in streams and leaves legacy streams untouched", func(t *testing.T) {
		advertiseMilestones(t)
		root := milestoneTree(t)
		writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-03-build-less-brittle-v2.md", canonical)
		writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-01-runner.md", compatible)
		// A legacy stream's stray milestones/ directory is not read at all.
		writeMilestoneFile(t, root, "docs/streams/legacy/milestones/m-01-junk.md", "not a record")
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]*Stream{}
		for _, s := range streams {
			byName[s.Name] = s
		}
		if legacy := byName["legacy"]; legacy == nil || legacy.MilestoneOptIn || legacy.Milestones != nil {
			t.Fatalf("legacy stream changed: %+v", legacy)
		}
		d := byName["delivery"]
		if d == nil || !d.MilestoneOptIn || d.Milestones == nil {
			t.Fatalf("opted-in stream not loaded: %+v", d)
		}
		for raw, want := range map[string]bool{
			"---\nstream: s\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: dispatch, schemas: [], semantics: [milestone-v1]}]}\n---\n": true,
			"---\nstream: s\nreader-requirements: {schema: reader-requirements-v1, required: []}\n---\n":                                                              false,
			"---\nstream: s\nwhy: milestone-v1 named only in prose\n---\nreader-requirements: milestone-v1\n":                                                         false,
		} {
			if _, got, err := milestoneOptIn([]byte(raw)); err != nil || got != want {
				t.Fatalf("opt-in(%q) = %v, %v; want %v", raw, got, err, want)
			}
		}
		if d.Milestones.Coverage != MilestoneCoverageKnown ||
			strings.Join(milestoneRefs(d.Milestones), ",") != "assay:assay:delivery:m-01@1,assay:assay:delivery:m-03@1" {
			t.Fatalf("production plan: %v\n%s", milestoneRefs(d.Milestones), milestoneReasons(d.Milestones))
		}
	})
}

// regression: milestone-support/03
func TestMilestoneUnknownCoverage(t *testing.T) {
	canonical := milestoneFixture(t, "canonical.md")
	compatible := milestoneFixture(t, "compatible.md")

	unknown := func(t *testing.T, plan *MilestonePlan, want string) {
		t.Helper()
		reasons := milestoneReasons(plan)
		if plan.Coverage != MilestoneCoverageUnknown || !strings.Contains(reasons, want) {
			t.Fatalf("coverage=%s, want unknown with %q:\n%s", plan.Coverage, want, reasons)
		}
	}

	t.Run("inaccessible owning root is unknown, not an empty plan", func(t *testing.T) {
		in := milestoneTestSource(t, milestoneEntry("m-01-runner.md", compatible))
		in.Owner, in.OwnerErr = nil, "alias registry unreadable"
		plan := loadMilestoneTest(t, in)
		unknown(t, plan, "owning root is not established")
		if len(plan.Records) != 0 {
			t.Fatalf("records attributed without an owning root: %v", milestoneRefs(plan))
		}
	})
	t.Run("unreadable milestones directory is unknown", func(t *testing.T) {
		in := milestoneTestSource(t)
		in.DirErr = fs.ErrPermission
		unknown(t, loadMilestoneTest(t, in), "milestones directory unreadable")
	})
	t.Run("absent milestones directory of an opted-in stream is unknown", func(t *testing.T) {
		in := milestoneTestSource(t)
		in.DirMissing = true
		unknown(t, loadMilestoneTest(t, in), "no canonical milestones directory")
	})
	t.Run("unreadable record is unknown and does not hide readable ones", func(t *testing.T) {
		in := milestoneTestSource(t,
			MilestoneEntry{Name: "m-02-locked.md", Err: fs.ErrPermission},
			milestoneEntry("m-01-runner.md", compatible))
		plan := loadMilestoneTest(t, in)
		unknown(t, plan, "milestones/m-02-locked.md: record unreadable")
		if strings.Join(milestoneRefs(plan), ",") != "assay:assay:delivery:m-01@1" {
			t.Fatalf("readable record hidden: %v", milestoneRefs(plan))
		}
	})
	t.Run("symlink or nested location is refused", func(t *testing.T) {
		for _, mode := range []fs.FileMode{fs.ModeSymlink, fs.ModeDir} {
			plan := loadMilestoneTest(t, milestoneTestSource(t,
				MilestoneEntry{Name: "m-02-elsewhere.md", Mode: mode, Content: []byte(compatible)},
				milestoneEntry("m-01-runner.md", compatible)))
			unknown(t, plan, "milestones/m-02-elsewhere.md: not a regular file")
		}
	})
	t.Run("unrecognized entry may hide a record", func(t *testing.T) {
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-01.markdown", compatible))), "unrecognized entry")
	})
	t.Run("escaping evidence or source path refuses the record", func(t *testing.T) {
		for _, c := range []struct{ name, old, new string }{
			{"relative evidence", "`docs/evidence/runner.md`", "`../outside/runner.md`"},
			{"absolute evidence", "`docs/evidence/runner.md`", "`/etc/passwd`"},
			{"relative source", `sources: ["docs/spec.md#runner"]`, `sources: ["../outside/spec.md"]`},
			{"absolute source", `sources: ["docs/spec.md#runner"]`, `sources: ["/etc/passwd"]`},
		} {
			t.Run(c.name, func(t *testing.T) {
				if strings.Count(compatible, c.old) != 1 {
					t.Fatalf("fixture anchor %q not unique", c.old)
				}
				escaping := strings.Replace(compatible, c.old, c.new, 1)
				plan := loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-01-runner.md", escaping)))
				unknown(t, plan, "escapes or is not a clean repository-relative path")
				if len(plan.Records) != 0 {
					t.Fatalf("refused record still loaded: %v", milestoneRefs(plan))
				}
				// The refused record must not satisfy a dependant's prerequisite.
				plan = loadMilestoneTest(t, milestoneTestSource(t,
					milestoneEntry("m-01-runner.md", escaping), milestoneEntry("m-03-x.md", canonical)))
				if got := strings.Join(milestoneRefs(plan), ","); got != "assay:assay:delivery:m-03@1" {
					t.Fatalf("records = %s, want only the dependant m-03@1", got)
				}
				unknown(t, plan, "prerequisite assay:assay:delivery:m-01@1 is not a loaded record version")
			})
		}
	})
	t.Run("missing evidence or source is unknown", func(t *testing.T) {
		missing := strings.Replace(compatible, "docs/evidence/runner.md", "docs/evidence/gone.md", 1)
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-01-runner.md", missing))), `evidence "docs/evidence/gone.md" could not be read`)
		in := milestoneTestSource(t, milestoneEntry("m-01-runner.md", compatible))
		in.Repo = nil
		unknown(t, loadMilestoneTest(t, in), "repository tree unavailable")
	})
	t.Run("uncovered criterion stays unknown", func(t *testing.T) {
		uncovered := strings.Replace(canonical, "| C02 | `assay:assay:delivery:b-12` | — |\n", "", 1)
		uncovered = strings.Replace(uncovered, `, "assay:assay:delivery:b-12@2"]`, `]`, 1)
		plan := loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-03-x.md", uncovered), milestoneEntry("m-01-runner.md", compatible)))
		unknown(t, plan, "criterion C02 has no coverage row")
	})
	t.Run("empty contributes is justified only by evidence", func(t *testing.T) {
		plan := loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-01-runner.md", compatible)))
		if plan.Coverage != MilestoneCoverageKnown {
			t.Fatalf("existing capability with evidence must be known:\n%s", milestoneReasons(plan))
		}
		unmapped := strings.Replace(compatible, "`docs/evidence/runner.md`", "—", 1)
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-01-runner.md", unmapped))), "criterion C01 has neither a contributing brief nor existing evidence")
	})
	t.Run("contribution mapped to no criterion is unmapped work", func(t *testing.T) {
		extra := strings.Replace(canonical, `"assay:assay:delivery:b-12@2"]`, `"assay:assay:delivery:b-12@2", "assay:assay:delivery:b-13"]`, 1)
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-03-x.md", extra), milestoneEntry("m-01-runner.md", compatible))), "contribution assay:assay:delivery:b-13 maps to no acceptance criterion")
	})
	t.Run("unloaded prerequisite version is unknown", func(t *testing.T) {
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-03-x.md", canonical))), "prerequisite assay:assay:delivery:m-01@1 is not a loaded record version")
		v2 := strings.Replace(canonical, "delivery:m-01@1", "delivery:m-01@2", 1)
		unknown(t, loadMilestoneTest(t, milestoneTestSource(t, milestoneEntry("m-03-x.md", v2), milestoneEntry("m-01-runner.md", compatible))), "prerequisite assay:assay:delivery:m-01@2")
	})

	t.Run("unadvertised reader does not read an opted-in stream's records", func(t *testing.T) {
		// This statusgen does not advertise milestone-v1, so on every command
		// path that loads streams the records stay unread and coverage unknown.
		root := milestoneTree(t)
		writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-01-runner.md", compatible)
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		var d *Stream
		for _, s := range streams {
			if s.Name == "delivery" {
				d = s
			}
		}
		if d == nil || d.Milestones == nil {
			t.Fatalf("opted-in stream has no plan: %+v", d)
		}
		unknown(t, d.Milestones, "does not support this stream's reader-requirements")
		if len(d.Milestones.Records) != 0 {
			t.Fatalf("records read by a reader that does not advertise milestone-v1: %v", milestoneRefs(d.Milestones))
		}
	})

	t.Run("production loader keeps a real tree's gaps unknown", func(t *testing.T) {
		advertiseMilestones(t)
		root := milestoneTree(t)
		writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-01-runner.md", compatible)
		outside := filepath.Join(t.TempDir(), "m-02-outside.md")
		if err := os.WriteFile(outside, []byte(compatible), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "docs/streams/delivery/milestones/m-02-outside.md")); err != nil {
			t.Fatal(err)
		}
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range streams {
			if s.Name == "delivery" {
				unknown(t, s.Milestones, "milestones/m-02-outside.md: not a regular file")
				if strings.Join(milestoneRefs(s.Milestones), ",") != "assay:assay:delivery:m-01@1" {
					t.Fatalf("readable record hidden: %v", milestoneRefs(s.Milestones))
				}
			}
		}

		bare := milestoneTree(t)
		streams, _, err = loadStreams(bare)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range streams {
			if s.Name == "delivery" {
				unknown(t, s.Milestones, "no canonical milestones directory")
			}
		}

		noRegistry := milestoneTree(t)
		writeMilestoneFile(t, noRegistry, "docs/streams/delivery/milestones/m-01-runner.md", compatible)
		if err := os.Remove(filepath.Join(noRegistry, "docs/streams/graph-repos.yaml")); err != nil {
			t.Fatal(err)
		}
		streams, _, err = loadStreams(noRegistry)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range streams {
			if s.Name == "delivery" {
				unknown(t, s.Milestones, "owning root is not established")
			}
		}
	})
}

// milestoneTree builds a minimal checkout: an opted-in stream "delivery", a
// legacy stream "legacy", the graph alias registry and the cited evidence.
// advertiseMilestones stands in for the release that advertises milestone-v1:
// the production loader reads an opted-in stream's records only when this
// statusgen supports the stream's reader-requirements.
func advertiseMilestones(t *testing.T) {
	t.Helper()
	prev := milestoneReader
	milestoneReader = func() readercaps.Advertisement {
		ad := statusgenReaderCaps()
		for i, op := range ad.Operations {
			if op.Operation == "syntax" {
				ad.Operations[i].Schemas = append(append([]string{}, op.Schemas...), "milestone-v1")
			}
		}
		return ad
	}
	t.Cleanup(func() { milestoneReader = prev })
}

func milestoneTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMilestoneFile(t, root, "docs/streams/graph-repos.yaml", "schema: graph-repos-v1\ncell: assay\nrepos:\n  assay: {cell: assay, repo: example/assay}\n  other: {cell: assay, repo: null, unpublished: true}\n")
	writeMilestoneFile(t, root, "docs/streams/delivery/README.md", "---\nstream: delivery\nstatus: active\npriority: P2\nreader-requirements:\n  schema: reader-requirements-v1\n  required:\n    - operation: syntax\n      schemas: [milestone-v1]\n      semantics: []\n---\n\n# delivery\n")
	writeMilestoneFile(t, root, "docs/streams/legacy/README.md", "---\nstream: legacy\nstatus: active\npriority: P2\n---\n\n# legacy\n")
	for _, p := range []string{"docs/spec.md", "docs/evidence/c01.md", "docs/evidence/runner.md"} {
		writeMilestoneFile(t, root, p, "evidence\n")
	}
	return root
}

func writeMilestoneFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
