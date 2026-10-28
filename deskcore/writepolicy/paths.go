package writepolicy

import (
	"path"
	"strings"
)

// pathSyntax is lexical only. Filesystem resolution belongs to the adapter.
// Bash command names remain slash-separated; native path observations select
// their platform explicitly, independently of the evaluator host.
type pathSyntax struct{ platform Platform }

func (p pathSyntax) separator() string {
	if p.platform == Windows {
		return "\\"
	}
	return "/"
}
func (p pathSyntax) slash(s string) string {
	if p.platform == Windows {
		return strings.ReplaceAll(s, "\\", "/")
	}
	return s
}
func (p pathSyntax) native(s string) string {
	if p.platform == Windows {
		return strings.ReplaceAll(s, "/", "\\")
	}
	return s
}
func (p pathSyntax) volume(s string) string {
	if p.platform != Windows {
		return ""
	}
	s = p.slash(s)
	if len(s) >= 2 && s[1] == ':' {
		return s[:2]
	}
	if strings.HasPrefix(s, "//") {
		parts := strings.SplitN(s[2:], "/", 3)
		if len(parts) >= 2 {
			return "//" + parts[0] + "/" + parts[1]
		}
	}
	return ""
}
func (p pathSyntax) Clean(s string) string {
	s = p.slash(s)
	v := p.volume(s)
	tail := strings.TrimPrefix(s, v)
	if tail == "" && strings.HasPrefix(v, "//") {
		return p.native(v)
	}
	if tail == "" && v != "" {
		tail = "."
	}
	return p.native(v + path.Clean(tail))
}
func (p pathSyntax) Join(parts ...string) string {
	var s string
	for _, part := range parts {
		if part == "" {
			continue
		}
		if s != "" && !strings.HasSuffix(s, "/") {
			s += "/"
		}
		s += p.slash(part)
	}
	if s == "" {
		return ""
	}
	return p.Clean(s)
}
func (p pathSyntax) Dir(s string) string {
	s = p.slash(s)
	v := p.volume(s)
	if strings.HasPrefix(v, "//") && (s == v || s == v+"/") {
		return p.native(v)
	}
	return p.native(v + path.Dir(strings.TrimPrefix(s, v)))
}
func (p pathSyntax) Base(s string) string { return path.Base(p.slash(s)) }
func (p pathSyntax) IsAbs(s string) bool {
	s = p.slash(s)
	if p.platform == Windows {
		v := p.volume(s)
		return v != "" && (strings.HasPrefix(v, "//") || strings.HasPrefix(strings.TrimPrefix(s, v), "/"))
	}
	return path.IsAbs(s)
}
