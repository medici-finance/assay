package archtest

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3752
func TestWritePolicyBoundary(t *testing.T) {
	t.Run("production", func(t *testing.T) {
		p := deskcorePolicy(module, module+"/writepolicy")
		// This owner needs only lexical/value operations. In particular time,
		// io, fmt, reflection and filesystem-capable path/filepath are absent.
		p.AllowedStd = []string{"crypto/sha256", "encoding/hex", "encoding/json", "path", "regexp", "sort", "strings"}
		pkgs := goList(t, "..", "./writepolicy")
		for _, v := range Check(pkgs, p) {
			t.Error(v)
		}
		var dirs []string
		for _, pkg := range pkgs {
			if strings.HasPrefix(pkg.ImportPath, module+"/") {
				dirs = append(dirs, strings.TrimPrefix(pkg.ImportPath, module+"/"))
			}
		}
		rules := deskcoreRules()
		for i := range rules {
			rules[i].Allow = nil
		} // this closure needs no exceptions
		findings, err := ScanRules(os.DirFS(".."), dirs, rules)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range findings {
			t.Error(f)
		}
	})
	t.Run("production-mutations", func(t *testing.T) {
		for _, m := range []struct{ name, imp, call string }{
			{"filesystem-helper", "os", `os.ReadFile("fixture")`},
			{"clock", "time", `time.Now()`},
			{"command", "os/exec", `exec.Command("fixture")`},
		} {
			t.Run(m.name, func(t *testing.T) {
				dst := copyPolicyModule(t)
				helper := filepath.Join(dst, "effecthelper")
				if err := os.Mkdir(helper, 0o755); err != nil {
					t.Fatal(err)
				}
				body := "package effecthelper\nimport " + `"` + m.imp + `"` + "\nfunc Observe() { " + m.call + " }\n"
				if err := os.WriteFile(filepath.Join(helper, "effect.go"), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				body = "package writepolicy\nimport \"" + module + "/effecthelper\"\nfunc plantedEffect() { effecthelper.Observe() }\n"
				if err := os.WriteFile(filepath.Join(dst, "writepolicy", "planted.go"), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("go", "test", "-count=1", "-timeout=30s", "-run", "^TestWritePolicyBoundary/production$", "./archtest")
				cmd.Dir = dst
				cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
				out, err := cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(out), m.imp) || !strings.Contains(string(out), "FAIL: TestWritePolicyBoundary") {
					t.Fatalf("mutation was not caught by the production boundary: err=%v\n%s", err, out)
				}
				t.Logf("%s: production boundary refused planted import through helper", m.name)
			})
		}
	})
}

func copyPolicyModule(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("..", p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}
