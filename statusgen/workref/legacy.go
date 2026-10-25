package workref

import (
	"fmt"
	"strings"
)

func parseQualified(ref string) (Reference, error) {
	switch {
	case ref == "":
		return Reference{}, fmt.Errorf("empty brief reference")
	case strings.TrimSpace(ref) != ref || strings.ContainsAny(ref, " \t\r\n"):
		return Reference{}, fmt.Errorf("%q: a brief reference carries no whitespace", ref)
	case issueRe.MatchString(ref):
		return Reference{}, fmt.Errorf("%q: an issue reference is not a brief identity; use <cell>:<repo>:<stream>:<NN>", ref)
	case strings.Contains(ref, "/") && !strings.Contains(ref, ":"):
		if strings.Count(ref, "/") == 1 && numberRe.MatchString(ref[strings.Index(ref, "/")+1:]) {
			return Reference{}, fmt.Errorf("%q: the brief-v1 <stream>/<NN> form is refused; use <cell>:<repo>:<stream>:<NN>", ref)
		}
		return Reference{}, fmt.Errorf("%q: a path is not a brief identity; use <cell>:<repo>:<stream>:<NN>", ref)
	case strings.Contains(ref, "/"):
		return Reference{}, fmt.Errorf("%q: a brief reference carries no path separator", ref)
	}
	segs := strings.Split(ref, ":")
	if len(segs) < 2 || len(segs) > 4 {
		return Reference{}, fmt.Errorf("%q: not a qualified brief reference; use <cell>:<repo>:<stream>:<NN>", ref)
	}
	nn := segs[len(segs)-1]
	if !numberRe.MatchString(nn) {
		return Reference{}, fmt.Errorf("%q: brief number %q is not two to four digits", ref, nn)
	}
	for _, s := range segs[:len(segs)-1] {
		if !nameRe.MatchString(s) {
			return Reference{}, fmt.Errorf("%q: segment %q is not a lower-case name", ref, s)
		}
	}
	id := Identity{Kind: Brief, Stream: segs[len(segs)-2], Number: nn}
	if len(segs) >= 3 {
		id.Repo = segs[len(segs)-3]
	}
	if len(segs) == 4 {
		id.Cell = segs[0]
	}
	return Reference{Identity: id, Spelling: ref}, nil
}

func parseTrailer(ref string) (Reference, error) {
	v := strings.TrimSpace(ref)
	var parts []string
	if strings.Contains(v, ":") {
		parts = strings.Split(v, ":")
		if len(parts) < 2 {
			return Reference{}, fmt.Errorf("invalid brief trailer")
		}
	} else {
		parts = strings.Split(v, "/")
		if len(parts) != 2 {
			return Reference{}, fmt.Errorf("invalid brief trailer")
		}
	}
	stream, nn := parts[len(parts)-2], parts[len(parts)-1]
	if stream == "" || !digits(nn) {
		return Reference{}, fmt.Errorf("invalid brief trailer")
	}
	return Reference{Identity: Identity{Kind: Brief, Stream: stream, Number: nn}, Spelling: ref}, nil
}
