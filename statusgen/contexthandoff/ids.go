package contexthandoff

import (
	"github.com/medici-finance/assay/statusgen/workref"
	"regexp"
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
	var context *workref.Context
	if declaring != nil {
		context = &workref.Context{Cell: declaring.Cell, Repo: declaring.Repo}
	}
	resolved, err := workref.Resolve(ref, context, workref.Registry{}, workref.LegacyQualified)
	if err != nil {
		return QID{}, err
	}
	return QID{Cell: resolved.Cell, Repo: resolved.Repo, Stream: resolved.Stream, NN: resolved.Number}, nil
}
