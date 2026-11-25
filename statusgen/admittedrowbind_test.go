package main

import (
	"slices"
	"strings"
	"testing"
)

// regression: #7134
// TestAdmittedRowSelfBound pins which reads of a scrubbed name are the row's
// own. A non-empty default, a read after the row's own assignment in the same
// shell, an exported assignment, and a single-quoted script under a leading
// NAME=value cmd see the row's value and do not count. A bare read, an empty
// default, ${NAME:?} and ${NAME:+x}, a read before or inside the assignment, a
// prefixed command's own arguments, and an assignment the row's shell never
// makes (a subshell, a pipeline, after ||, in the background, inside quotes or
// a compound body) still count. The same rule covers a dropped non-ASSAY name.
func TestAdmittedRowSelfBound(t *testing.T) {
	x := []string{"ASSAY_X"}
	cases := []struct {
		command string
		want    []string
	}{
		// The row's own value: not refused.
		{`test "${ASSAY_CONTRACT_HEAD_ROOT:-/work}" = /work`, nil},
		{`test "${ASSAY_SOURCE_SHA-unset}" = unset`, nil},
		{`: "${ASSAY_X:=/row}"; test "$ASSAY_X" = /row`, nil},
		{`: ${ASSAY_X=/row}; test -d "$ASSAY_X"`, nil},
		{`ASSAY_X=/row; test -d "$ASSAY_X"`, nil},
		{`ASSAY_X=/row && test -d "${ASSAY_X}"`, nil},
		{"ASSAY_X=/row\ntest -d \"$ASSAY_X\"", nil},
		{`ASSAY_A=1 ASSAY_B="two words"; echo "$ASSAY_A$ASSAY_B"`, nil},
		{`export ASSAY_X=/row; sh -c 'test -d "$ASSAY_X"'`, nil},
		{`ASSAY_X=/row sh -c 'test -d "$ASSAY_X"'`, nil},
		{`env ASSAY_X=/row sh -c 'test "$ASSAY_X" = /row'`, nil},
		{`sh -c ': ${ASSAY_X:=/row}; test -d "$ASSAY_X"'`, nil},
		{`test -n "${GITLAB_API_BASE:-https://example.invalid}"`, nil},
		// An inherited value would be observed: still refused.
		{`test -n "$ASSAY_X"`, x},
		{`test -n "${ASSAY_X}"`, x},
		{`test -n "${ASSAY_X:?}"`, x},
		{`test "${ASSAY_X:+set}" = set`, x},
		{`test -n "${ASSAY_X:-}"`, x},
		{`test -n "${ASSAY_X:-""}"`, x},
		{`test -n "${ASSAY_X-}"`, x},
		{`test -n "${ASSAY_X:-/row}" && test -n "$ASSAY_X"`, x},
		{`test -n "$ASSAY_X" && ASSAY_X=/row`, x},
		{`ASSAY_X=$ASSAY_X/sub; test -d "$ASSAY_X"`, x},
		{`ASSAY_X=/row test -n "$ASSAY_X"`, x},
		{`ASSAY_X=/row true; test -n "$ASSAY_X"`, x},
		{`ASSAY_X=/row; sh -c 'test -n "$ASSAY_X"'`, x},
		{`sh -c ': ${ASSAY_X:=/row}'; test -n "$ASSAY_X"`, x},
		{`(ASSAY_X=/row); test -n "$ASSAY_X"`, x},
		{`(ASSAY_X=/row; true); test -n "$ASSAY_X"`, x},
		{`sh -c "ASSAY_X=/row; test -n \"\$ASSAY_X\""`, x},
		{`true | ASSAY_X=/row; test -n "$ASSAY_X"`, x},
		{`true || ASSAY_X=/row; test -n "$ASSAY_X"`, x},
		{`ASSAY_X=/row & test -n "$ASSAY_X"`, x},
		{`echo "ASSAY_X=/row"; test -n "$ASSAY_X"`, x},
		{`if true; then ASSAY_X=/row; fi; test -n "$ASSAY_X"`, x},
		{"# ASSAY_X=/row\ntest -n \"$ASSAY_X\"", x},
		{`test "${ASSAY_X:-a}" = a && echo $env:ASSAY_X`, x},
		{`test "${#ASSAY_X}" -gt 0`, x},
		{`test -n "${GITLAB_API_BASE:-}"`, []string{"GITLAB_API_BASE"}},
	}
	for _, c := range cases {
		if got := admittedRowScrubbedRefs(c.command, nil); !slices.Equal(got, c.want) {
			t.Errorf("admittedRowScrubbedRefs(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

// selfBindPlant sets, in the invoking environment, an inherited value for
// every name the #7134 rows read, so a row that saw one would notice.
func selfBindPlant(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ASSAY_CONTRACT_HEAD_ROOT", "ASSAY_X", "ASSAY_SOURCE_SHA", "ASSAY_SOURCE_ROOT"} {
		t.Setenv(name, "/inherited")
	}
	t.Setenv("GITLAB_API_BASE", "https://inherited.invalid")
}

// regression: #7134
// TestAdmittedSelfDefaultRuns: under admission, a row that binds an ASSAY_*
// name itself runs, and sees its own default or empty value, never the value
// its caller set: each row passes only when the inherited value is absent.
// The unadmitted control, with the same values set, sees them and fails each
// row, which proves the plant lands.
func TestAdmittedSelfDefaultRuns(t *testing.T) {
	rows := map[string]string{
		"default":  `test "${ASSAY_CONTRACT_HEAD_ROOT:-/work}" = /work`,
		"assign":   `: "${ASSAY_X:=/row}"; test "$ASSAY_X" = /row`,
		"unset":    `test "${ASSAY_SOURCE_SHA-unset}" = unset`,
		"prefix":   `! printenv ASSAY_SOURCE_ROOT && ASSAY_SOURCE_ROOT=/row sh -c 'test "$ASSAY_SOURCE_ROOT" = /row'`,
		"export":   `! printenv ASSAY_X && export ASSAY_X=/row && sh -c 'test "$ASSAY_X" = /row'`,
		"standing": `test -z "$(printenv ASSAY_X)" && ASSAY_X=/row && test "$ASSAY_X" = /row`,
		"dropped":  `test "${GITLAB_API_BASE:-https://example.invalid}" = https://example.invalid`,
	}
	for name, row := range rows {
		for _, admit := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/admitted", false: "/control"}[admit], func(t *testing.T) {
				root, _ := admittedFixture(t, admit)
				selfBindPlant(t)
				got, out := captureVerifyrunOut(t, []string{"--brief", droppedInputBrief(t, row), "--root", root, "--dry-run"})
				if !admit {
					if got.code != verifyrunExitFail {
						t.Fatalf("plant did not land: the unadmitted row did not fail (exit %d):\n%s", got.code, out)
					}
					return
				}
				if got.code != verifyrunExitPass {
					t.Fatalf("admitted row that binds its own value gave exit %d, want pass (exit %d):\n%s",
						got.code, verifyrunExitPass, out)
				}
			})
		}
	}
}

