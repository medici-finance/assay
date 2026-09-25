package main

// state.go — what the two pollers share: the per-repo state files, the repo resolution order, the
// numeric knobs, the pacing clock and the rate-limit signature.
//
// Every piece here is the Go form of a line the bash oracles (plugins/assay/scripts/
// inbound-monitor.sh, pr-monitor.sh) already carry, and is kept BYTE-COMPATIBLE with it where a
// file or a line is observable from outside the process: the state-file NAMES (scanloop's arming
// read, ReadMonitorState, maps a slug to a file with the same rule), the state-file LINE FORMAT, and
// the sort order the lines are written in. A state dir the script seeded is a state dir this verb
// diffs against, and the reverse — that is what lets the two run side by side during the parity
// period without either one re-flooding the other.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// repoSlugRe is GitHub's own owner/name alphabet, anchored — the scripts' validation regexp.
var repoSlugRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// numericRe is the scripts' knob check: a non-negative integer, nothing else. A non-numeric value
// would poison the arithmetic guards and could silently disable a fail-closed check.
var numericRe = regexp.MustCompile(`^[0-9]+$`)

// errPrecondition is a precondition failure: the scripts' exit 1. Its message is printed to stderr
// verbatim, prefixed with the poller's name.
type errPrecondition struct{ msg string }

func (e *errPrecondition) Error() string { return e.msg }

func precondition(format string, a ...any) error {
	return &errPrecondition{msg: fmt.Sprintf(format, a...)}
}

// knob reads a numeric environment knob with the scripts' `${VAR:-default}` semantics (an empty
// value is the default) and their non-negative-integer check. name is the script's own variable
// name, so the refusal names the same thing the script's does.
func knob(env, name string, def int) (int, error) {
	raw := os.Getenv(env)
	if raw == "" {
		return def, nil
	}
	if !numericRe.MatchString(raw) {
		return 0, precondition("%s must be a non-negative integer, got '%s'", name, raw)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, precondition("%s must be a non-negative integer, got '%s'", name, raw)
	}
	return n, nil
}

// stateDirFrom resolves a poller's state dir: the named environment override when it is
// non-empty, else <os.TempDir()>/<leaf>. os.TempDir() is `${TMPDIR:-/tmp}` on unix — the scripts'
// own default, so the two resolve the same directory there — and %TMP%/%TEMP% on Windows, where no
// /tmp exists.
func stateDirFrom(env, leaf string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return filepath.Join(os.TempDir(), leaf)
}

