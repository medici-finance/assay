package gitcore

import (
	"os"
	"strconv"
)

// grepPins is one of the two command-priority config sets a reader here
// qualifies: the two grep keys a verifier's admitted row pins so a home's grep
// config cannot change a row's `git grep` (#2715). Both keys are read by
// `git grep` alone; neither can change a remote URL, a ref, an object,
// discovery or shallow state, so the library read and native git agree under
// them.
var grepPins = [...][2]string{
	{"grep.patternType", "default"},
	{"grep.extendedRegexp", "false"},
}

// quotePathPin is the other qualified set: core.quotePath=false alone (#7533).
// The key only selects whether native git C-quotes non-ASCII bytes in the paths
// it prints. It cannot change a remote URL, an insteadOf rule, a ref, an
// object, discovery or shallow state. The value is pinned to "false" because
// that is the setting under which native output spells a path the way the
// library read does.
var quotePathPin = [...][2]string{
	{"core.quotePath", "false"},
}

// qualifiedCommandConfig is the explicit allowlist of command-priority config
// sets. A set qualifies only as a whole: its count, keys and values byte for
// byte and in order. Sets are never combined, and no key is matched by prefix
// or by section.
var qualifiedCommandConfig = [...][][2]string{
	grepPins[:],
	quotePathPin[:],
}

// GrepPinEnv is the GIT_CONFIG_COUNT/KEY_n/VALUE_n spelling of the qualified
// grep pins. An admitted row's env appends it, so what admission writes and
// what CommandConfigQualified accepts cannot drift apart.
func GrepPinEnv() []string {
	env := []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(len(grepPins))}
	for i, kv := range grepPins {
		n := strconv.Itoa(i)
		env = append(env, "GIT_CONFIG_KEY_"+n+"="+kv[0], "GIT_CONFIG_VALUE_"+n+"="+kv[1])
	}
	return env
}

// CommandConfigQualified reports whether GIT_CONFIG_COUNT selects exactly
// nothing ("0") or exactly one set in qualifiedCommandConfig, byte for byte
// and in order. Any other count, key or value, an empty count included, is not
// qualified. GIT_CONFIG_PARAMETERS and GIT_CONFIG are outside this predicate:
// callers still refuse them on their own.
func CommandConfigQualified() bool {
	count, ok := os.LookupEnv("GIT_CONFIG_COUNT")
	if !ok {
		return false
	}
	if count == "0" {
		return true
	}
	for _, set := range qualifiedCommandConfig {
		if commandConfigIs(count, set) {
			return true
		}
	}
	return false
}

// commandConfigIs reports whether count and the GIT_CONFIG_KEY_n/VALUE_n
// entries spell exactly set.
func commandConfigIs(count string, set [][2]string) bool {
	if count != strconv.Itoa(len(set)) {
		return false
	}
	for i, kv := range set {
		n := strconv.Itoa(i)
		if os.Getenv("GIT_CONFIG_KEY_"+n) != kv[0] || os.Getenv("GIT_CONFIG_VALUE_"+n) != kv[1] {
			return false
		}
	}
	return true
}

// CommitIdentityConfigOnly reports whether the review kit supplies precisely
// its two author settings. A reader of existing commit history can admit this
// environment without changing the profile of an origin or authority reader.
func CommitIdentityConfigOnly() bool {
	if _, set := os.LookupEnv("GIT_CONFIG"); set {
		return false
	}
	if _, set := os.LookupEnv("GIT_CONFIG_PARAMETERS"); set {
		return false
	}
	return os.Getenv("GIT_CONFIG_COUNT") == "2" &&
		os.Getenv("GIT_CONFIG_KEY_0") == "user.name" && os.Getenv("GIT_CONFIG_VALUE_0") != "" &&
		os.Getenv("GIT_CONFIG_KEY_1") == "user.email" && os.Getenv("GIT_CONFIG_VALUE_1") != ""
}
