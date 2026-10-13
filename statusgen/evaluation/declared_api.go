package evaluation

// declared_api.go — the `files:` declaration rules the coverage evaluator
// applies, exported so the model auto-flip's delivering-PR overlap
// (statusgen/deliveryfiles.go) reads a brief's `files:` exactly as coverage
// does. There is one implementation; these are thin entry points to it.

// DeclaredEntries normalizes and de-duplicates declared `files:` entries: a
// leading `./` is dropped and a trailing `/**` reads as its directory.
func DeclaredEntries(entries []string) []string { return appendDeclaredEntries(nil, entries) }

// DeclaredEntrySupported reports whether d is an entry DeclaredEntryMatches can
// express (not `.`, absolute, parent-relative, brace syntax, an unrewritten
// `**`, or a malformed glob).
func DeclaredEntrySupported(d string) bool { return declaredEntrySupported(d) }

// DeclaredEntryMatches reports whether declared entry d covers repo-relative
// path p: p is d, p is under d as a directory, or d (as a glob) matches p or
// one of its parent directories.
func DeclaredEntryMatches(d, p string) bool { return declaredEntryMatches(d, p) }

// IsVerifyWrittenPath reports whether p is a file verify and regen necessarily
// write: STATUS.md, a verify-outcome log, a stream README or a brief file.
func IsVerifyWrittenPath(p string) bool { return isVerifyWrittenPath(p) }
