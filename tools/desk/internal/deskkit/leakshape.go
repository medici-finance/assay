package deskkit

// leakshape.go — the REGISTRY-INDEPENDENT shape stage of the disclosure sweep.
//
// WHY A SECOND STAGE. The token stage (the S2 disclosure sweep) certifies a tree when no
// REGISTERED token matches. A registry is only as complete as its list: an identity that is
// present in the tree but absent from the list certifies green, and nothing downstream of
// the registry can notice, because every later step trusts the registry's silence. This
// stage looks for the SHAPE of a leak instead of a listed value, so it needs no registry at
// all and cannot be blinded by one that is incomplete:
//
//   - home-path         an absolute per-user home directory: a users-directory prefix
//                       (the macOS one, the Linux one, or a Windows drive's) followed by an
//                       account name
//   - noreply-id-email  a numeric-id noreply email: digits, a plus sign, a login, then the
//                       forge noreply domain
//   - ssh-host-alias    an SSH remote whose host is not a forge host itself — a host ALIAS
//                       out of somebody's local ssh config, in scp form or ssh:// URL form
//   - tilde-root        a tilde-prefixed home-relative working root (a non-dot directory
//                       or file directly under the home directory; dot-config paths are
//                       tool conventions, not working roots)
//
// THE STAGE NEVER REPLACES ANOTHER. CertifyStages is the certify decision, and it is the AND
// of every stage's verdict: a token-clean tree that is shape-red does not certify, and
// neither does a shape-clean tree that is token-red. Nothing here relaxes the token stage.
//
// THREE STATES. checked-clean / checked-failed / could-not-check, the same words the token
// stage uses. could-not-check is never clean: an empty tree, a walk error, an allow file that
// does not load, or a rule that fails its own positive control (each rule carries a canary it
// MUST match before its silence over the tree means anything) all yield could-not-check.
//
// NO `\b`. Every pattern uses explicit character classes and anchors only. RE2 does accept
// the word-boundary escape, but other engines this repo's patterns get copied into (git
// grep's ERE among them) silently accept it and match nothing, which reads a dirty tree as
// clean. A test pins the absence.
//
// THE ALLOW FILE. A documented example — a test fixture that needs a home path to exercise a
// path guard, a placeholder remote in a doc comment — is a reviewed exception, not a blind
// spot. Each `allow:` entry names ONE file (exact repo-relative path, no directories, no
// globs), ONE shape, the exact matched values it permits, and a non-empty reason. A value is
// named by the SHA-256 digest of its matched text, never by the text: the allow file then
// carries no shape itself, so it is swept like every other file (no self-exclusion) and an
// allow entry never re-publishes what it permits. A value the entry does not list still
// fails, in that file or any other, so a real identity cannot ride in on a file that already
// holds an example. An entry that no longer matches anything is STALE and is reported, so
// the list cannot rot into a blanket.
//
// Matched VALUES are never echoed in a failure message by the gate: ShapeHit.Location is
// path:line, the shape and the value's digest — exactly what an allow entry needs, and
// nothing that copies the leak into a CI log.
//
// Register row: R-tree-leak-shape (docs/contracts.md).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"gopkg.in/yaml.v3"
)

// Sweep stage verdicts. The strings are the token stage's own words, so a combined report
// reads one vocabulary.
const (
	StageClean         = "checked-clean"
	StageFailed        = "checked-failed"
	StageCouldNotCheck = "could-not-check"
)

// LeakShape names one registry-independent leak shape.
type LeakShape string

// The shapes the stage detects.
const (
	ShapeHomePath    LeakShape = "home-path"
	ShapeNoreplyMail LeakShape = "noreply-id-email"
	ShapeSSHAlias    LeakShape = "ssh-host-alias"
	ShapeTildeRoot   LeakShape = "tilde-root"
)

// AllLeakShapes is every shape the stage detects, in report order.
var AllLeakShapes = []LeakShape{ShapeHomePath, ShapeNoreplyMail, ShapeSSHAlias, ShapeTildeRoot}

