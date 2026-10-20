package cellop

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Expiry is the fixed lifetime of every plan: it expires exactly this long after it was created.
const Expiry = 10 * time.Minute

// ConfirmLen is how many leading hex characters of the digest make the confirm code.
const ConfirmLen = 8

var (
	idRE   = regexp.MustCompile(`^[0-9a-f]{24}$`)
	nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	verbRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	hexRE  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ErrInvalid wraps every reason a plan document fails the contract.
var ErrInvalid = errors.New("invalid cell-op plan")

// Precondition is one piece of evidence an action depends on: the value Key measured when the
// plan was made. Apply measures every key again after the confirm and refuses plan-stale if any
// value differs.
type Precondition struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Action is one planned change to one cell.
type Action struct {
	ID            string         `json:"id"`
	Cell          string         `json:"cell"`
	Summary       string         `json:"summary"`
	Preconditions []Precondition `json:"preconditions"`
}

// ChildRef names one step of a batch plan: a stored child plan and the digest it must still have.
type ChildRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// Plan is the cell-op-plan document. A plan with Children is a batch: it carries no actions of its
// own, and its steps are its children, in order.
type Plan struct {
	Schema      string     `json:"schema"`
	Kind        string     `json:"kind"`
	ID          string     `json:"id"`
	Verb        string     `json:"verb"`
	Cells       []string   `json:"cells"`
	Actions     []Action   `json:"actions"`
	Children    []ChildRef `json:"children"`
	Created     string     `json:"created"`
	Expires     string     `json:"expires"`
	Digest      string     `json:"digest"`
	ConfirmCode string     `json:"confirm_code"`
}

// NewID returns a fresh random plan id (24 lowercase hex characters).
func NewID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("plan id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// NewPlan builds and seals a plan made at now. cells is sorted and de-duplicated; every action's
// cell must be one of them. A batch plan passes children and no actions; any other plan passes
// actions and no children.
func NewPlan(id, verb string, cells []string, actions []Action, children []ChildRef, now time.Time) (Plan, error) {
	set := map[string]bool{}
	for _, c := range cells {
		set[c] = true
	}
	sorted := make([]string, 0, len(set))
	for c := range set {
		sorted = append(sorted, c)
	}
	sort.Strings(sorted)
	if actions == nil {
		actions = []Action{}
	}
	if children == nil {
		children = []ChildRef{}
	}
	for i := range actions {
		if actions[i].Preconditions == nil {
			actions[i].Preconditions = []Precondition{}
		}
	}
	created := now.UTC().Truncate(time.Second)
	p := Plan{
		Schema: Schema, Kind: KindPlan, ID: id, Verb: verb, Cells: sorted,
		Actions: actions, Children: children,
		Created: Stamp(created), Expires: Stamp(created.Add(Expiry)),
	}
	if err := p.Seal(); err != nil {
		return Plan{}, err
	}
	return p, p.Verify()
}

// Canonical is the plan's RFC 8785 (JCS) canonical JSON with digest and confirm_code omitted: the
// bytes the digest covers. confirm_code is omitted with the digest because it is derived from it.
//
// The canonicaliser is deskkit.CanonicalizeJSON. It equals RFC 8785 for every document this
// contract allows: object keys are fixed ASCII member names (byte order and UTF-16 order agree on
// ASCII), strings are escaped by RFC 8785's rule, and a plan carries no numbers at all.
func (p Plan) Canonical() ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "digest")
	delete(m, "confirm_code")
	raw, err = json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return deskkit.CanonicalizeJSON(raw)
}

