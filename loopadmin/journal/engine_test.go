package journal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// embeddedEngines are import-path fragments of embedded or file-backed
// database engines. Any package in loopadmin's closure whose import path holds
// one is refused: the journal has one runtime database, PostgreSQL.
var embeddedEngines = []string{
	"sqlite", // mattn, modernc, glebarez, ncruces, zombiezen, crawshaw, libsql
	"modernc.org/ql",
	"go.etcd.io/bbolt",
	"github.com/boltdb/bolt",
	"github.com/dgraph-io/badger",
	"github.com/syndtr/goleveldb",
	"github.com/cockroachdb/pebble",
	"duckdb",
	"github.com/tidwall/buntdb",
	"github.com/rqlite/",
	"github.com/hashicorp/go-memdb",
	"github.com/nutsdb/nutsdb",
	"github.com/xujiajun/nutsdb",
}

const workflowModule = "github.com/medici-finance/assay/workflow"

// engineViolation reports why an import path is refused, or "".
func engineViolation(importPath string) string {
	if importPath == workflowModule || strings.HasPrefix(importPath, workflowModule+"/") {
		return "the workflow module"
	}
	lower := strings.ToLower(importPath)
	for _, frag := range embeddedEngines {
		if strings.Contains(lower, frag) {
			return "an embedded database engine"
		}
	}
	return ""
}

// closureViolations lists every package in dir's build-and-test closure that
// engineViolation refuses.
func closureViolations(dir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-test", "-json=ImportPath", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOFLAGS=-mod=readonly")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list the package closure: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var bad []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p struct{ ImportPath string }
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("invalid package listing")
		}
		if why := engineViolation(p.ImportPath); why != "" {
			bad = append(bad, p.ImportPath+": "+why)
		}
	}
	return bad, nil
}

// TestNoEmbeddedJournalEngine covers Verify row 8: no loopadmin package, test
// or dependency imports a SQLite or other embedded database driver, or the
// workflow module. Planted modules prove the scan fires through the same
// go-list boundary.
// regression: #5492
func TestNoEmbeddedJournalEngine(t *testing.T) {
	bad, err := closureViolations("..")
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Fatalf("loopadmin's closure must hold no embedded engine or workflow import:\n%s", strings.Join(bad, "\n"))
	}
	for _, planted := range []string{"github.com/mattn/go-sqlite3", "modernc.org/sqlite", "go.etcd.io/bbolt", workflowModule + "/store"} {
		root := t.TempDir()
		module := planted
		if strings.HasPrefix(planted, workflowModule) {
			module = workflowModule
		}
		stub := filepath.Join(root, "stub")
		pkgDir := filepath.Join(stub, strings.TrimPrefix(strings.TrimPrefix(planted, module), "/"))
		site := filepath.Join(root, "site")
		for _, d := range []string{pkgDir, site} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		files := map[string]string{
			filepath.Join(stub, "go.mod"):     "module " + module + "\n\ngo 1.27.0\n",
			filepath.Join(pkgDir, "stub.go"):  "package stub\n",
			filepath.Join(site, "go.mod"):     "module example.test/site\n\ngo 1.27.0\n\nrequire " + module + " v0.0.0\n\nreplace " + module + " => ../stub\n",
			filepath.Join(site, "journal.go"): "package site\n\nimport _ \"" + planted + "\"\n",
		}
		for path, body := range files {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		bad, err := closureViolations(site)
		if err != nil {
			t.Fatalf("planted %s: %v", planted, err)
		}
		if len(bad) == 0 {
			t.Fatalf("planted %s passed the embedded-engine guard", planted)
		}
	}
	if engineViolation("github.com/jackc/pgx/v5/stdlib") != "" || engineViolation("github.com/pressly/goose/v3") != "" {
		t.Fatal("the PostgreSQL driver and the migration runner are admitted")
	}
}