// forgeSSHHosts are the hosts an SSH remote may name without being an alias: the canonical
// hosts of the public forges, including their documented SSH-over-443 endpoints. A public
// forge's own host names no machine and no person, so it discloses nothing. Compared
// case-insensitively. Anything else in host position is a local alias, an internal host or
// a placeholder, and needs an allow entry.
var forgeSSHHosts = map[string]bool{
	"github.com":           true,
	"ssh.github.com":       true,
	"gitlab.com":           true,
	"altssh.gitlab.com":    true,
	"bitbucket.org":        true,
	"altssh.bitbucket.org": true,
}

// shapeRule is one matcher. group is the submatch index whose text is the hit's VALUE;
// hostGroup, when non-zero, is the submatch holding an SSH host, and a hit whose host is a
// forge host is dropped. canary is a sample the rule MUST match, built from fragments so the
// literal never sits in this source for the stage to find.
type shapeRule struct {
	shape     LeakShape
	re        *regexp.Regexp
	group     int
	hostGroup int
	prefilter []string // the body must contain one of these (lower-cased) for the regexp to run
	canary    string
}

// The default rules, one variable each so a rule is dropped from leakShapeRules by name.
// Left boundaries are explicit negated classes (RE2 has no look-behind, and `\b` is banned),
// so the boundary character is consumed outside the value group.

