package productintent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	repoRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)
	reqRE  = regexp.MustCompile(`^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	textRE = regexp.MustCompile(`[^ \t\r\n]`)
	dateRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// Decode accepts one JSON extension, or no bytes for a legacy unbound REQ.
// Unsupported versions are retained explicitly; their payload is never evaluated.
// semantic: S-product-intent
func Decode(data []byte) (Document, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Document{}, nil
	}
	if !utf8.Valid(data) {
		return Document{}, fmt.Errorf("product-intent: invalid UTF-8")
	}
	// Well-formedness (duplicate keys, trailing data, a null envelope) binds every
	// version. Nested nulls are refused only once the version is known supported:
	// refusing them in an unsupported payload would evaluate that payload.
	if err := wellFormed(data, false); err != nil {
		return Document{}, err
	}
	var header struct {
		Schema    string `json:"schema"`
		Mandatory bool   `json:"mandatory"`
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Document{}, err
	}
	if err := json.Unmarshal(envelope["schema"], &header.Schema); err != nil {
		return Document{}, fmt.Errorf("product-intent: schema must be a string")
	}
	if raw, ok := envelope["mandatory"]; ok {
		if string(raw) == "null" {
			return Document{}, fmt.Errorf("product-intent: mandatory must be a boolean")
		}
		if err := json.Unmarshal(raw, &header.Mandatory); err != nil {
			return Document{}, err
		}
	}
	if header.Schema == "" {
		return Document{}, fmt.Errorf("product-intent: schema is required")
	}
	d := Document{raw: append([]byte(nil), data...), version: header.Schema, mandatory: header.Mandatory}
	if header.Schema != Schema {
		return d, nil
	}
	if err := wellFormed(data, true); err != nil {
		return Document{}, err
	}
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return Document{}, err
	}
	if err := exactFields(tree, reflect.TypeOf(Record{})); err != nil {
		return Document{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r Record
	if err := dec.Decode(&r); err != nil {
		return Document{}, fmt.Errorf("product-intent: %w", err)
	}
	if err := Validate(r); err != nil {
		return Document{}, err
	}
	d.record = &r
	return d, nil
}

// wellFormed refuses duplicate member names at every depth, trailing JSON and a
// null envelope, including in unsupported envelopes. With nestedNull false a
// nested null is retained as payload; supported records pass nestedNull true.
func wellFormed(data []byte, nestedNull bool) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueKeys(dec, nestedNull, true); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("product-intent: trailing JSON")
	}
	return nil
}

// uniqueKeys uses the standard codec's tokens to refuse duplicate member names.
// It does not interpret product semantics.
func uniqueKeys(dec *json.Decoder, nestedNull, top bool) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil && (top || nestedNull) {
		return fmt.Errorf("product-intent: null is not a record value")
	}
	if delim, ok := tok.(json.Delim); ok {
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("product-intent: duplicate or invalid key")
				}
				seen[s] = true
			}
			if err := uniqueKeys(dec, nestedNull, false); err != nil {
				return err
			}
		}
		_, err = dec.Token()
	}
	return err
}

// encoding/json accepts case-folded field names. The wire contract is an exact,
// closed member set; inspect names against the declared tags before typed decode.
func exactFields(value any, typ reflect.Type) error {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		fields := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			fields[strings.Split(f.Tag.Get("json"), ",")[0]] = f.Type
		}
		if object, ok := value.(map[string]any); ok {
			for key, child := range object {
				field, known := fields[key]
				if !known {
					return fmt.Errorf("product-intent: unknown field %q", key)
				}
				if err := exactFields(child, field); err != nil {
					return err
				}
			}
		}
	case reflect.Slice:
		if items, ok := value.([]any); ok {
			for _, child := range items {
				if err := exactFields(child, typ.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func member(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func text(s string) bool { return textRE.MatchString(s) }
func reference(r Reference, kinds ...string) bool {
	return repoRE.MatchString(r.Repository) && nameRE.MatchString(r.ID) && nameRE.MatchString(r.Revision) && member(r.Kind, kinds...)
}
func confidence(s string) bool { return member(s, "low", "medium", "high", "unknown") }

// Validate checks supported record shape and internal identities. It never
// resolves external decisions, assessments, operational definitions or benefits.
func Validate(r Record) error {
	bad := func(field string) error { return fmt.Errorf("product-intent: invalid or missing %s", field) }
	if r.Schema != Schema {
		return bad("schema (unsupported version)")
	}
	if !repoRE.MatchString(r.Subject.Repository) || !reqRE.MatchString(r.Subject.Requirement) {
		return bad("subject")
	}
	if !member(r.Kind, "feature", "capability", "journey") || !text(r.Owner) || !member(r.Status, "active", "withdrawn") {
		return bad("kind, owner or status")
	}
	if len(r.Directions) == 0 || len(r.Targets) == 0 {
		return bad("directions or targets")
	}
	dirs, targets := map[string]bool{}, map[string]bool{}
	for _, d := range r.Directions {
		if !nameRE.MatchString(d.Revision) || dirs[d.Revision] || !text(d.Rationale) || !text(d.Owner) || !reference(d.Decision, "decision") {
			return bad("direction revision, fields or decision")
		}
		dirs[d.Revision] = true
	}
	for _, t := range r.Targets {
		if !nameRE.MatchString(t.Revision) || targets[t.Revision] || !dirs[t.DirectionRevision] || !text(t.Condition) || !text(t.Measure) || !text(t.Unit) || !text(t.Window) || !confidence(t.Confidence) || !text(t.Basis) || !reference(t.Decision, "decision") {
			return bad("target revision, direction, fields or decision")
		}
		targets[t.Revision] = true
	}
	for _, o := range r.Observed {
		if !targets[o.TargetRevision] || !reference(o.Assessment, "assessment") || !text(o.Source) || !text(o.AttributedTo) || !dateRE.MatchString(o.Date) || !text(o.Window) || !confidence(o.Confidence) || !text(o.Limitations) {
			return bad("observed target, assessment or attribution")
		}
	}
	for _, ref := range r.OperationalReferences {
		if !reference(ref, "journey", "operational-workflow", "practice") {
			return bad("operational reference kind or qualified identity")
		}
	}
	return nil
}

// CheckHistory verifies an append-only transition when a prior snapshot exists.
// It retains tombstones, names' historical owners, target directions and evidence.
// Neither the filesystem adapter nor this package invents a missing prior snapshot.
func CheckHistory(before, after Record) error {
	if err := Validate(before); err != nil {
		return err
	}
	if err := Validate(after); err != nil {
		return err
	}
	if before.Subject != after.Subject || before.Kind != after.Kind || (before.Status == "withdrawn" && after.Status != "withdrawn") {
		return fmt.Errorf("product-intent: identity or tombstone changed")
	}
	prefix := func(a, b any) bool {
		av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
		return bv.Len() >= av.Len() && (av.Len() == 0 || reflect.DeepEqual(a, bv.Slice(0, av.Len()).Interface()))
	}
	if !prefix(before.Directions, after.Directions) || !prefix(before.Targets, after.Targets) || !prefix(before.Observed, after.Observed) || !prefix(before.OperationalReferences, after.OperationalReferences) {
		return fmt.Errorf("product-intent: prior history overwritten or removed")
	}
	return nil
}

// Resolve returns detached historical selections. Authorities and decisions must
// be supplied independently of the record; an absent/mismatched/conflicting source
// remains unknown. A withdrawn identity never supplies a current approved target.
func Resolve(d Document, evidence Evidence, requirementStatus string) Resolution {
	out := Resolution{State: "unbound", Version: d.version, Mandatory: d.mandatory}
	if d.version == "" {
		return out
	}
	if d.record == nil {
		out.State = "unsupported"
		return out
	}
	r := d.Record()
	out.State = "known"
	statusKnown := member(requirementStatus, "proposed", "accepted", "satisfied", "withdrawn")
	withdrawn := r.Status == "withdrawn" || requirementStatus == "withdrawn"
	if withdrawn {
		out.State = "withdrawn"
	}
	dirs := map[string]Direction{}
	for _, dir := range r.Directions {
		dirs[dir.Revision] = dir
	}
	for _, target := range r.Targets {
		identity := Revision{r.Subject, "target", target.Revision}
		dir := dirs[target.DirectionRevision]
		approval := decision(target.Decision, identity, evidence)
		da := decision(dir.Decision, Revision{r.Subject, "direction", dir.Revision}, evidence)
		if approval != "rejected" && da != "approved" {
			approval = da
		}
		if !statusKnown {
			approval = "unknown"
		}
		state := "unknown"
		if approval == "approved" {
			state = "known"
		}
		if withdrawn {
			state = "withdrawn"
		} else if state == "unknown" {
			out.State = "unknown"
		}
		rt := ResolvedTarget{Identity: identity, Target: target, Direction: dir, Approval: approval, State: state}
		for _, o := range r.Observed {
			if o.TargetRevision == target.Revision {
				rt.Observed = append(rt.Observed, o)
			}
		}
		out.Targets = append(out.Targets, rt)
	}
	return out
}

func decision(ref Reference, subject Revision, e Evidence) string {
	result := "unknown"
	found := false
	for _, d := range e.Decisions {
		if d.Reference != ref {
			continue
		}
		verdict := "unknown"
		if d.Available && d.Subject == subject && text(d.Source) && strings.TrimSpace(d.Authority) != "" && member(d.Authority, e.Authorities...) && member(d.Verdict, "approved", "rejected") {
			verdict = d.Verdict
		}
		if found && verdict != result {
			return "unknown"
		}
		found, result = true, verdict
	}
	return result
}