// regression: #7134
// TestAdmittedUnboundRefused: under admission, a row that would observe an
// inherited ASSAY_* value is still could-not-run with the note naming it: a
// bare or guarded read, an empty default, a read before the row's assignment,
// a prefixed command's own argument, and an assignment made in a subshell. The
// unadmitted control, with the value set, passes each row.
func TestAdmittedUnboundRefused(t *testing.T) {
	rows := map[string]string{
		"alt":      `test "${ASSAY_X:+set}" = set`,
		"guard":    `test -n "${ASSAY_X:?}"`,
		"empty":    `test "${ASSAY_X:-}" = /inherited`,
		"before":   `test -n "$ASSAY_X" && ASSAY_X=/row`,
		"argument": `ASSAY_X=/row test "$ASSAY_X" = /inherited`,
		"subshell": `(ASSAY_X=/row); test "$ASSAY_X" = /inherited`,
	}
	for name, row := range rows {
		for _, admit := range []bool{true, false} {
			t.Run(name+map[bool]string{true: "/admitted", false: "/control"}[admit], func(t *testing.T) {
				root, _ := admittedFixture(t, admit)
				selfBindPlant(t)
				got, out := captureVerifyrunOut(t, []string{"--brief", droppedInputBrief(t, row), "--root", root, "--dry-run"})
				if !admit {
					if got.code != verifyrunExitPass {
						t.Fatalf("plant did not land: the unadmitted row did not pass (exit %d):\n%s", got.code, out)
					}
					return
				}
				if got.code != verifyrunExitCouldNot || !strings.Contains(out, "ASSAY_X") ||
					!strings.Contains(out, "not permission to run it outside admission") {
					t.Fatalf("admitted row that reads the inherited ASSAY_X gave exit %d, want could-not-run (exit %d) naming it:\n%s",
						got.code, verifyrunExitCouldNot, out)
				}
			})
		}
	}
}
