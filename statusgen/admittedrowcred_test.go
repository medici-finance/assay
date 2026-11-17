package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			if _, err := os.Stat(rowHome); !os.IsNotExist(err) {
				t.Fatalf("the row's HOME %q outlived the run (stat: %v)", rowHome, err)
			}
			if os.Getenv("HOME") != planted || os.Getenv("DESK_LOOP") != "verify-desk" {
				t.Fatal("the invoking process lost its own HOME or DESK_LOOP")
			}
		})
	}
}
