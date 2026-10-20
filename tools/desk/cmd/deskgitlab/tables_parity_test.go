package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The provisioner's tables must not drift from the two other sources that carry them:
//   - tools/fleet-gitlab-roles.sh, the shared role table, PAT lifetime and naming helpers the
//     renewal script (tools/renew-fleet-gitlab-tokens.sh) still sources — a renewal that
//     disagrees with the provisioner on a PAT name silently mints a duplicate credential;
//   - cmd/deskfleet's label table, the cross-platform verb's copy (its parity test reads this
//     package's table the same way).
//
// Each is read as SOURCE from this tree. A copy of this module outside the repository (the
// mutation harness's per-worker copy) has no tools/ sources: that is could-not-check, and the
// shell checks skip saying so; in the repository, where CI runs them, a missing table fails.

func rolesShell(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "fleet-gitlab-roles.sh"))
	if os.IsNotExist(err) {
		t.Skip("could-not-check: this tree carries no tools/fleet-gitlab-roles.sh to compare against")
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoleTableMatchesShell(t *testing.T) {
	src := rolesShell(t)
	m := regexp.MustCompile(`(?ms)^ROLE_TABLE='\n(.*?)^'$`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no ROLE_TABLE literal in tools/fleet-gitlab-roles.sh")
	}
	var shell []string
	for _, ln := range strings.Split(m[1], "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			shell = append(shell, ln)
		}
	}
	var ours []string
	for _, r := range fleetRoles {
		ours = append(ours, r.Role+":"+r.AccessName+":"+r.AccessLevel+":"+r.Scopes)
	}
	if strings.Join(ours, "\n") != strings.Join(shell, "\n") {
		t.Errorf("fleetRoles drifted from tools/fleet-gitlab-roles.sh ROLE_TABLE:\n%s\nwant:\n%s",
			strings.Join(ours, "\n"), strings.Join(shell, "\n"))
	}
}

func TestPATDaysAndNamesMatchShell(t *testing.T) {
	src := rolesShell(t)
	m := regexp.MustCompile(`(?m)^FLEET_PAT_DAYS=(\d+)$`).FindStringSubmatch(src)
	if m == nil || m[1] != strconv.Itoa(defaultPATDays) {
		t.Errorf("defaultPATDays %d, tools/fleet-gitlab-roles.sh FLEET_PAT_DAYS %v", defaultPATDays, m)
	}
	// fleet_username / fleet_pat_name are one printf each; render them with the same inputs.
	render := func(fn string, args ...string) string {
		f := regexp.MustCompile(`(?m)^` + fn + `\(\) \{ printf '([^']*)'`).FindStringSubmatch(src)
		if f == nil {
			t.Fatalf("no %s printf in tools/fleet-gitlab-roles.sh", fn)
		}
		out := f[1]
		for _, a := range args {
			out = strings.Replace(out, "%s", a, 1)
		}
		return out
	}
	if got, want := fleetUsername("exorg", "worker"), render("fleet_username", "exorg", "worker"); got != want {
		t.Errorf("fleetUsername = %q, shell fleet_username = %q", got, want)
	}
	if got, want := fleetPATName("worker"), render("fleet_pat_name", "worker"); got != want {
		t.Errorf("fleetPATName = %q, shell fleet_pat_name = %q", got, want)
	}
}

func TestLabelTableMatchesDeskfleet(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "deskfleet", "tables.go"))
	if err != nil {
		t.Fatalf("reading cmd/deskfleet/tables.go: %v", err)
	}
	block := regexp.MustCompile(`(?s)var fleetLabels = \[\]fleetLabel\{\n(.*?)\n\}`).FindStringSubmatch(string(b))
	if block == nil {
		t.Fatal("no fleetLabels literal in cmd/deskfleet/tables.go")
	}
	row := regexp.MustCompile(`^\s*\{("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*")\},$`)
	var theirs []string
	for _, ln := range strings.Split(block[1], "\n") {
		if s := strings.TrimSpace(ln); s == "" || strings.HasPrefix(s, "//") {
			continue
		}
		m := row.FindStringSubmatch(ln)
		if m == nil {
			t.Fatalf("unparsed deskfleet fleetLabels row: %q", ln)
		}
		var f [3]string
		for i := range f {
			if f[i], err = strconv.Unquote(m[i+1]); err != nil {
				t.Fatalf("row %q: %v", ln, err)
			}
		}
		theirs = append(theirs, strings.Join(f[:], "|"))
	}
	var ours []string
	for _, l := range fleetLabels {
		ours = append(ours, l.Name+"|"+l.Color+"|"+l.Description)
	}
	if strings.Join(ours, "\n") != strings.Join(theirs, "\n") {
		t.Errorf("fleetLabels drifted from cmd/deskfleet's:\n%s\nwant:\n%s", strings.Join(ours, "\n"), strings.Join(theirs, "\n"))
	}
}
