package domain

import "encoding/json"

// MarshalJSON renders an escalation with its derivation honesty intact.
//
// The one rule this method exists to enforce: a field the index COULD NOT
// DERIVE renders as an explicit null with a reason beside it in
// `field_confidence` — never as a zero value, never omitted. Concretely:
//
//   - `automation` is null (not `{"cell":"","role":""}`) when the filing
//     identity could not be joined to a configured loop. An unjoinable row
//     is UNRESOLVED, and it is still served — dropping it would hide exactly
//     the filing nobody owns.
//   - `options` is null for a prose-v1 body (correct: there were none) and
//     also null for a v2 body whose block did not parse — the two are told
//     apart by `field_confidence.options`, clean in the first case and
//     could-not-check in the second, with `raw_body` carried alongside.
//
// A consumer that reads only the value sees null and must handle it. A
// consumer that reads only `field_confidence` sees why. Neither can see a
// confident zero, because this method never writes one.
func (e Escalation) MarshalJSON() ([]byte, error) {
	type alias Escalation // break the recursion

	out := struct {
		alias
		Automation *AutomationRef `json:"automation"`
		Unresolved []string       `json:"unresolved_fields,omitempty"`
	}{alias: alias(e)}

	if !e.Automation.Zero() {
		a := e.Automation
		out.Automation = &a
	}
	out.Unresolved = e.Fields.Unresolved()

	// Defensive: an escalation whose Fields map is invalid is a programming
	// error in the adapter, and it must not reach a consumer looking fine.
	if err := e.Fields.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores an escalation, including the Automation field that
// MarshalJSON renders through a pointer.
//
// Without this, a decoded escalation would silently lose its automation
// (the field is tagged `json:"-"` so the custom marshaller can render null),
// and a consumer reading over HTTP would see every row as unattributable —
// turning a clean join into a fabricated could-not-check. A custom
// marshaller without its matching unmarshaller is a one-way lie.
func (e *Escalation) UnmarshalJSON(b []byte) error {
	type alias Escalation

	var in struct {
		alias
		Automation *AutomationRef `json:"automation"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	*e = Escalation(in.alias)
	if in.Automation != nil {
		e.Automation = *in.Automation
	}
	return nil
}
