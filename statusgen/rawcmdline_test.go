package main

import (
	"strings"
	"testing"
)

// A `cmd` Verify row authored in native-Windows syntax: findstr with a quoted
// search string and a backslash path. This is the exact shape from issue #1424
// that runs fine when typed at a prompt yet fails under verifyrun.
const cmdRowSample = `findstr /c:"a b" docs\streams\x\spec.md`

// escapeArgReplica is a byte-for-byte replica of Go's syscall.EscapeArg
// (src/syscall/exec_windows.go), which os/exec on Windows applies to EVERY argv
// element when SysProcAttr.CmdLine is unset. syscall.EscapeArg does not compile
// off Windows, so this replica lets the portable test show WHAT the default
// (pre-fix) path produced for the sample row — the nested, backslash-escaped
// quoting that `cmd /s /c` then fails to strip.
func escapeArgReplica(s string) string {
	if len(s) == 0 {
		return `""`
	}
	needsBackslash, hasSpace := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			needsBackslash = true
		case ' ', '\t':
			hasSpace = true
		}
	}
	if !needsBackslash && !hasSpace {
		return s
	}
	if !needsBackslash {
		return `"` + s + `"`
	}
	var b strings.Builder
	if hasSpace {
		b.WriteByte('"')
	}
	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte('\\')
		default:
			slashes = 0
		}
		b.WriteByte(c)
	}
	if hasSpace {
		for ; slashes > 0; slashes-- {
			b.WriteByte('\\')
		}
		b.WriteByte('"')
	}
	return b.String()
}

// TestWinCmdLine_CmdRow_RawSingleOuterPair pins the fix: the raw command line for
// a `cmd` row is the interpreter tokens verbatim followed by the row wrapped in
// exactly ONE outer double-quote pair, with the row's own quotes left untouched.
//
// Fail-first: on the pre-fix code winCmdLine does not exist, so this test does
// not compile; and once the builder is present, pointing it at the default
// os/exec escaping (see the documented sub-assertion below) makes the exact-match
// and no-`\"` assertions fail. Both were observed red before the raw builder
// landed (recorded in the PR body).
func TestWinCmdLine_CmdRow_RawSingleOuterPair(t *testing.T) {
	argv := []string{"cmd", "/d", "/s", "/c", cmdRowSample}
	got := winCmdLine(argv)

	want := `cmd /d /s /c "` + cmdRowSample + `"`
	if got != want {
		t.Fatalf("winCmdLine =\n  %q\nwant\n  %q", got, want)
	}

	// The row's own quotes must survive verbatim: no backslash-escaped quote may
	// appear, because `cmd /s /c` does not unescape one and findstr would then
	// reject its /c: argument. This is the property the default escaping breaks.
	if strings.Contains(got, `\"`) {
		t.Fatalf("raw command line carries a backslash-escaped quote (the #1424 defect): %q", got)
	}

	// Exactly one leading and one trailing quote wrap the row — the outer pair
	// `/s` strips — and the two inner quotes of /c:"a b" are those two only.
	if strings.Count(got, `"`) != 4 {
		t.Fatalf("expected 4 double quotes (one outer pair + the row's own pair), got %d in %q", strings.Count(got, `"`), got)
	}
}

// TestWinCmdLine_DiffersFromDefaultEscaping documents the mechanism the fix
// works around: os/exec's default per-argument escaping (syscall.EscapeArg)
// produces a DIFFERENT, broken command line for the same row — the inner quotes
// backslash-escaped — which `cmd /s /c` cannot restore. This is the "escaped
// argv vs the new raw line" comparison, made portable via escapeArgReplica.
func TestWinCmdLine_DiffersFromDefaultEscaping(t *testing.T) {
	// What the pre-fix path launched: prefix tokens (no spaces, unescaped) plus
	// the row run through EscapeArg.
	defaultLine := "cmd /d /s /c " + escapeArgReplica(cmdRowSample)

	// The default escaping DOES carry the nested backslash-escaped quotes that
	// break findstr — assert it, so the test fails loudly if a future Go changes
	// EscapeArg out from under this reasoning.
	if !strings.Contains(defaultLine, `\"`) {
		t.Fatalf("expected the default (EscapeArg) line to backslash-escape the row's quotes; got %q", defaultLine)
	}

	if got := winCmdLine([]string{"cmd", "/d", "/s", "/c", cmdRowSample}); got == defaultLine {
		t.Fatalf("raw builder must differ from the default escaping; both were %q", got)
	}
}

// TestWinCmdLine_Empty guards the degenerate input.
func TestWinCmdLine_Empty(t *testing.T) {
	if got := winCmdLine(nil); got != "" {
		t.Fatalf("winCmdLine(nil) = %q, want empty", got)
	}
}
