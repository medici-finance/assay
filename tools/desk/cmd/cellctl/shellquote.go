package main

import (
	"fmt"
	"strings"
)

// bashQuote reproduces bash's `printf %q` byte for byte. It is not a convenience: the [plan]
// argv line and the scrubbed launch's command line are both built with %q in the oracle, and the
// parity harness diffs them character by character, so any difference in the quoting rule is a
// divergence.
//
// The rule, derived from bash itself rather than from memory:
//   - the empty string renders as a pair of single quotes
//   - a byte in bashQuoteSafe passes through, EXCEPT a leading '~' or '#', which bash escapes
//     because they are only special in word-initial position
//   - a control byte (or DEL, or a high byte) forces the whole word into ANSI-C form $'...'
//   - every other byte is backslash-escaped
var bashQuoteSafe = map[byte]bool{}

func init() {
	// Exactly the set `printf %q x<c>x` leaves untouched, enumerated from bash:
	//   #%+-./0-9:=@A-Z_a-z~
	for _, c := range []byte("#%+-./:=@_~") {
		bashQuoteSafe[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		bashQuoteSafe[c] = true
	}
	for c := byte('A'); c <= 'Z'; c++ {
		bashQuoteSafe[c] = true
	}
	for c := byte('a'); c <= 'z'; c++ {
		bashQuoteSafe[c] = true
	}
}

func needsANSIC(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

func bashQuote(s string) string {
	if s == "" {
		return "''"
	}
	if needsANSIC(s) {
		var b strings.Builder
		b.WriteString("$'")
		for i := 0; i < len(s); i++ {
			c := s[i]
			switch c {
			case '\a':
				b.WriteString(`\a`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			case '\v':
				b.WriteString(`\v`)
			case '\\':
				b.WriteString(`\\`)
			case '\'':
				b.WriteString(`\'`)
			default:
				if c < 0x20 || c == 0x7f {
					fmt.Fprintf(&b, `\%03o`, c)
				} else {
					b.WriteByte(c)
				}
			}
		}
		b.WriteString("'")
		return b.String()
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if bashQuoteSafe[c] && !(i == 0 && (c == '~' || c == '#')) {
			b.WriteByte(c)
			continue
		}
		// A multibyte character passes through whole: bash escapes BYTES, not runes, only in a
		// single-byte locale, and splitting a UTF-8 sequence with backslashes would corrupt a
		// value neither implementation is entitled to change.
		if c >= 0x80 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('\\')
		b.WriteByte(c)
	}
	return b.String()
}
