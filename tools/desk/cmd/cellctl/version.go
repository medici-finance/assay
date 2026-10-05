package main

import "runtime/debug"

// unstampedVersion is cellctlVersion's value when no `-X main.cellctlVersion=<tag>` was given at
// link time — a source build.
const unstampedVersion = "dev"

// shortRevisionLen is how many leading characters of the VCS revision a source build reports.
const shortRevisionLen = 12

// readBuildInfo is the build-info source `--version` consults. It is a variable only so a test
// can inject a fake; production code never reassigns it.
var readBuildInfo = debug.ReadBuildInfo

// versionString is what `cellctl --version` prints.
//
// A stamped release tag is authoritative and returned unaltered: anything other than the exact
// unstamped value "dev" comes back as given. Only an unstamped build consults the Go toolchain's
// embedded VCS stamp, so two source builds from different commits can be told apart:
//
//	dev-<first 12 hex of vcs.revision>         clean tree
//	dev-<first 12 hex of vcs.revision>-dirty   vcs.modified=true
//
// With no build info, no vcs.revision (`go run`, `-buildvcs=false`, a build outside a VCS tree),
// or a revision that is not hexadecimal, it falls back to plain "dev": it never invents a hash.
// A hex revision shorter than 12 characters is reported whole, never padded.
func versionString(stamped string, read func() (*debug.BuildInfo, bool)) string {
	if stamped != unstampedVersion {
		return stamped
	}
	if read == nil {
		return unstampedVersion
	}
	info, ok := read()
	if !ok || info == nil {
		return unstampedVersion
	}
	revision, modified := "", ""
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if revision == "" || !isHex(revision) {
		return unstampedVersion
	}
	if len(revision) > shortRevisionLen {
		revision = revision[:shortRevisionLen]
	}
	v := unstampedVersion + "-" + revision
	if modified == "true" {
		v += "-dirty"
	}
	return v
}

// isHex reports whether s is non-empty and made only of hexadecimal digits.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
