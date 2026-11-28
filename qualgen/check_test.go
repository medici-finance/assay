package main

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// --- fixture helpers: real git repos via requireGit/gitCmd/mine (mine_test.go,
// instructionbrittle_test.go), so `check` is dereferenced against genuine
// mined history and a genuine HEAD tree, not a mock. ---

// nested and flat are the same complexity-proxy shapes hotspot_test.go uses.
var (
	nestedLines = []string{"        x", "        x", "        x"}
	flatLines   = []string{"x"}
)

func writeNested(t *testing.T, dir, name string, n int) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < n; i++ {
		for _, l := range nestedLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	writeFile(t, dir, name, b.String())
}

// hotspotFixtureRepo builds a repo with hot.go — touched by several commits
// with deeply nested content — and quiet.go — touched once, flat content —
// so hot.go's decayed hotspot strictly outranks quiet.go's (hotspot_test.go's
// TestHotspotIsProductNotFactor pins the same product-of-both-factors shape).
func hotspotFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")

	writeFile(t, dir, "quiet.go", strings.Join(flatLines, "\n")+"\n")
	gitCmd(t, dir, "add", "quiet.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add quiet.go")

	for i := 0; i < 5; i++ {
		writeNested(t, dir, "hot.go", i+1)
		gitCmd(t, dir, "add", "hot.go")
		gitCmd(t, dir, "commit", "-q", "-m", "churn hot.go")
	}
	return dir
}

// couplingFixtureRepo builds a repo where a.go and b.go are co-changed in
// every one of three commits — a historically coupled pair well above
// DefaultCouplingParams' MinRatio/MinCoChanges.
func couplingFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	for i := 0; i < 3; i++ {
		writeFile(t, dir, "a.go", strings.Repeat("a", i+1)+"\n")
		writeFile(t, dir, "b.go", strings.Repeat("b", i+1)+"\n")
		gitCmd(t, dir, "add", "a.go", "b.go")
		gitCmd(t, dir, "commit", "-q", "-m", "co-change a.go and b.go")
	}
	return dir
}

// findAdvisory reports whether files contains path with an advisory of kind.
func findAdvisory(files []FileScreenResult, path string, kind AdvisoryKind) (Advisory, bool) {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		for _, a := range f.Advisories {
			if a.Kind == kind {
				return a, true
			}
		}
	}
	return Advisory{}, false
}

func resultFor(files []FileScreenResult, path string) (FileScreenResult, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return FileScreenResult{}, false
}

// TestCheck_HotspotFileFlaggedStrongerTier is Verify row 3: the churned,
// deeply-nested file draws the stronger-tier advisory; the quiet file does
// not.
func TestCheck_HotspotFileFlaggedStrongerTier(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"hot.go", "quiet.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}

	if _, ok := findAdvisory(result.Files, "hot.go", AdvisoryStrongerTier); !ok {
		t.Errorf("hot.go: expected a stronger-tier advisory, got %+v", result.Files)
	}
	if a, ok := findAdvisory(result.Files, "quiet.go", AdvisoryStrongerTier); ok {
		t.Errorf("quiet.go: expected NO stronger-tier advisory, got %+v", a)
	}
}

// TestCheck_CouplingPartnerNamed is Verify row 4: files A and B are
// historically coupled; `check A` (B not in the screened set) names B in
// the coupling-partner advisory.
func TestCheck_CouplingPartnerNamed(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"a.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}

	adv, ok := findAdvisory(result.Files, "a.go", AdvisoryCouplingPartner)
	if !ok {
		t.Fatalf("a.go: expected a coupling-partner advisory, got %+v", result.Files)
	}
	if !strings.Contains(adv.Detail, "b.go") {
		t.Errorf("coupling-partner advisory must name b.go specifically, got %q", adv.Detail)
	}
}

