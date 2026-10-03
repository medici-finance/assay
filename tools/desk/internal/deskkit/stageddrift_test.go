package deskkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type stagedDeclaration struct {
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

// Class guard for undeclared staged/live differences. Pending proposals are
// explicitly classified, not silently compared as if they claimed parity.
func stagedDrift(root string) error {
	dir := filepath.Join(root, "ci", "staged-workflows")
	b, err := os.ReadFile(filepath.Join(dir, "declared-changes.json"))
	if err != nil {
		return err
	}
	var declarations map[string]stagedDeclaration
	if err := json.Unmarshal(b, &declarations); err != nil {
		return err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, file := range files {
		name := file.Name()
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}
		seen[name] = true
		d, ok := declarations[name]
		if !ok {
			// A file-local classification lets parallel proposals declare pending
			// promotion without racing on the shared inventory.
			reason, err := os.ReadFile(filepath.Join(dir, name+".pending"))
			if err != nil || len(bytes.TrimSpace(reason)) == 0 {
				return fmt.Errorf("%s: no declared staged change", name)
			}
			d = stagedDeclaration{Mode: "pending", Reason: string(reason)}
		}
		staged, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var document yaml.Node
		if err := yaml.Unmarshal(staged, &document); err != nil {
			return fmt.Errorf("%s: invalid staged YAML: %w", name, err)
		}
		live, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		switch d.Mode {
		case "identical":
			if err != nil || !bytes.Equal(staged, live) {
				return fmt.Errorf("%s: undeclared drift from live", name)
			}
		case "pending":
			if strings.TrimSpace(d.Reason) == "" {
				return fmt.Errorf("%s: pending change has no classification", name)
			}
		default:
			return fmt.Errorf("%s: invalid declaration mode", name)
		}
	}
	for name := range declarations {
		if !seen[name] {
			return fmt.Errorf("%s: declaration without staged file", name)
		}
	}
	return nil
}

func TestStagedDrift(t *testing.T) {
	if err := stagedDrift(filepath.Join("..", "..", "..", "..")); err != nil {
		t.Fatal(err)
	}
}

func TestStagedDriftPlant(t *testing.T) {
	r := t.TempDir()
	stage := filepath.Join(r, "ci", "staged-workflows")
	live := filepath.Join(r, ".github", "workflows")
	for _, dir := range []string{stage, live} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, text string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(stage, "declared-changes.json"), `{"other.yml":{"mode":"identical"}}`)
	write(filepath.Join(stage, "other.yml"), "jobs: old\n")
	write(filepath.Join(live, "other.yml"), "jobs: new\n")
	if stagedDrift(r) == nil {
		t.Fatal("planted second workflow drift was accepted")
	}
	write(filepath.Join(stage, "other.yml"), "jobs: new\n")
	if err := stagedDrift(r); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(stage, "unlisted.yaml"), "jobs: omitted\n")
	if stagedDrift(r) == nil {
		t.Fatal("undeclared new staged workflow was accepted")
	}
	write(filepath.Join(stage, "unlisted.yaml.pending"), "Pending maintainer activation; new workflow.\n")
	if err := stagedDrift(r); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(stage, "unlisted.yaml"), "jobs: [\n")
	if stagedDrift(r) == nil {
		t.Fatal("malformed pending YAML was accepted")
	}
	write(filepath.Join(stage, "unlisted.yaml"), "jobs: omitted\n")
	write(filepath.Join(stage, "unlisted.yaml.pending"), "  \n")
	if stagedDrift(r) == nil {
		t.Fatal("empty pending classification was accepted")
	}
}
