package deskkit

import "strings"

// IsAbsFor reports path syntax for the requested host, without touching the filesystem.
// Windows rooted paths lacking a volume (\rooted or /rooted) are not absolute.
func IsAbsFor(goos, p string) bool {
	if goos != "windows" {
		return strings.HasPrefix(p, "/")
	}
	sep := func(c byte) bool { return c == '/' || c == '\\' }
	if len(p) >= 3 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) && p[1] == ':' && sep(p[2]) {
		return true
	}
	return len(p) > 2 && sep(p[0]) && sep(p[1])
}
