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
// marker itself in an ASSAY_* and a GH_* variable, and in a row input that
// passes the scrub by name only when it holds a commit id. It also plants a
// pointer at that key directory under a family no list names, and an agent
// socket path: the pass list drops both by default. Stub `gh` and `desktoken`
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
	t.Setenv("ASSAY_DELIVERY_BASE_SHA", rowCredMarker)
	t.Setenv("EXAMPLE_TOOL_CONFIG_DIR", keyDir)
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(home, "agent.sock"))
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
		"for v in DESK_LOOP GH_CONFIG_DIR XDG_CONFIG_HOME ASSAY_ROLE_KEY_FILE ASSAY_FIXTURE_LOGIN GH_FIXTURE_LOGIN CELLCTL_GH_AMBIENT EXAMPLE_TOOL_CONFIG_DIR SSH_AUTH_SOCK; do\n" +
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
// its case. Each credential pointer is refused by the deny layer itself
// (admittedRowDenied), not only left off the pass list, so a pass-list entry
// added later cannot carry one through. A variable that only resembles one is
// not refused by that layer.
func TestAdmittedEnvScrubsCred(t *testing.T) {
	gone := []string{"HOME=/h", "Home=/h", "DESK_LOOP=verify-desk", "desk_loop=x",
		"XDG_CONFIG_HOME=/h/.config", "CELLCTL_GH_AMBIENT=x", "GH_CONFIG_DIR=/h/gh",
		"GH_HOST=example.invalid", "gh_anything=x", "ASSAY_APP_PEM_FILE=/k", "Assay_Roster=x",
		"GIT_CONFIG_GLOBAL=/h/.gitconfig", runCredentialEnv + "=/elsewhere",
		rowreadSocketVar + "=/elsewhere/sock", "BASH_ENV=/h/rc", "GITHUB_TOKEN=x"}
	lookalike := []string{"PATH=/bin", "HOMEBREW_PREFIX=/opt", "EXAMPLE_SETTING=s", "XDG_CACHE_HOME=/c",
		"GHOST=x", "MY_ASSAY_NOTE=x", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	env := admittedRowEnv(append(append([]string{}, gone...), lookalike...))
	for _, kv := range gone {
		if slices.Contains(env, kv) {
			t.Errorf("admitted row environment carries %s", kv)
		}
		if !admittedRowDenied(kv) {
			t.Errorf("the deny layer does not refuse %s", kv)
		}
	}
	for _, kv := range lookalike {
		if admittedRowDenied(kv) {
			t.Errorf("the deny layer refuses %s, which only resembles a credential pointer", kv)
		}
	}
	for _, kv := range []string{"PATH=/bin", "GIT_CONFIG_GLOBAL=" + os.DevNull} {
		if !slices.Contains(env, kv) {
			t.Errorf("admitted row environment dropped %s", kv)
		}
	}
}

// regression: #6792
// TestAdmittedEnvPassList: the scrub is an allow-list. A variable reaches an
// admitted row only when it is on the pass list (admittedRowPassNames, or a
// locale LC_* setting), so a family nobody listed is dropped by default: an
// agent socket, a runtime directory, another tool's config pointer, a CI
// runtime variable, the Windows profile, and a name that only extends a
// listed one. A listed proxy or module proxy that carries a user name in its
// URL is dropped too.
func TestAdmittedEnvPassList(t *testing.T) {
	kept := []string{"PATH=/bin", "Path=C:\\bin", "LANG=C.UTF-8", "LC_ALL=C", "TERM=dumb", "TZ=UTC",
		"CI=true", "SSL_CERT_FILE=/etc/ssl/cert.pem", "GOFLAGS=-mod=readonly", "GOMODCACHE=/go/pkg/mod",
		"GOPROXY=off", "GOTOOLCHAIN=local", "GODEBUG=multipathtcp=0", "GOCACHE=/c", "GOPATH=/g",
		"HTTPS_PROXY=http://proxy.example.invalid:3128", "no_proxy=localhost", "SystemRoot=C:\\Windows"}
	gone := []string{"SSH_AUTH_SOCK=/tmp/agent", "SSH_ASKPASS=/bin/ask", "XDG_RUNTIME_DIR=/run/user/1",
		"DOCKER_CONFIG=/h/.docker", "KUBECONFIG=/h/.kube/config", "EXAMPLE_TOOL_CONFIG_DIR=/h/tool",
		"OTHERFORGE_LOGIN=x", "ACTIONS_RUNTIME_URL=https://example.invalid", "CI_EXAMPLE_LOGIN=x",
		"PATHEXTRA=x", "GOAUTH=git", "GOENV=/h/go.env", "APPDATA=D:\\prof\\AppData", "USERPROFILE=D:\\prof",
		"LCX=x", "HTTPS_PROXY=http://fixture@proxy.example.invalid:3128", "GOPROXY=https://fixture@mod.example.invalid"}
	for _, kv := range kept {
		if !slices.Contains(admittedRowEnv([]string{kv}), kv) {
			t.Errorf("admitted row environment dropped the listed %s", kv)
		}
	}
	for _, kv := range gone {
		if slices.Contains(admittedRowEnv([]string{kv}), kv) {
			t.Errorf("admitted row environment carries %q, which is not on the pass list", kv)
		}
	}
}

// rowInputSHA and rowInputBase are delivery coordinates of the shape the
// container verify runner exports for an admitted run.
const (
	rowInputSHA  = "0123456789abcdef0123456789abcdef01234567"
	rowInputBase = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// regression: F5
// TestAdmittedRowInputsPass: the named delivery coordinates (the ASSAY_* pair
// and the unprefixed DELIVERY_HEAD/DELIVERY_BASE pair) reach an admitted row,
// by exact name and only with a full commit id as the value. Every other
// ASSAY_* variable is still dropped: one that only extends or shortens an
// allowed name, a lower-case copy, a database DSN, a cell API setting, a role
// key path, and an allowed name carrying any other value.
func TestAdmittedRowInputsPass(t *testing.T) {
	kept := []string{"ASSAY_DELIVERY_SHA=" + rowInputSHA, "ASSAY_DELIVERY_BASE_SHA=" + rowInputBase,
		"DELIVERY_HEAD=" + rowInputSHA, "DELIVERY_BASE=" + rowInputBase}
	for _, kv := range kept {
		if !slices.Contains(admittedRowEnv([]string{kv}), kv) {
			t.Errorf("admitted row environment dropped the named row input %s", kv)
		}
	}
	gone := []string{"ASSAY_DELIVERY_SHA_EXTRA=" + rowInputSHA, "ASSAY_DELIVERY=" + rowInputSHA,
		"ASSAY_DELIVERY_REF=" + rowInputSHA, "assay_delivery_sha=" + rowInputSHA, "Assay_Delivery_Base_Sha=" + rowInputSHA,
		"ASSAY_TEST_POSTGRES_DSN=postgres://example.invalid/db", "ASSAY_CELL_API_URL=https://example.invalid",
		"ASSAY_ROLE_KEY_FILE=/k", "ASSAY_DELIVERY_SHA=" + rowCredMarker, "ASSAY_DELIVERY_SHA=" + rowInputSHA[:12],
		"ASSAY_DELIVERY_BASE_SHA=" + strings.ToUpper(rowInputSHA), "ASSAY_DELIVERY_BASE_SHA=" + rowInputSHA + "\nx",
		"DELIVERY_HEAD=" + rowInputSHA[:12], "DELIVERY_BASE=" + rowCredMarker, "delivery_head=" + rowInputSHA,
		"DELIVERY_HEAD_EXTRA=" + rowInputSHA, "DELIVERY=" + rowInputSHA}
	for _, kv := range gone {
		if slices.Contains(admittedRowEnv([]string{kv}), kv) {
			t.Errorf("admitted row environment carries %q, which is outside the named row inputs", kv)
		}
	}
}

// regression: F5
// TestAdmittedRowSeesDelivery: end to end through verifyrun, an admitted row
// reads the delivery coordinates the runner exported, while a scrubbed ASSAY_*
// credential pointer and a database DSN stay gone. The unadmitted control
// proves the plant lands: there the row sees the DSN too. The row asks for the
// scrubbed names with printenv, not $NAME: a $NAME read of a scrubbed name
// makes an admitted row could-not-run (TestAdmittedScrubbedInputCouldNotRun).
func TestAdmittedRowSeesDelivery(t *testing.T) {
	row := `test "$ASSAY_DELIVERY_SHA" = ` + rowInputSHA + ` && test "$ASSAY_DELIVERY_BASE_SHA" = ` + rowInputBase +
		` && ! printenv ASSAY_ROLE_KEY_FILE && ! printenv ASSAY_TEST_POSTGRES_DSN`
	for _, admit := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
			root, _ := admittedFixture(t, admit)
			t.Setenv("ASSAY_DELIVERY_SHA", rowInputSHA)
			t.Setenv("ASSAY_DELIVERY_BASE_SHA", rowInputBase)
			t.Setenv("ASSAY_ROLE_KEY_FILE", "/inherited/role.key")
			t.Setenv("ASSAY_TEST_POSTGRES_DSN", "postgres://example.invalid/db")
			code := verifyrunRow(t, root, row)
			if admit && code != verifyrunExitPass {
				t.Fatalf("admitted row lost a delivery coordinate or kept a scrubbed ASSAY_* variable (exit %d)", code)
			}
			if !admit && code != verifyrunExitFail {
				t.Fatalf("plant did not land: the unadmitted row did not see the DSN (exit %d)", code)
			}
		})
	}
}

