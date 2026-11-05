package credential

// installid.go — the two facts that let a "reuse cached token" call cost NOTHING.
//
// The token cache path is `<role>-token-<installID>`, so without these the installation id
// would have to be resolved BEFORE the cache could be looked at — reading the App private
// key, signing a JWT and calling `GET /app/installations` to rediscover an installation that
// changes only when somebody installs or uninstalls the App (#1036).
//
// THE TWO FACTS. (1) An owner SIDECAR beside each cached token, recording which App and
// which account that file was minted for — so a cached file can be recognised without
// knowing its installation id first. (2) An install-id CACHE per (App, account), so a cold
// token cache still costs one installations call a day rather than one per invocation.
//
// FAIL CLOSED, ALWAYS. Every acceptance below is a POSITIVE match of both the App name and
// the account, on a file whose mode and age were checked. Absence, ambiguity, a wrong mode,
// a malformed name, an unreadable file — every one of them falls through to authoritative
// resolution. A cache in front of a CREDENTIAL may never guess: handing back the
// right-shaped token for the wrong identity does not error, it authenticates as somebody
// else.

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// installIDMaxAge is how long a resolved installation id may be reused from disk. A stale
// one is not a silent wrong answer: its only consumer is the token exchange, which fails
// closed against GitHub, and that failure invalidates the entry (invalidateInstallIDCache).
const installIDMaxAge = 24 * time.Hour

// cacheNameOK reports whether s is safe to place in a cache FILE NAME and to compare as a
// recorded identity. Anything outside [A-Za-z0-9._-] is not sanitised or rewritten — it
// simply means no cache is read and none is written.
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

// CacheNameOK is the exported spelling of the cache-name rule.
func CacheNameOK(s string) bool { return cacheNameOK(s) }

// allDigits reports whether s is a non-empty run of ASCII digits — the shape of every
// installation id.
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

// OwnerSidecarPath is the file recording which App and which account a cached token file
// belongs to. A missing or non-matching `.owner` REFUSES the fast path.
func OwnerSidecarPath(tokenPath string) string { return tokenPath + ".owner" }

// writeOwnerSidecar records "<appName> <owner>" beside the token cache, 0600. Not fatal:
// a missing sidecar costs one installations call next time rather than a wrong answer.
// Atomic like the token itself: a racing reader sees the old sidecar or the new one, never a
// torn one, and a link at the path is replaced rather than written through.
func writeOwnerSidecar(tokenPath, appName, owner string) {
	if !cacheNameOK(appName) || !cacheNameOK(owner) {
		return
	}
	_ = WriteTokenCache(OwnerSidecarPath(tokenPath), appName+" "+owner+"\n", nil)
}

// readOwnerSidecar returns the App name and account a sidecar records; ok=false covers
// every doubt.
func readOwnerSidecar(tokenPath string) (appName, owner string, ok bool) {
	b, err := os.ReadFile(OwnerSidecarPath(tokenPath))
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

// SidecarMatches reports whether tokenPath's sidecar records exactly this App and this
// account. Both halves are compared: the App half stops two Apps bound to one role on one
// account from reading each other's cache file.
func SidecarMatches(tokenPath, appName, owner string) bool {
	a, o, ok := readOwnerSidecar(tokenPath)
	if !ok {
		return false
	}
	return a == appName && strings.EqualFold(o, owner)
}

// --- The install-id cache ---------------------------------------------------------------

// InstallIDCacheName is the per-(App, account) cache file name. Both halves are in the NAME,
// so two Apps on one account, or one App on two accounts, can never collide.
func InstallIDCacheName(appName, owner string) (string, bool) {
	if !cacheNameOK(appName) || !cacheNameOK(owner) {
		return "", false
	}
	return appName + "-install-" + owner, true
}

// ReadInstallIDCache returns the cached installation id for (appName, owner) when there is
// one that is 0600, younger than installIDMaxAge, and holds nothing but digits.
func ReadInstallIDCache(p Paths, now time.Time, appName, owner string) (string, bool) {
	return readInstallIDCache(p, now, appName, owner)
}

func readInstallIDCache(p Paths, now time.Time, appName, owner string) (string, bool) {
	name, ok := InstallIDCacheName(appName, owner)
	if !ok {
		return "", false
	}
	path, found := p.find(name)
	if !found {
		return "", false
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		return "", false
	}
	if now.Sub(fi.ModTime()) >= installIDMaxAge {
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
// App-credential search path. Best-effort: a failure costs one API call next time.
func writeInstallIDCache(p Paths, appName, owner, id string) {
	name, ok := InstallIDCacheName(appName, owner)
	if !ok || !allDigits(id) {
		return
	}
	path, ok := p.writePath(name)
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
}

// invalidateInstallIDCache removes the entry for (appName, owner) from EVERY directory on
// the search path — the read searched all of them, so clearing only the head would leave a
// shadowed stale copy to be found again.
func invalidateInstallIDCache(p Paths, appName, owner string) {
	name, ok := InstallIDCacheName(appName, owner)
	if !ok {
		return
	}
	for _, dir := range p.Dirs {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// --- The cache probe --------------------------------------------------------------------

// ProbeCachedInstallID answers "is there already a usable token on disk for this role, App
// and account, and what installation was it minted under?" WITHOUT a network call.
func ProbeCachedInstallID(p Paths, now time.Time, role, appName, owner string) (string, bool) {
	return probeCachedInstallID(p, now, role, appName, owner)
}

// probeCachedInstallID globs `<role>-token-*` across the search path and accepts a
// candidate only when ALL hold: the suffix is digits, the file is a regular 0600 file, its
// age is under ReuseWindow, and its `.owner` sidecar records exactly this App and account.
// Two DISTINCT ids is ambiguity, which falls through to authoritative resolution.
func probeCachedInstallID(p Paths, now time.Time, role, appName, owner string) (string, bool) {
	if !cacheNameOK(role) || !cacheNameOK(appName) || !cacheNameOK(owner) {
		return "", false
	}
	found := map[string]bool{}
	for _, dir := range p.Dirs {
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
			if now.Sub(fi.ModTime()) >= ReuseWindow {
				continue
			}
			if !SidecarMatches(m, appName, owner) {
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
