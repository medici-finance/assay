package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #1800
func TestWorldRecheckRuntime(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-runtime", true: "other-runtime"}[changed], func(t *testing.T) {
			home, _ := world50Dispatch(t, attestVerifierDispatch)
			if changed {
				cmd := exec.Command("git", "-C", home, "rev-parse", "--path-format=absolute", "--git-path", deskkit.VerifierAttestationRecord)
				rawPath, err := cmd.Output()
				if err != nil {
					t.Fatal(err)
				}
				path := strings.TrimSpace(string(rawPath))
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var record map[string]any
				if err := json.Unmarshal(raw, &record); err != nil {
					t.Fatal(err)
				}
				world, ok := record["World"].(map[string]any)
				if !ok {
					t.Fatal("prepared receipt has no world")
				}
				inputs, ok := world["Inputs"].(map[string]any)
				if !ok {
					t.Fatalf("prepared world has no inputs: %+v", world)
				}
				if _, ok := inputs[deskkit.WorldRuntime]; !ok {
					t.Fatal("runtime was not bound")
				}
				inputs[deskkit.WorldRuntime] = "other-runtime"
				raw, err = json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := cmdVerifierAttestation("--check-verifier", []string{"--root", home, "--brief", "spec.md"})
			if changed {
				if err == nil || !strings.Contains(err.Error(), "environment changed since admission: runtime") {
					t.Fatalf("different runtime admitted: %v", err)
				}
			} else if err != nil {
				t.Fatalf("unchanged runtime refused: %v", err)
			}
		})
	}
}