// regression: #6802
// TestAdmittedRowBareDelivery: end to end through verifyrun, an admitted row
// written against the unprefixed delivery pair (DELIVERY_HEAD, DELIVERY_BASE),
// the form older consumer-diff Verify rows read, still sees both while each
// holds a full commit id. The allow-list would otherwise drop them, and such a
// row would fail at its own test -n instead of checking the delivery. The
// unadmitted control proves the row itself passes with the pair set.
func TestAdmittedRowBareDelivery(t *testing.T) {
	row := `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$DELIVERY_HEAD" = ` + rowInputSHA +
		` && test "$DELIVERY_BASE" = ` + rowInputBase
	for _, admit := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
			root, _ := admittedFixture(t, admit)
			t.Setenv("DELIVERY_HEAD", rowInputSHA)
			t.Setenv("DELIVERY_BASE", rowInputBase)
			if code := verifyrunRow(t, root, row); code != verifyrunExitPass {
				t.Fatalf("row lost the unprefixed delivery pair (exit %d)", code)
			}
		})
	}
}

// regression: F6
// TestAdmittedScrubbedInputCouldNotRun: under admission, a row that reads an
// ASSAY_* input the scrub removes is could-not-run, with a note naming the
// variable, never fail: it cannot run as written there (desk decisions 10 and
// 11). Each quoting form a row guards with is covered. The unadmitted control,
// with the same inputs set, runs and passes each row.
func TestAdmittedScrubbedInputCouldNotRun(t *testing.T) {
	rows := map[string]string{
		"ASSAY_TEST_POSTGRES_DSN":     `test -n "$ASSAY_TEST_POSTGRES_DSN"`,
		"ASSAY_TEST_PERF_DSN":         `[ -n "${ASSAY_TEST_PERF_DSN}" ]`,
		"ASSAY_CELL_API_PROFILE":      `test -n "${ASSAY_CELL_API_PROFILE:-}"`,
		"ASSAY_OPERATOR_TEST_IMAGE":   `test "${#ASSAY_OPERATOR_TEST_IMAGE}" -gt 0`,
		"ASSAY_DISPOSABLE_KUBECONFIG": `test -n "${ASSAY_DELIVERY_SHA}" && test "${ASSAY_DISPOSABLE_KUBECONFIG+set}" = set`,
	}
	for name, row := range rows {
		for _, admit := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/admitted", false: "/control"}[admit], func(t *testing.T) {
				root, _ := admittedFixture(t, admit)
				for _, kv := range []string{"ASSAY_TEST_POSTGRES_DSN=postgres://example.invalid/db",
					"ASSAY_TEST_PERF_DSN=postgres://example.invalid/perf", "ASSAY_CELL_API_PROFILE=fixture",
					"ASSAY_OPERATOR_TEST_IMAGE=example.invalid/op:1", "ASSAY_DISPOSABLE_KUBECONFIG=/k",
					"ASSAY_DELIVERY_SHA=" + rowInputSHA} {
					k, v, _ := strings.Cut(kv, "=")
					t.Setenv(k, v)
				}
				tick := string(rune(96))
				brief := filepath.Join(t.TempDir(), "brief.md")
				body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + row + tick + " | exit 0 |\n\n## Evidence\n\n"
				if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				got, out := captureVerifyrunOut(t, []string{"--brief", brief, "--root", root, "--dry-run"})
				if !admit {
					if got.code != verifyrunExitPass {
						t.Fatalf("plant did not land: the unadmitted row did not pass (exit %d):\n%s", got.code, out)
					}
					return
				}
				if got.code != verifyrunExitCouldNot {
					t.Fatalf("admitted row that reads the scrubbed %s gave exit %d, want could-not-run (exit %d):\n%s",
						name, got.code, verifyrunExitCouldNot, out)
				}
				if !strings.Contains(out, name) || !strings.Contains(out, "outside admission") {
					t.Fatalf("could-not-run note does not name %s and say to run outside admission:\n%s", name, out)
				}
				if !strings.Contains(out, "not permission to run it outside admission") ||
					!strings.Contains(out, "once the row itself has been reviewed") {
					t.Fatalf("could-not-run note reads as permission to run outside admission:\n%s", out)
				}
			})
		}
	}
}

