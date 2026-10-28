package perfrec

// buildstamp.go — where a binary's source revision comes from when no link-time stamp names it.
//
// The desk tools are stamped at link time (`-X …/deskkit.SourceSHA=<short sha>`), and a record
// whose stamp is not hex records "unpinned" (SourceSHA). A build path that stamps only a release
// tag — statusgen's release build sets `-X main.statusgenVersion=vX.Y.Z` and nothing else — or a
// plain `go build` / `go install` therefore wrote "unpinned" although the binary knows exactly
// which commit it was built from: the Go toolchain embeds vcs.revision and vcs.time in the
// binary's build info whenever it builds inside a VCS checkout.
//
// Revision and BuildTime resolve the value in a fixed order:
//
//  1. the link-time stamp, when one was set — an explicit stamp always wins, even a malformed
//     one (SourceSHA still maps a non-hex stamp to "unpinned"; a fallback never overrides it);
//  2. else the toolchain's vcs.revision / vcs.time;
//  3. else "" — the caller's existing "unpinned" substitution applies, so a build with no VCS
//     information (`go run`, `-buildvcs=false`, a tree with no repository) stays explicit.
//
// vcs.modified is deliberately not consulted. The link-time stamp records HEAD whether or not
// the tree is clean, and a release job that has already written build outputs into its
// checkout reports modified=true for every later binary; refusing a modified revision would
// put every such release build back to "unpinned".

import (
	"regexp"
	"runtime/debug"
	"time"
)

// RevisionLen is how many characters of vcs.revision a build-info fallback keeps: a short,
// unambiguous prefix, the same form as the link-time short-sha stamp.
const RevisionLen = 12

// BuildInfoFunc reads a binary's embedded build info; debug.ReadBuildInfo is the real one.
type BuildInfoFunc func() (*debug.BuildInfo, bool)

// vcsRevision is what an accepted vcs.revision looks like: lowercase hex, SHA-1 or SHA-256 width.
var vcsRevision = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Revision returns stamp when it is set, else this binary's toolchain-embedded vcs.revision
// shortened to RevisionLen, else "".
func Revision(stamp string) string { return RevisionFrom(stamp, debug.ReadBuildInfo) }

// BuildTime returns stamp when it is set, else this binary's toolchain-embedded vcs.time (the
// commit time, RFC 3339 UTC), else "".
func BuildTime(stamp string) string { return BuildTimeFrom(stamp, debug.ReadBuildInfo) }

// RevisionFrom is Revision with the build-info reader supplied, so a test can present a binary
// with, without, or with malformed VCS information.
func RevisionFrom(stamp string, read BuildInfoFunc) string {
	if stamp != "" {
		return stamp
	}
	rev := vcsSetting(read, "vcs.revision")
	if !vcsRevision.MatchString(rev) {
		return ""
	}
	return rev[:RevisionLen]
}

// BuildTimeFrom is BuildTime with the build-info reader supplied.
func BuildTimeFrom(stamp string, read BuildInfoFunc) string {
	if stamp != "" {
		return stamp
	}
	t, err := time.Parse(time.RFC3339, vcsSetting(read, "vcs.time"))
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// vcsSetting returns one build setting, "" when there is no build info or no such key.
func vcsSetting(read BuildInfoFunc, key string) string {
	if read == nil {
		return ""
	}
	info, ok := read()
	if !ok || info == nil {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
