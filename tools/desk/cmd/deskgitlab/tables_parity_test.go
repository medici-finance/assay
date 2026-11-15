package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/forgelabels"
)

// The provisioner's tables must not drift from the shell source that also carries them:
// tools/fleet-gitlab-roles.sh, the shared role table, PAT lifetime and naming helpers the
// renewal script (tools/renew-fleet-gitlab-tokens.sh) still sources — a renewal that
// disagrees with the provisioner on a PAT name silently mints a duplicate credential. (The
// label table is no longer a shell copy: this provisioner and cmd/deskfleet both build it
// from internal/forgelabels, and TestLabelTableMatchesDeskfleet holds them to one source.)
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

// TestPushTokenMatchesDeskfleet pins the board-push credential's name, variable, scope and
// role to deskfleet's: the two provisioners must mint the same credential class.
func TestPushTokenMatchesDeskfleet(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "deskfleet", "tables.go"))
	if err != nil {
		t.Fatalf("reading cmd/deskfleet/tables.go: %v", err)
	}
	ours := map[string]string{"pushTokenVariable": pushTokenVariable, "pushTokenName": pushTokenName,
		"pushTokenScope": pushTokenScope, "pushTokenAccessLevel": pushTokenAccessLevel}
	for name, want := range ours {
		m := regexp.MustCompile(`(?m)^\s*` + name + `\s*=\s*(\S+)\s*$`).FindStringSubmatch(string(b))
		if m == nil {
			t.Errorf("no %s constant in cmd/deskfleet/tables.go", name)
			continue
		}
		theirs := m[1]
		if s, err := strconv.Unquote(theirs); err == nil {
			theirs = s
		}
		if theirs != want {
			t.Errorf("%s = %q here, %q in deskfleet", name, want, theirs)
		}
	}
}

// TestLabelTableMatchesDeskfleet — this provisioner and cmd/deskfleet must create the same
// label set. Both now build it from internal/forgelabels, so the property is that cmd/deskfleet's
// fleetLabels declaration is this one's, or a literal whose rows equal this set. It reads
// cmd/deskfleet/tables.go as SOURCE (two main packages); the mirror test there reads this one.
func TestLabelTableMatchesDeskfleet(t *testing.T) {
	theirs, err := os.ReadFile(filepath.Join("..", "deskfleet", "tables.go"))
	if err != nil {
		t.Fatalf("reading cmd/deskfleet/tables.go: %v", err)
	}
	ours, err := os.ReadFile("tables.go")
	if err != nil {
		t.Fatalf("reading tables.go: %v", err)
	}
	decl := regexp.MustCompile(`(?m)^var fleetLabels = (.*)$`)
	o, th := decl.FindStringSubmatch(string(ours)), decl.FindStringSubmatch(string(theirs))
	if o == nil || th == nil {
		t.Fatalf("a fleetLabels declaration is missing (here: %v, deskfleet: %v)", o != nil, th != nil)
	}
	if strings.TrimSpace(o[1]) == strings.TrimSpace(th[1]) {
		return
	}
	block := regexp.MustCompile(`(?s)var fleetLabels = \[\]fleetLabel\{\n(.*?)\n\}`).FindStringSubmatch(string(theirs))
	if block == nil {
		t.Fatalf("deskfleet's fleetLabels is built from %s, this provisioner's from %s", strings.TrimSpace(th[1]), strings.TrimSpace(o[1]))
	}
	row := regexp.MustCompile(`^\s*\{("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*")\},$`)
	var got []string
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
		got = append(got, strings.Join(f[:], "|"))
	}
	var want []string
	for _, l := range fleetLabels {
		want = append(want, l.Name+"|"+l.Color+"|"+l.Description)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("deskfleet fleetLabels drifted:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// regression: #6127
// TestFleetLabelsCarryEveryStampDeskfileReaches — the defect class is a label table that is
// a hand copy instead of a projection of the role vocabulary deskfile stamps with: it lacked
// every `to:<role>` row, so each addressed filing went UNADDRESSED. Whatever fleetLabels is
// built from, it must carry, for every documented desk role, the raised-by and to labels
// spelled with deskkit's own prefixes (the ones deskfile applies).
func TestFleetLabelsCarryEveryStampDeskfileReaches(t *testing.T) {
	have := map[string]bool{}
	for _, l := range fleetLabels {
		have[l.Name] = true
	}
	for _, role := range forgelabels.DocumentedRoles() {
		for _, name := range []string{deskkit.RaisedByPrefix + role, deskkit.AddressedToPrefix + role} {
			if !have[name] {
				t.Errorf("fleetLabels lacks %q, which deskfile stamps for role %q", name, role)
			}
		}
	}
}