// regression: F-caller-set-row-inputs
// TestAdmittedDroppedInputNotRun: under admission, a row that reads a variable
// its caller exported and the allow-list drops (the kit variables Verify rows
// name: DT03_KIT, a PR number, a pointer such as XDG_RUNTIME_DIR) is
// could-not-run with a note naming it, never fail: the row would otherwise run
// with the variable unset and record a defect in a delivery that may be correct.
// The unadmitted control, with the same inputs set, runs and passes each row. A
// row that sets the same name itself still runs under admission and passes.
func TestAdmittedDroppedInputNotRun(t *testing.T) {
	rows := map[string]string{
		"DT03_KIT":        `test -r "$DT03_KIT/probe.txt"`,
		"PR":              `test "${PR:?needs the pr number}" = 42`,
		"XDG_RUNTIME_DIR": `test -n "${XDG_RUNTIME_DIR:-}"`,
	}
	kit := t.TempDir()
	if err := os.WriteFile(filepath.Join(kit, "probe.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, row := range rows {
		for _, admit := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/admitted", false: "/control"}[admit], func(t *testing.T) {
				root, _ := admittedFixture(t, admit)
				t.Setenv("DT03_KIT", kit)
				t.Setenv("PR", "42")
				t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
				got, out := captureVerifyrunOut(t, []string{"--brief", droppedInputBrief(t, row), "--root", root, "--dry-run"})
				if !admit {
					if got.code != verifyrunExitPass {
						t.Fatalf("plant did not land: the unadmitted row did not pass (exit %d):\n%s", got.code, out)
					}
					return
				}
				if got.code != verifyrunExitCouldNot || !strings.Contains(out, name) ||
					!strings.Contains(out, "not permission to run it outside admission") {
					t.Fatalf("admitted row that reads the dropped %s gave exit %d, want could-not-run (exit %d) naming it:\n%s",
						name, got.code, verifyrunExitCouldNot, out)
				}
			})
		}
	}
	t.Run("sets its own name", func(t *testing.T) {
		root, _ := admittedFixture(t, true)
		t.Setenv("DT03_KIT", kit)
		row := `for DT03_KIT in "$PWD"; do test -d "$DT03_KIT"; done && D=$(pwd) && test -d "$D"`
		if got, out := captureVerifyrunOut(t, []string{"--brief", droppedInputBrief(t, row), "--root", root, "--dry-run"}); got.code != verifyrunExitPass {
			t.Fatalf("admitted row that sets its own variables gave exit %d, want pass:\n%s", got.code, out)
		}
	})
}

