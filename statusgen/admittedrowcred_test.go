package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// rowCredMarker is the planted stored login. A row that prints it found it.
const rowCredMarker = "planted-stored-login-marker"

// rowCredPlant plants, in the invoking environment, every place a forge CLI or
// a role-credential minter finds a stored login: a home whose forge CLI config
// holds the marker, the variables that point at that config (GH_CONFIG_DIR,
// XDG_CONFIG_HOME), a role key file named by an ASSAY_* variable, the verify
// loop's DESK_LOOP, a cell's CELLCTL_GH_AMBIENT copy of the login, and the
// marker itself in an ASSAY_* and a GH_* variable. Stub `gh` and `desktoken`
// on PATH answer with the marker whenever their inputs lead them to it. It
// returns the planted home.
func rowCredPlant(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	ghDir := filepath.Join(home, ".config", "gh")
	keyDir := filepath.Join(home, ".config", "assay")
	for _, d := range []string{ghDir, keyDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte("github.com:\n  user: "+rowCredMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "role.key"), []byte(rowCredMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	stubs := map[string]string{
		"gh": "#!/bin/sh\n[ \"$1 $2\" = \"auth token\" ] || exit 2\n" +
			"cat \"${GH_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/gh}/hosts.yml\"\n",
		"desktoken": "#!/bin/sh\n[ -n \"$DESK_LOOP\" ] || exit 2\n" +
			"cat \"${ASSAY_ROLE_KEY_FILE:-$HOME/.config/assay/role.key}\"\n",
	}
	for name, script := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", home)
	t.Setenv("GH_CONFIG_DIR", ghDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("ASSAY_ROLE_KEY_FILE", filepath.Join(keyDir, "role.key"))
	t.Setenv("ASSAY_FIXTURE_LOGIN", rowCredMarker)
	t.Setenv("GH_FIXTURE_LOGIN", rowCredMarker)
	t.Setenv("CELLCTL_GH_AMBIENT", rowCredMarker)
	return home
}

// rowCredProbe is the planted row: it records its HOME outside the run, then
// fails if any planted variable reached it, if the forge CLI or the minter
// answers with the stored login, if the login is anywhere under its HOME or in
// its environment, or if its HOME is not an empty directory.
func rowCredProbe(t *testing.T, rec string) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe.sh")
	script := "printf '%s' \"$HOME\" > '" + rec + "'\n" +
		"for v in DESK_LOOP GH_CONFIG_DIR XDG_CONFIG_HOME ASSAY_ROLE_KEY_FILE ASSAY_FIXTURE_LOGIN GH_FIXTURE_LOGIN CELLCTL_GH_AMBIENT; do\n" +
		"  eval \"[ -z \\\"\\${$v+set}\\\" ]\" || { echo \"$v reached the row\" >&2; exit 11; }\n" +
		"done\n" +
		"out=$(gh auth token 2>/dev/null; desktoken worker 2>/dev/null; cat \"$HOME/.config/gh/hosts.yml\" \"$HOME/.config/assay/role.key\" 2>/dev/null; env)\n" +
		"case \"$out\" in *" + rowCredMarker + "*) echo 'the row read the stored login' >&2; exit 12;; esac\n" +
		"[ -d \"$HOME\" ] || exit 13\n" +
		"[ -z \"$(ls -A \"$HOME\")\" ] || exit 14\n"
	if err := os.WriteFile(probe, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	return "sh '" + probe + "'"
}

// regression: #6770
// TestAdmittedRowsNoCred: an admitted row cannot read the stored forge login
// or mint a role credential through anything the invoking environment points
// at: HOME, DESK_LOOP and every ASSAY_* and GH_* variable are scrubbed, and
// the row's HOME is a fresh empty directory removed after the run. The
// unadmitted control proves the plant lands.
func TestAdmittedRowsNoCred(t *testing.T) {
	for _, admit := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
			root, _ := admittedFixture(t, admit)
			planted := rowCredPlant(t)
			if admit {
				t.Setenv("DESK_LOOP", "verify-desk")
			}
			rec := filepath.Join(t.TempDir(), "row-home")
			code := verifyrunRow(t, root, rowCredProbe(t, rec))
			if !admit {
				if code != verifyrunExitFail {
					t.Fatalf("plant did not land: the unadmitted row did not find the stored login (exit %d)", code)
				}
				return
			}
			if code != verifyrunExitPass {
				t.Fatalf("admitted row reached the stored login or a planted variable (exit %d)", code)
			}
			b, err := os.ReadFile(rec)
			if err != nil {
				t.Fatalf("the row did not record its HOME: %v", err)
			}
			rowHome := string(b)
			if rowHome == "" || rowHome == planted || strings.HasPrefix(rowHome, planted+string(os.PathSeparator)) {
				t.Fatalf("admitted row HOME = %q, want a fresh directory outside the invoking HOME %q", rowHome, planted)
			}
			for _, dir := range []string{rowHome, filepath.Dir(filepath.Dir(rowHome))} {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("the row's HOME or the run's scratch %q outlived the run (stat: %v)", dir, err)
				}
			}
			if os.Getenv("HOME") != planted || os.Getenv("DESK_LOOP") != "verify-desk" {
				t.Fatal("the invoking process lost its own HOME or DESK_LOOP")
			}
		})
	}
}

