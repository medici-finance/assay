package readercaps_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
)

// regression: #4531
func TestReaderCapsCompat(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name          string
		Advertisement json.RawMessage
		Requirements  json.RawMessage
		Observation   readercaps.ObservationState
		Want          readercaps.State
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			a, err := readercaps.DecodeAdvertisement(c.Advertisement)
			if err != nil {
				t.Fatal(err)
			}
			r, err := readercaps.DecodeRequirements(c.Requirements)
			if err != nil {
				t.Fatal(err)
			}
			if got := readercaps.Assess(r, readercaps.Observation{State: c.Observation, Advertisement: a}); got.State != c.Want {
				t.Fatalf("got %+v want %s", got, c.Want)
			}
		})
	}
	valid := `{"schema":"reader-requirements-v1","required":[]}`
	for _, bad := range []string{
		`{"schema":"reader-requirements-v2","required":[]}`,
		`{"schema":"reader-requirements-v1"}`,
		`{"schema":"reader-requirements-v1","required":null}`,
		`{"schema":"reader-requirements-v1","required":[],"required":[]}`,
		`{"schema":"reader-requirements-v1","required":[],"optional":["must-not-hide"]}`,
		valid + valid,
		`{"schema":"reader-requirements-v1","required":[{"operation":"syntax","schemas":[],"semantics":null}]}`,
		`{"schema":"reader-requirements-v1","required":[{"operation":"syntax","schemas":[],"semantics":[]},{"operation":"syntax","schemas":[],"semantics":[]}]}`,
	} {
		if _, err := readercaps.DecodeRequirements([]byte(bad)); err == nil {
			t.Errorf("accepted malformed requirements: %s", bad)
		}
	}
	legacy, _ := readercaps.DecodeRequirements([]byte(valid))
	if got := readercaps.Assess(legacy, readercaps.Observation{}); got.State != readercaps.Supported {
		t.Fatal(got)
	}
	need := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{{Operation: "dispatch", Schemas: []string{}, Semantics: []string{"milestone-acceptance-v1"}}}}
	for _, state := range []readercaps.ObservationState{"", readercaps.Missing, readercaps.Unreadable, readercaps.Malformed, readercaps.Stale, "future"} {
		if got := readercaps.Assess(need, readercaps.Observation{State: state}); got.State != readercaps.CouldNotCheck {
			t.Errorf("%s: %+v", state, got)
		}
	}
	if got := readercaps.Assess(need, readercaps.Observation{State: readercaps.Current}); got.State != readercaps.CouldNotCheck {
		t.Fatal(got)
	}
}

// regression: #4531
func TestReaderCapsBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range tree.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "bytes", "encoding/json", "fmt", "io", "strings":
			default:
				t.Errorf("pure readercaps imports %s in %s", name, file)
			}
		}
	}
	cmd := exec.Command("go", "list", "-deps", "-json", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p struct {
			ImportPath string
			Standard   bool
			Error      any
		}
		err := d.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if p.Error != nil || !p.Standard && p.ImportPath != "github.com/medici-finance/assay/statusgen/readercaps" || p.ImportPath == "net/http" || p.ImportPath == "os/exec" {
			t.Errorf("forbidden closure: %+v", p)
		}
	}
}
