package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The bash provisioner stays the Unix path, so its tables and this verb's must not drift.
// These tests read the tables out of the shell sources in this tree — not out of a copy in
// this file — so an edit to either side that the other does not repeat fails here.
//
// The role table may live in tools/create-fleet-gitlab.sh (today) or in the shared
// tools/fleet-gitlab-roles.sh the script sources once that file lands. Every source that
// carries a literal table is checked; at least one must.
//
// A copy of this module outside the repository (the mutation harness's per-worker copy) has
// no tools/ sources: that is could-not-check, and the tests skip saying so. In the repository,
// where CI runs them, a missing table is a failure, never a skip.
var bashRoleSources = []string{"fleet-gitlab-roles.sh", "create-fleet-gitlab.sh"}

// toolsDir is the repository's tools/ directory, from this package (tools/desk/cmd/deskfleet).
func toolsDir() string { return filepath.Join("..", "..", "..") }

// bashTable returns the lines of the single-quoted shell assignment NAME='…' in src, or
// ok=false when src assigns no literal to NAME (for example, when it sources the table).
func bashTable(t *testing.T, src, name string) (rows []string, ok bool) {
	t.Helper()
	re := regexp.MustCompile(`(?ms)^` + name + `='\n(.*?)^'$`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	// The shell's '"'"' idiom writes one literal apostrophe inside the quoted table.
	body := strings.ReplaceAll(m[1], `'"'"'`, `'`)
	for _, ln := range strings.Split(body, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			rows = append(rows, ln)
		}
	}
	return rows, true
}

func readTool(t *testing.T, name string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(toolsDir(), name))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatalf("reading tools/%s: %v", name, err)
	}
	return string(b), true
}

// requireBashSources skips, as could-not-check, when this tree carries no bash provisioner.
func requireBashSources(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(toolsDir(), "create-fleet-gitlab.sh")); os.IsNotExist(err) {
		t.Skip("could-not-check: this tree carries no tools/create-fleet-gitlab.sh to compare against")
	}
}

func TestRoleTableMatchesBash(t *testing.T) {
	requireBashSources(t)
	var ours []string
	for _, r := range fleetRoles {
		ours = append(ours, r.Role+":"+r.AccessName+":"+strconv.Itoa(r.AccessLevel)+":"+strings.Join(r.Scopes, ","))
	}
	checked := 0
	for _, f := range bashRoleSources {
		src, ok := readTool(t, f)
		if !ok {
			continue
		}
		rows, ok := bashTable(t, src, "ROLE_TABLE")
		if !ok {
			continue
		}
		checked++
		if strings.Join(rows, "\n") != strings.Join(ours, "\n") {
			t.Errorf("tables.go fleetRoles drifted from tools/%s ROLE_TABLE:\n%s\nwant:\n%s",
				f, strings.Join(ours, "\n"), strings.Join(rows, "\n"))
		}
	}
	if checked == 0 {
		t.Fatalf("no ROLE_TABLE literal found in tools/%s — the parity check has nothing to compare",
			strings.Join(bashRoleSources, " or tools/"))
	}
}

func TestLabelTableMatchesBash(t *testing.T) {
	requireBashSources(t)
	src, ok := readTool(t, "create-fleet-gitlab.sh")
	if !ok {
		t.Fatal("tools/create-fleet-gitlab.sh is missing — the label parity check has nothing to compare")
	}
	rows, ok := bashTable(t, src, "LABEL_TABLE")
	if !ok {
		t.Fatal("no LABEL_TABLE literal in tools/create-fleet-gitlab.sh")
	}
	var ours []string
	for _, l := range fleetLabels {
		ours = append(ours, l.Name+"|"+l.Color+"|"+l.Description)
	}
	if strings.Join(rows, "\n") != strings.Join(ours, "\n") {
		t.Errorf("tables.go fleetLabels drifted from LABEL_TABLE:\n%s\nwant:\n%s",
			strings.Join(ours, "\n"), strings.Join(rows, "\n"))
	}
}

// The default PAT lifetime is spec §5's expiry backstop; the two paths must agree on it.
func TestPATDaysDefaultMatchesBash(t *testing.T) {
	requireBashSources(t)
	re := regexp.MustCompile(`(?m)^(?:FLEET_PAT_DAYS|PAT_EXPIRY_DAYS)=(\d+)$`)
	var found []string
	for _, f := range bashRoleSources {
		if src, ok := readTool(t, f); ok {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				found = append(found, m[1])
			}
		}
	}
	if len(found) == 0 {
		t.Fatal("no numeric default PAT lifetime found in the bash sources")
	}
	o, err := parseProvisionFlags([]string{"--group", "g", "--prefix", "p", "--out-dir", t.TempDir()}, &env{stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range found {
		if d != strconv.Itoa(o.patExpiryDays) {
			t.Errorf("default --pat-expiry-days %d, bash default %s", o.patExpiryDays, d)
		}
	}
}

// The script's avatar flags: --avatars-dir, --avatars-only and --no-avatars. --no-avatars
// skips the step with a NOTICE naming the flag, and contradicts the other two.
func TestFleetNoAvatarsFlag(t *testing.T) {
	h := newHarness(t, nil)
	h.e.http = &http.Client{Transport: &failingTransport{t: t}}
	if code := run(h.provisionArgs("--no-avatars", "--dry-run"), h.e); code != exitOK {
		t.Fatalf("--no-avatars --dry-run: exit %d\n%s", code, h.err.String())
	}
	if !strings.Contains(h.out.String(), "--no-avatars was given, so the avatar step (PUT /user/avatar) is SKIPPED") {
		t.Errorf("--no-avatars dry run does not name the skip:\n%s", h.out.String())
	}
	for _, extra := range [][]string{
		{"--group", fakeGroupPath, "--avatars-dir", h.dir},
		{"--avatars-only", "--avatars-dir", h.dir},
	} {
		h.err.Reset()
		args := append([]string{"provision", "--no-avatars", "--prefix", fakePrefix, "--out-dir", h.dir, "--dry-run"}, extra...)
		if code := run(args, h.e); code != exitUsage {
			t.Errorf("%v: exit %d, want %d (usage)", args, code, exitUsage)
		}
		if !strings.Contains(h.err.String(), "--no-avatars contradicts") {
			t.Errorf("%v: stderr does not name the contradiction:\n%s", args, h.err.String())
		}
	}
}
