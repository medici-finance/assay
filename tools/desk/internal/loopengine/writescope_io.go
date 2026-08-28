package loopengine

// writescope_io.go — the OFFLINE, best-effort readers that turn a repo root into the
// in-flight-claim universe for the advisory overlap warning (writescope.go). Kept apart from
// the pure derivation/overlap logic because these touch git and the filesystem.
//
// OFFLINE ENVELOPE. The only git this runs is `git for-each-ref refs/dispatch/*` against LOCAL
// refs — never `git ls-remote`, never a fetch. No network is contacted. Every failure (not a
// git repo, git absent, an unreadable brief) degrades to "no in-flight items": the warning is
// advisory, so an unreadable claim universe prints nothing rather than failing a plan.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InFlightClaimScopes reads the root repo's live `refs/dispatch/*` dispatch claims and resolves
// each to the brief it names under the same root, returning one Item per resolved claim
// carrying the brief's derived write-scopes. This is the "overlap universe" for the advisory
// warning: the items already claimed for this root. Claims that resolve to no brief under this
// root are dropped (their scopes are unknowable here). Best-effort and offline: any failure
// returns nil.
func InFlightClaimScopes(root string) []Item {
	keys := DispatchClaimKeys(root)
	if len(keys) == 0 {
		return nil
	}
	briefs := briefsUnderRoot(root)
	if len(briefs) == 0 {
		return nil
	}
	var out []Item
	for _, b := range briefs {
		if claimKeyMatches(keys, b.suffix) {
			out = append(out, Item{ID: b.id, WriteScopes: b.scopes})
		}
	}
	return out
}

// DispatchClaimKeys returns the last path segment of every LOCAL `refs/dispatch/*` ref in the
// root git repo — the claim keys (`<repo>--<stream>--<NN>`, `<stream>--<NN>`, `<repo>--issue-<NN>`).
// Offline (local refs only). Returns nil on any error.
func DispatchClaimKeys(root string) []string {
	cmd := exec.Command("git", "-C", root, "for-each-ref", "--format=%(refname)", "refs/dispatch/")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var keys []string
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		key := ln[strings.LastIndexByte(ln, '/')+1:]
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// briefRef is a brief resolved under a root with its derived scopes and claim-key suffix.
type briefRef struct {
	id     string // stream/num
	suffix string // stream--num — the claim-key tail a dispatch ref carries
	scopes WriteScopeSet
}

// briefsUnderRoot globs every brief under <root>/docs/streams/*/brief-*.md and derives each
// one's id, claim-key suffix, and write-scopes.
func briefsUnderRoot(root string) []briefRef {
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", "*", "brief-*.md"))
	var out []briefRef
	for _, m := range matches {
		rel, err := filepath.Rel(filepath.Join(root, "docs", "streams"), m)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			continue
		}
		stream := parts[0]
		num := briefNum(parts[len(parts)-1])
		if num == "" {
			continue
		}
		raw, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		out = append(out, briefRef{
			id:     stream + "/" + num,
			suffix: stream + "--" + num,
			scopes: DeriveWriteScopes(string(raw)),
		})
	}
	return out
}

// briefNum extracts <num> from a `brief-<num>-<slug>.md` filename, keeping the `issue-<NN>`
// shape together.
func briefNum(base string) string {
	s := strings.TrimPrefix(base, "brief-")
	if s == base {
		return ""
	}
	s = strings.TrimSuffix(s, ".md")
	if strings.HasPrefix(s, "issue-") {
		rest := strings.TrimPrefix(s, "issue-")
		if i := strings.IndexByte(rest, '-'); i >= 0 {
			return "issue-" + rest[:i]
		}
		return "issue-" + rest
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		return s[:i]
	}
	return s
}

// claimKeyMatches reports whether any live claim key names the brief with the given
// `stream--num` suffix — exactly, or with a mandatory `<repo>--` prefix. Matching against known
// briefs (rather than parsing the key) is robust to whether the key carries a repo prefix.
func claimKeyMatches(keys []string, suffix string) bool {
	for _, k := range keys {
		if k == suffix || strings.HasSuffix(k, "--"+suffix) {
			return true
		}
	}
	return false
}
