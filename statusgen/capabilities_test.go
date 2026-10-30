package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
)

// regression: #4531
func TestStatusgenCapsTruth(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "statusgen")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	for _, form := range []string{"--version", "version"} {
		out, err := exec.Command(bin, form).Output()
		if err != nil || string(out) != statusgenVersion+"\n" {
			t.Fatalf("bare version changed: %q %v", out, err)
		}
		out, err = exec.Command(bin, form, "--json").Output()
		if err != nil {
			t.Fatal(err)
		}
		var v versionJSON
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatal(err)
		}
		if v.Version != statusgenVersion || !reflect.DeepEqual(v.ReaderCapabilities, statusgenReaderCaps()) {
			t.Fatalf("discovery mismatch: %s", out)
		}
		if err := v.ReaderCapabilities.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(v.ReaderCapabilities.Operations) != 2 || !reflect.DeepEqual(v.ReaderCapabilities.Operations[0].Schemas, []string{"brief-v1", "brief-v2", "brief-v3"}) {
			t.Fatalf("unsupported advertisement: %s", out)
		}
		for _, op := range []string{"assessment", "dispatch", "acceptance", "author"} {
			req := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{{Operation: op, Schemas: []string{}, Semantics: []string{"milestone-v1"}}}}
			if got := readercaps.Assess(req, readercaps.Observation{State: readercaps.Current, Advertisement: v.ReaderCapabilities}); got.State != readercaps.Unsupported {
				t.Fatalf("false %s support: %+v", op, got)
			}
		}
		req := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{{Operation: "syntax", Schemas: []string{"milestone-v1"}, Semantics: []string{}}}}
		if got := readercaps.Assess(req, readercaps.Observation{State: readercaps.Stale, Advertisement: v.ReaderCapabilities}); got.State != readercaps.CouldNotCheck {
			t.Fatal(got)
		}
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "sample")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("---\nstream: sample\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: dispatch, schemas: [], semantics: [milestone-v1]}]}\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	if code := refuseIfTreeTooNew([]string{root}, "dev", &diagnostic); code != 5 || !strings.Contains(diagnostic.String(), "unsupported required") {
		t.Fatalf("consumer admitted unsupported record: %d %s", code, &diagnostic)
	}
}

// regression: #4531
func TestReaderCapsSchema(t *testing.T) {
	raw, err := os.ReadFile("../schemas/reader-capabilities-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := parseSchema(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("readercaps/testdata/conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Advertisement map[string]any
		Requirements  map[string]any
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		for _, value := range []map[string]any{c.Advertisement, c.Requirements} {
			if problems := validateValue(schema, value, ""); len(problems) > 0 {
				t.Fatalf("fixture/schema mismatch: %v", problems)
			}
		}
	}
	for _, text := range []string{`{"schema":"reader-requirements-v1","required":null}`, `{"schema":"future-v1","required":[]}`, `{"schema":"reader-capabilities-v1","provider":"statusgen","operations":[]}`, `{"schema":"reader-requirements-v1","required":[],"unknown":true}`} {
		var value map[string]any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			t.Fatal(err)
		}
		if problems := validateValue(schema, value, ""); len(problems) == 0 {
			t.Errorf("schema admitted %s", text)
		}
	}
}
