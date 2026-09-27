// Package samplepkg is TestCountsFixture's refusals-dimension fixture: one call to each
// of the three refusal constructors, unqualified, plus one call through a qualifier —
// counted the same way ("any qualifier") — for four refusal call sites total.
package samplepkg

import "example.invalid/deskkit"

func doWork() error {
	if err := Refused("first — unqualified Refused"); err != nil {
		return err
	}
	if err := RefusedWithCause("second — unqualified RefusedWithCause", nil); err != nil {
		return err
	}
	if err := RefusedFinding("third — unqualified RefusedFinding", nil); err != nil {
		return err
	}
	return deskkit.Refused("fourth — qualified Refused")
}

func Refused(msg string) error                       { return nil }
func RefusedWithCause(msg string, cause error) error { return nil }
func RefusedFinding(msg string, finding any) error   { return nil }
