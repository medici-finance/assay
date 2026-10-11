package forgeread

// deps_test.go — the dependency-closure half of the offline/frozen boundary: the root package's
// WHOLE closure (not just its own import lines) carries no HTTP client, no process launch and no
// subpackage of this module. TestRootImportsNothingOnline reads the root's own import lines; this
// asks the toolchain what actually gets linked, so a transitive route shows up too.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var onlineDep = regexp.MustCompile(`^(net/http|os/exec)$|(^|/)forgeread/`)

// rootDeps runs `go list -deps .` in dir with the workspace off.
func rootDeps(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

func offenders(deps []string) []string {
	var bad []string
	for _, d := range deps {
		if onlineDep.MatchString(d) {
			bad = append(bad, d)
		}
	}
	return bad
}

// regression: #2395
func TestRootDepsCarryNothingOnline(t *testing.T) {
	deps, err := rootDeps(t, ".")
	if err != nil {
		t.Fatalf("go list -deps . failed: %v", err)
	}
	if bad := offenders(deps); len(bad) != 0 {
		t.Errorf("the root package's closure reaches %v — it is the one package statusgen links", bad)
	}

	// Negative controls: a copy of the root with an online import planted must be caught, and a
	// directory with no module must fail rather than count zero.
	for name, plant := range map[string]string{
		"net/http":      "package forgeread\n\nimport _ \"net/http\"\n",
		"os/exec":       "package forgeread\n\nimport _ \"os/exec\"\n",
		"module subpkg": "package forgeread\n\nimport _ \"github.com/medici-finance/assay/forgeread/adapters\"\n",
	} {
		t.Run("planted "+name, func(t *testing.T) {
			tmp := t.TempDir()
			copyRoot(t, tmp)
			if err := os.MkdirAll(filepath.Join(tmp, "adapters"), 0o755); err != nil {
				t.Fatal(err)
			}
			for f, body := range map[string]string{
				"adapters/a.go": "package adapters\n",
				"zz_planted.go": plant,
			} {
				if err := os.WriteFile(filepath.Join(tmp, f), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			deps, err := rootDeps(t, tmp)
			if err != nil {
				t.Fatalf("go list on the planted copy: %v", err)
			}
			if len(offenders(deps)) == 0 {
				t.Fatalf("planted %s import was not seen — the check is blind", name)
			}
		})
	}
	if _, err := rootDeps(t, t.TempDir()); err == nil {
		t.Fatal("a directory with no module listed cleanly — a missing module must fail the row, not count zero")
	}
}

func copyRoot(t *testing.T, dst string) {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	files = append(files, "go.mod")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
