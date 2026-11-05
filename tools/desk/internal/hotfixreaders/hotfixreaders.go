// Package hotfixreaders loads the shared hotfix-identity fixture matrix
// (internal/deskkit/testdata/hotfix-readers.json) that every Go tag reader's
// `Test*HotfixReaders` test consumes, so a reader taught a different grammar fails its own
// test. Imported only from _test.go files; no binary links it.
package hotfixreaders

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MatrixEnv overrides the fixture path; the negative control points it at a temp copy with
// one verdict flipped and watches the reader's test fail.
const MatrixEnv = "ASSAY_HOTFIX_READERS_MATRIX"

// Case is one tag and the verdict each reader must reach on it.
type Case struct {
	Tag      string            `json:"tag"`
	Verdicts map[string]string `json:"verdicts"`
}

// Matrix is the fixture: the pin every case is read against, the per-reader rationale, and
// the cases.
type Matrix struct {
	Schema  string            `json:"schema"`
	Pin     string            `json:"pin"`
	Readers map[string]string `json:"readers"`
	Cases   []Case            `json:"cases"`
}

// Path is the fixture path: MatrixEnv when set, else the committed file.
func Path() string {
	if p := os.Getenv(MatrixEnv); p != "" {
		return p
	}
	// Tests run in their package directory (and may be built -trimpath, so the source path
	// is not usable): walk up to the tools/desk module root.
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "internal", "deskkit", "testdata", "hotfix-readers.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Join("internal", "deskkit", "testdata", "hotfix-readers.json")
		}
		dir = parent
	}
}

// Load reads the matrix and checks that reader is listed and every case names its verdict.
func Load(reader string) (*Matrix, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	var m Matrix
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Schema != "hotfix-readers-v1" {
		return nil, fmt.Errorf("matrix schema %q", m.Schema)
	}
	if _, ok := m.Readers[reader]; !ok {
		return nil, fmt.Errorf("reader %q is not in the matrix inventory", reader)
	}
	for _, c := range m.Cases {
		if _, ok := c.Verdicts[reader]; !ok {
			return nil, fmt.Errorf("case %q has no verdict for reader %q", c.Tag, reader)
		}
	}
	return &m, nil
}
