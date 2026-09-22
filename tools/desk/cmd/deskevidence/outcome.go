package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var outcomeGuardFn = guardVerifiedOutcomes
var closureCheckFn = checkVerifiedClosure

// Check only added records: historical successes must not prevent recording a
// later failure. Reuse the canonical brief parser and witness validator.
func guardVerifiedOutcomes(root, target string, before, after []byte, fg deskkit.Forge, repo deskkit.ForgeRepo, branch string) error {
	if !isVerifyOutcomesSidecar(target) {
		return nil
	}
	seen := map[string]bool{}
	retained := map[string]int{}
	for _, line := range bytes.Split(before, []byte("\n")) {
		retained[string(line)]++
	}
	for _, line := range bytes.Split(after, []byte("\n")) {
		if retained[string(line)] > 0 {
			retained[string(line)]--
			continue
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Brief   string `json:"brief"`
			Outcome string `json:"outcome"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return deskkit.Refused("invalid verify-outcomes JSON record")
		}
		if row.Outcome != "verified" || seen[row.Brief] {
			continue
		}
		seen[row.Brief] = true
		file, err := closureCheckFn(root, row.Brief)
		if err != nil {
			return err
		}
		// A locally staged closure is insufficient for this single-file write.
		// Both the board row and its evidence must already be on the target branch.
		for _, rel := range []string{file, path.Join(path.Dir(file), "README.md")} {
			// Archived brief files live under done/; their board is one level up.
			if path.Base(path.Dir(file)) == "done" && strings.HasSuffix(rel, "/README.md") {
				rel = path.Join(path.Dir(path.Dir(file)), "README.md")
			}
			local, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return deskkit.Unverifiable("cannot read closure source "+rel, err)
			}
			remote, err := fg.ReadFile(repo, deskkit.ReadFileInput{File: rel, Ref: branch})
			if err != nil {
				return err
			}
			if !remote.Exists || !bytes.Equal(local, remote.Content) {
				return deskkit.Refused("verified outcome requires closure already landed on target branch: " + rel + " differs; refresh --root after landing the status and Evidence")
			}
		}
	}
	if len(seen) > 0 {
		problems, err := statusgenLintFn(root)
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			return deskkit.Refused("verified outcome requires a lint-valid closure tree:\n" + strings.Join(problems, "\n"))
		}
	}
	return nil
}

func checkVerifiedClosure(root, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "statusgen", "brief", "--root", root, "--json", "--check-verified", key)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return "", deskkit.Refused("verified outcome refused: " + strings.TrimSpace(stderr.String()))
		}
		return "", deskkit.Unverifiable("cannot validate verified closure for "+key+": "+strings.TrimSpace(stderr.String()), err)
	}
	var info struct {
		Key  string `json:"key"`
		File string `json:"file"`
	}
	if err := json.Unmarshal(out, &info); err != nil || info.Key != key {
		return "", deskkit.Unverifiable("invalid statusgen brief response for "+key, err)
	}
	file := info.File
	if !strings.HasPrefix(file, "docs/streams/") || path.Clean(file) != file || strings.Contains(file, "\\") {
		return "", deskkit.Unverifiable(fmt.Sprintf("invalid closure source path %q", file), nil)
	}
	return file, nil
}
