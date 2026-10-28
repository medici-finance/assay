package kubecompat

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func schemaKeys(t *testing.T, node map[string]any, typ reflect.Type) {
	t.Helper()
	properties := node["properties"].(map[string]any)
	want := []string{}
	for n := 0; n < typ.NumField(); n++ {
		want = append(want, typ.Field(n).Tag.Get("json"))
	}
	got := []string{}
	for key := range properties {
		got = append(got, key)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Fatal("schema/source member drift", want, got)
	}
	required := []string{}
	for _, v := range node["required"].([]any) {
		required = append(required, v.(string))
	}
	sort.Strings(required)
	if !reflect.DeepEqual(want, required) || node["additionalProperties"] != false {
		t.Fatal("closed/required schema drift")
	}
}

// regression: #4759
func TestKubeCompatSchema(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "containers", "k8s", "compatibility", "cellctl-config.schema.json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var schema map[string]any
	if e = json.Unmarshal(raw, &schema); e != nil {
		t.Fatal(e)
	}
	schemaKeys(t, schema, reflect.TypeOf(Profile{}))
	props := schema["properties"].(map[string]any)
	schemaKeys(t, props["custody"].(map[string]any), reflect.TypeOf(Custody{}))
	for _, role := range Roles {
		node := props["roles"].(map[string]any)["properties"].(map[string]any)[role].(map[string]any)
		schemaKeys(t, node, reflect.TypeOf(Role{}))
	}
}

// regression: #4759
func TestKubeCompatHashParity(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("could-not-check: Python unavailable for actual producer parity")
	}
	path, e := filepath.Abs(filepath.Join("..", "..", "..", "..", "containers", "k8s", "compatibility", "desk-session.py"))
	if e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"&<>.md", "é.md", "line\u2028separator.md"} {
		p := testProfile()
		role := p.Roles["worker-desk"]
		role.Instructions = "/work/" + suffix
		raw, _ := json.Marshal(role.LaunchDescriptor("worker-desk"))
		cmd := exec.Command(python, "-c", `import sys,importlib.util,json,hashlib;sys.dont_write_bytecode=True;s=importlib.util.spec_from_file_location('launcher',sys.argv[1]);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);print(hashlib.sha256(m.canonical(json.loads(sys.stdin.read()))).hexdigest())`, path)
		cmd.Stdin = bytes.NewReader(raw)
		out, e := cmd.CombinedOutput()
		if e != nil || strings.TrimSpace(string(out)) != role.LaunchDigest("worker-desk") {
			t.Fatal("actual Go/Python launch hash differs", e, string(out), role.LaunchDigest("worker-desk"))
		}
	}
}
