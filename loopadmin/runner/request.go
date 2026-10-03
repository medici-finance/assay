package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// Version is the protocol version string a v1 request carries.
const Version = "loop-admin-runner/v1"

// Mode is the execution mode of a launch request.
type Mode string

const (
	// ModeStandingDesk launches a role session for an existing desk or queue.
	// It needs a desk binding and no graph reference.
	ModeStandingDesk Mode = "standing-desk"
	// ModeWorkflowStage launches one bounded stage of a workflow instance. It
	// needs the canonical work, node and attempt references.
	ModeWorkflowStage Mode = "workflow-stage"
)

// Trust records where a piece of content came from. The contract preserves it;
// it never raises it.
type Trust string

const (
	// TrustOperator is content from an operator-approved profile or packet.
	TrustOperator Trust = "operator"
	// TrustCaller is content the caller assembled from its own records.
	TrustCaller Trust = "caller"
	// TrustUntrusted is model output, or content from outside the caller. It is
	// untrusted until a caller's own check accepts it.
	TrustUntrusted Trust = "untrusted"
)

// Capability names an optional adapter facility a request may make mandatory.
type Capability string

const (
	CapResume        Capability = "resume"
	CapCancelAck     Capability = "cancel-ack"
	CapBudgetLaunch  Capability = "budget-launch"
	CapBudgetRequest Capability = "budget-request"
	CapTelemetry     Capability = "telemetry"
	CapSnapshot      Capability = "snapshot"
	CapLaunchDedupe  Capability = "launch-dedupe"
)

var knownCapabilities = map[Capability]bool{
	CapResume: true, CapCancelAck: true, CapBudgetLaunch: true,
	CapBudgetRequest: true, CapTelemetry: true, CapSnapshot: true,
	CapLaunchDedupe: true,
}

// BudgetScope says whether a reservation bounds the whole launch or each
// request inside it. An adapter declares which it can enforce.
type BudgetScope string

const (
	BudgetLaunch  BudgetScope = "launch"
	BudgetRequest BudgetScope = "request"
)

// RolePacket is the role's instructions as a reference plus the trust of its
// source. It never carries credentials.
type RolePacket struct {
	Role  string `json:"role"`
	Ref   string `json:"ref"`
	Hash  string `json:"hash,omitempty"`
	Trust Trust  `json:"trust"`
}

// Profile is the operator-approved, pinned model/skill/tool profile. Tools is
// the complete list of tools the invocation may request; anything else is
// refused whatever the model says.
type Profile struct {
	ID    string   `json:"id"`
	Model string   `json:"model"`
	Skill string   `json:"skill,omitempty"`
	Tools []string `json:"tools"`
}

// Authority is the externally validated claim or ownership generation the
// launch runs under. The contract does not mint it and does not validate the
// claim itself; ValidatedBy names the component that did. Key identifies what
// the generation fences (a desk claim key, or a workflow attempt key).
type Authority struct {
	Key         string `json:"key"`
	Generation  uint64 `json:"generation"`
	ValidatedBy string `json:"validated_by"`
}

// Reservation is a reference to a budget reservation made by the caller. The
// contract carries it and the adapter enforces it at the declared scope.
type Reservation struct {
	ID            string      `json:"id"`
	Scope         BudgetScope `json:"scope"`
	MaxTokens     int64       `json:"max_tokens,omitempty"`
	MaxCostMicros int64       `json:"max_cost_micros,omitempty"`
}

// DeskRef is the standing-desk reference.
type DeskRef struct {
	BindingID string `json:"binding_id"`
}

// WorkRef holds the canonical workflow references. They are references to
// existing identities, never replacements for them.
type WorkRef struct {
	WorkID    string `json:"work_id"`
	NodeID    string `json:"node_id"`
	AttemptID string `json:"attempt_id"`
}

// SessionRef names a session to resume. It is pinned to a role and profile.
type SessionRef struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	ProfileID string `json:"profile_id"`
}

// LaunchRequest starts one invocation. (Caller, ID) is its stable identity: the
// same pair always means the same request.
type LaunchRequest struct {
	Version    string                     `json:"version"`
	Caller     string                     `json:"caller"`
	ID         string                     `json:"id"`
	Mode       Mode                       `json:"mode"`
	Desk       *DeskRef                   `json:"desk,omitempty"`
	Work       *WorkRef                   `json:"work,omitempty"`
	Packet     RolePacket                 `json:"packet"`
	Profile    Profile                    `json:"profile"`
	Workspace  string                     `json:"workspace"`
	Authority  Authority                  `json:"authority"`
	Budget     Reservation                `json:"budget"`
	Resume     *SessionRef                `json:"resume,omitempty"`
	Require    []string                   `json:"require,omitempty"`
	Extensions map[string]json.RawMessage `json:"ext,omitempty"`
}

// Ref is the stable handle for an invocation, derived from its request.
type Ref struct {
	Caller     string `json:"caller"`
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
}

