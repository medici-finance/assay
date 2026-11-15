package deskkit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

// BriefSubject is an observed executable record, separate from logical identity,
// a revision selector and the store's compare-and-swap version. It grants nothing.
type BriefSubject struct {
	UUID       string `json:"uuid"`
	Version    uint64 `json:"version"`
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Path       string `json:"path"`
}

var subjectUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var subjectCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var workLabel = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (s BriefSubject) Validate() error {
	if !subjectUUID.MatchString(s.UUID) || s.Version == 0 || !ValidRepoSlug(s.Repository) || !subjectCommit.MatchString(s.Commit) {
		return Refused("invalid exact brief subject")
	}
	if s.Path == "" || path.Clean(s.Path) != s.Path || strings.ContainsAny(s.Path, "\\ \t\r\n:") || strings.HasPrefix(s.Path, "/") || strings.HasPrefix(s.Path, "../") {
		return Refused("invalid repository-relative brief subject path")
	}
	return nil
}

// WorkRecord is supplied by the host's existing source observation boundary.
// Historical aliases belong to workref.Binding, never guessed from filenames.
type WorkRecord struct {
	Identity        workref.Identity
	Subject         BriefSubject
	Label           string
	PhysicalAliases []string
}

type WorkSelection struct {
	Reference workref.Result
	Subject   BriefSubject
	ClaimKey  string
	Aliases   []string
}

type WorkCatalog struct {
	Declaring workref.Context
	Registry  workref.Registry
	Records   []WorkRecord
}

// Select resolves first, then binds the observed record. A fallback logical key
// from workref is not proof of a UUID, available version or executable record.
func (c WorkCatalog) Select(value string) (WorkSelection, error) {
	r, err := ResolveWorkTrailer(value, &c.Declaring, c.Registry)
	if err != nil {
		return WorkSelection{}, Refused("work reference: " + err.Error())
	}
	var found *WorkRecord
	for i := range c.Records {
		rec := &c.Records[i]
		if rec.Identity != r.Identity {
			continue
		}
		if found != nil {
			return WorkSelection{}, Refused("ambiguous brief source observation")
		}
		found = rec
	}
	if found == nil {
		return WorkSelection{}, Unverifiable("exact brief source observation unavailable", nil)
	}
	if err := found.Subject.Validate(); err != nil {
		return WorkSelection{}, err
	}
	if !workLabel.MatchString(found.Label) {
		return WorkSelection{}, Refused("ambiguous repository claim label")
	}
	if r.LogicalKey != found.Subject.UUID {
		return WorkSelection{}, Refused("resolved logical key does not bind the observed brief UUID")
	}
	if r.Revision != 0 && r.Revision != found.Subject.Version {
		return WorkSelection{}, Unverifiable("requested exact brief revision unavailable", nil)
	}
	key := found.Label + "--" + r.Stream + "--" + r.Number
	return WorkSelection{Reference: r, Subject: found.Subject, ClaimKey: key, Aliases: append([]string(nil), found.PhysicalAliases...)}, nil
}

// Validate repeats the selected subject/kind/key check at an effect boundary.
// Exported values and successful upper preflight are never authority.
func (s WorkSelection) Validate() error {
	if err := s.Subject.Validate(); err != nil {
		return err
	}
	r := s.Reference
	if !strings.HasPrefix(s.Subject.Path, "docs/streams/"+r.Stream+"/brief-"+r.Number+"-") || !strings.HasSuffix(s.Subject.Path, ".md") {
		return Refused("selected subject path differs from brief identity")
	}
	if _, err := workref.Parse(r.Spelling, workref.TypedBrief); err != nil {
		return Refused("selected original spelling is invalid")
	}
	if r.Kind != workref.Brief || r.LogicalKey != s.Subject.UUID || r.Compatibility != r.Identity.Compatibility() || (r.Revision != 0 && r.Revision != s.Subject.Version) {
		return Refused("selected brief kind, UUID or exact revision mismatch")
	}
	suffix := "--" + r.Stream + "--" + r.Number
	label, ok := strings.CutSuffix(s.ClaimKey, suffix)
	if !ok || !workLabel.MatchString(label) {
		return Refused("selected physical claim key differs from logical brief identity")
	}
	for _, alias := range s.Aliases {
		if alias == "" || strings.ContainsAny(alias, " \t\r\n~^:?*[\\") || strings.Contains(alias, "..") || strings.Contains(alias, "@{") {
			return Refused("invalid supplied physical alias")
		}
	}
	return nil
}

// CheckWorkOperation assesses this operation, and every supplied writer for a
// claim. The host must establish completeness independently; a scan cannot do it.
func CheckWorkOperation(operation string, peers []readercaps.Observation, complete bool) error {
	if !complete || len(peers) == 0 {
		return Unverifiable("complete participating reader/writer observations unavailable", nil)
	}
	requirements := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{{Operation: operation, Schemas: []string{"brief-v4"}, Semantics: []string{"qualified-brief-subject-v1"}}}}
	for _, peer := range peers {
		result := readercaps.Assess(requirements, peer)
		if result.State == readercaps.Unsupported {
			return Refused(result.Reason)
		}
		if result.State != readercaps.Supported {
			return Unverifiable(result.Reason, nil)
		}
	}
	return nil
}

