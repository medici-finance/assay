package deskkit

// addressto.go — the `to:<role>` desk-inbox addressee stamp.
//
// WHAT THIS IS. A `raised-by:<role>` label answers "which desk NOTICED this"; a
// `to:<role>` label answers "which desk is this FOR". They are the two halves of the
// same provenance vocabulary — an issue can be raised by the worker desk and addressed
// to the verifier — and they DELIBERATELY share one resolver (boundRole, raisedby.go)
// so a role the roster does not bind can be neither stamped nor addressed. A second
// hand-maintained role list is the drift raisedby.go's derive-or-diff design exists to
// prevent, so there is none here either: the vocabulary is RaisedByRoles(), a pure
// projection of the roster, and `to:` is just a second prefix over it.
//
// WHY IT MATTERS. Before this label a desk could not be addressed at all: cross-desk
// coordination ran through the operator relaying a typed "tell the-desk…" by hand. A
// `to:<role>` item is a durable, forge-visible message TO a desk — its own desk's sweep
// leads with it (fanoutloop/issueboard), and a second, independent layer (issueboard's
// SLA arm) surfaces an unread one past the SLA without the addressee's cooperation.
//
// THE READER IS THREE-STATE, exactly as RaisedByOf is: an inbox that folds "no
// addressee" and "two conflicting addressees" into one answer routes work on a guess.
// AddressedToOf returns the same RaisedByState enum, reusing its Unknown / Stamped /
// Indeterminate meanings — see stampOf.

// AddressedToPrefix is the label prefix carrying the desk-inbox addressee stamp. Like
// RaisedByPrefix it is the ONE spelling of it: writers build the label through
// AddressedToLabel and readers match through AddressedToOf, so no consumer restates it.
const AddressedToPrefix = "to:"

// AddressedToLabel validates role against the roster's DERIVED desk vocabulary (the same
// boundRole resolver `--raised-by` uses — one resolver, two flags) and returns the
// `to:<role>` label to apply. An unbound role is REFUSED (exit 5) with the bound set
// named; everything else about the stamp degrades rather than blocks, exactly as the
// raised-by stamp does (deskfile's resolveAddressedToStamp).
func AddressedToLabel(role string) (string, error) {
	r, err := boundRole(role)
	if err != nil {
		return "", err
	}
	return AddressedToPrefix + r, nil
}

// AddressedToOf is the READER contract for the addressee stamp: given an issue's label
// names, answer which desk it is addressed to, in the three states RaisedByState
// enumerates. The role is returned lowercased and verbatim from the label and is NOT
// re-validated against the current roster (a stamp is a historical fact, same rule as
// RaisedByOf). role is "" for every state except RaisedByStamped.
func AddressedToOf(labels []string) (role string, state RaisedByState) {
	return stampOf(labels, AddressedToPrefix)
}
