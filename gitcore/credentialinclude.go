package gitcore

import (
	"bytes"
	"io"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	format "github.com/go-git/go-git/v5/plumbing/format/config"
)

// credentialIncludeLimit bounds the bytes read from an included file. The file
// actions/checkout writes is one header line and one key line.
const credentialIncludeLimit = 4 << 10

// The two line forms `git config --file <f> http.<url>.extraheader <v>` writes, and
// nothing else. Neither admits a quote, backslash, comment character, control byte
// or surrounding whitespace, so Git and the go-git decoder cannot read either line
// as anything but this key: no continuation, no quoting, no second key.
var (
	credentialHeaderLine = regexp.MustCompile(`^\[http "[^"\\\x00-\x20\x7f]+"\]$`)
	credentialValueLine  = regexp.MustCompile(`^\textraheader = [^"\\#;\x00-\x1f\x7f]*$`)
)

// credentialIncludeReason reports why the includes in raw (the repository config
// as written) are not the shape actions/checkout persists its push credential
// through, or "" when they are. The library reader merges no include, so it is
// accepted only when every file Git may merge through it holds nothing but
// http.<url>.extraheader keys, which change how Git sends an HTTP request and
// never which objects, refs, worktree or alternates a read resolves.
//
// Accepted: includeIf "gitdir:<path>" and includeIf "gitdir:<path>/worktrees/*"
// entries, <path> a clean absolute literal, each holding only absolute clean path
// keys, at least one naming this repository's git directory. An entry that may
// match is read: its target must be a plain file whose whole text is
// credentialHeaderLine and credentialValueLine lines. A target behind an entry Git
// provably does not match (conditionMayMatch) is never opened, as Git never opens
// it (checkout writes container-workspace entries that name no host repository).
//
// Refused: an unconditional include, any other condition, any other key in an
// includeIf section, a relative or unclean path, a missing, unreadable, symlinked
// or non-regular target, a nested include, and any other key in a target. The
// reason never quotes a path or included text: the target holds a credential.
func (r *Repo) credentialIncludeReason(raw *format.Config) string {
	if runtime.GOOS == "windows" {
		return "include conditions are not modelled on this platform"
	}
	texts, err := gitDirTexts(r.dir)
	if err != nil {
		return "the git directory an include condition names could not be resolved"
	}
	matched := 0
	for _, section := range raw.Sections {
		if section.IsName("include") {
			return "an unconditional include"
		}
		if !section.IsName("includeIf") {
			continue
		}
		if len(section.Options) != 0 {
			return "an includeIf key without a condition"
		}
		for _, sub := range section.Subsections {
			dir, worktrees, ok := checkoutCondition(sub.Name)
			if !ok {
				return "an include condition other than a literal gitdir path"
			}
			if len(sub.Options) == 0 {
				return "an includeIf entry without a path"
			}
			for _, option := range sub.Options {
				if !option.IsKey("path") {
					return "an includeIf entry carries a key other than path"
				}
				if !cleanAbsolute(option.Value) {
					return "an include path is not a clean absolute path"
				}
			}
			if !conditionMayMatch(dir, worktrees, texts) {
				continue
			}
			for _, option := range sub.Options {
				if why := credentialOnlyFile(option.Value); why != "" {
					return why
				}
			}
			matched++
		}
	}
	if matched == 0 {
		return "no include condition names this repository"
	}
	return ""
}

// checkoutCondition decodes the two condition spellings actions/checkout writes:
// gitdir:<dir> and gitdir:<dir>/worktrees/*. Any other grammar (gitdir/i:,
// onbranch:, hasconfig:, a relative, "~", "./" or trailing-slash pattern, or a
// glob anywhere else) is refused rather than evaluated. The root "/" is refused
// too: it is clean, but it ends in "/", and Git expands such a pattern to "/**",
// which matches every git directory.
func checkoutCondition(condition string) (dir string, worktrees, ok bool) {
	dir, ok = strings.CutPrefix(condition, "gitdir:")
	if !ok {
		return "", false, false
	}
	dir, worktrees = strings.CutSuffix(dir, "/worktrees/*")
	if dir == "/" || !cleanAbsolute(dir) || strings.ContainsAny(dir, "*?[\\") {
		return "", false, false
	}
	return dir, worktrees, true
}

func cleanAbsolute(p string) bool {
	return strings.HasPrefix(p, "/") && path.Clean(p) == p
}

// conditionMayMatch over-approximates Git's match of a literal gitdir pattern: a
// case-only difference still counts as a match (see originIncludes.match), and a
// non-ASCII byte in the pattern or in any spelling counts as a match, as it does
// in wildmatchMay (Git may spell such a path in another Unicode form than Go
// does), so a target is skipped only when Git provably never opens it. "*" in the
// worktrees form matches any run of non-slash bytes, the empty run included.
func conditionMayMatch(dir string, worktrees bool, texts []string) bool {
	if !isASCII(dir) {
		return true
	}
	for _, text := range texts {
		if !isASCII(text) {
			return true
		}
		if !worktrees {
			if strings.EqualFold(text, dir) {
				return true
			}
			continue
		}
		prefix := dir + "/worktrees/"
		if len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix) && !strings.Contains(text[len(prefix):], "/") {
			return true
		}
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// openIncludeTarget opens an include target; a test stands in for it to swap the
// file between the Lstat and the open.
var openIncludeTarget = os.Open

// credentialOnlyFile reads target once and reports why it is not a credential-only
// config file, or "".
func credentialOnlyFile(target string) string {
	before, err := os.Lstat(target)
	if err != nil {
		return "an include target is missing or unreadable"
	}
	if !before.Mode().IsRegular() {
		return "an include target is not a plain file"
	}
	f, err := openIncludeTarget(target)
	if err != nil {
		return "an include target is missing or unreadable"
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "an include target changed while it was read"
	}
	// One fixed buffer, cleared on return, so the credential is held in one place.
	buf := make([]byte, credentialIncludeLimit+1)
	defer clear(buf)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "an include target is missing or unreadable"
	}
	if n > credentialIncludeLimit {
		return "an include target is larger than a credential file"
	}
	if !credentialOnlyText(buf[:n]) {
		return "an include target holds config other than http.<url>.extraheader"
	}
	return ""
}

// credentialOnlyText reports whether data is one or more [http "<url>"] headers,
// each followed by extraheader lines, every line newline-terminated. It works on
// the bytes as read and makes no copy of them.
func credentialOnlyText(data []byte) bool {
	text, ok := bytes.CutSuffix(data, []byte("\n"))
	if !ok {
		return false
	}
	header, keys := false, 0
	for _, line := range bytes.Split(text, []byte("\n")) {
		switch {
		case credentialHeaderLine.Match(line):
			header = true
		case header && credentialValueLine.Match(line):
			keys++
		default:
			return false
		}
	}
	return keys > 0
}
