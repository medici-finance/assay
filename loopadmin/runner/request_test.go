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
		strings.Repeat("-", 5) + "BEGIN " + "PRIVATE KEY" + strings.Repeat("-", 5),
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

// TestValidateRefusesBlankTool: encoding/json decodes a null list element to
// an empty string, so a request that reached Validate by any route that does
// not run DecodeRequest's null check must not carry a blank tool name at any
// position. The schema types each tool as a string, so a null that slipped
// through would make the module accept a request the schema rejects.
func TestValidateRefusesBlankTool(t *testing.T) {
	for _, file := range []string{"standing-desk-request.json", "workflow-stage-request.json"} {
		req, err := runner.DecodeRequest(fixture(t, file))
		if err != nil {
			t.Fatal(err)
		}
		req.Profile.Tools = []string{"read", "write", "exec"}
		if err := req.Validate(); err != nil {
			t.Fatalf("%s: positive control: %v", file, err)
		}
		for pos, i := range map[string]int{"first": 0, "middle": 1, "last": 2} {
			for _, blank := range []string{"", " ", "\t"} {
				r := req
				r.Profile.Tools = []string{"read", "write", "exec"}
				r.Profile.Tools[i] = blank
				if err := r.Validate(); !errors.Is(err, runner.ErrInvalidRequest) {
					t.Errorf("%s: blank tool %q at the %s position: got %v, want %v", file, blank, pos, err, runner.ErrInvalidRequest)
				}
			}
		}
	}
}

// TestCredentialFloorFalsePositives pins the floor's precision: text and keys
// that merely contain a slot word are not credentials. Each negative sits
// beside a positive control that differs only in what the negative lacks, so
// loosening the floor to pass the negatives cannot go unseen.
func TestCredentialFloorFalsePositives(t *testing.T) {
	longVal := "abcdefgh12"
	for _, s := range []string{
		"pallbearer=" + longVal,                  // no left boundary: "bearer" inside a word
		"pallbearer: " + longVal,                 //
		"a1auth=" + longVal,                      // "auth" inside a word
		"auth: disabled",                         // a setting, not a secret
		"auth=required",                          //
		`{"auth":"external"}`,                    //
		"oauth: enabled",                         //
		"http://git.example:8080?contact=a@b.io", // a port and an @ in the query
		"http://git.example:8080#frag@x",         // ... or in the fragment
		"http://git.example:8080/p?c=a@b.io",     // ... or after a path
		"ssh://git@git.example:22/r",             // a user and a port, no password
		"basic functionality only",
		"basic aGVsbG93b3JsZA==", // base64 of text with no user:password pair
		"Basic OnBhc3M=",         // base64 of ":pass": no user
		"author: " + longVal,
		"authority=" + longVal,
	} {
		if runner.LooksLikeCredential(s) {
			t.Errorf("must not flag %q", s)
		}
	}
	for _, s := range []string{
		"Basic dXNlcjpwYXNzd29yZA==", // base64 of user:password
		"auth=" + longVal,
		"auth: disabled,realsecret1234", // a setting word does not excuse what follows it
		"auth: " + longVal,
		`{"auth": "` + longVal + `"}`,
		"x-auth: " + longVal,
		"basic_auth=" + longVal,
		"oauth=" + longVal,
		"bearer=" + longVal,
		"privkey=" + longVal,
		"access_key=" + longVal,
		"aws_access_key_id=" + longVal,
		"api_key=" + longVal,
		"https://deploy:x9y8z7w6@git.example/r",
		"https://deploy:x9y8z7w6@git.example:8443/r",
		"https://deploy:x9y8z7w6@git.example",
	} {
		if !runner.LooksLikeCredential(s) {
			t.Errorf("must flag %q", s)
		}
	}
	for _, k := range []string{
		"auth_method", "auth_required", "oauth_scopes", "bearer_format",
		"access_key_rotation_days", "accessKeyRotationDays", "AuthMethod",
		"author", "authority", "auth-mode", "oauth_provider",
	} {
		if runner.CredentialKey(k) {
			t.Errorf("must not flag the key %q", k)
		}
	}
	for _, k := range []string{
		"auth", "x-auth", "basic_auth", "oauth", "bearer", "privkey",
		"access_key", "access_key_id", "accessKeyId", "AWS_ACCESS_KEY_ID",
		"accesskey", "auth_header", "Bearer", "x-bearer",
	} {
		if !runner.CredentialKey(k) {
			t.Errorf("must flag the key %q", k)
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