// shapeRuleHome matches an absolute per-user home directory.
var shapeRuleHome = shapeRule{
	shape: ShapeHomePath,
	// An absolute home directory: the macOS or Linux users-directory prefix, or a Windows
	// drive's (one or two backslashes, so a Go/JSON-escaped spelling matches too), then an
	// account name that does not end in a dot. The left boundary keeps a URL path segment
	// (a "home" segment deep in a web URL's path) from matching, while still matching a
	// file:// URL.
	re:        regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])((?:/(?:Users|home)/|[A-Za-z]:\\{1,2}Users\\{1,2})[A-Za-z0-9_](?:[A-Za-z0-9_.-]*[A-Za-z0-9_-])?)`),
	group:     1,
	prefilter: []string{"/users/", "/home/", "users\\"},
	canary:    "cd " + "/Us" + "ers/" + "example-user" + "/src",
}

// shapeRuleNoreply matches a numeric-id forge noreply email.
var shapeRuleNoreply = shapeRule{
	shape: ShapeNoreplyMail,
	// Digits, a plus sign, a login (an optional bot suffix), then the forge noreply
	// domain, case-insensitively on the domain.
	re:        regexp.MustCompile(`([0-9]+\+[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\[bot\])?@(?i:users\.noreply\.github\.com))`),
	group:     1,
	prefilter: []string{"noreply"},
	canary:    "author " + "1234567" + "+" + "example-login" + "@" + "users.noreply" + ".github.com",
}

// shapeRuleSSHScp matches an scp-form SSH remote.
var shapeRuleSSHScp = shapeRule{
	shape: ShapeSSHAlias,
	// scp form: the git user, an at sign, a host, a colon, then the path.
	re:        regexp.MustCompile(`(?:^|[^A-Za-z0-9._-])(git@([A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?)):`),
	group:     1,
	hostGroup: 2,
	prefilter: []string{"git@"},
	canary:    "origin " + "git" + "@" + "example-alias" + ":" + "owner/repo.git",
}

// shapeRuleSSHURL matches an ssh:// URL remote.
var shapeRuleSSHURL = shapeRule{
	shape: ShapeSSHAlias,
	// URL form: the ssh scheme, an optional user, a host (a name or a bracketed IPv6
	// literal), an optional port, then the path.
	re:        regexp.MustCompile(`(ssh://(?:[A-Za-z0-9._-]+@)?([A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?|\[[0-9A-Fa-f:.]+\]))`),
	group:     1,
	hostGroup: 2,
	prefilter: []string{"ssh://"},
	canary:    "origin " + "ssh" + "://" + "git@" + "example-alias" + "/owner/repo.git",
}

// shapeRuleTilde matches a tilde-prefixed home-relative working root.
var shapeRuleTilde = shapeRule{
	shape: ShapeTildeRoot,
	// A tilde-prefixed home-relative root whose first segment is not a dot entry.
	re:        regexp.MustCompile(`(?:^|[^A-Za-z0-9_~/.-])(~/[A-Za-z0-9_](?:[A-Za-z0-9_.-]*[A-Za-z0-9_-])?)`),
	group:     1,
	prefilter: []string{"~/"},
	canary:    "cd " + "~" + "/" + "example-work" + "/repo",
}

// leakShapeRules is the default rule set.
var leakShapeRules = []shapeRule{
	shapeRuleHome,
	shapeRuleNoreply,
	shapeRuleSSHScp,
	shapeRuleSSHURL,
	shapeRuleTilde,
}

// ShapeHit is one shape match the allow file does not cover.
type ShapeHit struct {
	Path  string    // repo-relative, slash-separated
	Line  int       // 1-based; 0 for a symlink target
	Shape LeakShape // which shape matched
	Value string    // the matched text — for tests and the allow file, never for a CI log
}

// Digest is the hit's allow-file token: the SHA-256 of the matched text, lower-case hex.
func (h ShapeHit) Digest() string { return ShapeValueDigest(h.Value) }

// Location is the hit as path:line, its shape and its value's digest, WITHOUT the matched
// value.
func (h ShapeHit) Location() string {
	if h.Line == 0 {
		return fmt.Sprintf("%s (symlink target) [%s] sha256:%s", h.Path, h.Shape, h.Digest())
	}
	return fmt.Sprintf("%s:%d [%s] sha256:%s", h.Path, h.Line, h.Shape, h.Digest())
}

// ShapeValueDigest is how an allow entry names a value: the SHA-256 of its exact matched
// text, as 64 lower-case hex digits.
func ShapeValueDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// ShapeAllow is one reviewed `allow:` exception. SHA256 lists the digests
// (ShapeValueDigest) of the exact values the entry permits.
type ShapeAllow struct {
	Path   string   `yaml:"path"`
	Shape  string   `yaml:"shape"`
	SHA256 []string `yaml:"sha256"`
	Reason string   `yaml:"reason"`
}

// ShapeAllowList is a loaded allow file.
type ShapeAllowList struct {
	Entries []ShapeAllow
}

var reSHA256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

type shapeAllowFile struct {
	Allow []ShapeAllow `yaml:"allow"`
}

// LoadShapeAllow reads and validates the allow file at file; rel is its repo-relative path.
// Any defect — an unreadable file, an unknown key, an entry missing its path, shape, digests
// or reason, a digest that is not 64 lower-case hex, an unknown shape, a non-canonical path,
// a duplicated (path, shape) — is an error, and a caller must read an error as could-not-check, never as an empty list.
func LoadShapeAllow(file, rel string) (ShapeAllowList, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return ShapeAllowList{}, err
	}
	var doc shapeAllowFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return ShapeAllowList{}, fmt.Errorf("%s: %w", rel, err)
	}
	list := ShapeAllowList{Entries: doc.Allow}
	if err := list.validate(); err != nil {
		return ShapeAllowList{}, fmt.Errorf("%s: %w", rel, err)
	}
	return list, nil
}

func knownShape(s string) bool {
	for _, k := range AllLeakShapes {
		if string(k) == s {
			return true
		}
	}
	return false
}

