package runner_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "runner", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixtureRequests(t *testing.T) {
	for _, tc := range []struct {
		file     string
		decode   error
		validate error
	}{
		{"standing-desk-request.json", nil, nil},
		{"workflow-stage-request.json", nil, nil},
		{"workflow-stage-missing-refs.json", nil, runner.ErrModeFields},
		{"unknown-field-request.json", runner.ErrInvalidRequest, nil},
		{"future-version-request.json", nil, runner.ErrUnsupportedVersion},
	} {
		t.Run(tc.file, func(t *testing.T) {
			req, err := runner.DecodeRequest(fixture(t, tc.file))
			if !errors.Is(err, tc.decode) || (tc.decode == nil && err != nil) {
				t.Fatalf("decode: got %v, want %v", err, tc.decode)
			}
			if err != nil {
				return
			}
			err = req.Validate()
			if err == nil {
				err = req.CheckCapabilities(allCaps())
			}
			if tc.validate == nil && err != nil || tc.validate != nil && !errors.Is(err, tc.validate) {
				t.Fatalf("validate: got %v, want %v", err, tc.validate)
			}
		})
	}
}

func allCaps() runner.Capabilities {
	return runner.Capabilities{
		Resume: true, CancelAck: true, Snapshot: true, LaunchDedupe: true,
		BudgetScopes: []runner.BudgetScope{runner.BudgetLaunch, runner.BudgetRequest},
		Telemetry:    runner.TelemetryComplete,
	}
}

func TestZeroCapabilitiesDeclareNothing(t *testing.T) {
	var c runner.Capabilities
	for _, cap := range []runner.Capability{runner.CapResume, runner.CapCancelAck, runner.CapSnapshot, runner.CapLaunchDedupe, runner.CapTelemetry} {
		if c.Has(cap) {
			t.Errorf("zero capabilities must not declare %q", cap)
		}
	}
}

func TestUsageUnknownStaysUnknown(t *testing.T) {
	a := runner.Usage{InputTokens: runner.Int64(4)}
	b := runner.Usage{InputTokens: runner.Int64(6), CostMicros: runner.Int64(0)}
	sum := a.Add(b)
	if sum.InputTokens == nil || *sum.InputTokens != 10 {
		t.Fatalf("measured figures sum, got %+v", sum)
	}
	if sum.CostMicros != nil || sum.OutputTokens != nil {
		t.Fatalf("a figure missing on either side stays unknown, got %+v", sum)
	}
	if sum.Complete() {
		t.Fatal("a usage with unknown figures is not complete")
	}
	if !(runner.Usage{CostMicros: runner.Int64(0), InputTokens: runner.Int64(0), OutputTokens: runner.Int64(0)}).Complete() {
		t.Fatal("a measured zero is a real reading")
	}
}

func TestCredentialShapes(t *testing.T) {
	yes := []string{
		"gh" + "p_" + strings.Repeat("a1", 20),
		"-----BEGIN " + "PRIVATE KEY-----",
		"AKIA" + strings.Repeat("A", 16),
		"Bearer " + strings.Repeat("a", 30),
	}
	for _, s := range yes {
		if !runner.LooksLikeCredential(s) {
			t.Errorf("must flag a credential shape: %.8s...", s)
		}
	}
	for _, s := range []string{"", "packets/worker-v1", "claim/desk-1", "the model-a profile"} {
		if runner.LooksLikeCredential(s) {
			t.Errorf("must not flag %q", s)
		}
	}
	for _, k := range []string{"api_key", "token", "Password", "secret"} {
		if !runner.CredentialKey(k) {
			t.Errorf("must flag credential key %q", k)
		}
	}
}

func TestOptionalExtensionIsIgnoredMandatoryIsRefused(t *testing.T) {
	req, err := runner.DecodeRequest(fixture(t, "standing-desk-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	req.Extensions = map[string]json.RawMessage{"hint": json.RawMessage(`"x"`)}
	if err := req.CheckCapabilities(allCaps()); err != nil {
		t.Fatalf("an optional extension must be ignored: %v", err)
	}
	req.Require = []string{"ext:hint"}
	if err := req.CheckCapabilities(allCaps()); !errors.Is(err, runner.ErrUnsupportedMandatory) {
		t.Fatalf("a mandatory extension the adapter does not declare must be refused: %v", err)
	}
	c := allCaps()
	c.Extensions = []string{"hint"}
	if err := req.CheckCapabilities(c); err != nil {
		t.Fatalf("a declared extension satisfies the requirement: %v", err)
	}
}

// TestSchemaMatchesGoTags keeps schemas/loop-admin-runner-v1.json and the Go
// wire types from drifting: every property the schema names for the request,
// result, observation, usage and capabilities is a json tag of the matching Go
// type, and the reverse.
func TestSchemaMatchesGoTags(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "loop-admin-runner-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	props := func(n map[string]any) map[string]any {
		p, _ := n["properties"].(map[string]any)
		return p
	}
	def := func(name string) map[string]any {
		return s["$defs"].(map[string]any)[name].(map[string]any)
	}
	type pair struct {
		name   string
		schema map[string]any
		typ    reflect.Type
	}
	root := props(s)
	pairs := []pair{
		{"request", s, reflect.TypeOf(runner.LaunchRequest{})},
		{"request.desk", root["desk"].(map[string]any), reflect.TypeOf(runner.DeskRef{})},
		{"request.work", root["work"].(map[string]any), reflect.TypeOf(runner.WorkRef{})},
		{"request.packet", root["packet"].(map[string]any), reflect.TypeOf(runner.RolePacket{})},
		{"request.profile", root["profile"].(map[string]any), reflect.TypeOf(runner.Profile{})},
		{"request.authority", root["authority"].(map[string]any), reflect.TypeOf(runner.Authority{})},
		{"request.budget", root["budget"].(map[string]any), reflect.TypeOf(runner.Reservation{})},
		{"request.resume", root["resume"].(map[string]any), reflect.TypeOf(runner.SessionRef{})},
		{"usage", def("usage"), reflect.TypeOf(runner.Usage{})},
		{"artifact", def("artifact"), reflect.TypeOf(runner.Artifact{})},
		{"toolRequest", def("toolRequest"), reflect.TypeOf(runner.ToolRequest{})},
		{"result", def("result"), reflect.TypeOf(runner.Result{})},
		{"observation", def("observation"), reflect.TypeOf(runner.Observation{})},
		{"capabilities", def("capabilities"), reflect.TypeOf(runner.Capabilities{})},
	}
	for _, p := range pairs {
		var want, got []string
		for k := range props(p.schema) {
			want = append(want, k)
		}
		for i := 0; i < p.typ.NumField(); i++ {
			tag, _, _ := strings.Cut(p.typ.Field(i).Tag.Get("json"), ",")
			if tag != "" && tag != "-" {
				got = append(got, tag)
			}
		}
		sort.Strings(want)
		sort.Strings(got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: schema properties %v != Go json tags %v", p.name, want, got)
		}
	}
}

// TestNoGraphDependency: the module builds without the workflow module, the
// instance store or any other module.
func TestNoGraphDependency(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) > 0 && (f[0] == "require" || f[0] == "replace") {
			t.Errorf("loopadmin must stay dependency-free, go.mod has %q", line)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "go.work")); err == nil {
		t.Error("the repository root must carry no go.work")
	}
}