// droppedInputBrief writes a one-row brief and returns its path.
func droppedInputBrief(t *testing.T, row string) string {
	t.Helper()
	tick := string(rune(96))
	brief := filepath.Join(t.TempDir(), "brief.md")
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + row + tick + " | exit 0 |\n\n## Evidence\n\n"
	if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return brief
}

// regression: #6773
// TestScrubbedRowNeverGreen: a row's own text can make it could-not-run under
// admission, so could-not-run must never clear anything. The verdict is
// could-not-run even when the row's recorded exit equals the expected one,
// and in a table where the other row passes or fails the run's exit is
// could-not-run, never pass, with the row recorded could-not-run.
func TestScrubbedRowNeverGreen(t *testing.T) {
	if state, _ := (expectSpec{exitCode: 0}).verdict(runResult{exit: 0, couldNotRun: true, reason: "r"}); state != stateCouldNotRun {
		t.Fatalf("a could-not-run result with the expected exit reads %s, want %s", state, stateCouldNotRun)
	}
	for _, other := range []string{"true", "false"} {
		t.Run(other, func(t *testing.T) {
			root, _ := admittedFixture(t, true)
			t.Setenv("ASSAY_TEST_POSTGRES_DSN", "postgres://example.invalid/db")
			tick := string(rune(96))
			brief := filepath.Join(t.TempDir(), "brief.md")
			body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n" +
				"| 1 | " + tick + other + tick + " | exit 0 |\n" +
				"| 2 | " + tick + `test -n "$ASSAY_TEST_POSTGRES_DSN"` + tick + " | exit 0 |\n\n## Evidence\n\n"
			if err := os.WriteFile(brief, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, out := captureVerifyrunOut(t, []string{"--brief", brief, "--root", root, "--dry-run"})
			if got.code != verifyrunExitCouldNot {
				t.Fatalf("run with a could-not-run row beside a %q row gave exit %d, want could-not-run (exit %d):\n%s",
					other, got.code, verifyrunExitCouldNot, out)
			}
			if !strings.Contains(out, "row 2: could-not-run") {
				t.Fatalf("row 2 is not recorded could-not-run:\n%s", out)
			}
		})
	}
}

// regression: F6
// TestAdmittedRowScrubbedRefs pins the rule's boundary. A read counts when it
// is written as $NAME, ${NAME...}, $env:NAME or %NAME% of a name the scrub
// drops whatever its value, wherever it sits in the text: any ASSAY_* name, or
// another capitalised name the allow-list drops that the row only reads. The
// delivery coordinates, a name the run set after the scrub, the pass list, the
// named refusals that were dropped before the allow-list, a name the row also
// assigns or names bare, a name the row binds itself (a non-empty default or
// its own assignment before the read, #7134), and a name read without that
// syntax do not count.
func TestAdmittedRowScrubbedRefs(t *testing.T) {
	cred := runCredentialEnv + "=/run/cred"
	cases := []struct {
		command string
		env     []string
		want    []string
	}{
		{`test -n "$ASSAY_TEST_POSTGRES_DSN" && go test ./x`, nil, []string{"ASSAY_TEST_POSTGRES_DSN"}},
		{`[ -n "${ASSAY_TEST_PERF_DSN}" ]`, nil, []string{"ASSAY_TEST_PERF_DSN"}},
		// A non-empty default is the row's own value (#7134); a length or an
		// indirection still reads the inherited name.
		{`x=${ASSAY_CELL_API_CONTEXT:-default}; y=${#ASSAY_COHORT_REPLAY}; z=${!ASSAY_CELL}`, nil,
			[]string{"ASSAY_COHORT_REPLAY", "ASSAY_CELL"}},
		{`echo '$ASSAY_OPERATOR_TEST_IMAGE' # $ASSAY_OPERATOR_TEST_IMAGE`, nil, []string{"ASSAY_OPERATOR_TEST_IMAGE"}},
		{`echo $assay_lower_copy`, nil, []string{"assay_lower_copy"}},
		{`if ($env:Assay_Test_Postgres_Dsn) { exit 0 }`, nil, []string{"Assay_Test_Postgres_Dsn"}},
		{`if defined ASSAY_X (echo %ASSAY_DISPOSABLE_CONTEXT%)`, nil, []string{"ASSAY_DISPOSABLE_CONTEXT"}},
		{`test -n "$ASSAY_DELIVERY_SHA" && test -n "${ASSAY_DELIVERY_BASE_SHA}"`, nil, nil},
		{`cat "$` + runCredentialEnv + `"`, []string{cred}, nil},
		{`cat "$` + runCredentialEnv + `"`, nil, []string{runCredentialEnv}},
		{`test -n "$HOME" && test -z "$GH_TOKEN" && echo "$DESK_LOOP" "$GIT_DIR"`, nil, nil},
		{`printenv ASSAY_TEST_POSTGRES_DSN; grep -c ASSAY_TEST_POSTGRES_DSN f`, nil, nil},
		// Any other variable the allow-list drops counts while the row only reads it.
		{`test -r "$DT03_KIT/a.txt" && test -x "$DT03_SWEEP" && cat "${DT03_KIT}/b"`, nil, []string{"DT03_KIT", "DT03_SWEEP"}},
		{`test -n "$D" && git rev-parse "$D^1"`, nil, []string{"D"}},
		{`H=$(gh pr view "$PR"); test -n "${SIBLING_PR:?need it}"`, nil, []string{"PR", "SIBLING_PR"}},
		{`test -n "${GITLAB_API_BASE:-}" && echo %USERPROFILE% $env:Kit_Dir`, nil, []string{"GITLAB_API_BASE", "USERPROFILE", "Kit_Dir"}},
		{`echo $XDG_RUNTIME_DIR $SSH_AUTH_SOCK $GITHUB_ACTIONS`, nil, []string{"XDG_RUNTIME_DIR", "SSH_AUTH_SOCK", "GITHUB_ACTIONS"}},
		// A name the row sets or names bare itself is the row's own.
		{`for D in a b; do test -n "$D"; done`, nil, nil},
		{`D=$(mktemp -d) && test -d "$D"`, nil, nil},
		{`read -r KIT < f && test -r "$KIT"`, nil, nil},
		{`printenv DT03_KIT; test -r "$DT03_KIT"`, nil, nil},
		// Lower and mixed case $name is a local; the explicit env forms count in any case.
		{`n=1; test "$n" = 1 && test -n "${kit_dir}" && $true`, nil, nil},
		// Shell and run variables, the pass list, the named inputs and the named refusals do not count.
		{`cd "$PWD" && echo "$RANDOM" "$TMPDIR" "$PATH" "$USER" "$LANG" "$LC_ALL" "$GOFLAGS" "$CI" "$LASTEXITCODE"`, nil, nil},
		{`test -n "$DELIVERY_HEAD" && test -n "${DELIVERY_BASE}"`, nil, nil},
		{`test -z "$GITHUB_TOKEN" && test -z "$GH_HOST" && echo "$GIT_DIR" "$ENV" "$BASH_ENV"`, nil, nil},
		// A name already in the admitted environment is not removed.
		{`test -r "$DT03_KIT"`, []string{"DT03_KIT=/k"}, nil},
	}
	for _, c := range cases {
		if got := admittedRowScrubbedRefs(c.command, c.env); !slices.Equal(got, c.want) {
			t.Errorf("admittedRowScrubbedRefs(%q) = %q, want %q", c.command, got, c.want)
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
	show := `printf 'cred=%s\nkey=%s\nhome=%s\n' "$(printenv ` + runCredentialEnv + ` || echo none)" "$(printenv ASSAY_ROLE_KEY_FILE || echo none)" "$HOME"`
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
