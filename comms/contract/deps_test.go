package contract

// deps_test.go — the dependency-closure half of the contract's boundary: the
// module's WHOLE closure (not just its own import lines) carries no Assay
// desk-tools package, no deskkit, no forge client, no credential/token package,
// no HTTP client and no process launch. A review of import lines misses a
// transitive route; this asks the toolchain what is actually linked.
//
// The rule is an allow-list in spirit: the closure may contain the standard
// library and this module's own packages and nothing else, minus a short list of
// standard-library packages that would put the network or a child process behind
// the contract.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const modulePath = "github.com/medici-finance/assay/comms"

// forbiddenStd names standard-library packages that must not be in the closure:
// the network client and server, process launch, and dynamic loading.
var forbiddenStd = regexp.MustCompile(`^(net|net/http|net/http/.+|net/rpc|os/exec|os/user|plugin|syscall/js)$`)

// forbiddenAssay names Assay packages that must never be reachable from the
// public contract, whatever module path they arrive under.
var forbiddenAssay = regexp.MustCompile(`(^|/)(tools/desk|deskkit|forge|forgeread|cred|credential|token|desktoken|deskpr|deskfile)(/|$)`)

// listed is the part of one `go list -json` record closure reads.
type listed struct {
	ImportPath string
	DepOnly    bool
	Error      *struct{ Err string }
}

// closure lists every package in the module's non-test closure with the
// workspace off. -e keeps an unresolvable import in the listing (as its import
// path) instead of aborting, so a planted route to a package that is not in this
// module's build list is still SEEN rather than counted as zero.
//
// Because -e also turns a pattern that matched nothing into a successful exit
// (outside any module, Go 1.27 prints the pseudo-package "./..." carrying a load
// error and exits 0), neither the exit status nor a non-empty listing proves the
// module was walked. The proof is positive instead: the packages the pattern
// matched (DepOnly false) must load without error, and at least one of them must
// be this module's own. Load errors on DEPENDENCIES are kept, because that is how
// a planted unresolvable import shows up.
func closure(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,DepOnly,Error", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var deps []string
	own := false
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listed
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		if !p.DepOnly {
			if p.Error != nil {
				return nil, fmt.Errorf("matched package %s failed to load: %s", p.ImportPath, p.Error.Err)
			}
			if p.ImportPath == modulePath || strings.HasPrefix(p.ImportPath, modulePath+"/") {
				own = true
			}
		}
		deps = append(deps, p.ImportPath)
	}
	if !own {
		// A walk that never reached this module's own packages is a failed
		// walk, never a clean one — whatever else it listed.
		return nil, errors.New("go list matched none of the module's own packages")
	}
	return deps, nil
}

// offenders classifies a closure. Anything with a dotted first path element is
// third-party or Assay; the only such path allowed is this module's own.
func offenders(deps []string) []string {
	var bad []string
	for _, d := range deps {
		first, _, _ := strings.Cut(d, "/")
		switch {
		case forbiddenStd.MatchString(d), forbiddenAssay.MatchString(d):
			bad = append(bad, d)
		case strings.Contains(first, ".") && d != modulePath && !strings.HasPrefix(d, modulePath+"/"):
			bad = append(bad, d)
		}
	}
	return bad
}

// regression: #5201
func TestContractDependencyBoundary(t *testing.T) {
	deps, err := closure(t, "..")
	if err != nil {
		t.Fatalf("go list -deps ./... failed: %v", err)
	}
	if bad := offenders(deps); len(bad) != 0 {
		t.Errorf("the comms module's closure reaches %v — the public contract must stay free of desk-tools, deskkit, forge, credential, network and process-launch packages", bad)
	}

	// Negative controls: a copy of the module with a forbidden import planted
	// must be caught, and a directory with no module must fail, not count zero.
	for name, plant := range map[string]string{
		"net/http":      "package contract\n\nimport _ \"net/http\"\n",
		"os/exec":       "package contract\n\nimport _ \"os/exec\"\n",
		"tools/desk":    "package contract\n\nimport _ \"github.com/medici-finance/assay/tools/desk/internal/comms\"\n",
		"deskkit":       "package contract\n\nimport _ \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\n",
		"forgeread":     "package contract\n\nimport _ \"github.com/medici-finance/assay/forgeread\"\n",
		"third party":   "package contract\n\nimport _ \"gopkg.in/yaml.v3\"\n",
		"network (net)": "package contract\n\nimport _ \"net\"\n",
	} {
		t.Run("planted "+name, func(t *testing.T) {
			tmp := t.TempDir()
			copyModule(t, tmp)
			if err := os.WriteFile(filepath.Join(tmp, "contract", "zz_planted.go"), []byte(plant), 0o644); err != nil {
				t.Fatal(err)
			}
			deps, err := closure(t, tmp)
			if err != nil {
				t.Fatalf("go list on the planted copy: %v", err)
			}
			if len(offenders(deps)) == 0 {
				t.Fatalf("planted %s import was not seen — the check is blind", name)
			}
		})
	}
	if _, err := closure(t, t.TempDir()); err == nil {
		t.Fatal("a directory with no module listed cleanly — a missing module must fail the row, not count zero")
	}
	// A different module that is itself clean must fail too: walking the wrong
	// module is not a walk of this one.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "go.mod"), []byte("module other\n\ngo 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "p", "p.go"), []byte("package p\n\nimport _ \"strings\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := closure(t, other); err == nil {
		t.Fatal("an unrelated module listed cleanly — a walk that misses this module must fail the row, not count zero")
	}
	// A module package that does not load hides its imports from the walk: an
	// unterminated import block drops the forbidden import it carries from the
	// listing, so the walk must fail rather than list clean.
	broken := t.TempDir()
	copyModule(t, broken)
	if err := os.WriteFile(filepath.Join(broken, "contract", "zz_broken.go"), []byte("package contract\n\nimport (\n\t_ \"net/http\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := closure(t, broken); err == nil {
		t.Fatal("a module package with a load error listed cleanly — an unloadable package must fail the row, not count zero")
	}
}

// copyModule copies the module's go.mod and every non-test Go file under it.
func copyModule(t *testing.T, dst string) {
	t.Helper()
	root := ".."
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if strings.HasSuffix(p, "_test.go") || strings.Contains(rel, "testdata") {
			return nil
		}
		if !strings.HasSuffix(p, ".go") && filepath.Base(p) != "go.mod" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
