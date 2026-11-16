package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/forgelabels"
)

// The Unix provisioning path (tools/create-fleet-gitlab.sh, now an adapter onto cmd/deskgitlab)
// and this verb must not drift. These tests read the tables out of the sources in this tree —
// the shell role table and the deskgitlab label table — not out of a copy in this file, so an
// edit to either side that the other does not repeat fails here.
//
// The role table lives in the shared tools/fleet-gitlab-roles.sh (the renewal script sources
// it). tools/create-fleet-gitlab.sh is now an adapter onto cmd/deskgitlab and carries no
// table; it stays in the list so a table re-added there is still checked. Every source that
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

// The script's LABEL_TABLE moved into internal/forgelabels, the one canonical set this verb
// and cmd/deskgitlab both build from (TestLabelTableMatchesBash holds both to it). This test
// pins that the documented-role fallback names exactly the desk roles this
// provisioner's role table creates service accounts for (board-writer is the one fleet account
// that is not a desk role and is never a label target).
func TestDocumentedRolesMatchFleetRoles(t *testing.T) {
	var desk []string
	for _, r := range fleetRoles {
		if r.Role != "board-writer" {
			desk = append(desk, r.Role)
		}
	}
	got := forgelabels.DocumentedRoles()
	sort.Strings(desk)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(desk, ",") {
		t.Errorf("forgelabels.DocumentedRoles() = %v; fleetRoles' desk roles = %v", got, desk)
	}
}

// TestLabelTableMatchesBash — the script's LABEL_TABLE moved, row for row, into the Go
// provisioner the script now execs (cmd/deskgitlab, desktools-v2/30), and both that
// provisioner and this verb now build their set from internal/forgelabels. The test keeps its
// name (brief Verify rows require it) and its property: the label set the shell path creates
// is the one this verb creates. It reads cmd/deskgitlab/tables.go as SOURCE (two main
// packages) and passes only when that file's fleetLabels is this verb's own declaration, or a
// literal whose rows equal this verb's set; and no shell source re-adds a LABEL_TABLE literal,
// which would be a second copy no Go table is built from.
func TestLabelTableMatchesBash(t *testing.T) {
	theirs, err := os.ReadFile(filepath.Join("..", "deskgitlab", "tables.go"))
	if err != nil {
		t.Fatalf("reading cmd/deskgitlab/tables.go (the provisioner's label table): %v", err)
	}
	ours, err := os.ReadFile("tables.go")
	if err != nil {
		t.Fatalf("reading tables.go: %v", err)
	}
	var rows []string
	for _, l := range fleetLabels {
		rows = append(rows, l.Name+"|"+l.Color+"|"+l.Description)
	}
	if msg := labelDeclAgrees(string(ours), string(theirs), rows); msg != "" {
		t.Errorf("cmd/deskgitlab's fleetLabels drifted from this verb's: %s", msg)
	}
	for _, f := range bashRoleSources {
		b, err := os.ReadFile(filepath.Join(toolsDir(), f))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := bashTable(t, string(b), "LABEL_TABLE"); ok {
			t.Errorf("tools/%s carries a LABEL_TABLE literal again: a second label table, which internal/forgelabels replaced", f)
		}
	}
}

// labelDeclAgrees compares two `var fleetLabels = …` declarations: equal right-hand sides
// agree (both build from the same expression), a literal agrees when its rows equal want,
// and anything else is drift. It returns "" when they agree, else what differs.
func labelDeclAgrees(ours, theirs string, want []string) string {
	decl := regexp.MustCompile(`(?m)^var fleetLabels = (.*)$`)
	o, th := decl.FindStringSubmatch(ours), decl.FindStringSubmatch(theirs)
	switch {
	case o == nil:
		return "this package declares no fleetLabels"
	case th == nil:
		return "no fleetLabels declaration found"
	case strings.TrimSpace(o[1]) == strings.TrimSpace(th[1]):
		return ""
	}
	block := regexp.MustCompile(`(?s)var fleetLabels = \[\]fleetLabel\{\n(.*?)\n\}`).FindStringSubmatch(theirs)
	if block == nil {
		return "it is built from " + strings.TrimSpace(th[1]) + ", this verb from " + strings.TrimSpace(o[1])
	}
	row := regexp.MustCompile(`^\s*\{("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*"), ("(?:[^"\\]|\\.)*")\},$`)
	var got []string
	for _, ln := range strings.Split(block[1], "\n") {
		if s := strings.TrimSpace(ln); s == "" || strings.HasPrefix(s, "//") {
			continue
		}
		m := row.FindStringSubmatch(ln)
		if m == nil {
			return "unparsed literal row " + strconv.Quote(ln)
		}
		var f [3]string
		for i := range f {
			v, err := strconv.Unquote(m[i+1])
			if err != nil {
				return "literal row " + strconv.Quote(ln) + ": " + err.Error()
			}
			f[i] = v
		}
		got = append(got, strings.Join(f[:], "|"))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return "literal rows:\n" + strings.Join(got, "\n") + "\nwant:\n" + strings.Join(want, "\n")
	}
	return ""
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
	o := provisionDefaults(t)
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

// provisionDefaults runs `provision` through the command tree with only the required flags
// and returns the typed options that reached the handler — its declared defaults.
func provisionDefaults(t *testing.T) provisionOpts {
	t.Helper()
	var got provisionOpts
	calls := 0
	prev := provisionFn
	provisionFn = func(o provisionOpts, _ *env) int { calls++; got = o; return exitOK }
	defer func() { provisionFn = prev }()
	var errb bytes.Buffer
	e := &env{stdout: io.Discard, stderr: &errb, lookupEnv: func(string) (string, bool) { return "", false }}
	if code := run([]string{"provision", "--group", "g", "--prefix", "p", "--out-dir", t.TempDir(), "--dry-run"}, e); code != exitOK || calls != 1 {
		t.Fatalf("provision with only the required flags: exit %d, handler %d\n%s", code, calls, errb.String())
	}
	return got
}
