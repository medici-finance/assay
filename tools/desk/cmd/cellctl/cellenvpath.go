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
//   - READ: on Windows the loader keeps a `\` that precedes an ordinary character literal
//     (cellEnvEscapableFor), so a hand-edited or `cellctl set` value in native form still loads.
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

// cellEnvWinEscapable is the set of bytes an UNQUOTED `\` still escapes on Windows — exactly
// the ones whose backslash form is how a shell writer (bash's own `printf %q`, bashQuote) spells
// them, so `C:\\x` and `a\ b` keep their bash meaning. Before anything else the backslash is a
// path separator and stays.
const cellEnvWinEscapable = "\\\"'$` \t"

// cellEnvEscapableFor reports whether an unquoted `\` followed by next is a shell escape for
// goos. Off Windows it always is (bash semantics, which the parity harness pins).
func cellEnvEscapableFor(goos string, next byte) bool {
	if goos != "windows" {
		return true
	}
	return strings.IndexByte(cellEnvWinEscapable, next) >= 0
}