// ComputeDigest is the sha256 (lowercase hex) of the plan's canonical form.
func (p Plan) ComputeDigest() (string, error) {
	c, err := p.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

// Seal sets the digest and the confirm code from the plan's current content.
func (p *Plan) Seal() error {
	d, err := p.ComputeDigest()
	if err != nil {
		return err
	}
	p.Digest, p.ConfirmCode = d, d[:ConfirmLen]
	return nil
}

// IsBatch reports whether the plan is a batch of child plans.
func (p Plan) IsBatch() bool { return len(p.Children) > 0 }

// ExpiresAt is the parsed expiry; Verify has already proved it parses.
func (p Plan) ExpiresAt() time.Time {
	t, _ := time.Parse(time.RFC3339, p.Expires)
	return t
}

// Expired reports whether now is at or past the plan's expiry.
func (p Plan) Expired(now time.Time) bool { return !now.Before(p.ExpiresAt()) }

// HasCell reports whether cell is one of the cells the plan affects.
func (p Plan) HasCell(cell string) bool {
	i := sort.SearchStrings(p.Cells, cell)
	return i < len(p.Cells) && p.Cells[i] == cell
}

// Verify checks the plan against the contract: its markers, its id, sorted unique cells, every
// action on one of those cells, an expiry exactly Expiry after creation, and a digest and confirm
// code that recompute from its content. Every failure wraps ErrInvalid.
func (p Plan) Verify() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
	}
	if p.Schema != Schema || p.Kind != KindPlan {
		return bad("schema %q kind %q, want %q %q", p.Schema, p.Kind, Schema, KindPlan)
	}
	if !idRE.MatchString(p.ID) {
		return bad("id %q is not 24 lowercase hex characters", p.ID)
	}
	if !verbRE.MatchString(p.Verb) {
		return bad("verb %q", p.Verb)
	}
	if len(p.Cells) == 0 {
		return bad("a plan affects at least one cell")
	}
	for i, c := range p.Cells {
		if !nameRE.MatchString(c) {
			return bad("cell name %q", c)
		}
		if i > 0 && p.Cells[i-1] >= c {
			return bad("cells are not sorted and unique")
		}
	}
	if p.Actions == nil || p.Children == nil {
		return bad("actions and children are lists, never null")
	}
	if p.IsBatch() == (len(p.Actions) > 0) {
		return bad("a plan carries either actions or children (a batch), and not both")
	}
	seen := map[string]bool{}
	for _, a := range p.Actions {
		if a.ID == "" || seen[a.ID] {
			return bad("action id %q is empty or repeated", a.ID)
		}
		seen[a.ID] = true
		if !p.HasCell(a.Cell) {
			return bad("action %s is on cell %q, which the plan does not list", a.ID, a.Cell)
		}
		if a.Preconditions == nil {
			return bad("action %s: preconditions is a list, never null", a.ID)
		}
		keys := map[string]bool{}
		for _, pc := range a.Preconditions {
			if pc.Key == "" || keys[pc.Key] {
				return bad("action %s: precondition key %q is empty or repeated", a.ID, pc.Key)
			}
			keys[pc.Key] = true
		}
	}
	for _, c := range p.Children {
		if !idRE.MatchString(c.ID) || !hexRE.MatchString(c.Digest) || c.ID == p.ID {
			return bad("child %q", c.ID)
		}
	}
	created, err := time.Parse(time.RFC3339, p.Created)
	if err != nil || Stamp(created) != p.Created {
		return bad("created %q is not an RFC 3339 UTC time in whole seconds", p.Created)
	}
	if p.Expires != Stamp(created.Add(Expiry)) {
		return bad("expires %q is not created + %s", p.Expires, Expiry)
	}
	d, err := p.ComputeDigest()
	if err != nil {
		return bad("canonical form: %v", err)
	}
	if p.Digest != d {
		return bad("digest does not recompute from the plan's canonical form")
	}
	if p.ConfirmCode != d[:ConfirmLen] {
		return bad("confirm_code is not the digest's first %d characters", ConfirmLen)
	}
	return nil
}

// DecodePlan parses a stored plan strictly (no unknown fields, one JSON value) and verifies it.
func DecodePlan(raw []byte) (Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if dec.More() {
		return Plan{}, fmt.Errorf("%w: trailing data after the plan", ErrInvalid)
	}
	if err := p.Verify(); err != nil {
		return Plan{}, err
	}
	// encoding/json matches member names case-insensitively and reads a null as an absent
	// value. The stored bytes must be exactly the document that was decoded: compare the two
	// canonical forms, so a "Verb" member or a null list is refused instead of read as another
	// plan than the one on disk.
	stored, err := deskkit.CanonicalizeJSON(raw)
	if err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	again, err := json.Marshal(p)
	if err != nil {
		return Plan{}, err
	}
	decoded, err := deskkit.CanonicalizeJSON(again)
	if err != nil || !bytes.Equal(stored, decoded) {
		return Plan{}, fmt.Errorf("%w: the stored document is not exactly the plan it decodes to", ErrInvalid)
	}
	return p, nil
}

// Render is the plan as the operator reads it before typing the confirm code: the verb, the
// cells, every action with its precondition evidence, the expiry and the code. children are the
// batch's child plans, in order (nil for a plan that is not a batch).
func Render(p Plan, children []Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "plan %s: %s on %s\n", p.ID, p.Verb, strings.Join(p.Cells, ", "))
	writeActions := func(indent string, actions []Action) {
		for _, a := range actions {
			fmt.Fprintf(&b, "%s- [%s] %s: %s\n", indent, a.Cell, a.ID, a.Summary)
			for _, pc := range a.Preconditions {
				fmt.Fprintf(&b, "%s    requires %s = %q\n", indent, pc.Key, pc.Value)
			}
		}
	}
	writeActions("  ", p.Actions)
	for i, c := range children {
		fmt.Fprintf(&b, "  step %d: plan %s: %s on %s\n", i+1, c.ID, c.Verb, strings.Join(c.Cells, ", "))
		writeActions("    ", c.Actions)
	}
	fmt.Fprintf(&b, "expires %s\n", p.Expires)
	fmt.Fprintf(&b, "confirm code: %s\n", p.ConfirmCode)
	return b.String()
}
