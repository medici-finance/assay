package main

// installidcache.go — the two facts that let a "reuse cached token" call cost NOTHING.
//
// THE DEFECT THIS CLOSES (#1036). The token cache path is `<role>-token-<installID>`, so
// until this file existed the installation id had to be resolved BEFORE the cache could be
// looked at — and resolving it means reading the App private key, signing a JWT and calling
// `GET /app/installations` to rediscover an installation that changes only when somebody
// installs or uninstalls the App. Measured on one operating desk host: 18,639 reuses
// against 122 mints in 20,000 audit rows, every one of those reuses paying an API round
// trip. The reuse was free of a MINT and never free of a CALL.
//
// THE TWO FACTS. (1) An owner SIDECAR beside each cached token, recording which App and
// which account that file was minted for — so a cached file can be recognised without
// knowing its installation id first. (2) An install-id CACHE per (App, account), so a cold
// token cache still costs one installations call a day rather than one per invocation.
//
// FAIL CLOSED, ALWAYS. Every acceptance below is a POSITIVE match of both the App name and
// the account, on a file whose mode and age were checked. Absence, ambiguity, a wrong mode,
// a malformed name, an unreadable file — every one of them falls through to authoritative
// resolution, which is exactly the code that ran before this file existed. A cache in front
// of a CREDENTIAL may never guess: handing back the right-shaped token for the wrong
// identity does not error, it authenticates as somebody else.

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// installIDMaxAge is how long a resolved installation id may be reused from disk. An
// installation id changes only when the App is installed on, or uninstalled from, an
// account — so a day is generous in the direction that matters, and a stale one is not a
// silent wrong answer: its only consumer is the token exchange, which fails closed against
// GitHub, and that failure invalidates the entry (see invalidateInstallIDCache).
const installIDMaxAge = 24 * time.Hour

