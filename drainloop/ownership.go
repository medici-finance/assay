package drainloop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
)

// CellCheckpointSchema identifies the passive Cell checkpoint record contract.
const CellCheckpointSchema = "cell-checkpoint/v1"

// CellOwnership is a generation inside one operator incarnation. Generation is
// opaque, not an increasing integer whose value can survive rollback as authority.
type CellOwnership struct {
	CellID        string `json:"cell_id"`
	IncarnationID string `json:"incarnation_id"`
	OwnerID       string `json:"owner_id"`
	Generation    string `json:"generation"`
	Backend       string `json:"backend"`
}

// CellCheckpoint is a passive persistence contract, not a second lease store.
// Outstanding effects, cancellation, stop and overrides must survive ordinary
// restart and restore. Missing history is retained as unknown, never new allowance.
// A consumer must not use Activation to select the current operator incarnation.
// Decode it with DecodeCellCheckpoint: a plain json.Unmarshal reads an absent,
// null, repeated or case-variant hold flag as not held.
type CellCheckpoint struct {
	Schema          string         `json:"schema"`
	Ownership       CellOwnership  `json:"ownership"`
	Activation      CellActivation `json:"activation"`
	Budget          CellBudget     `json:"budget"`
	CancelRequested bool           `json:"cancel_requested"`
	Stopped         bool           `json:"stopped"`
	OverrideRefs    []string       `json:"override_refs"`
	PendingEffects  []string       `json:"pending_effects"`
}

// DecodeCellCheckpoint decodes one checkpoint record. It refuses an absent field
// at any depth, an explicit null where the field is not nullable, a repeated key,
// a key that is not exactly a field name, trailing data and an unsupported
// schema. A bool or a zero count cannot say "absent", and encoding/json reads a
// null bool as false and keeps the last of two keys that match a field without
// regard to case, so each of those shapes is refused here instead of decoding as
// not held or as zero. An explicit null nullable count stays unknown. A decoded
// record grants no authority and selects no current incarnation.
func DecodeCellCheckpoint(body []byte) (CellCheckpoint, error) {
	// Preserve encoding/json's syntax and nesting bound before recursive token reading.
	if !json.Valid(body) {
		return CellCheckpoint{}, errors.New("checkpoint: invalid JSON")
	}
	tree := json.NewDecoder(bytes.NewReader(body))
	tree.UseNumber()
	raw, err := readStrict(tree)
	if err != nil {
		return CellCheckpoint{}, err
	}
	if _, err := tree.Token(); !errors.Is(err, io.EOF) {
		return CellCheckpoint{}, errors.New("checkpoint: trailing data")
	}
	if err := requireFields(raw, reflect.TypeOf(CellCheckpoint{}), "checkpoint"); err != nil {
		return CellCheckpoint{}, err
	}
	// The first pass has refused trailing data, repeated keys and inexact keys,
	// so this decode sees one value whose keys each name exactly one field.
	var state CellCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return CellCheckpoint{}, err
	}
	if state.Schema != CellCheckpointSchema {
		return CellCheckpoint{}, fmt.Errorf("checkpoint: unsupported schema %q", state.Schema)
	}
	return state, nil
}

// readStrict reads one JSON value into a generic tree and refuses a key that
// appears twice in one object. A map decode would keep the last one silently.
func readStrict(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		obj := map[string]any{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyTok.(string)
			if !ok {
				return nil, errors.New("checkpoint: object key is not a string")
			}
			if _, seen := obj[key]; seen {
				return nil, fmt.Errorf("checkpoint: repeated key %q", key)
			}
			if obj[key], err = readStrict(dec); err != nil {
				return nil, err
			}
		}
		_, err := dec.Token()
		return obj, err
	case json.Delim('['):
		items := []any{}
		for dec.More() {
			item, err := readStrict(dec)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		_, err := dec.Token()
		return items, err
	}
	return tok, nil
}

