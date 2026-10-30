// Package readercaps assesses supplied reader contracts. It performs no I/O and
// grants no authority to execute, publish, claim or accept work.
package readercaps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const AdvertisementSchema = "reader-capabilities-v1"
const RequirementsSchema = "reader-requirements-v1"

type Operation struct {
	Operation string   `json:"operation"`
	Schemas   []string `json:"schemas"`
	Semantics []string `json:"semantics"`
}

type Advertisement struct {
	Schema     string      `json:"schema"`
	Provider   string      `json:"provider"`
	Version    string      `json:"version"`
	Operations []Operation `json:"operations"`
}

// Requirements are mandatory, operation-scoped obligations, never hints. All
// entries must be satisfied; an unrecognized feature remains an obligation.
type Requirements struct {
	Schema   string      `json:"schema"`
	Required []Operation `json:"required"`
}

type State string

const (
	Supported     State = "supported"
	Unsupported   State = "unsupported"
	CouldNotCheck State = "could-not-check"
)

// ObservationState is established by the host's existing observation boundary.
// Provider/version text establishes neither authenticity nor currentness.
type ObservationState string

const (
	Current    ObservationState = "current"
	Missing    ObservationState = "missing"
	Unreadable ObservationState = "unreadable"
	Malformed  ObservationState = "malformed"
	Stale      ObservationState = "stale"
)

type Observation struct {
	State         ObservationState
	Advertisement Advertisement
}

type Result struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
}

func validText(s string) bool { return s != "" && strings.TrimSpace(s) == s }

func validateOperations(ops []Operation) error {
	if ops == nil {
		return fmt.Errorf("operation list must be explicit")
	}
	seen := map[string]bool{}
	for _, op := range ops {
		if !validText(op.Operation) || seen[op.Operation] || op.Schemas == nil || op.Semantics == nil {
			return fmt.Errorf("invalid or duplicate operation %q", op.Operation)
		}
		seen[op.Operation] = true
		for _, list := range [][]string{op.Schemas, op.Semantics} {
			features := map[string]bool{}
			for _, feature := range list {
				if !validText(feature) || features[feature] {
					return fmt.Errorf("invalid or duplicate feature %q", feature)
				}
				features[feature] = true
			}
		}
	}
	return nil
}

func (a Advertisement) Validate() error {
	if a.Schema != AdvertisementSchema || !validText(a.Provider) || !validText(a.Version) {
		return fmt.Errorf("unknown advertisement schema or missing provider/version")
	}
	return validateOperations(a.Operations)
}

func (r Requirements) Validate() error {
	if r.Schema != RequirementsSchema {
		return fmt.Errorf("unknown requirements schema %q", r.Schema)
	}
	return validateOperations(r.Required)
}

// Decode strictly decodes the versioned envelope, including trailing input. The
// JSON schema describes shape; these decoders also check duplicate operations.
func decode(raw []byte, target any) error {
	// encoding/json otherwise accepts duplicate keys using last-value wins,
	// which could erase a required feature before assessment.
	check := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueValue(check); err != nil {
		return err
	}
	if _, err := check.Token(); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON envelope")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON envelope")
	}
	return nil
}

func uniqueValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid JSON key %q", key)
			}
			seen[name] = true
		}
		if err := uniqueValue(d); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func DecodeAdvertisement(raw []byte) (Advertisement, error) {
	var a Advertisement
	if err := decode(raw, &a); err != nil {
		return a, err
	}
	return a, a.Validate()
}

func DecodeRequirements(raw []byte) (Requirements, error) {
	var r Requirements
	if err := decode(raw, &r); err != nil {
		return r, err
	}
	return r, r.Validate()
}

func contains(list []string, wanted string) bool {
	for _, item := range list {
		if item == wanted {
			return true
		}
	}
	return false
}

// Assess uses exact declared schema/semantic versions. It never infers support
// from a build version, parser permissiveness or another operation's support.
func Assess(required Requirements, peer Observation) Result {
	if err := required.Validate(); err != nil {
		return Result{CouldNotCheck, err.Error()}
	}
	if len(required.Required) == 0 {
		return Result{State: Supported} // no negotiated requirement: legacy behavior
	}
	if peer.State != Current {
		return Result{CouldNotCheck, "peer advertisement is not current: " + string(peer.State)}
	}
	if err := peer.Advertisement.Validate(); err != nil {
		return Result{CouldNotCheck, err.Error()}
	}
	for _, need := range required.Required {
		var support *Operation
		for i := range peer.Advertisement.Operations {
			if peer.Advertisement.Operations[i].Operation == need.Operation {
				support = &peer.Advertisement.Operations[i]
				break
			}
		}
		if support == nil {
			return Result{Unsupported, "unsupported required operation: " + need.Operation}
		}
		for _, pair := range []struct{ need, have []string }{{need.Schemas, support.Schemas}, {need.Semantics, support.Semantics}} {
			for _, feature := range pair.need {
				if !contains(pair.have, feature) {
					return Result{Unsupported, "unsupported required feature for " + need.Operation + ": " + feature}
				}
			}
		}
	}
	return Result{State: Supported}
}