// cacheNameOK reports whether s is safe to place in a cache FILE NAME and to compare as a
// recorded identity. GitHub logins and this tool's App names are alphanumeric with dots,
// dashes and underscores; anything else is not sanitised, escaped or rewritten — it simply
// means no cache is read and none is written, and resolution proceeds. A credential-plane
// cache is not a place to be clever about a path.
func cacheNameOK(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// allDigits reports whether s is a non-empty run of ASCII digits — the shape of every
// installation id this tool handles. It is the content check on both cache reads, so a
// file holding anything else is ignored rather than parsed.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// --- The owner sidecar ------------------------------------------------------------------

// ownerSidecarPath is the file recording which App and which account a cached token file
// belongs to. It is the same shape as permsPath's `.perms` sidecar, with one difference
// that matters: a missing or non-matching `.perms` degrades a preflight check to
// could-not-check, whereas a missing or non-matching `.owner` REFUSES the fast path.
func ownerSidecarPath(tokenPath string) string { return tokenPath + ".owner" }

// writeOwnerSidecar records "<appName> <owner>" beside the token cache, 0600. Like
// writePerms, a failure is NOT fatal: the token is the deliverable, and a missing sidecar
// costs one installations call next time rather than a wrong answer.
func writeOwnerSidecar(tokenPath, appName, owner string) {
	if !cacheNameOK(appName) || !cacheNameOK(owner) {
		return
	}
	_ = os.WriteFile(ownerSidecarPath(tokenPath), []byte(appName+" "+owner+"\n"), 0o600)
}

// readOwnerSidecar returns the App name and account a sidecar records. ok=false covers
// every doubt — missing, unreadable, empty, malformed — and every one of them means "fall
// through", never "assume it matches".
func readOwnerSidecar(tokenPath string) (appName, owner string, ok bool) {
	b, err := os.ReadFile(ownerSidecarPath(tokenPath))
	if err != nil {
		return "", "", false
	}
	fields := strings.Fields(string(b))
	if len(fields) != 2 {
		return "", "", false
	}
	if !cacheNameOK(fields[0]) || !cacheNameOK(fields[1]) {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// sidecarMatches reports whether tokenPath's sidecar records exactly this App and this
// account. Both halves are compared: the App half is what stops two Apps bound to one role
// on one account from reading each other's cache file, which is the guarantee the
// per-install filename suffix already provides and this probe must not undo.
func sidecarMatches(tokenPath, appName, owner string) bool {
	a, o, ok := readOwnerSidecar(tokenPath)
	if !ok {
		return false
	}
	return a == appName && strings.EqualFold(o, owner)
}

// --- The install-id cache ---------------------------------------------------------------

// installIDCacheName is the per-(App, account) cache file name. Both halves are in the
// NAME, so two Apps on one account, or one App on two accounts, can never collide.
func installIDCacheName(appName, owner string) (string, bool) {
	if !cacheNameOK(appName) || !cacheNameOK(owner) {
		return "", false
	}
	return appName + "-install-" + owner, true
}

// readInstallIDCache returns the cached installation id for (appName, owner) when there is
// one that is 0600, younger than installIDMaxAge, and holds nothing but digits.
func readInstallIDCache(appName, owner string) (string, bool) {
	name, ok := installIDCacheName(appName, owner)
	if !ok {
		return "", false
	}
	path, _, found := deskkit.FindConfigFile(name)
	if !found {
		return "", false
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		return "", false
	}
	if time.Since(fi.ModTime()) >= installIDMaxAge {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	id := strings.TrimSpace(string(b))
	if !allDigits(id) {
		return "", false
	}
	return id, true
}

// writeInstallIDCache records a freshly RESOLVED installation id, 0600, at the head of the
// App-credential search path — the same place the token cache is written, so a deployment
// that points ASSAY_CONFIG_HOME at its provisioning directory keeps key, cache, sidecar and
// this file in one directory. Best-effort: a failure costs one API call next time.
func writeInstallIDCache(appName, owner, id string) {
	name, ok := installIDCacheName(appName, owner)
	if !ok || !allDigits(id) {
		return
	}
	path := deskkit.ConfigHomeWritePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
}

// invalidateInstallIDCache removes the entry for (appName, owner) from EVERY directory on
// the search path — the read searched all of them, so a removal that cleared only the head
// would leave a shadowed stale copy to be found again on the next run. Best-effort by
// design: a file that is already gone is the state this wanted.
func invalidateInstallIDCache(appName, owner string) {
	name, ok := installIDCacheName(appName, owner)
	if !ok {
		return
	}
	for _, dir := range deskkit.ConfigHomeDirs() {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// --- The cache probe --------------------------------------------------------------------

// probeCachedInstallID answers "is there already a usable token on disk for this role, App
// and account, and what installation was it minted under?" WITHOUT a network call.
//
// It globs `<role>-token-*` across the App-credential search path and accepts a candidate
// only when ALL of these hold: the suffix is a run of digits (an installation id), the file
// is mode 0600 (the same custody bar the reuse path enforces), its age is under cacheMaxAge
// (the same reuse window), and its `.owner` sidecar records exactly this App and this
// account.
//
// Accepted candidates are collapsed by installation id: the same id found in two search
// directories is one installation, which is what the search path's shadowing already means.
// Two DISTINCT ids is genuine ambiguity, and ambiguity falls through to authoritative
// resolution rather than picking a winner.
func probeCachedInstallID(role, appName, owner string) (string, bool) {
	if !cacheNameOK(role) || !cacheNameOK(appName) || !cacheNameOK(owner) {
		return "", false
	}
	found := map[string]bool{}
	for _, dir := range deskkit.ConfigHomeDirs() {
		matches, err := filepath.Glob(filepath.Join(dir, role+"-token-*"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			id := strings.TrimPrefix(filepath.Base(m), role+"-token-")
			if !allDigits(id) {
				continue
			}
			fi, serr := os.Stat(m)
			if serr != nil || fi.IsDir() || fi.Mode().Perm() != 0o600 {
				continue
			}
			if time.Since(fi.ModTime()) >= cacheMaxAge {
				continue
			}
			if !sidecarMatches(m, appName, owner) {
				continue
			}
			found[id] = true
		}
	}
	if len(found) != 1 {
		return "", false
	}
	for id := range found {
		return id, true
	}
	return "", false
}
