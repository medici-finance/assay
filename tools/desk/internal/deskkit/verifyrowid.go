package deskkit

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// VerifyRowID preserves a Verify table's identity: 6 and 6a are different rows.
// Numeric identities retain the legacy JSON integer form; any other uses a string.
// It confers no execution, attestation or lifecycle authority.
//
// It is the one row-identity grammar every recorded-row reader and writer shares
// (outcome-record writer, wake receipts, scheduling), and it accepts each `#` cell
// shape verifyrun records: a positive integer (6), an integer with one letter
// suffix (6a), or a letter-led ASCII label (R, S1). A label never starts with a
// digit, so it cannot alias a numeric or suffixed identity; case stays significant.
type VerifyRowID string

var (
	verifyRowNumberPattern = regexp.MustCompile(`^[1-9][0-9]*[a-zA-Z]?$`)
	verifyRowLabelPattern  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)
	// The only identity a bare (unquoted) JSON value may carry.
	verifyRowIntegerPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
)

func ParseVerifyRowID(s string) (VerifyRowID, bool) {
	if verifyRowLabelPattern.MatchString(s) {
		return VerifyRowID(s), true
	}
	if !verifyRowNumberPattern.MatchString(s) {
		return "", false
	}
	digits := s
	if last := s[len(s)-1]; last < '0' || last > '9' {
		digits = s[:len(s)-1]
	}
	if _, err := strconv.Atoi(digits); err != nil {
		return "", false
	}
	return VerifyRowID(s), true
}

func (r VerifyRowID) MarshalJSON() ([]byte, error) {
	if _, ok := ParseVerifyRowID(string(r)); !ok {
		return nil, fmt.Errorf("invalid Verify row identity %q", r)
	}
	if _, err := strconv.Atoi(string(r)); err == nil {
		return []byte(r), nil
	}
	return json.Marshal(string(r))
}

func (r *VerifyRowID) UnmarshalJSON(raw []byte) error {
	s := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
	}
	id, ok := ParseVerifyRowID(s)
	// Outside a JSON string only the integer form is an identity: a bare
	// literal (null, true) is never read as a label.
	if unquoted := len(raw) == 0 || raw[0] != '"'; unquoted && ok && !verifyRowIntegerPattern.MatchString(s) {
		ok = false
	}
	if !ok {
		return fmt.Errorf("Verify row must be a positive integer, a suffixed identity (6a) or a letter-led label (R, S1), got %s", raw)
	}
	*r = id
	return nil
}
