package main

// installidcache.go — the CLI's view of the two facts that let a "reuse cached token" call
// cost NOTHING (#1036): the owner SIDECAR beside each cached token and the per-(App, account)
// install-id CACHE.
//
// Both now live in the provider (internal/credential, installid.go), which the CLI and the
// in-process lookup both run, so the cache a CLI mint writes is the cache an in-process lookup
// reuses and vice versa — one cache, one set of sidecars. What remains here are the spellings
// this package's tests read, each a direct call into the provider over the SAME search path
// (deskkit.CredentialPaths) and wall clock the mint uses.
//
// FAIL CLOSED, ALWAYS. Every acceptance is a POSITIVE match of both the App name and the
// account, on a file whose mode and age were checked; any doubt falls through to authoritative
// resolution. A cache in front of a CREDENTIAL may never guess.

import (
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ownerSidecarPath is the file recording which App and which account a cached token file
// belongs to.
func ownerSidecarPath(tokenPath string) string { return credential.OwnerSidecarPath(tokenPath) }

// sidecarMatches reports whether tokenPath's sidecar records exactly this App and account.
func sidecarMatches(tokenPath, appName, owner string) bool {
	return credential.SidecarMatches(tokenPath, appName, owner)
}

// installIDCacheName is the per-(App, account) install-id cache file name.
func installIDCacheName(appName, owner string) (string, bool) {
	return credential.InstallIDCacheName(appName, owner)
}

// readInstallIDCache returns the cached installation id for (appName, owner), if usable.
func readInstallIDCache(appName, owner string) (string, bool) {
	return credential.ReadInstallIDCache(deskkit.CredentialPaths(), time.Now(), appName, owner)
}

// probeCachedInstallID answers, without a network call, whether a usable token is already on
// disk for this role, App and account, and under which installation it was minted.
func probeCachedInstallID(role, appName, owner string) (string, bool) {
	return credential.ProbeCachedInstallID(deskkit.CredentialPaths(), time.Now(), role, appName, owner)
}