// TestCheck_AdvisoryPostureExitsZeroOnFlag is Verify row 5: `check` over a
// fixture hotspot file flags AND still returns exit 0 — advisory NOTICE
// posture, never a gate (spec §9.2).
func TestCheck_AdvisoryPostureExitsZeroOnFlag(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	var stdout, stderr bytes.Buffer
	rc := runCheck([]string{"hot.go", "--out", store.root, "--repo", dir}, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("runCheck returned %d, want 0 (advisory posture never fails on a flag); stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), string(AdvisoryStrongerTier)) {
		t.Errorf("stdout must contain the stronger-tier advisory, got:\n%s", stdout.String())
	}
}

// TestCheck_NoHistoryIsCouldNotScreen is Verify row 6: a path with no mined
// history (never committed) screens as could-not-screen, never an implied
// all-clear.
func TestCheck_NoHistoryIsCouldNotScreen(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "readme.txt", "hello\n")
	gitCmd(t, dir, "add", "readme.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add readme")
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"never/committed.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}
	fr, ok := resultFor(result.Files, "never/committed.go")
	if !ok {
		t.Fatalf("expected a result for the screened path, got %+v", result.Files)
	}
	if !fr.CouldNotScreen {
		t.Errorf("expected CouldNotScreen=true for a never-mined path, got %+v", fr)
	}
	if fr.Reason == "" {
		t.Errorf("could-not-screen must carry a reason, got empty")
	}
	if len(fr.Advisories) != 0 {
		t.Errorf("a could-not-screen file must raise no advisory (nothing measured to flag on), got %+v", fr.Advisories)
	}
}

// Exercise the public CLI with a real mined snapshot and an independent JSON
// consumer. In particular, a diff path can contain glob syntax and newlines.
func testCheckOutputFormats(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)
	head := strings.TrimSpace(gitCmd(t, dir, "rev-parse", "HEAD"))
	const unusualPath = "-missing[1]: path\n.go"

	var stdout, stderr bytes.Buffer
	textArgs := []string{"check", "--out", store.root, "--repo", dir, "*.go"}
	if rc := dispatch(textArgs, &stdout, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Fatalf("check: exit %d, stderr %q", rc, stderr.String())
	}
	const wantText = "a.go: no advisory signal in this screen\nb.go: no advisory signal in this screen\n"
	if stdout.String() != wantText {
		t.Fatalf("default check output = %q, want %q", stdout.String(), wantText)
	}
	stdout.Reset()

	args := []string{"check", "--json", "--literal-paths", "--out", store.root, "--repo", dir,
		"--", "a.go", unusualPath, "a.go"}
	if rc := dispatch(args, &stdout, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Fatalf("check --json: exit %d, stderr %q", rc, stderr.String())
	}

	var report struct {
		SchemaVersion string          `json:"schema_version"`
		RepoHead      string          `json:"repo_head"`
		Extensions    json.RawMessage `json:"extensions"`
		Files         []struct {
			Path           string `json:"path"`
			CouldNotScreen *bool  `json:"could_not_screen"`
			Reason         string `json:"reason"`
			Advisories     []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"advisories"`
		} `json:"files"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected one JSON document, got %q: %v", stdout.String(), err)
	}
	if report.SchemaVersion != "qualgen-check-v1" || report.RepoHead != head || len(report.Files) != 2 {
		t.Fatalf("unexpected screen envelope: %+v", report)
	}
	if string(report.Extensions) != "{}" {
		t.Errorf("extensions = %q, want an empty object", report.Extensions)
	}
	unmeasured, advised := report.Files[0], report.Files[1]
	if unmeasured.Path != unusualPath || unmeasured.CouldNotScreen == nil || !*unmeasured.CouldNotScreen ||
		unmeasured.Reason == "" || unmeasured.Advisories == nil || len(unmeasured.Advisories) != 0 {
		t.Fatalf("unexpected unmeasured row: %+v", unmeasured)
	}
	if advised.Path != "a.go" || advised.CouldNotScreen == nil || *advised.CouldNotScreen || advised.Reason != "" ||
		len(advised.Advisories) != 1 || advised.Advisories[0].Kind != "coupling-partner" ||
		!strings.Contains(advised.Advisories[0].Detail, "b.go") {
		t.Fatalf("unexpected advisory row: %+v", advised)
	}
	assertCheckOutputSchema(t, stdout.Bytes())
}

