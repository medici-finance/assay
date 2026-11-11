//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// aclEntryLine is one `ls -e` access-control entry line (` 0: user:… allow read`).
var aclEntryLine = regexp.MustCompile(`^\s*\d+:\s`)

// probeACL reports whether path (not followed) carries an access-control list. macOS exposes no
// list API without cgo, so this reads /bin/ls -lde: an entry line, or a `+` after the mode, is a
// list. Output that cannot be read that way is an error, which refuses the binding.
func probeACL(path string) (bool, error) {
	out, err := exec.Command("/bin/ls", "-lde", "--", path).Output()
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return false, fmt.Errorf("empty listing")
	}
	mode, _, _ := strings.Cut(lines[0], " ")
	if len(mode) < 10 {
		return false, fmt.Errorf("unreadable listing")
	}
	if strings.ContainsRune(mode[10:], '+') {
		return true, nil
	}
	for _, l := range lines[1:] {
		if aclEntryLine.MatchString(l) {
			return true, nil
		}
	}
	return false, nil
}