// Ref returns the invocation's handle.
func (r LaunchRequest) Ref() Ref {
	return Ref{Caller: r.Caller, ID: r.ID, Generation: r.Authority.Generation}
}

func (r Ref) key() string { return r.Caller + "\x00" + r.ID }

// DecodeRequest parses a request and refuses any field this version does not
// define: an unknown field might be mandatory, and a v1 consumer cannot tell.
// Optional additions travel under "ext" instead.
//
// Decoding is strict in the ways encoding/json alone is not: a key must match a
// defined field EXACTLY (encoding/json matches case-insensitively, so a
// case-variant duplicate could silently overwrite a mandatory field), no object
// may repeat a key, a defined field may not be null, and nothing may follow the
// request object. Refusals name the defined path or the byte offset, never the
// offending content.
func DecodeRequest(b []byte) (LaunchRequest, error) {
	if err := scanStrict(b); err != nil {
		return LaunchRequest{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	if err := checkKeys(json.RawMessage(b), reflect.TypeOf(LaunchRequest{}), "request"); err != nil {
		return LaunchRequest{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	var r LaunchRequest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return LaunchRequest{}, fmt.Errorf("%w: %w", ErrInvalidRequest, decodeError(err))
	}
	return r, nil
}

// decodeError reduces an encoding/json error to its position, so a refusal
// never carries request content back to whoever logs it.
func decodeError(err error) error {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syn):
		return fmt.Errorf("syntax error at offset %d", syn.Offset)
	case errors.As(err, &typ):
		return fmt.Errorf("wrong value type at offset %d", typ.Offset)
	}
	return errors.New("does not decode as a v1 request")
}

// scanStrict walks the raw document token by token: one JSON value, no
// repeated key in any object, and nothing after it.
func scanStrict(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := scanValue(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing data after offset %d", dec.InputOffset())
	}
	return nil
}

func scanValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return decodeError(err)
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return decodeError(err)
			}
			k, _ := kt.(string)
			if seen[k] {
				return fmt.Errorf("duplicate key at offset %d", dec.InputOffset())
			}
			seen[k] = true
			if err := scanValue(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := scanValue(dec); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // the closing delimiter
		return decodeError(err)
	}
	return nil
}

// fieldPath names a place in a request by the contract's own definition: a
// constant, a json tag of a defined struct field, or an array index. It is the
// one non-constant string a refusal may format, and nothing from a request can
// become one: the source guard (TestNoPayloadInErrors) refuses a conversion to
// it outside these methods unless it converts a constant, and these methods
// take a type and an index, never a string.
type fieldPath string

// field extends the path by the json tag of t's i-th field.
func (p fieldPath) field(t reflect.Type, i int) fieldPath {
	tag, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
	return p + "." + fieldPath(tag)
}

// index extends the path by an array index.
func (p fieldPath) index(i int) fieldPath {
	return p + fieldPath("["+strconv.Itoa(i)+"]")
}

