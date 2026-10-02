package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Explicit markers belong to the row, not a guessed host OS. Shell alone is
// not an exemption: sh/bash still hide portability assumptions on other hosts.
var verifyOSMarker = regexp.MustCompile(`(?i)\bOS\s*:\s*(windows|linux|darwin|macos|posix)\b|\b(windows|linux|darwin|macos|posix)[ -]only\b`)
var verifyShellC = regexp.MustCompile(`(^|[^[:alnum:]_])(?:sh|bash)[[:space:]]+-c([[:space:]]|$)`)
var verifyFindstr = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])findstr([[:space:]]|$)`)
var verifyTmpPath = regexp.MustCompile(`(^|[^[:alnum:]_])/tmp($|[/[:space:]"'` + "`" + `])`)

func verifyPortability(r verifyRowCells) bool {
	if verifyOSMarker.MatchString(r.Command + " " + r.Expect) {
		return false
	}
	command := strings.ReplaceAll(r.Command, "${TMPDIR:-/tmp}", "${TMPDIR}")
	return verifyTmpPath.MatchString(command) || verifyShellC.MatchString(command) || verifyFindstr.MatchString(command)
}

func verifyPortabilityNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				if verifyPortability(r) {
					notices = append(notices, fmt.Sprintf("%s: Verify row %s [verify-row-portability] OS-specific command without explicit OS marker; prefer Go tests or mktemp/${TMPDIR:-/tmp}", path, r.Num))
				}
			})
		}
	}
	return notices
}