// These contract checks pin the subset of the published schema owned by the
// check command. They compare it with the real CLI document, including both
// reason states, without bringing a schema interpreter into the module.
func assertCheckOutputSchema(t *testing.T, output []byte) {
	t.Helper()
	raw, err := os.ReadFile("../schemas/qualgen-check-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		checkObjectSchema
		Defs map[string]checkObjectSchema `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	assertCheckObjectSchema(t, "result", schema.checkObjectSchema, result)
	if schema.Properties["schema_version"].Const != result["schema_version"] {
		t.Fatal("schema must pin the emitted core version")
	}
	files := result["files"].([]any)
	unmeasured, measured := files[0].(map[string]any), files[1].(map[string]any)
	assertCheckObjectSchema(t, "file", schema.Defs["file"], unmeasured, measured)
	assertCheckObjectSchema(t, "advisory", schema.Defs["advisory"], measured["advisories"].([]any)[0].(map[string]any))

	file := schema.Defs["file"]
	if schema.Properties["files"].Items.Ref != "#/$defs/file" ||
		file.Properties["advisories"].Items.Ref != "#/$defs/advisory" ||
		file.If.Properties["could_not_screen"].Const != true ||
		!slices.Equal(file.Then.Required, []string{"reason"}) ||
		!slices.Equal(file.Else.Not.Required, []string{"reason"}) {
		t.Fatal("schema must link the file and advisory shapes and condition reason on could_not_screen")
	}

	extensions := schema.Properties["extensions"]
	name := extensions.PropertyNames.Pattern
	ok, err := regexp.MatchString(name, "future-analysis")
	if err != nil || !ok || name == "" || extensions.AdditionalProperties.Ref != "#/$defs/extension" {
		t.Fatal("schema must admit a future optional extension by name")
	}
	extension := schema.Defs["extension"]
	assertCheckObjectSchema(t, "extension", extension, map[string]any{"version": 2, "data": map[string]any{}})
	version := extension.Properties["version"]
	if version.Minimum != 1 || version.Maximum != 2147483647 {
		t.Fatal("extension versions must be independently chosen positive integers")
	}
}

type checkSchemaProperty struct {
	Type    string `json:"type"`
	Const   any    `json:"const"`
	Pattern string `json:"pattern"`
	Minimum int    `json:"minimum"`
	Maximum int    `json:"maximum"`
	Items   struct {
		Ref string `json:"$ref"`
	} `json:"items"`
	PropertyNames struct {
		Pattern string `json:"pattern"`
	} `json:"propertyNames"`
	AdditionalProperties struct {
		Ref string `json:"$ref"`
	} `json:"additionalProperties"`
}

type checkObjectSchema struct {
	Type                 string                         `json:"type"`
	AdditionalProperties *bool                          `json:"additionalProperties"`
	Required             []string                       `json:"required"`
	Properties           map[string]checkSchemaProperty `json:"properties"`
	If                   struct {
		Properties map[string]checkSchemaProperty `json:"properties"`
	} `json:"if"`
	Then struct {
		Required []string `json:"required"`
	} `json:"then"`
	Else struct {
		Not struct {
			Required []string `json:"required"`
		} `json:"not"`
	} `json:"else"`
}

func assertCheckObjectSchema(t *testing.T, name string, schema checkObjectSchema, samples ...map[string]any) {
	t.Helper()
	if schema.Type != "object" || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatalf("%s must be a closed object in the schema", name)
	}
	seen := make(map[string]int)
	for _, sample := range samples {
		for _, key := range schema.Required {
			if _, ok := sample[key]; !ok {
				t.Fatalf("%s is missing required property %q", name, key)
			}
		}
		for key, value := range sample {
			property, declared := schema.Properties[key]
			if !declared {
				t.Fatalf("%s property %q is missing from the schema", name, key)
			}
			var wantType string
			switch value.(type) {
			case string:
				wantType = "string"
			case bool:
				wantType = "boolean"
			case []any:
				wantType = "array"
			case map[string]any:
				wantType = "object"
			case int:
				wantType = "integer"
			default:
				t.Fatalf("unexpected %s.%s value type %T", name, key, value)
			}
			if property.Const != nil {
				if property.Const != value {
					t.Fatalf("%s.%s does not match the schema constant", name, key)
				}
			} else if property.Type != wantType {
				t.Fatalf("%s.%s schema type = %q, want %q", name, key, property.Type, wantType)
			}
			seen[key]++
		}
	}
	if len(seen) != len(schema.Properties) {
		t.Fatalf("%s schema lists properties absent from all wire examples", name)
	}
	for key, count := range seen {
		if (count == len(samples)) != slices.Contains(schema.Required, key) {
			t.Fatalf("%s.%s requiredness does not match the wire examples", name, key)
		}
	}
}