// NegotiatedWorkSyntax identifies inputs which may never fall back to a legacy
// reducer, even when the typed parser refuses them. This is routing, not parsing.
func NegotiatedWorkSyntax(value string) bool {
	return strings.Contains(value, "@") || strings.Contains(value, ":b-") || strings.Contains(value, "/b-") || strings.Contains(value, ":m-") || strings.Contains(value, "/m-")
}

const subjectToken = "subject-v1="

// EncodeClaimSubject appends one token after the five historical leading fields.
// A note cannot supply subject metadata. Canonical JSON also removes decoder
// disagreement about duplicate/unknown/case-folded keys from this wire format.
func EncodeClaimSubject(message string, subject BriefSubject) (string, error) {
	if err := subject.Validate(); err != nil {
		return "", err
	}
	fields := strings.Fields(message)
	if !claimSubjectPrefix(fields) {
		return "", Refused("invalid claim payload prefix")
	}
	for _, field := range fields[5:] {
		if claimSubjectField(field) {
			return "", Refused("duplicate or note-injected claim subject token")
		}
	}
	raw, err := json.Marshal(subject)
	if err != nil {
		return "", err
	}
	prefix := strings.Join(fields[:5], " ") + " " + subjectToken + base64.RawURLEncoding.EncodeToString(raw)
	if len(fields) > 5 {
		prefix += " " + strings.Join(fields[5:], " ")
	}
	return prefix, nil
}

// DecodeClaimSubject keeps a historical payload readable as held (present=false),
// but rejects duplicate, misplaced, malformed and note-injected subject tokens.
func DecodeClaimSubject(message string) (subject BriefSubject, present bool, err error) {
	fields := strings.Fields(message)
	prefixValid := claimSubjectPrefix(fields)
	token := ""
	for i, field := range fields {
		// A well-formed header's opaque values are not metadata tokens. If the
		// header is malformed, a misplaced subject token still refuses below.
		if i < 5 && prefixValid {
			continue
		}
		if !claimSubjectField(field) {
			continue
		}
		if i != 5 || token != "" || !strings.HasPrefix(field, subjectToken) {
			return subject, false, Refused("duplicate or note-injected claim subject token")
		}
		token = strings.TrimPrefix(field, subjectToken)
		if token == "" {
			return subject, false, Refused("empty claim subject token")
		}
	}
	if token == "" {
		return subject, false, nil
	}
	if !prefixValid {
		return subject, false, Refused("invalid claim payload prefix")
	}
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) > 2048 {
		return subject, false, Refused("malformed claim subject token")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&subject); e != nil {
		return subject, false, Refused("malformed claim subject")
	}
	canonical, e := json.Marshal(subject)
	if e != nil || !bytes.Equal(raw, canonical) {
		return subject, false, Refused("noncanonical or duplicate claim subject fields")
	}
	if e := subject.Validate(); e != nil {
		return subject, false, e
	}
	return subject, true, nil
}

// ResolveTrailerWork preserves link kinds, source and multiplicity; Authors never
// becomes a delivery edge. The caller still proves changed-file completeness.
func ResolveTrailerWork(body []byte, catalog WorkCatalog) (TrailerKind, []WorkSelection, error) {
	trailers, err := ParseTrailers(body)
	if err != nil {
		return "", nil, err
	}
	if len(trailers) == 0 {
		return "", nil, nil
	}
	link := trailers[0]
	if link.Kind == TrailerIssue {
		return link.Kind, nil, nil
	}
	values := []string{link.Value}
	if link.Kind == TrailerAuthors {
		values = strings.FieldsFunc(link.Value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	}
	if len(values) == 0 {
		return link.Kind, nil, Refused("Authors names no brief")
	}
	var selected []WorkSelection
	seen := map[string]bool{}
	for _, value := range values {
		work, err := catalog.Select(value)
		if err != nil {
			return link.Kind, nil, err
		}
		local := work.Reference.Cell == catalog.Declaring.Cell && work.Reference.Repo == catalog.Declaring.Repo
		if (link.Kind == TrailerCrossRepoBrief && local) || (link.Kind != TrailerCrossRepoBrief && !local) {
			return link.Kind, nil, Refused("brief trailer owning root disagrees with link kind")
		}
		if seen[work.Reference.LogicalKey] {
			return link.Kind, nil, Refused("duplicate logical brief trailer")
		}
		seen[work.Reference.LogicalKey] = true
		selected = append(selected, work)
	}
	return link.Kind, selected, nil
}

func (s WorkSelection) String() string {
	return fmt.Sprintf("%s@%d", s.Reference.Identity.String(), s.Subject.Version)
}

func claimSubjectPrefix(fields []string) bool {
	if len(fields) < 5 || fields[0] != "dispatch-claim" {
		return false
	}
	for i, key := range []string{"owner=", "state=", "branch="} {
		if !strings.HasPrefix(fields[i+2], key) || len(fields[i+2]) == len(key) {
			return false
		}
	}
	return fields[3] == "state=claimed" || fields[3] == "state=dispatched"
}

// Reserve metadata keys, while retaining ordinary historical note prose such as
// "discuss the subject". A versioned subject token embedded in note= still holds.
func claimSubjectField(field string) bool {
	return (strings.HasPrefix(field, "subject") && strings.Contains(field, "=")) || strings.Contains(field, "subject-v")
}
