package runner_test

import (
	"encoding/json"
	"errors"
	"fmt"
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

// TestCredentialFloorFalsePositives pins the floor's precision and its recall
// side by side: text and keys that merely contain a slot word are not
// credentials, and every spelling the floor refused before the precision fixes
// is still refused. Each negative sits beside a positive control that differs
// only in what the negative lacks, so loosening the floor to pass the negatives
// (SEC-5, B4) cannot go unseen.
func TestCredentialFloorFalsePositives(t *testing.T) {
	longVal := "abcdefgh12"
	for _, s := range []string{
		"pallbearer=" + longVal,                         // an English compound, not a slot
		"pallbearer: " + longVal,                        //
		"cupbearer=" + longVal,                          // ... nor is this one
		"auth: disabled",                                // a setting, not a secret
		"auth=required",                                 //
		`{"auth":"external"}`,                           //
		`{"auth":"disabled","note":"fine"}`,             // a setting followed by another field
		"oauth: enabled",                                //
		`{"note":"auth: disabled","k":"fine"}`,          // the string closes after the setting word
		"http://git.example:8080?contact=a@example.com", // a port and an @ in the query
		"http://git.example:8080#frag@x",                // ... or in the fragment
		"http://git.example:8080/p?c=a@example.com",     // ... or after a path
		"ssh://git@git.example:22/r",                    // a user and a port, no password
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
		// A slot word glued to a prefix is a slot, whatever the prefix.
		"userprivkey=" + longVal, "sshprivkey=" + longVal, "sshauth=" + longVal,
		"myauth=" + longVal, "proxyauth=" + longVal, "awsoauth=" + longVal,
		"awsaccesskey=" + longVal, "AWSAccessKeyId=" + longVal, "userbearer=" + longVal,
		"9bearer=" + longVal, "a1auth=" + longVal,
		// A setting word excuses only a value that is nothing else.
		`{"auth":"disabled","privkey":"` + longVal + `"}`,
		`{"auth":"disabled","bearer":"` + longVal + `"}`,
		`{"auth":"disabled","oauth":"` + longVal + `"}`,
		`{"auth":"disabled","access_key":"` + longVal + `"}`,
		`{"auth":"disabled","accessKeyId":"` + longVal + `"}`,
		`auth="disabled",privkey="` + longVal + `"`,
		`auth=disabled,privkey=` + longVal,
		`privkey:"disabled"` + longVal,
		`auth=disabled"` + longVal,
		"auth: disabled" + longVal,
		// A URL password may carry a question mark or a hash.
		"https://user:abc#def123@git.example",
		"https://user:pa?ss12@git.example",
		"https://deploy:x9y8z7w6#frag@git.example/r",
	} {
		if !runner.LooksLikeCredential(s) {
			t.Errorf("must flag %q", s)
		}
	}
	for _, k := range []string{
		"auth_method", "auth_required", "oauth_scopes", "bearer_format",
		"access_key_rotation_days", "accessKeyRotationDays", "AuthMethod",
		"author", "authority", "auth-mode", "oauth_provider", "pallbearer",
		"AWSAccessKeyRotationDays", "privKeyVersion", "bearer_type",
	} {
		if runner.CredentialKey(k) {
			t.Errorf("must not flag the key %q", k)
		}
	}
	for _, k := range []string{
		"auth", "x-auth", "basic_auth", "oauth", "bearer", "privkey",
		"access_key", "access_key_id", "accessKeyId", "AWS_ACCESS_KEY_ID",
		"accesskey", "auth_header", "Bearer", "x-bearer",
		// camelCase, acronym runs and run-together spellings of a slot.
		"privKey", "PrivKey", "sshPrivKey", "userPrivKey", "sshprivkey",
		"privkeypem", "privKeyPem", "priv_key", "AWSAccessKeyId", "AWSAccessKey",
		"awsaccesskey", "myaccesskey", "userbearer", "accessKey", "AccessKeyId",
		"authKey", "AWSAuthKey", "xAuth", "pallbearerx", "pall_bearer",
	} {
		if !runner.CredentialKey(k) {
			t.Errorf("must flag the key %q", k)
		}
	}
}

// TestCredentialFloorVocabulary walks each word the floor's lists name, one at
// a time, so removing or misspelling any single entry is seen: every key word,
// every qualifier that lets a slot key describe rather than hold, every setting
// word that lets a slot value state rather than hold, every excused compound,
// and the length and boundary cases of the value forms.
func TestCredentialFloorVocabulary(t *testing.T) {
	longVal := "abcdefgh12"
	for _, w := range []string{
		"token", "secret", "password", "passwd", "passphrase", "credential",
		"apikey", "api_key", "api-key", "private_key", "privatekey", "private-key",
		"authorization", "cookie", "session",
	} {
		if !runner.CredentialKey(strings.ToUpper("x_"+w+"_y")) || !runner.CredentialKey("x_"+w+"_y") {
			t.Errorf("a key containing %q must be flagged", w)
		}
		if runner.CredentialKey("x_" + w[:len(w)-1]) {
			t.Errorf("a key containing only %q must not be flagged", w[:len(w)-1])
		}
	}
	for _, w := range []string{
		"method", "methods", "required", "scope", "scopes", "format", "type", "mode",
		"enabled", "rotation", "days", "ttl", "expiry", "timeout", "provider", "realm",
		"version", "policy",
	} {
		for _, slot := range []string{"auth", "oauth", "bearer", "privkey", "access_key"} {
			if runner.CredentialKey(slot + "_" + w) {
				t.Errorf("the key %q only describes a slot", slot+"_"+w)
			}
			if !runner.CredentialKey(slot + "_" + w + "_header") {
				t.Errorf("the key %q holds a slot", slot+"_"+w+"_header")
			}
		}
	}
	for _, k := range []string{"auth_method_header", "pallbearer_bearer", "pallbear_er", "pallprivkey", "ringaccesskey", "AUTH", "X-AUTH", "v2Auth", "oauth_header", "privkey_id", "accesskeyid"} {
		if !runner.CredentialKey(k) {
			t.Errorf("must flag the key %q", k)
		}
	}
	for _, k := range []string{"oauth2", "oauth2_provider", "authz", "author_name", "access", "key"} {
		if runner.CredentialKey(k) {
			t.Errorf("must not flag the key %q", k)
		}
	}
	for _, w := range []string{"disabled", "required", "optional", "external", "internal", "enabled", "inherit"} {
		for _, text := range []string{
			`{"auth":"` + w + `"}`, `{'auth':'` + w + `'}`, "auth='" + w + "'", "auth=" + strings.ToUpper(w),
			`x='auth: ` + w + `', y`, "(auth: " + w + ")", "auth: " + w + ".", "auth: " + w + " for local runs",
		} {
			if runner.LooksLikeCredential(text) {
				t.Errorf("must not flag the setting %q", text)
			}
		}
		for _, text := range []string{
			"auth=" + w + "x9y8z7w6", "auth=" + w + ",x9y8z7w6", `auth="` + w + `',x9y8z7w6`,
			`auth="` + w, `auth="` + w + `"x9y8z7w6`, "auth=" + w + `"x9y8z7w6`,
		} {
			if !runner.LooksLikeCredential(text) {
				t.Errorf("must flag %q", text)
			}
		}
	}
	for _, w := range []string{"pall", "cup", "standard", "torch", "flag", "sword", "ring"} {
		for _, text := range []string{w + "bearer=" + longVal, strings.Title(w) + "bearer: " + longVal, strings.ToUpper(w) + "BEARER=" + longVal} {
			if runner.LooksLikeCredential(text) {
				t.Errorf("must not flag the compound %q", text)
			}
		}
		if runner.CredentialKey(w + "bearer") {
			t.Errorf("the key %qbearer is a compound word", w)
		}
		if !runner.LooksLikeCredential(w + "auth=" + longVal) {
			t.Errorf("the compound excuse is for bearer only: %qauth", w)
		}
	}
	for _, w := range []string{"api_key", "secret", "password", "passwd", "token", "authorization", "cookie"} {
		if !runner.LooksLikeCredential(w + "=" + longVal) {
			t.Errorf("must flag %s=value", w)
		}
		if !runner.LooksLikeCredential(w + "=abcdefgh") {
			t.Errorf("a value of exactly eight characters is a secret: %s", w)
		}
		if runner.LooksLikeCredential(w + "=short") {
			t.Errorf("a value of five characters is not a secret: %s=short", w)
		}
	}
	for _, w := range []string{"auth", "bearer", "privkey", "access_key", "access-key", "accesskeyid", "access_key_id"} {
		for _, text := range []string{w + "=abcdefgh", w + `":"abcdefgh`, w + " : abcdefgh"} {
			if !runner.LooksLikeCredential(text) {
				t.Errorf("a value of exactly eight characters is a secret: %q", text)
			}
		}
		if runner.LooksLikeCredential(w + "=abcdefg") {
			t.Errorf("a value of seven characters is not: %s", w)
		}
		if runner.LooksLikeCredential(w+"= abcdefghij") && w == "never" {
			t.Fatal("unreachable")
		}
		if !runner.LooksLikeCredential("WITH " + strings.ToUpper(w) + "=" + longVal) {
			t.Errorf("the match ignores case: %s", w)
		}
		// The value is the run up to the next space, so prose after it is not part of it.
		if runner.LooksLikeCredential(w + "=short and then a longer sentence") {
			t.Errorf("a short value followed by prose is short: %s", w)
		}
	}
	for _, u := range []struct {
		url  string
		cred bool
	}{
		{"https://user:?abcd1234@git.example", true},
		{"https://user:12-34#x@git.example", true},
		{"https://user:1234abcd?x@git.example", true},
		{"HTTPS://deploy:x9y8z7w6@git.example", true},
		{"https://user:12345?q@git.example", false},
		{"https://user:12345#q@git.example", false},
		{"https://host?next=a:b@c.io", false},
		{"https://host#a:b@c.io", false},
		{"https://git.example:8080/path/a@example.com", false},
		{"see https://host:80 then mail a@example.com", false},
		{"https://user:@git.example", false},
	} {
		if got := runner.LooksLikeCredential(u.url); got != u.cred {
			t.Errorf("%q: flagged %v, want %v", u.url, got, u.cred)
		}
	}
}

// TestCredentialSlotsInRequests carries the slot spellings through the request
// itself: as an extension key at the top level, nested, and written with a
// JSON unicode escape, and as text in the string fields a packet carries.
func TestCredentialSlotsInRequests(t *testing.T) {
	base, err := runner.DecodeRequest(fixture(t, "standing-desk-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	refused := func(edit func(*runner.LaunchRequest)) bool {
		r := base
		r.Extensions = nil
		edit(&r)
		return errors.Is(r.Validate(), runner.ErrCredential)
	}
	if refused(func(r *runner.LaunchRequest) {}) {
		t.Fatal("positive control: the fixture request must pass the floor")
	}
	hex := strings.Repeat("ab", 32)
	for _, key := range []string{
		"privKey", "PrivKey", "sshPrivKey", "AWSAccessKeyId", "awsaccesskey",
		"myaccesskey", "userbearer", "accessKeyId", "AWSAuthKey",
	} {
		esc := fmt.Sprintf(`\u%04x`, key[0]) + key[1:] // the first letter written as a JSON escape
		for name, ext := range map[string]json.RawMessage{
			"top level": json.RawMessage(`"` + hex + `"`),
			"nested":    json.RawMessage(`{"cfg":{"` + key + `":"` + hex + `"}}`),
			"escaped":   json.RawMessage(`{"` + esc + `":"x"}`),
		} {
			k := key
			if name != "top level" {
				k = "cfg"
			}
			if !refused(func(r *runner.LaunchRequest) { r.Extensions = map[string]json.RawMessage{k: ext} }) {
				t.Errorf("an extension key %q (%s) must be refused", key, name)
			}
		}
	}
	for _, key := range []string{"auth_method", "oauth_scopes", "bearer_format", "access_key_rotation_days", "author", "pallbearer"} {
		ext := map[string]json.RawMessage{"cfg": json.RawMessage(`{"` + key + `":"x"}`), key: json.RawMessage(`"x"`)}
		if refused(func(r *runner.LaunchRequest) { r.Extensions = ext }) {
			t.Errorf("an extension key %q only describes a slot and must pass", key)
		}
	}
	for _, text := range []string{
		"userprivkey=abcdefgh12", "AWSAccessKeyId=abcdefgh12", "myauth=abcdefgh12",
		`{"auth":"disabled","privkey":"abcdefgh12"}`, `privkey:"disabled"abcdefgh12`,
		"https://user:abc#def123@git.example",
	} {
		for name, edit := range map[string]func(*runner.LaunchRequest){
			"packet ref": func(r *runner.LaunchRequest) { r.Packet.Ref = text },
			"workspace":  func(r *runner.LaunchRequest) { r.Workspace = text },
			"ext string": func(r *runner.LaunchRequest) {
				b, _ := json.Marshal(text)
				r.Extensions = map[string]json.RawMessage{"cfg": b}
			},
		} {
			if !refused(edit) {
				t.Errorf("the text %q in the %s must be refused", text, name)
			}
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

// TestSchemaRefusesBlankTool: the schema and Validate agree on a blank tool
// name. The schema types each tool as a non-empty string, so the empty string
// that decoding a null list element would otherwise produce is outside it, and
// Validate refuses the empty string (and, being stricter, whitespace) too.
func TestSchemaRefusesBlankTool(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schemas", "loop-admin-runner-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Props struct {
			Profile struct {
				Props struct {
					Tools struct {
						Items struct {
							Type      string `json:"type"`
							MinLength *int   `json:"minLength"`
						} `json:"items"`
					} `json:"tools"`
				} `json:"properties"`
			} `json:"profile"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	items := s.Props.Profile.Props.Tools.Items
	if items.Type != "string" || items.MinLength == nil || *items.MinLength != 1 {
		t.Fatalf("profile.tools.items must be a string with minLength 1, got %+v", items)
	}
	req, err := runner.DecodeRequest(fixture(t, "standing-desk-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	req.Profile.Tools = []string{"read"}
	if err := req.Validate(); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	req.Profile.Tools = []string{"read", ""}
	if err := req.Validate(); !errors.Is(err, runner.ErrInvalidRequest) {
		t.Fatalf("an empty tool name is outside the schema and must be refused, got %v", err)
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
