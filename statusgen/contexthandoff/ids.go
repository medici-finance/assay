package contexthandoff

import (
	"fmt"
	"regexp"
	"strings"
)

// QID is a qualified brief identity in its canonical four-segment form,
// <cell>:<repo>:<stream>:<NN>.
type QID struct {
	Cell, Repo, Stream, NN string
}

// String renders the canonical four-segment form.
func (q QID) String() string {
	return q.Cell + ":" + q.Repo + ":" + q.Stream + ":" + q.NN
}

var (
	idSegmentRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	idNumberRe  = regexp.MustCompile(`^[0-9]{2,4}[a-z]?$`)
	issueRefRe  = regexp.MustCompile(`^[A-Za-z0-9._/-]*#[0-9]+$`)
)

// ResolveQualifiedID parses a brief reference and expands it to the canonical
// four-segment form. The accepted forms are exactly:
//
//	<cell>:<repo>:<stream>:<NN>   canonical
//	<repo>:<stream>:<NN>          cell taken from the declaring record
//	<stream>:<NN>                 cell and repo taken from the declaring record
//
// declaring is the declaring record's own identity; nil means there is none,
// so only the canonical form is accepted. An issue reference ("#N"), a path,
// the brief-v1 "<stream>/<NN>" form and any other unqualified string are
// refused.
func ResolveQualifiedID(ref string, declaring *QID) (QID, error) {
	switch {
	case ref == "":
		return QID{}, fmt.Errorf("empty brief reference")
	case strings.TrimSpace(ref) != ref || strings.ContainsAny(ref, " \t\r\n"):
		return QID{}, fmt.Errorf("%q: a brief reference carries no whitespace", ref)
	case issueRefRe.MatchString(ref):
		return QID{}, fmt.Errorf("%q: an issue reference is not a brief identity; use <cell>:<repo>:<stream>:<NN>", ref)
	case strings.Contains(ref, "/") && !strings.Contains(ref, ":"):
		if strings.Count(ref, "/") == 1 && idNumberRe.MatchString(ref[strings.Index(ref, "/")+1:]) {
			return QID{}, fmt.Errorf("%q: the brief-v1 <stream>/<NN> form is refused; use <cell>:<repo>:<stream>:<NN>", ref)
		}
		return QID{}, fmt.Errorf("%q: a path is not a brief identity; use <cell>:<repo>:<stream>:<NN>", ref)
	case strings.Contains(ref, "/"):
		return QID{}, fmt.Errorf("%q: a brief reference carries no path separator", ref)
	}
	segs := strings.Split(ref, ":")
	if len(segs) < 2 || len(segs) > 4 {
		return QID{}, fmt.Errorf("%q: not a qualified brief reference; use <cell>:<repo>:<stream>:<NN>", ref)
	}
	nn := segs[len(segs)-1]
	if !idNumberRe.MatchString(nn) {
		return QID{}, fmt.Errorf("%q: brief number %q is not two to four digits", ref, nn)
	}
	for _, s := range segs[:len(segs)-1] {
		if !idSegmentRe.MatchString(s) {
			return QID{}, fmt.Errorf("%q: segment %q is not a lower-case name", ref, s)
		}
	}
	if len(segs) < 4 && declaring == nil {
		return QID{}, fmt.Errorf("%q: an elided reference needs a declaring record; the declaring identity itself is always <cell>:<repo>:<stream>:<NN>", ref)
	}
	switch len(segs) {
	case 4:
		return QID{Cell: segs[0], Repo: segs[1], Stream: segs[2], NN: nn}, nil
	case 3:
		return QID{Cell: declaring.Cell, Repo: segs[0], Stream: segs[1], NN: nn}, nil
	default:
		return QID{Cell: declaring.Cell, Repo: declaring.Repo, Stream: segs[0], NN: nn}, nil
	}
}