// checkKeys refuses a key that is not EXACTLY a json tag of the struct it
// decodes into, and a null for a defined field, at every depth of the defined
// shape. A map field (ext) takes any key; its values are checked elsewhere.
func checkKeys(raw json.RawMessage, t reflect.Type, path fieldPath) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return fmt.Errorf("%s is not an object", path)
		}
		fields := map[string]int{}
		for i := 0; i < t.NumField(); i++ {
			tag, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			if tag != "" && tag != "-" {
				fields[tag] = i
			}
		}
		for k, v := range obj {
			fi, ok := fields[k]
			if !ok {
				return fmt.Errorf("%s carries a key %s does not define", path, Version)
			}
			// The path is built from the defined field, never from k.
			sub := path.field(t, fi)
			if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return fmt.Errorf("%s is null", sub)
			}
			if err := checkKeys(v, t.Field(fi).Type, sub); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if t.Elem().Kind() != reflect.Struct && t.Elem().Kind() != reflect.Pointer {
			return nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("%s is not an array", path)
		}
		for i, v := range items {
			if err := checkKeys(v, t.Elem(), path.index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate checks everything about a request that does not depend on an
// adapter: version, identity, mode references, pinned profile, authority,
// budget and credential exclusion. Credential exclusion runs first, and no
// refusal formats request content into its error: a caller that logs refusals
// must never log what the request carried.
func (r LaunchRequest) Validate() error {
	if err := r.rejectCredentials(); err != nil {
		return err
	}
	if r.Version != Version {
		return ErrUnsupportedVersion
	}
	for name, v := range map[fieldPath]string{
		"caller": r.Caller, "id": r.ID, "packet.role": r.Packet.Role,
		"packet.ref": r.Packet.Ref, "profile.id": r.Profile.ID,
		"profile.model": r.Profile.Model, "workspace": r.Workspace,
		"authority.key": r.Authority.Key, "authority.validated_by": r.Authority.ValidatedBy,
		"budget.id": r.Budget.ID,
	} {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%w: %s is required", ErrInvalidRequest, name)
		}
	}
	if r.Authority.Generation == 0 {
		return fmt.Errorf("%w: authority.generation is required", ErrInvalidRequest)
	}
	switch r.Packet.Trust {
	case TrustOperator, TrustCaller, TrustUntrusted:
	default:
		return fmt.Errorf("%w: packet.trust is not a defined trust", ErrInvalidRequest)
	}
	if r.Profile.Tools == nil {
		return fmt.Errorf("%w: profile.tools is required (an empty list pins no tools)", ErrInvalidRequest)
	}
	switch r.Budget.Scope {
	case BudgetLaunch, BudgetRequest:
	default:
		return fmt.Errorf("%w: budget.scope is not a defined scope", ErrInvalidRequest)
	}
	if r.Budget.MaxTokens < 0 || r.Budget.MaxCostMicros < 0 {
		return fmt.Errorf("%w: a budget limit is negative", ErrInvalidRequest)
	}
	if r.Budget.MaxTokens == 0 && r.Budget.MaxCostMicros == 0 {
		return fmt.Errorf("%w: budget carries no limit", ErrInvalidRequest)
	}
	if err := r.validateMode(); err != nil {
		return err
	}
	if r.Resume != nil {
		if strings.TrimSpace(r.Resume.SessionID) == "" || r.Resume.Role != r.Packet.Role || r.Resume.ProfileID != r.Profile.ID {
			return fmt.Errorf("%w: resume must name a session pinned to this role and profile", ErrInvalidRequest)
		}
	}
	return nil
}

func (r LaunchRequest) validateMode() error {
	switch r.Mode {
	case ModeStandingDesk:
		if r.Desk == nil || strings.TrimSpace(r.Desk.BindingID) == "" {
			return fmt.Errorf("%w: standing-desk mode requires desk.binding_id", ErrModeFields)
		}
		if r.Work != nil {
			return fmt.Errorf("%w: standing-desk mode takes no workflow references", ErrModeFields)
		}
	case ModeWorkflowStage:
		w := r.Work
		if w == nil || blank(w.WorkID) || blank(w.NodeID) || blank(w.AttemptID) {
			return fmt.Errorf("%w: workflow-stage mode requires work_id, node_id and attempt_id", ErrModeFields)
		}
		if r.Desk != nil {
			return fmt.Errorf("%w: workflow-stage mode takes no desk binding", ErrModeFields)
		}
	default:
		return fmt.Errorf("%w: mode is not a defined mode", ErrInvalidRequest)
	}
	return nil
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func (r LaunchRequest) rejectCredentials() error {
	fields := []string{r.Version, string(r.Mode), string(r.Packet.Trust), string(r.Budget.Scope),
		r.Caller, r.ID, r.Packet.Role, r.Packet.Ref, r.Packet.Hash,
		r.Profile.ID, r.Profile.Model, r.Profile.Skill, r.Workspace,
		r.Authority.Key, r.Authority.ValidatedBy, r.Budget.ID}
	fields = append(fields, r.Profile.Tools...)
	fields = append(fields, r.Require...)
	if r.Desk != nil {
		fields = append(fields, r.Desk.BindingID)
	}
	if r.Work != nil {
		fields = append(fields, r.Work.WorkID, r.Work.NodeID, r.Work.AttemptID)
	}
	if r.Resume != nil {
		fields = append(fields, r.Resume.SessionID, r.Resume.Role, r.Resume.ProfileID)
	}
	for _, f := range fields {
		if LooksLikeCredential(f) {
			return ErrCredential
		}
	}
	for k, v := range r.Extensions {
		if !json.Valid(v) {
			return fmt.Errorf("%w: an extension value is not JSON", ErrInvalidRequest)
		}
		if extensionCarriesCredential(k, v) {
			// The key itself may be the credential: never echo it.
			return fmt.Errorf("%w: in an extension", ErrCredential)
		}
	}
	return nil
}

// CheckCapabilities refuses the request when it marks anything mandatory that
// the adapter does not provide, or that this contract does not define. A
// requested resume the adapter cannot do is refused too, never started fresh.
func (r LaunchRequest) CheckCapabilities(c Capabilities) error {
	for i, name := range r.Require {
		if ext, ok := strings.CutPrefix(name, "ext:"); ok {
			if !c.hasExtension(ext) {
				return fmt.Errorf("%w: require[%d] names an extension the adapter does not declare", ErrUnsupportedMandatory, i)
			}
			continue
		}
		cap := Capability(name)
		if !knownCapabilities[cap] {
			return fmt.Errorf("%w: require[%d] is not defined by %s", ErrUnsupportedMandatory, i, Version)
		}
		if !c.Has(cap) {
			return fmt.Errorf("%w: require[%d] names a capability the adapter does not declare", ErrUnsupportedMandatory, i)
		}
	}
	if r.Resume != nil && !c.Resume {
		return ErrResumeUnsupported
	}
	if !c.supportsBudget(r.Budget.Scope) {
		return fmt.Errorf("%w: adapter cannot enforce the budget's scope", ErrUnsupportedMandatory)
	}
	return nil
}
