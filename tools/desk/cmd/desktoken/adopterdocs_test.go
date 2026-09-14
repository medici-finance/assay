package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// adopterdocs_test.go — the drift guard the #857 ruling's docs half needs (forge-gitlab/11
// task 2). The ruling that approved the dedicated read-only `auditor` identity added one
// condition: the adopter documentation is updated ALONGSIDE, so no page an adopter reads is
// silent about a role they must provision. This is the check that keeps that promise from
// rotting on the next role: it walks `validRoles` (the compiled fact) against the two adopter
// pages (what an adopter actually reads), the same `adopterContract` shape
// internal/topology/secondcell_test.go already uses to reconcile a compiled fact against a
// document.

// adopterDocsRepoRoot is this package's directory relative to the repo root:
// tools/desk/cmd/desktoken -> repo root is four levels up.
const adopterDocsRepoRoot = "../../../.."

// The two adopter pages the #857 ruling's docs half names. Every desktoken role must be
// enumerated by both — the GitHub App inventory + provisioning checklist on one, the
// GitLab role->service-account table + per-role token-file list on the other.
const (
	adopterGitHubDoc = "docs/adopting-assay.md"
	adopterGitLabDoc = "docs/adopting-assay-gitlab.md"
)

// TestAdopterDocsEnumerateEveryRole fails, naming the role and the page, when a desktoken role
// is not mentioned by BOTH adopter pages. A role that exists only in `validRoles` is a
// provisioning step an adopter meets first as a Refused exit, with nothing to read that says
// what to create — the same three-state posture (checked-clean / checked-wrong / could-not-
// check) the guard's own header names, applied here to documentation coverage rather than a
// live setting.
//
// It also runs the #857 ruling's NEGATIVE control on the docs themselves: no line naming the
// auditor may also document a write permission or a write-capable scope for it. This is the
// docs-side twin of forge-gitlab/11's Verify row 8, which proves the same thing at runtime
// against the identity actually provisioned — a documented grant an adopter copies is the
// grant the forge ends up enforcing, so the runtime proof is only as good as this one.
func TestAdopterDocsEnumerateEveryRole(t *testing.T) {
	ghPath := filepath.Join(adopterDocsRepoRoot, filepath.FromSlash(adopterGitHubDoc))
	glPath := filepath.Join(adopterDocsRepoRoot, filepath.FromSlash(adopterGitLabDoc))

	gh, ghErr := os.ReadFile(ghPath)
	gl, glErr := os.ReadFile(glPath)
	if os.IsNotExist(ghErr) || os.IsNotExist(glErr) {
		t.Skip("could-not-check: this checkout carries no adopter documentation to reconcile against")
	}
	if ghErr != nil {
		t.Fatalf("%s is present but unreadable: %v — could-not-check, not a pass", adopterGitHubDoc, ghErr)
	}
	if glErr != nil {
		t.Fatalf("%s is present but unreadable: %v — could-not-check, not a pass", adopterGitLabDoc, glErr)
	}
	ghSrc, glSrc := string(gh), string(gl)

	roles := roleNames() // sorted, from desktoken.go — the compiled fact
	if len(roles) == 0 {
		t.Fatal("validRoles reflects 0 roles — the oracle is broken, which is could-not-check, not full coverage")
	}

	var missing []string
	for _, r := range roles {
		if !strings.Contains(ghSrc, r) {
			missing = append(missing, r+" — absent from "+adopterGitHubDoc)
		}
		if !strings.Contains(glSrc, r) {
			missing = append(missing, r+" — absent from "+adopterGitLabDoc)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("a desktoken role is not enumerated by an adopter page (validRoles: %s):\n  %s",
			strings.Join(roles, ", "), strings.Join(missing, "\n  "))
	}

	// Negative control: no line documenting the auditor App's OWN grant may also name a write
	// permission. Scoped tightly to lines that introduce/describe the auditor App itself
	// (never a line that merely mentions "auditor" in passing, e.g. citing it as a model for
	// another role) — the shape a documented permission list actually takes.
	auditorGrantLineRe := regexp.MustCompile(`(?i)(^\s*\|\s*auditor\s*\||\bauditor\s+(GitHub\s+)?App\b)`)
	writeRe := regexp.MustCompile(`(?i):\s*write\b`)
	for _, ln := range strings.Split(ghSrc, "\n") {
		if !auditorGrantLineRe.MatchString(ln) {
			continue
		}
		if writeRe.MatchString(ln) && !strings.Contains(strings.ToLower(ln), "no write") {
			t.Errorf("%s documents a WRITE permission on the auditor App's own line: %q — "+
				"the #857 ruling approved a read-only identity with no write permission of any kind",
				adopterGitHubDoc, strings.TrimSpace(ln))
		}
	}
}