// requireFields reports the first position of v that does not fit t: a null
// where the Go field is not a pointer (a pointer is how this contract spells
// unknown), an object or array position holding something else, an absent
// field, or a key that is not exactly one of t's JSON field names.
func requireFields(v any, t reflect.Type, path string) error {
	if v == nil {
		if t.Kind() == reflect.Pointer {
			return nil
		}
		return fmt.Errorf("%s: null where the field is not nullable", path)
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", path)
		}
		names := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			names[name] = true
			child, ok := obj[name]
			if !ok {
				return fmt.Errorf("%s.%s: field absent", path, name)
			}
			if err := requireFields(child, t.Field(i).Type, path+"."+name); err != nil {
				return err
			}
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !names[key] {
				return fmt.Errorf("%s.%s: not exactly a field name", path, key)
			}
		}
	case reflect.Slice:
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: want an array", path)
		}
		for i, item := range items {
			if err := requireFields(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ErrCellConflict means a transaction lost its expected-version comparison.
var ErrCellConflict = errors.New("cell version conflict")

// CellState belongs to the existing coordination authority, not a second store.
// Reservations, ownership and the effect intent commit in one transaction.
// CellClaim is the existing item's claim record in the same authority. Its
// generation is distinct from Cell ownership generation and reevaluation identity.
// The existing Claimer acquires/transfers it; this library creates no new lease.
type CellClaim struct {
	ItemID, WorkID, AttemptID, Generation string
	Owner                                 CellOwnership
}

type CellState struct {
	Claims      []CellClaim
	Version     uint64
	Checkpoint  CellCheckpoint
	Pending     []CellAdmission
	Completed   []CellAdmission
	Pacing      []CellPacing
	Settlements []CellSettlement
	Resolutions []CellResolutionRecord
	Receipts    []Event
}

// CellStore invokes change on an isolated copy under a cross-process atomic
// expected-version check. Errors publish none of the copy. A successful durable
// commit increments Version exactly once. ReadCell returns an isolated snapshot.
// RequiredDurability refuses unqualified adapters; implementing this interface
// does not prove durability. GE/19 implements and qualifies PostgreSQL.
type CellStore interface {
	RequiredDurability(context.Context) error
	ReadCell(context.Context, string) (CellState, error)
	Transaction(context.Context, string, uint64, func(*CellState) error) error
}

// CellPermit is an immutable capability reference, not authority by itself.
// CapabilityCheck authenticates caller, tuple, scope and validity independently.
type CellPermit struct {
	Ownership                         CellOwnership
	CapabilityRef                     string
	WorkID, AttemptID, RequestID, Key string
	AdmissionDigest                   string
	ClaimGeneration                   string
}

// CellCapabilityCheck owns authentication, expiry, scope and qualified provider
// restrictions. Absent legacy-credential cutover must refuse. The provider calls
// it independently at its atomic effect boundary, not only in the scheduler.
type CellCapabilityCheck interface {
	CheckCellCapability(context.Context, CellPermit) error
}

// CellAdmission couples one stable operation to its reservations and capability.
// An uncertain operation remains pending until authoritative reconciliation.
type CellAdmission struct {
	Permit                    CellPermit
	Intent                    Intent
	Reservations              []BudgetReservation
	Phase                     string
	ControlBoundary           string
	UpperBoundRef             string
	PacingScope, PacingWindow string
}

// CellAuthority is the store-neutral admission seam. Current comes from protected
// operator state independently of the runtime database. No adapter is installed.
type CellAuthority struct {
	Store        CellStore
	Current      IncarnationSource
	Capabilities CellCapabilityCheck
	Pacing       CellPacingAuthority
	Usage        CellUsageAuthority
	Resolution   CellResolutionAuthority
	Now          func() time.Time
}

func (a CellAuthority) ready(ctx context.Context) error {
	if a.Store == nil || a.Current == nil || a.Capabilities == nil || a.Now == nil {
		return errors.New("cell authority unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.Store.RequiredDurability(ctx)
}
func (a CellAuthority) check(ctx context.Context, s CellState, p CellPermit) error {
	current, err := a.Current.CurrentIncarnation(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	if err = MatchCellFence(current, s.Checkpoint.Activation, s.Checkpoint.Ownership, p.Ownership); err != nil {
		return err
	}
	if s.Checkpoint.Schema != CellCheckpointSchema || s.Checkpoint.Stopped || s.Checkpoint.CancelRequested {
		return errors.New("cell stopped or cancelled")
	}
	if p.CapabilityRef == "" || p.WorkID == "" || p.AttemptID == "" || p.RequestID == "" || p.Key == "" || p.ClaimGeneration == "" {
		return errors.New("cell capability incomplete")
	}
	return a.Capabilities.CheckCellCapability(ctx, p)
}

// Commit reserves all balances and records the intent in the same transaction.
// It never returns an allowance on conflict, unavailable state or commit failure.
func (a CellAuthority) Commit(ctx context.Context, r CellAdmission) error {
	if err := a.ready(ctx); err != nil {
		return err
	}
	p := r.Permit
	s, err := a.Store.ReadCell(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	return a.Store.Transaction(ctx, p.Ownership.CellID, s.Version, func(s *CellState) error {
		if p.AdmissionDigest == "" || p.AdmissionDigest != CellAdmissionDigest(r) {
			return errors.New("admission capability binding mismatch")
		}
		if err := a.check(ctx, *s, p); err != nil {
			return err
		}
		e := r.Intent.Event
		if err := checkCellClaim(*s, r); err != nil {
			return err
		}
		if e.Kind != "EFFECT-INTENT" || e.Key != p.Key || e.RunID != p.WorkID || e.AttemptID != p.AttemptID || e.Generation != p.ClaimGeneration || e.ItemID == "" {
			return errors.New("cell intent identity mismatch")
		}
		for _, old := range append(append([]CellAdmission{}, s.Pending...), s.Completed...) {
			if old.Permit.Key == p.Key {
				return errors.New("operation recorded; reconcile")
			}
		}

		for _, old := range s.Resolutions {
			if old.Admission.Permit.Key == p.Key {
				if old.Admission.Permit.WorkID != p.WorkID || old.Admission.Permit.AttemptID == p.AttemptID || old.Admission.Permit.RequestID == p.RequestID {
					return errors.New("resolved operation requires fresh attempt/request lineage")
				}
			}
		}
		switch r.Phase {
		case "preparation", "summary", "execution", "fallback", "verification":
		default:
			return errors.New("request phase unavailable")
		}
		if r.ControlBoundary != "request" && (r.ControlBoundary != "launch" || r.UpperBoundRef == "") {
			return errors.New("runner request bound unavailable")
		}
		if err := reserveBudget(&s.Checkpoint.Budget, r, a.Now()); err != nil {
			return err
		}
		if r.PacingScope != "" {
			if a.Pacing == nil {
				return errors.New("pacing reset authority unavailable")
			}
			epoch, err := a.Pacing.PacingEpoch(ctx, p.Ownership.CellID, r.PacingScope, r.PacingWindow, p.Ownership.IncarnationID)
			if err != nil {
				return err
			}
			if err := cellPacing(*s, r.PacingScope, r.PacingWindow, p.Ownership.IncarnationID, epoch); err != nil {
				return err
			}
		}
		s.Pending = append(s.Pending, r)
		s.Checkpoint.PendingEffects = append(s.Checkpoint.PendingEffects, p.Key)
		return nil
	})
}

// CheckEffect belongs INSIDE a separately enforcing provider's atomic apply/dedupe
// boundary. A check followed by an unrestricted external write is not a fence.
// It re-reads ownership, protected incarnation, authenticated scope and intent.
func (a CellAuthority) CheckEffect(ctx context.Context, p CellPermit) error {
	if err := a.ready(ctx); err != nil {
		return err
	}
	s, err := a.Store.ReadCell(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	if err := a.check(ctx, s, p); err != nil {
		return err
	}
	for _, r := range s.Pending {
		if r.Permit == p {
			if err := checkCellClaim(s, r); err != nil {
				return err
			}
			if CellAdmissionDigest(r) != p.AdmissionDigest {
				return errors.New("stored admission binding changed")
			}
			for _, required := range r.Reservations {
				present := false
				for _, held := range s.Checkpoint.Budget.Reservations {
					if held.ID == required.ID && held.WorkID == required.WorkID && held.AttemptID == required.AttemptID && held.RequestID == required.RequestID && held.BalanceRef == required.BalanceRef && held.Amount == required.Amount {
						present = true
					}
				}
				if !present {
					return errors.New("effect reservation unavailable")
				}
				known := false
				for _, balance := range s.Checkpoint.Budget.Balances {
					if balance.Ref == required.BalanceRef && balance.Consumed != nil {
						if err := balanceWindow(balance, a.Now()); err != nil {
							return err
						}
						known = true
					}
				}
				if !known {
					return errors.New("effect brake usage unavailable")
				}
			}
			return nil
		}
	}
	return errors.New("effect has no admitted intent")
}

// CellJournal joins the existing transactional outbox flow to CellAuthority.
// Request maps an enumerated effect to trusted policy, never a model-authored cap.
// No Loop method or ordinary zero-value Config behavior changes.
type CellJournal struct {
	Authority CellAuthority
	CellID    string
	Request   func(Intent) (CellAdmission, error)
}

func (j CellJournal) Generation(ctx context.Context, item string) (string, error) {
	if err := j.Authority.ready(ctx); err != nil {
		return "", err
	}
	s, err := j.Authority.Store.ReadCell(ctx, j.CellID)
	if err != nil {
		return "", err
	}
	generation := ""
	for _, claim := range s.Claims {
		if claim.ItemID == item && claim.Owner == s.Checkpoint.Ownership {
			if generation != "" || claim.Generation == "" {
				return "", errors.New("claim generation ambiguous/unavailable")
			}
			generation = claim.Generation
		}
	}
	if generation == "" {
		return "", errors.New("current item claim unavailable")
	}
	return generation, nil
}
func (j CellJournal) CommitIntent(ctx context.Context, i Intent) error {
	if j.Request == nil {
		return errors.New("cell request mapping absent")
	}
	r, err := j.Request(i)
	if err != nil {
		return err
	}
	if r.Intent != i || r.Permit.Ownership.CellID != j.CellID {
		return errors.New("cell mapping changed intent")
	}
	return j.Authority.Commit(ctx, r)
}
func (j CellJournal) Pending(ctx context.Context, run string) ([]Intent, error) {
	if err := j.Authority.ready(ctx); err != nil {
		return nil, err
	}
	s, err := j.Authority.Store.ReadCell(ctx, j.CellID)
	if err != nil {
		return nil, err
	}
	out := []Intent{}
	for _, r := range s.Pending {
		if r.Intent.Event.RunID == run {
			out = append(out, r.Intent)
		}
	}
	return out, nil
}
func (j CellJournal) Record(e Event) error {
	if e.Kind == "EFFECT-INTENT" {
		return j.CommitIntent(context.Background(), Intent{Event: e})
	}
	if e.Kind != "EFFECT-DONE" {
		return nil
	} // ordinary attribution is not an admission receipt.
	ctx := context.Background()
	if err := j.Authority.ready(ctx); err != nil {
		return err
	}
	s, err := j.Authority.Store.ReadCell(ctx, j.CellID)
	if err != nil {
		return err
	}
	return j.Authority.Store.Transaction(ctx, j.CellID, s.Version, func(s *CellState) error {
		for n, r := range s.Pending {
			if r.Intent.Event.Key == e.Key && r.Intent.Event.Generation == e.Generation && r.Intent.Event.RunID == e.RunID && r.Intent.Event.ItemID == e.ItemID && e.Ref != "" {
				s.Completed = append(s.Completed, r)
				s.Receipts = append(s.Receipts, e)
				s.Pending = append(s.Pending[:n], s.Pending[n+1:]...)
				for n, key := range s.Checkpoint.PendingEffects {
					if key == e.Key {
						s.Checkpoint.PendingEffects = append(s.Checkpoint.PendingEffects[:n], s.Checkpoint.PendingEffects[n+1:]...)
						break
					}
				}
				// Keep reservations charged until authoritative usage reconciliation.
				return nil
			}
		}
		return errors.New("completion has no matching key/generation")
	})
}

// CellRecovery is a trusted operator port. Confirm validates whole-group stop,
// legacy credentials/sockets invalidation and reconciliation, and rotates identity
// AND signing key outside PostgreSQL. Artifact strings alone grant no authority.
type CellRecovery interface {
	ConfirmCellRecovery(context.Context, CellOwnership) (OperatorIncarnation, CellActivation, CellOwnership, error)
}

// Transfer retains budgets, stop/cancel and overrides. Unknown effects or usage
// prevent handback; pause is durable before rotation, including on later failure.
func (a CellAuthority) Transfer(ctx context.Context, cell string, operator CellRecovery) error {
	if err := a.ready(ctx); err != nil {
		return err
	}
	if operator == nil {
		return errors.New("operator recovery unavailable")
	}
	s, err := a.Store.ReadCell(ctx, cell)
	if err != nil {
		return err
	}
	if err := a.Store.Transaction(ctx, cell, s.Version, func(s *CellState) error {
		if len(s.Pending) != 0 || len(s.Checkpoint.PendingEffects) != 0 {
			return errors.New("unknown effects prevent handback")
		}
		for _, r := range s.Checkpoint.Budget.Reservations {
			if r.Unknown {
				return errors.New("unknown usage prevents handback")
			}
		}
		s.Checkpoint.Activation.Paused = true
		return nil
	}); err != nil {
		return err
	}
	current, active, owned, err := operator.ConfirmCellRecovery(ctx, s.Checkpoint.Ownership)
	if err != nil {
		return err
	}
	if current.ID == s.Checkpoint.Ownership.IncarnationID || current.KeyRef == s.Checkpoint.Activation.KeyRef || owned.Generation == s.Checkpoint.Ownership.Generation {
		return errors.New("recovery did not rotate authority")
	}
	protected, err := a.Current.CurrentIncarnation(ctx, cell)
	if err != nil || protected != current {
		return errors.New("recovery is not protected current state")
	}
	if err := MatchCellFence(current, active, owned, owned); err != nil {
		return err
	}
	return a.Store.Transaction(ctx, cell, s.Version+1, func(s *CellState) error {
		s.Checkpoint.Ownership, s.Checkpoint.Activation = owned, active
		return nil
	})
}

// CellAdmissionDigest binds all reservation, runner and pacing policy coordinates
// to the authenticated capability. The issuer authorizes this digest, not fields
// subsequently supplied by a worker. This hash supplies integrity, not identity.
func CellAdmissionDigest(r CellAdmission) string {
	r.Permit.AdmissionDigest = ""
	body, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

// CellPacingAuthority reads the reset anchor outside the lost runtime store. Epoch
// zero means intact history; one means the single recorded unseeded reset. Missing
// or second-reset state returns error. This port installs no reset/reopen policy.
type CellPacingAuthority interface {
	PacingEpoch(context.Context, string, string, string, string) (uint64, error)
}

// CellSettlement is authoritative usage for one reservation. Nil charge retains
// the entire reservation as unknown; numeric usage measurements stay untouched.
type CellSettlement struct {
	ReservationID     string
	PreviousSourceRef string
	Charged           *uint64
	Usage             AttemptUsage
}

// CellUsageAuthority resolves authenticated, trust-gated usage under the original
// work/policy and separate account/provider/unit balances. No raw provider payload
// or credential is accepted. A syntax-valid SourceRef alone is not qualification.
type CellUsageAuthority interface {
	CellUsage(context.Context, CellPermit) ([]CellSettlement, error)
}

// Settle records every attempt phase (failed/fallback/verification included).
// Missing telemetry remains charged; authoritative zero is distinct from unknown.
func (a CellAuthority) Settle(ctx context.Context, p CellPermit) error {
	if err := a.ready(ctx); err != nil {
		return err
	}
	if a.Usage == nil {
		return errors.New("usage authority unavailable")
	}
	settlements, err := a.Usage.CellUsage(ctx, p)
	if err != nil {
		return err
	}
	s, err := a.Store.ReadCell(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	return a.Store.Transaction(ctx, p.Ownership.CellID, s.Version, func(s *CellState) error { return settleCellState(s, p, settlements) })
}

func settleCellState(s *CellState, p CellPermit, settlements []CellSettlement) error {
	// Reconciliation of old work is allowed while paused, but an authenticated
	// exact recorded operation is required; it grants no new effect.
	found := false
	phase := ""
	admissions := append(append([]CellAdmission{}, s.Pending...), s.Completed...)
	for _, record := range s.Resolutions {
		admissions = append(admissions, record.Admission)
	}
	for _, r := range admissions {
		if r.Permit == p {
			found = true
			phase = r.Phase
		}
	}
	if !found {
		return errors.New("usage has no recorded operation")
	}
	for _, x := range settlements {
		u := x.Usage
		if u.Phase != phase {
			return errors.New("usage phase differs from signed request")
		}
		if u.WorkID != p.WorkID || u.AttemptID != p.AttemptID || u.RequestID != p.RequestID || u.SourceRef == "" || u.Account == "" || u.Provider == "" || u.Model == "" || u.PricingRef == "" {
			return errors.New("usage identity/policy mismatch")
		}
		switch u.Phase {
		case "preparation", "summary", "execution", "fallback", "verification":
		default:
			return errors.New("unknown attempt phase")
		}
		duplicate := false
		for _, old := range s.Settlements {
			if old.ReservationID == x.ReservationID && old.Usage.SourceRef == u.SourceRef && old.Usage.Phase == u.Phase {
				if !reflect.DeepEqual(old, x) {
					return errors.New("settlement changed after recording")
				}
				duplicate = true
			}
		}
		if duplicate {
			continue
		}
		seenUsage := false
		latestSource := ""
		for _, prior := range s.Settlements {
			if prior.ReservationID == x.ReservationID && prior.Usage.WorkID == u.WorkID && prior.Usage.AttemptID == u.AttemptID && prior.Usage.RequestID == u.RequestID && prior.Usage.Phase == u.Phase {
				latestSource = prior.Usage.SourceRef
			}
		}
		for _, old := range s.Checkpoint.Budget.Usage {
			if old.WorkID == u.WorkID && old.AttemptID == u.AttemptID && old.RequestID == u.RequestID && old.Phase == u.Phase {
				if old.SourceRef == u.SourceRef {
					if !reflect.DeepEqual(old, u) {
						return errors.New("usage evidence changed")
					}
					seenUsage = true
				}
			}
		}
		if latestSource != x.PreviousSourceRef {
			return errors.New("usage reconciliation has no exact predecessor")
		}
		index := -1
		for i, q := range s.Checkpoint.Budget.Reservations {
			if q.ID == x.ReservationID && q.WorkID == p.WorkID && q.AttemptID == p.AttemptID && q.RequestID == p.RequestID {
				index = i
				break
			}
		}
		if index < 0 {
			return errors.New("usage reservation unavailable")
		}
		q := s.Checkpoint.Budget.Reservations[index]
		if x.Charged == nil {
			s.Checkpoint.Budget.Reservations[index].Unknown = true
		} else {
			balance := -1
			for i, b := range s.Checkpoint.Budget.Balances {
				if b.Ref == q.BalanceRef {
					if balance >= 0 {
						return errors.New("usage balance ambiguous")
					}
					balance = i
				}
			}
			if balance < 0 {
				return errors.New("usage balance unavailable")
			}
			b := &s.Checkpoint.Budget.Balances[balance]
			if b.Consumed == nil || b.Account != u.Account || b.Provider != u.Provider || *x.Charged > ^uint64(0)-*b.Consumed {
				return errors.New("usage count unavailable/overflow")
			}
			charged := *b.Consumed + *x.Charged
			b.Consumed = &charged
			s.Checkpoint.Budget.Reservations = append(s.Checkpoint.Budget.Reservations[:index], s.Checkpoint.Budget.Reservations[index+1:]...)
		}
		if !seenUsage {
			s.Checkpoint.Budget.Usage = append(s.Checkpoint.Budget.Usage, u)
		}
		s.Settlements = append(s.Settlements, x)
	}
	return nil
}

// checkCellClaim is evaluated at admission AND by the separately invoked provider.
// A live Cell tuple alone cannot revive a released/transferred work-item claim.
func checkCellClaim(s CellState, r CellAdmission) error {
	matches := 0
	for _, claim := range s.Claims {
		if claim.ItemID == r.Intent.Event.ItemID {
			matches++
			if claim.Owner != s.Checkpoint.Ownership || claim.Owner != r.Permit.Ownership || claim.WorkID != r.Permit.WorkID || claim.AttemptID != r.Permit.AttemptID || claim.Generation == "" || claim.Generation != r.Permit.ClaimGeneration || claim.Generation != r.Intent.Event.Generation {
				return errors.New("item claim owner/generation mismatched")
			}
		}
	}
	if matches != 1 {
		return errors.New("item claim unavailable/ambiguous")
	}
	return nil
}

// CellResolution is returned only by a qualified authoritative resolution port.
// NotApplied includes definitive absence/proven failure plus old-writer fencing;
// references alone are not evidence. Every reservation needs known usage.
type CellResolution struct {
	Ref, FenceRef string
	NotApplied    bool
	Settlements   []CellSettlement
}
type CellResolutionRecord struct {
	Admission     CellAdmission
	Ref, FenceRef string
}

// CellResolutionAuthority authenticates current resolution scope, independently
// fences the old writer/capability, then proves absence and authoritative usage.
// A receipt lookup racing an unfenced writer is not a conforming implementation.
type CellResolutionAuthority interface {
	ResolveCellOperation(context.Context, CellPermit) (CellResolution, error)
}

// ResolvePending retires a definitively absent operation only after independent
// old-writer fencing and complete known usage. Settlement and retirement commit
// together. Unknown usage stays charged/held; no restart assumes a free attempt.
// The existing same-generation claim may retry with a NEW attempt and request.
func (a CellAuthority) ResolvePending(ctx context.Context, p CellPermit) error {
	if err := a.ready(ctx); err != nil {
		return err
	}
	if a.Resolution == nil {
		return errors.New("operation resolution unavailable")
	}
	current, err := a.Current.CurrentIncarnation(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	if current.Schema != IncarnationSchema || current.CellID != p.Ownership.CellID || current.ID == "" || current.KeyRef == "" {
		return errors.New("resolution current authority unavailable")
	}
	result, err := a.Resolution.ResolveCellOperation(ctx, p)
	if err != nil {
		return err
	}
	if !result.NotApplied || result.Ref == "" || result.FenceRef == "" {
		return errors.New("absence/fencing unconfirmed")
	}
	s, err := a.Store.ReadCell(ctx, p.Ownership.CellID)
	if err != nil {
		return err
	}
	return a.Store.Transaction(ctx, p.Ownership.CellID, s.Version, func(s *CellState) error {
		index := -1
		for n, r := range s.Pending {
			if r.Permit == p {
				if index >= 0 {
					return errors.New("resolution admission ambiguous")
				}
				index = n
			}
		}
		if index < 0 {
			return errors.New("resolution admission unavailable")
		}
		old := s.Pending[index]
		if err := settleCellState(s, p, result.Settlements); err != nil {
			return err
		}
		for _, q := range old.Reservations {
			for _, held := range s.Checkpoint.Budget.Reservations {
				if held.ID == q.ID {
					return errors.New("unknown usage prevents retirement")
				}
			}
			known := false
			for _, x := range s.Settlements {
				if x.ReservationID == q.ID && x.Charged != nil && x.Usage.WorkID == p.WorkID && x.Usage.AttemptID == p.AttemptID && x.Usage.RequestID == p.RequestID {
					known = true
				}
			}
			if !known {
				return errors.New("complete usage proof unavailable")
			}
		}
		s.Resolutions = append(s.Resolutions, CellResolutionRecord{Admission: old, Ref: result.Ref, FenceRef: result.FenceRef})
		s.Pending = append(s.Pending[:index], s.Pending[index+1:]...)
		for n, key := range s.Checkpoint.PendingEffects {
			if key == p.Key {
				s.Checkpoint.PendingEffects = append(s.Checkpoint.PendingEffects[:n], s.Checkpoint.PendingEffects[n+1:]...)
				break
			}
		}
		return nil
	})
}