func (l ShapeAllowList) validate() error {
	seen := map[string]bool{}
	var problems []string
	for i, e := range l.Entries {
		at := fmt.Sprintf("allow[%d]", i)
		switch {
		case e.Path == "":
			problems = append(problems, at+": no path")
			continue
		case path.IsAbs(e.Path) || strings.Contains(e.Path, "\\") || path.Clean(e.Path) != e.Path ||
			e.Path == "." || strings.HasPrefix(e.Path, "../") || strings.ContainsAny(e.Path, "*?["):
			problems = append(problems, fmt.Sprintf("%s: path %q is not one exact repo-relative file", at, e.Path))
		}
		if !knownShape(e.Shape) {
			problems = append(problems, fmt.Sprintf("%s (%s): unknown shape %q", at, e.Path, e.Shape))
		}
		if strings.TrimSpace(e.Reason) == "" {
			problems = append(problems, fmt.Sprintf("%s (%s): no reason — an unexplained exception is a blind spot", at, e.Path))
		}
		if len(e.SHA256) == 0 {
			problems = append(problems, fmt.Sprintf("%s (%s): no sha256 — an entry permits only the exact values it lists", at, e.Path))
		}
		for _, d := range e.SHA256 {
			if !reSHA256Hex.MatchString(d) {
				problems = append(problems, fmt.Sprintf("%s (%s): sha256 %q is not 64 lower-case hex digits", at, e.Path, d))
			}
		}
		key := e.Path + "\x00" + e.Shape
		if seen[key] {
			problems = append(problems, fmt.Sprintf("%s: duplicate entry for %s [%s]", at, e.Path, e.Shape))
		}
		seen[key] = true
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// ShapeReport is one run of the shape stage.
type ShapeReport struct {
	State   string
	Hits    []ShapeHit // unallowed matches, sorted by path then line
	Allowed int        // matches an allow entry covered
	Stale   []string   // allow entries (path [shape] value) that matched nothing
	Scanned int        // text files read
	Skipped int        // files skipped as binary
	Problem string     // why the state is could-not-check, when it is
}

// ShapeSweep runs the default shape rules over root with the given allow list.
func ShapeSweep(root string, allow ShapeAllowList) (ShapeReport, error) {
	return shapeSweep(root, allow, leakShapeRules)
}

// shapeSelfTest is each rule's positive control: a rule that does not match its own canary
// is broken, and its silence over a tree proves nothing.
func shapeSelfTest(rules []shapeRule) []string {
	var broken []string
	for _, r := range rules {
		if len(r.match(r.canary)) == 0 {
			broken = append(broken, string(r.shape)+": "+r.re.String())
		}
	}
	return broken
}

type shapeMatch struct {
	line  int
	value string
}

// match returns every match of r in body, forge-host remotes dropped.
func (r shapeRule) match(body string) []shapeMatch {
	if len(r.prefilter) > 0 {
		lower := strings.ToLower(body)
		hit := false
		for _, p := range r.prefilter {
			if strings.Contains(lower, p) {
				hit = true
				break
			}
		}
		if !hit {
			return nil
		}
	}
	var out []shapeMatch
	for _, m := range r.re.FindAllStringSubmatchIndex(body, -1) {
		if m[2*r.group] < 0 {
			continue
		}
		if r.hostGroup > 0 && m[2*r.hostGroup] >= 0 &&
			forgeSSHHosts[strings.ToLower(body[m[2*r.hostGroup]:m[2*r.hostGroup+1]])] {
			continue
		}
		start := m[2*r.group]
		out = append(out, shapeMatch{
			line:  1 + strings.Count(body[:start], "\n"),
			value: body[start:m[2*r.group+1]],
		})
	}
	return out
}

// sweepText decodes a file body the way the token stage does: a BOM-prefixed UTF-16 file is
// text (not binary, despite its NUL bytes); anything else holding a NUL byte is binary and
// skipped.
func sweepText(raw []byte) (string, bool) {
	decode := func(b []byte, big bool) string {
		if len(b)%2 != 0 {
			b = b[:len(b)-1]
		}
		units := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			if big {
				units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
			} else {
				units = append(units, uint16(b[i+1])<<8|uint16(b[i]))
			}
		}
		return string(utf16.Decode(units))
	}
	switch {
	case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
		return decode(raw[2:], false), true
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		return decode(raw[2:], true), true
	case bytes.IndexByte(raw, 0) >= 0:
		return "", false
	}
	return string(raw), true
}

func shapeSweep(root string, allow ShapeAllowList, rules []shapeRule) (ShapeReport, error) {
	var rep ShapeReport
	cnc := func(why string) (ShapeReport, error) {
		rep.State, rep.Problem = StageCouldNotCheck, why
		return rep, nil
	}
	if err := allow.validate(); err != nil {
		return cnc("allow list invalid: " + err.Error())
	}
	if broken := shapeSelfTest(rules); len(broken) > 0 {
		return cnc("rule(s) failed their positive control: " + strings.Join(broken, "; "))
	}

	type allowKey struct{ path, shape, digest string }
	allowed := map[allowKey]bool{}
	used := map[allowKey]bool{}
	for _, e := range allow.Entries {
		for _, d := range e.SHA256 {
			allowed[allowKey{e.Path, e.Shape, d}] = true
		}
	}

	record := func(rel string, line int, shape LeakShape, value string) {
		k := allowKey{rel, string(shape), ShapeValueDigest(value)}
		if allowed[k] {
			used[k] = true
			rep.Allowed++
			return
		}
		rep.Hits = append(rep.Hits, ShapeHit{Path: rel, Line: line, Shape: shape, Value: value})
	}
	scan := func(rel, body string, symlink bool) {
		for _, r := range rules {
			for _, m := range r.match(body) {
				line := m.line
				if symlink {
					line = 0
				}
				record(rel, line, r.shape, m.value)
			}
		}
	}

	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			// Never-authored third-party and VCS trees, excluded by name exactly as the token
			// stage excludes them. No other exclusion exists: an authored file is swept.
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".git" {
			// A linked worktree's .git is a FILE pointing at the shared repository — VCS
			// plumbing, not authored content, so it is skipped like the directory form.
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			// The link's target is text checked into the tree; a link pointed into a home
			// directory discloses it as surely as file content would.
			if target, lerr := os.Readlink(p); lerr == nil {
				scan(rel, target, true)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		body, isText := sweepText(raw)
		if !isText {
			rep.Skipped++
			return nil
		}
		rep.Scanned++
		scan(rel, body, false)
		return nil
	})
	if err != nil {
		return cnc("walk failed: " + err.Error())
	}
	if rep.Scanned == 0 {
		return cnc("no text file was read under " + root + " — a zero-file sweep certifies nothing")
	}

	sort.Slice(rep.Hits, func(i, j int) bool {
		if rep.Hits[i].Path != rep.Hits[j].Path {
			return rep.Hits[i].Path < rep.Hits[j].Path
		}
		if rep.Hits[i].Line != rep.Hits[j].Line {
			return rep.Hits[i].Line < rep.Hits[j].Line
		}
		return rep.Hits[i].Shape < rep.Hits[j].Shape
	})
	for _, e := range allow.Entries {
		for _, d := range e.SHA256 {
			if !used[allowKey{e.Path, e.Shape, d}] {
				rep.Stale = append(rep.Stale, fmt.Sprintf("%s [%s] sha256:%s", e.Path, e.Shape, d))
			}
		}
	}
	sort.Strings(rep.Stale)
	if len(rep.Hits) > 0 {
		rep.State = StageFailed
	} else {
		rep.State = StageClean
	}
	return rep, nil
}

// CertifyStages is the certify decision over every stage's verdict: the AND of them. Any
// failed stage fails the tree; otherwise any could-not-check stage — or an unrecognised
// verdict, or no verdict at all — is could-not-check; only when every stage is clean is the
// tree clean. No stage can relax another.
func CertifyStages(states ...string) string {
	if len(states) == 0 {
		return StageCouldNotCheck
	}
	failed, unknown := false, false
	for _, s := range states {
		switch s {
		case StageClean:
		case StageFailed:
			failed = true
		default:
			unknown = true
		}
	}
	switch {
	case failed:
		return StageFailed
	case unknown:
		return StageCouldNotCheck
	}
	return StageClean
}
