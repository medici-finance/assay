package deskkit

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// VerifyRowID preserves a Verify table's identity: 6 and 6a are different rows.
// Numeric identities retain the legacy JSON integer form; a suffix uses a string.
// It confers no execution, attestation or lifecycle authority.
type VerifyRowID string

var verifyRowIDPattern = regexp.MustCompile(`^[1-9][0-9]*[a-zA-Z]?$`)

func ParseVerifyRowID(s string) (VerifyRowID, bool) {
	if !verifyRowIDPattern.MatchString(s) {
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
	if !ok {
		return fmt.Errorf("Verify row must be a positive integer or exact suffixed identity, got %s", raw)
	}
	*r = id
	return nil
}
