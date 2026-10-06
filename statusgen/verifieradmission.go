package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// statusgen is a separate module. Admission delegates to the installed shared
// reader, never duplicates forge custody or accepts a caller-supplied receipt.
func verifierAdmission(root, brief string) (string, error) {
	required := strings.Contains(strings.ToLower(os.Getenv("DESK_LOOP")), "verify")
	meta, e := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "assay-verifier-attestation.json").Output()
	if e == nil {
		if _, err := os.Stat(strings.TrimSpace(string(meta))); err == nil {
			required = true
		}
	}
	if !required {
		return "", nil
	}
	out, err := exec.Command("deskdispatch", "--check-verifier", "--root", root, "--brief", brief).Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return "", fmt.Errorf("pre-work verifier admission failed; no Verify rows executed: %w: %s", err, detail)
	}
	var receipt struct {
		Issue   int
		Binding struct{ Run, Repo, Source, Brief, Model, Tier string }
	}
	if err = json.Unmarshal(out, &receipt); err != nil {
		return "", fmt.Errorf("unreadable pre-work verifier receipt: %w", err)
	}
	b := receipt.Binding
	if receipt.Issue <= 0 || b.Run == "" || b.Source == "" || b.Model == "" || b.Tier == "" {
		return "", fmt.Errorf("incomplete pre-work verifier receipt")
	}
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", b.Repo, receipt.Issue, b.Run, b.Source, b.Brief, b.Model, b.Tier), nil
}
