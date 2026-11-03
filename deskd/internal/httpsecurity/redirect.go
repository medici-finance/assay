package httpsecurity

import (
	"net/url"
	"strings"
)

// ReturnPath accepts only a rooted same-origin path, rejecting browser URL
// normalization tricks, including encoded slashes and backslashes in the path.
func ReturnPath(s string) string {
	if s == "" || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.Contains(s, "\\") {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.Host != "" || u.Scheme != "" || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") {
		return ""
	}
	// Browsers trim some control characters before interpreting a Location URL.
	if strings.IndexFunc(u.Path, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
		return ""
	}
	return s
}
