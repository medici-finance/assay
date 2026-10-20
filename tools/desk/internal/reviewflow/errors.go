// Package reviewflow owns deterministic review classification and planning.
// Its inputs are values, not credentials, claims or permission to act.
package reviewflow

import "fmt"

type ErrorKind string

const (
	SchemaError  ErrorKind = "schema"
	PartialError ErrorKind = "partial"
	StaleError   ErrorKind = "stale"
)

type InputError struct {
	Kind   ErrorKind
	Detail string
	Cause  error
}

func (e *InputError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Detail, e.Cause)
	}
	return string(e.Kind) + ": " + e.Detail
}
func (e *InputError) Unwrap() error { return e.Cause }
func invalid(detail string, cause error) error {
	return &InputError{Kind: SchemaError, Detail: detail, Cause: cause}
}