// TestAdmittedEnvScrubsCred pins the scrub one variable at a time, whatever
// its case, beside variables that only resemble one.
func TestAdmittedEnvScrubsCred(t *testing.T) {
	gone := []string{"HOME=/h", "Home=/h", "DESK_LOOP=verify-desk", "desk_loop=x",
		"XDG_CONFIG_HOME=/h/.config", "CELLCTL_GH_AMBIENT=x", "GH_CONFIG_DIR=/h/gh",
		"GH_HOST=example.invalid", "gh_anything=x", "ASSAY_APP_PEM_FILE=/k", "Assay_Roster=x",
		"GIT_CONFIG_GLOBAL=/h/.gitconfig", runCredentialEnv + "=/elsewhere",
		rowreadSocketVar + "=/elsewhere/sock"}
	kept := []string{"PATH=/bin", "HOMEBREW_PREFIX=/opt", "EXAMPLE_SETTING=s", "XDG_CACHE_HOME=/c",
		"GHOST=x", "MY_ASSAY_NOTE=x", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	env := admittedRowEnv(append(append([]string{}, gone...), kept...))
	for _, kv := range gone {
		if slices.Contains(env, kv) {
			t.Errorf("admitted row environment carries %s", kv)
		}
	}
	for _, kv := range kept {
		if !slices.Contains(env, kv) {
			t.Errorf("admitted row environment dropped %s", kv)
		}
	}
}

// TestRowreadBrokerClosed: the run's row-read socket answers only the rowread
// verb's own closed set (a write verb is refused before any reader starts),
// and a row whose run cannot be reached gets a could-not-check, never an
// empty answer.
func TestRowreadBrokerClosed(t *testing.T) {
	f := newRowreadForge(t, `"x"`)
	sock, dir, err := rowreadSocketPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if dir != "" {
		t.Cleanup(func() { os.RemoveAll(dir) })
	}
	stop, err := serveRowread(sock, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	var out, errb strings.Builder
	if code := rowreadForward(sock, []string{"post", "--issue", "example-org/one#7"}, &out, &errb); code != rowreadExitUsage {
		t.Fatalf("brokered write verb: exit %d, want %d (refused)", code, rowreadExitUsage)
	}
	if f.started() {
		t.Fatal("a refused brokered request started the reader")
	}
	stop()
	if code := rowreadForward(sock, []string{"comments", "--issue", "example-org/one#7"}, &out, &errb); code != rowreadExitCouldNot {
		t.Fatalf("unreachable run: exit %d, want %d (could-not-check)", code, rowreadExitCouldNot)
	}
}

// regression: #6770
// TestAdmittedRowOwnHome: each row of an admitted run gets its own fresh,
// empty HOME and TMPDIR, so a row finds nothing an earlier row of the same run
// left there. Each is removed when its row ends.
func TestAdmittedRowOwnHome(t *testing.T) {
	plan := resolveShellPlan()
	if !plan.ok {
		t.Skip("no runnable shell: " + plan.reason)
	}
	plan.env = admittedRowEnv(os.Environ())
	plan.rowScratch = t.TempDir()
	row := `[ -z "$(ls -A "$HOME")" ] && [ -z "$(ls -A "$TMPDIR")" ] || { echo left-behind; exit 3; }; ` +
		`echo x > "$HOME/planted"; echo x > "$TMPDIR/planted"; printf 'home=%s\ntmp=%s\n' "$HOME" "$TMPDIR"`
	var seen []string
	for i := 0; i < 2; i++ {
		got := runVerifyCommandWith(t.TempDir(), row, 30*time.Second, nil, plan, rowShellSh)
		if got.couldNotRun || got.exit != 0 {
			t.Fatalf("row %d: %+v (output %q)", i+1, got, got.output)
		}
		for _, line := range strings.Split(string(got.output), "\n") {
			_, dir, ok := strings.Cut(line, "=")
			if !ok || (!strings.HasPrefix(line, "home=") && !strings.HasPrefix(line, "tmp=")) {
				continue
			}
			if !strings.HasPrefix(dir, plan.rowScratch+string(os.PathSeparator)) {
				t.Errorf("row %d: %s is not under the run's scratch", i+1, line)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("row %d: %s outlived its row (stat: %v)", i+1, line, err)
			}
			seen = append(seen, dir)
		}
	}
	if len(seen) != 4 || seen[0] == seen[2] || seen[1] == seen[3] {
		t.Fatalf("two rows did not get a HOME and TMPDIR each of their own: %q", seen)
	}
}

// regression: #6770
// TestAdmittedRunCredAfterScrub: the scrub removes every inherited ASSAY_*
// variable, the run credential's own name included, and the run then hands
// its read-only credential path to a forge-reading row after the scrub. A row
// that reads no forge gets no path. Both get their own HOME.
func TestAdmittedRunCredAfterScrub(t *testing.T) {
	plan := resolveShellPlan()
	if !plan.ok {
		t.Skip("no runnable shell: " + plan.reason)
	}
	path := rcCredFile(t, rcRead)
	prev := runReadCredentialMint
	runReadCredentialMint = func(string, string) (string, error) { return "", nil }
	t.Cleanup(func() { runReadCredentialMint = prev })
	cred := &runReadCredential{repo: "example-org/one"}
	cred.once.Do(func() { cred.path, cred.secret = path, rcMinted })
	t.Setenv(runCredentialEnv, "/inherited/elsewhere")
	t.Setenv("ASSAY_ROLE_KEY_FILE", "/inherited/role.key")
	plan.env = admittedRowEnv(os.Environ())
	plan.readCred = cred
	plan.rowScratch = t.TempDir()
	show := `printf 'cred=%s\nkey=%s\nhome=%s\n' "${` + runCredentialEnv + `:-none}" "${ASSAY_ROLE_KEY_FILE:-none}" "$HOME"`
	for command, want := range map[string]string{
		show + " # --acceptance-forge": "cred=" + path + "\nkey=none\nhome=" + plan.rowScratch,
		show:                           "cred=none\nkey=none\nhome=" + plan.rowScratch,
	} {
		got := runVerifyCommandWith(t.TempDir(), command, 30*time.Second, nil, plan, rowShellSh)
		if got.couldNotRun || got.exit != 0 {
			t.Fatalf("row did not run: %+v", got)
		}
		if !strings.Contains(string(got.output), want) {
			t.Errorf("%q: row saw %q, want it to start %q", command, got.output, want)
		}
	}
}

// regression: #6770
// TestAdmittedRunHandsCred: end to end through verifyrun, an admitted run
// hands its own read-only credential path to a --acceptance-forge row after
// the scrub, never the path an inherited variable named.
func TestAdmittedRunHandsCred(t *testing.T) {
	root, _ := admittedFixture(t, true)
	prev := runReadCredentialMint
	runReadCredentialMint = func(_, out string) (string, error) {
		if err := os.WriteFile(out+".perms", []byte(rcRead), 0o600); err != nil {
			return "", err
		}
		return out, os.WriteFile(out, []byte(rcMinted), 0o600)
	}
	t.Cleanup(func() { runReadCredentialMint = prev })
	t.Setenv(runCredentialEnv, "/inherited/elsewhere")
	rec := filepath.Join(t.TempDir(), "cred")
	row := `printf '%s' "${` + runCredentialEnv + `:-none}" > '` + rec + `' # --acceptance-forge`
	if code := verifyrunRow(t, root, row); code != verifyrunExitPass {
		t.Fatalf("admitted forge row exit %d, want pass", code)
	}
	b, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got == "none" || got == "/inherited/elsewhere" || filepath.Base(got) != "read-credential" {
		t.Fatalf("admitted forge row saw credential path %q, want the run's own", got)
	}
}
