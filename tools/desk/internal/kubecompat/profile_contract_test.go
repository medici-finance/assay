package kubecompat

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// regression: #4192
func TestKubeCompatCheckerReadOnly(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("could-not-check: Python required for checker regression")
	}
	root := t.TempDir()
	packageDir := filepath.Join(root, "containers", "k8s")
	for _, name := range []string{"check.py", "testdata/mutations.py"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "containers", "k8s", name))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(packageDir, name)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", root}, {"-C", root, "add", "containers/k8s"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("synthetic source repository: %v\n%s", err, out)
		}
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.WalkDir(packageDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			key, err := filepath.Rel(packageDir, path)
			if err != nil {
				return err
			}
			files[key] = "directory"
			if !entry.IsDir() {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[key] = string(raw)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	// Run the real source-self-test control flow, fixture imports and Git
	// inventory. Only the external scanner is replaced; no scanner qualification
	// is claimed here. Isolated Python ignores inherited PYTHON* settings but
	// deliberately permits bytecode writes so the regression can fail.
	const driver = `import runpy, sys
from pathlib import Path
checker = Path(sys.argv[1])
sys.path.insert(0, str(checker.parent))
namespace = runpy.run_path(str(checker))
calls = 0
def scanner(paths):
    global calls
    calls += 1
    if calls == 1:
        return len(paths)
    raise namespace['Invalid']('credential detected by synthetic scanner')
namespace['main'].__globals__['scan_sources'] = scanner
sys.argv = [str(checker), '--source-secrets', '--self-test']
namespace['main']()
assert calls == 3, 'source and both positive controls must execute'
`
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-I", "-c", driver, filepath.Join(packageDir, "check.py"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source self-test failed: %v\n%s", err, out)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		var added []string
		for path := range after {
			if _, found := before[path]; !found {
				added = append(added, path)
			}
		}
		sort.Strings(added)
		t.Fatalf("checker changed package tree; added paths: %v", added)
	}
}

// regression: #4772
func TestKubeCompatPreparation(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("could-not-check: Python required for cold-preparation flow")
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "containers", "k8s", "compatibility", "prepare.test.py"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, path)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "KUBECONFIG=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cold-preparation flow failed: %v\n%s", err, out)
	}
}

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
