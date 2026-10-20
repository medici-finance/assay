package perfrec

// deps_test.go — the dependency closure of the shared writer carries nothing online and
// nothing from the desk kit: no HTTP client, no process launch, no tools/desk package, and no
// module outside the standard library. Both the desk tools and statusgen link this package, so
// what it imports, they all inherit.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var forbiddenDep = regexp.MustCompile(`^(net|net/http|os/exec)$|^[^/]+\.[^/]+/`)

func depsOf(t *testing.T, dir string) ([]string, error) {
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

func forbidden(deps []string) []string {
	var bad []string
	for _, d := range deps {
		// perfrec itself is a module path (it has a dot-host first element).
		if d == "github.com/medici-finance/assay/perfrec" {
			continue
		}
		if forbiddenDep.MatchString(d) {
			bad = append(bad, d)
		}
	}
	return bad
}

func TestClosureCarriesNothingOnline(t *testing.T) {
	deps, err := depsOf(t, ".")
	if err != nil {
		t.Fatalf("go list -deps . failed: %v", err)
	}
	if bad := forbidden(deps); len(bad) != 0 {
		t.Errorf("perfrec's closure reaches %v — it must stay standard-library, offline", bad)
	}

	// Negative controls: a copy with a network or a process-launch import planted
	// must be caught.
	for name, plant := range map[string]string{
		"net/http": "package perfrec\n\nimport _ \"net/http\"\n",
		"os/exec":  "package perfrec\n\nimport _ \"os/exec\"\n",
	} {
		t.Run("planted "+name, func(t *testing.T) {
			tmp := t.TempDir()
			entries, _ := filepath.Glob("*.go")
			for _, f := range append(entries, "go.mod") {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(tmp, f), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(tmp, "zz_planted.go"), []byte(plant), 0o644); err != nil {
				t.Fatal(err)
			}
			deps, err := depsOf(t, tmp)
			if err != nil {
				t.Fatalf("go list on the planted copy: %v", err)
			}
			if len(forbidden(deps)) == 0 {
				t.Fatalf("planted %s import was not seen — the check is blind", name)
			}
		})
	}
}

func TestManifestIsStdlibOnly(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "require") || strings.HasPrefix(l, "replace") {
			t.Errorf("perfrec/go.mod carries %q — it takes no dependency", l)
		}
	}
}
