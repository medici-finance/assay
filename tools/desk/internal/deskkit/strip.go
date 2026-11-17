package deskkit

import "regexp"

// controlSeq matches the two classes of terminal-active bytes that must never survive
// into a rendered board line:
//
//   - a CSI escape sequence (ESC '[' ... final byte) — colour, cursor moves, and the
//     erase/scroll codes that let a crafted PR title rewrite lines ABOVE it;
//   - any other C0 control character (U+0000-U+001F) except tab and newline, including a
//     bare ESC and carriage return (line-overwrite), and DEL.
//
// The character class stops at DEL (U+007F): a C1 control (U+0080-U+009F) is NOT removed,
// and neither is a bidi format character. Text that may carry either is
// quoted for display instead (DisplayPath).
//
// Tab and newline are kept: the renderers use them for layout and neither can move the
// cursor backwards.
var controlSeq = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|[\x00-\x08\x0b-\x1f\x7f]")

// StripControl removes ANSI escape sequences and control characters from public-origin
// text (PR/issue titles) before it is printed as ordinary board text.
//
// The quarantine lanes render untrusted text through inert(), which quotes it; the
// ACTIONABLE lanes print titles raw, so a title is a live injection channel into the
// operator's terminal — and into any agent reading the board — unless it is sanitized.
// StripControl is that sanitizer. It is deliberately shared by deskboard, issueboard and
// the audit log rather than reimplemented per command.
func StripControl(s string) string {
	return controlSeq.ReplaceAllString(s, "")
}