// prepareStateDir is the scripts' `mkdir -p` + `-w` check.
func prepareStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return precondition("cannot create state dir '%s'", dir)
	}
	// Writability is PROVEN, not inferred from mode bits: mode bits are synthetic on Windows and
	// an ACL can deny a write the bits allow.
	f, err := os.CreateTemp(dir, ".writable-*")
	if err != nil {
		return precondition("state dir '%s' is not writable", dir)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// stateFileName maps a slug to its state file: slashes are the only reserved character and map to
// "__". It must stay byte-identical to the scripts' rule and to scanloop's ReadMonitorState, or the
// arming read silently reports every repo unseeded.
func stateFileName(slug string) string {
	return strings.ReplaceAll(slug, "/", "__") + ".state"
}

func statePath(dir, slug string) string { return filepath.Join(dir, stateFileName(slug)) }

// hasState is the seed marker: no file means this repo has never been polled.
func hasState(dir, slug string) bool {
	st, err := os.Stat(statePath(dir, slug))
	return err == nil && !st.IsDir()
}

// readStateLines returns the NON-EMPTY lines of a state file, in file order. It is the scripts'
// `grep -c .` count and the baseline their diff reads.
func readStateLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

// writeState replaces a state file with lines, one per line, newline-terminated — byte-identical
// to the scripts' `cp "$TMP_CUR" "$sf"` of a sorted keyset. It writes a sibling temp file and
// renames it over the old one, so a killed poll never leaves a half-written baseline behind.
func writeState(dir, slug string, lines []string) error {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	f, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, statePath(dir, slug)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// sortC is `LC_ALL=C sort`: byte order.
func sortC(lines []string) []string {
	out := append([]string(nil), lines...)
	sort.Strings(out)
	return out
}

// resolveRepos is the scripts' repo resolution order: the repo args, else ./.assay/repos.txt
// (blank and #-comment lines dropped), else the current checkout's `origin` remote. Entries are
// kept RAW — a line with leading whitespace is not trimmed into a valid slug, it is refused by the
// validation that follows, exactly as the script's `read -r` keeps it.
func resolveRepos(args []string) []string {
	var raw []string
	switch {
	case len(args) > 0:
		for _, a := range args {
			raw = append(raw, strings.Split(a, "\n")...)
		}
	default:
		reposTxt := filepath.Join(".", ".assay", "repos.txt")
		if st, err := os.Stat(reposTxt); err == nil && st.Mode().IsRegular() {
			// The file's presence decides the source, as the script's `-f` test does: an
			// unreadable one yields no repos (a precondition failure), never a silent fall
			// through to the origin remote.
			if b, rerr := os.ReadFile(reposTxt); rerr == nil {
				skip := regexp.MustCompile(`^[[:space:]]*(#|$)`)
				for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
					if !skip.MatchString(l) {
						raw = append(raw, l)
					}
				}
			}
		} else if slug := originSlug(); slug != "" {
			raw = append(raw, slug)
		}
	}
	var out []string
	for _, l := range raw {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// originSlug reads the current checkout's origin remote and reduces it to owner/name with the
// scripts' three rewrites. git is a local tool, not a forge CLI; an unreadable origin is "no
// origin", which the caller reports as no repos to query.
func originSlug() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	u := strings.TrimRight(string(out), "\r\n")
	if u == "" {
		return ""
	}
	u = strings.TrimPrefix(u, "git@github.com:")
	u = strings.TrimPrefix(u, "https://github.com/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// validateRepos refuses anything outside GitHub's owner/name alphabet before it reaches a request.
func validateRepos(repos []string) error {
	for _, r := range repos {
		if !repoSlugRe.MatchString(r) {
			return precondition("invalid repo '%s' (expected owner/name)", r)
		}
	}
	return nil
}

// sleepFn is the pacing clock, swapped in tests so a paced cycle costs no wall time.
var sleepFn = time.Sleep

func secondsOf(n int) time.Duration { return time.Duration(n) * time.Second }

// graphQLAnswerPrefix is how deskkit's open-changes read reports a GraphQL errors[] envelope (an
// HTTP 200 whose errors[] names the failure): Unverifiable("open-changes GraphQL error: <m1>; <m2>").
// What follows it is the forge's own messages — the text gh prints as `GraphQL: <m1>, <m2>`. The
// parity fixture testdata/parity/pr-graphql-envelope.json goes red if deskkit's wording moves.
const graphQLAnswerPrefix = "open-changes GraphQL error: "

// isRateLimited is the stop-on-limit test: a tripped limit must end the cycle, never be
// compounded by the remaining reads. It decides every failure the way the oracle scripts do —
// their is_ratelimit greps gh's stderr, which carries the forge's ANSWER and nothing the verb's own
// error text adds (the request path and so the repo slug, the fake's or proxy's host:port) — so the
// scripts' pattern (deskkit.GhRateLimitSignature) is applied to that answer text only:
//
//   - a non-2xx: `HTTP <status>: <message + errors[] lines>`, decided in deskkit
//     (ForgeAPIError.RateLimited; a 429 from any backend also counts);
//   - a GraphQL errors[] envelope: its messages;
//   - anything that is no forge answer — a dropped connection, a precondition, an unparseable or
//     empty body — never stops the cycle, as gh's "error connecting to …" never matches.
//
// No header is read: gh never prints one, so a Retry-After alone cannot stop a cycle the scripts
// would have continued (#1640 review F1).
//
// The one stderr text left out is the URL gh appends to an HTTP failure: gh's own endpoint on the
// configured host (`https://api.github.com/graphql`), never this read's repo or path. It could
// only matter on a GitHub host whose NAME contains 429 — there the scripts would stop on every
// failed read. That host-name case is the declared residue; testdata/parity pins every other
// class (headers, status codes, message and errors[] variants, GraphQL envelopes, the verb's own
// text, a dropped connection).
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var ae *deskkit.ForgeAPIError
	if errors.As(err, &ae) {
		return deskkit.IsForgeRateLimited(err)
	}
	var de *deskkit.DeskError
	if errors.As(err, &de) && de.Err == nil && strings.HasPrefix(de.Msg, graphQLAnswerPrefix) {
		return deskkit.GhRateLimitSignature.MatchString(strings.TrimPrefix(de.Msg, graphQLAnswerPrefix))
	}
	return false
}

// diagnostic renders a failed read's error for the one-line MONITOR-DEGRADED report: newlines
// folded to spaces, the way the scripts fold gh's stderr with `tr '\n' ' '`. The text is the
// forge's own error — never discarded, because it is the only thing that tells an operator their
// token expired or their identity is wrong.
func diagnostic(err error) string {
	if err == nil {
		return ""
	}
	var pe *errPrecondition
	if errors.As(err, &pe) {
		return pe.msg
	}
	return strings.ReplaceAll(strings.ReplaceAll(err.Error(), "\r", ""), "\n", " ")
}
