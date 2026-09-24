package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// materializeTxtar reads testdata/<name> — a minimal txtar-shaped fixture
// archive — and writes it out to a fresh t.TempDir(), returning that
// directory's path.
//
// Format: any text before the first "-- <path> --" marker line is a
// free-form comment and is discarded; each marker starts a new file whose
// content runs to the next marker (or EOF). This is a deliberately minimal
// reimplementation of golang.org/x/tools/txtar's marker syntax rather than a
// dependency on that module: the brief's "Fixture hazard" fact requires
// these fixtures to never be real, committed go.mod files (ci.yml's
// `git ls-files '*go.mod'` loop and this tool's own discovery would both
// pick one up), and a few lines of parsing avoid pulling in an extra module
// dependency — and the network fetch that would need — just to materialize
// them at test time.
func materializeTxtar(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture testdata/%s: %v", name, err)
	}
	dir := t.TempDir()

	var curPath string
	var curLines []string
	flush := func() {
		if curPath == "" {
			return
		}
		full := filepath.Join(dir, filepath.FromSlash(curPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for fixture file %s: %v", curPath, err)
		}
		content := strings.Join(curLines, "\n")
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("writing fixture file %s: %v", curPath, err)
		}
	}

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "-- ") && strings.HasSuffix(line, " --") {
			flush()
			curPath = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "-- "), " --"))
			curLines = nil
			continue
		}
		if curPath != "" {
			curLines = append(curLines, line)
		}
	}
	flush()

	return dir
}
