package main

import "strings"

// cellenvpath.go — how a PATH survives a trip through cell.env on Windows.
//
// cell.env is a shell-assignment file: bash sources it (the oracle), and the Go loader
// (parseCellEnv) reads it with the same quoting rules. In that grammar an unquoted `\` is an
// escape, so a native Windows path written bare — `CELL_REPO=C:\src\x` — reads back as
// `C:srcx`, and a CELL_ROOTS entry stops being absolute. Two halves close that:
//
//   - WRITE: `cellctl new` emits every path it was handed with forward slashes on Windows
//     (cellEnvPathFor / cellEnvRootsFor). `C:/src/x` is a path every Win32 API accepts and that
//     no reader — bash or Go — treats as an escape, so a scaffolded file round-trips everywhere.
//   - READ: on Windows the loader keeps a `\` that precedes a byte bashQuote never escapes there
//     (cellEnvEscapableFor), so a hand-edited or `cellctl set` value in native form still loads
//     and every value the package's own %q writer emits still reads back as written.
//
// Every helper takes the target goos explicitly, so the Windows rules are table-tested on any
// host; production callers pass runtime.GOOS. On every other goos each one is the identity —
// the bytes the parity harness diffs against the shell oracle do not move.

// cellEnvPathFor is one path as `cellctl new` writes it into cell.env for goos.
func cellEnvPathFor(goos, p string) string {
	if goos != "windows" {
		return p
	}
	return strings.ReplaceAll(p, `\`, "/")
}

// cellEnvRootsFor is a CELL_ROOTS value (`<owner>/<repo>=<abs path>,...`) as `cellctl new`
// writes it for goos: only the PATH half of each entry is rewritten, so a malformed name
// (`owner\repo`) stays malformed for rootsValid to refuse rather than being repaired into a
// valid-looking one. Entry separators (`,` and whitespace) are preserved byte for byte.
func cellEnvRootsFor(goos, roots string) string {
	if goos != "windows" {
		return roots
	}
	var b strings.Builder
	inPath := false
	for i := 0; i < len(roots); i++ {
		c := roots[i]
		switch {
		case c == ',' || c == ' ' || c == '\t' || c == '\n':
			inPath = false
		case c == '=' && !inPath:
			inPath = true
		case c == '\\' && inPath:
			c = '/'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// cellEnvEscapableFor reports whether an unquoted `\` followed by next is a shell escape for
// goos; wordStart says the `\` is the first byte of the value. Off Windows it always is (bash
// semantics, which the parity harness pins).
//
// On Windows the rule is DERIVED from the writer, not kept as a second list: the `\` is an escape
// exactly when next is a byte bashQuote (bash's own `printf %q`) would backslash-escape there —
// any ASCII byte outside bashQuoteSafe, plus `~` and `#` in word-initial position. So every value
// the package writes loads back unchanged (TestCellEnvBashQuoteRoundTrip). Before a safe byte
// (`#%+-./:=@_~`, a letter, a digit, mid-value), before a non-ASCII byte, or at the end of the
// value, the `\` is a path separator and stays: `C:\src\x` loads as written.
//
// The cost, for a NATIVE path hand-written unquoted: a `\` directly before any other byte
// (space, `(`, `,`, `$`, `{`, `!`, `&`, `[`, …) still reads as an escape, so `C:\{guid}` loads as
// `C:{guid}`. Write such a path with forward slashes, or single-quote it (`'C:\{guid}'`).
func cellEnvEscapableFor(goos string, next byte, wordStart bool) bool {
	if goos != "windows" {
		return true
	}
	if next >= 0x80 {
		return false
	}
	return !bashQuoteSafe[next] || (wordStart && (next == '~' || next == '#'))
}
