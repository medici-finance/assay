package gitcore

import (
	"os"
	"strconv"
)

// grepPins is the only command-priority config a reader here qualifies: the
// two grep keys a verifier's admitted row pins so a home's grep config cannot
// change a row's `git grep` (#2715). Both keys are read by `git grep` alone;
// neither can change a remote URL, a ref, an object, discovery or shallow
// state, so the library read and native git agree under them.
var grepPins = [...][2]string{
	{"grep.patternType", "default"},
	{"grep.extendedRegexp", "false"},
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
// nothing ("0") or exactly the grep pins, byte for byte and in order. Any
// other count, key or value, an empty count included, is not qualified.
// GIT_CONFIG_PARAMETERS and GIT_CONFIG are outside this predicate: callers
// still refuse them on their own.
func CommandConfigQualified() bool {
	count, ok := os.LookupEnv("GIT_CONFIG_COUNT")
	if !ok {
		return false
	}
	if count == "0" {
		return true
	}
	if count != strconv.Itoa(len(grepPins)) {
		return false
	}
	for i, kv := range grepPins {
		n := strconv.Itoa(i)
		if os.Getenv("GIT_CONFIG_KEY_"+n) != kv[0] || os.Getenv("GIT_CONFIG_VALUE_"+n) != kv[1] {
			return false
		}
	}
	return true
}
