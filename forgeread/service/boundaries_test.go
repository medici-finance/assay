package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type dependency struct {
	ImportPath string
	Standard   bool
}

func goCommand(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOENV=off", "KUBECONFIG=/dev/null")
	return cmd.CombinedOutput()
}
func dependencyClosure(t *testing.T, dir, pkg string) []dependency {
	t.Helper()
	b, err := goCommand(t, dir, "list", "-deps", "-json", pkg)
	if err != nil {
		t.Fatalf("dependency instrument failed: %v\n%s", err, b)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	var result []dependency
	for {
		var p dependency
		err := d.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, p)
	}
	if len(result) == 0 {
		t.Fatal("dependency instrument did not look")
	}
	return result
}
func allowedClosure(deps []dependency, root, pure bool) error {
	const module = "github.com/medici-finance/assay/forgeread"
	for _, d := range deps {
		if d.ImportPath == "os/exec" || d.ImportPath == "database/sql" || (pure && d.ImportPath == "net/http") {
			return fmt.Errorf("forbidden closure %s", d.ImportPath)
		}
		if d.Standard {
			continue
		}
		if root && d.ImportPath != module {
			return fmt.Errorf("root imported %s", d.ImportPath)
		}
		switch d.ImportPath {
		case module, module + "/mirror", module + "/service", module + "/client":
		default:
			return fmt.Errorf("unqualified SDK dependency %s", d.ImportPath)
		}
		if pure && d.ImportPath == module+"/client" {
			return fmt.Errorf("pure service imported client")
		}
	}
	return nil
}
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// regression: class service-import-boundary
func TestServicePackageBoundaries(t *testing.T) {
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(moduleRoot)
	for _, tc := range []struct {
		pkg        string
		root, pure bool
	}{{".", true, true}, {"./service", false, true}, {"./client", false, false}} {
		deps := dependencyClosure(t, moduleRoot, tc.pkg)
		if err := allowedClosure(deps, tc.root, tc.pure); err != nil {
			t.Fatal(err)
		}
		if tc.pkg == "./client" {
			found := false
			for _, d := range deps {
				found = found || d.ImportPath == "net/http"
			}
			if !found {
				t.Fatal("client closure did not inspect HTTP adapter")
			}
		}
	}
	mod, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(mod), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (f[0] == "require" || f[0] == "replace") {
			t.Fatal("stdlib-only module gained a dependency")
		}
	}
	checker := filepath.Join(t.TempDir(), "source-check")
	if b, err := goCommand(t, repoRoot, "build", "-o", checker, "tools/forge-import-check/check_env_imports.go"); err != nil {
		t.Fatalf("actual source checker did not compile: %v\n%s", err, b)
	}
	check := func(root, target string) (string, error) {
		cmd := exec.Command(checker, target)
		cmd.Dir = root
		b, err := cmd.CombinedOutput()
		return string(b), err
	}
	for _, target := range []string{"forgeread/service", "forgeread/client"} {
		if b, err := check(repoRoot, target); err != nil || strings.TrimSpace(b) != "0" {
			t.Fatalf("actual source target: %v %s", err, b)
		}
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/boundary\n\ngo 1.27.0\n")
			writeFile(t, filepath.Join(root, target, "entry.go"), "package fixture\nimport _ \"fixture.invalid/boundary/"+target+"/helper\"\n")
			// The forbidden source is transitive, compiler-valid and escaped. No
			// environment-read function is executed by this fixture.
			writeFile(t, filepath.Join(root, target, "helper", "helper.go"), `package helper
import _ "\x6f\x73"
`)
			if b, err := goCommand(t, root, "test", "-run", "^$", "-timeout=20s", "./"+target); err != nil {
				t.Fatalf("planted source is not compiler-valid: %v\n%s", err, b)
			}
			if b, err := check(root, target); err == nil || !strings.Contains(b, "forbidden import os") {
				t.Fatalf("actual checker missed transitive encoded import: %v %s", err, b)
			}
		})
	}
	if b, err := check(t.TempDir(), "forgeread/service"); err == nil || strings.TrimSpace(b) == "0" {
		t.Fatal("missing source became a clean closure")
	}
	t.Run("internal", func(t *testing.T) {
		root := t.TempDir()
		producer := filepath.Join(root, "producer")
		consumer := filepath.Join(root, "consumer")
		writeFile(t, filepath.Join(producer, "go.mod"), "module github.com/medici-finance/assay/tools/desk\n\ngo 1.27.0\n")
		writeFile(t, filepath.Join(producer, "internal", "reviewflow", "types.go"), "package reviewflow\ntype Observation struct{}\n")
		writeFile(t, filepath.Join(consumer, "go.mod"), "module fixture.invalid/consumer\n\ngo 1.27.0\n\nrequire github.com/medici-finance/assay/tools/desk v0.0.0\nreplace github.com/medici-finance/assay/tools/desk => "+strconv.Quote(filepath.ToSlash(producer))+"\n")
		writeFile(t, filepath.Join(consumer, "consumer.go"), "package consumer\nimport _ \"github.com/medici-finance/assay/tools/desk/internal/reviewflow\"\n")
		if b, err := goCommand(t, consumer, "test", "-run", "^$", "."); err == nil || !strings.Contains(string(b), "use of internal package") {
			t.Fatalf("Go internal boundary was not qualified: %v\n%s", err, b)
		}
	})
}
