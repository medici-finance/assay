package workspace_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/workspace"
	"github.com/medici-finance/assay/loopadmin/workspace/fake"
)

// requiredLists returns the json paths of every list a request must carry: a
// slice field without omitempty, at any depth of non-pointer struct fields.
// A nil one encodes as null, which Decode refuses.
func requiredLists(t reflect.Type, at string) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if strings.Contains(opts, "omitempty") {
			continue
		}
		p := strings.TrimPrefix(at+"."+tag, ".")
		switch f.Type.Kind() {
		case reflect.Slice:
			out = append(out, p)
		case reflect.Struct:
			out = append(out, requiredLists(f.Type, p)...)
		}
	}
	return out
}

// setNil sets the list at a dotted json path of v to nil.
func setNil(v reflect.Value, p string) {
	for _, name := range strings.Split(p, ".") {
		for i := 0; i < v.NumField(); i++ {
			tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			if tag == name {
				v = v.Field(i)
				break
			}
		}
	}
	v.Set(reflect.Zero(v.Type()))
}

// TestReceiptNilListsRefused: every list a request must carry is refused
// when nil, by the request's own validation, so the port never stores or
// returns a ready receipt that Decode refuses; an empty writable list is a
// valid request whose ready receipt round-trips and is admitted; a request
// over the decoder's size cap is refused by Validate, one whose receipt would
// exceed the cap is never stored, neither is returned ready, and Admit
// refuses a receipt over the cap; and a backslash path is not a clean
// relative path.
// regression: F-receipt-roundtrip-nil-writable
func TestReceiptNilListsRefused(t *testing.T) {
	lists := requiredLists(reflect.TypeOf(workspace.Request{}), "")
	// Positive control: the walk finds the lists the contract defines.
	for _, want := range []string{"repositories", "environment.tools", "environment.skills", "writable"} {
		if !strings.Contains(","+strings.Join(lists, ",")+",", ","+want+",") {
			t.Fatalf("the required-list walk missed %s (found %v)", want, lists)
		}
	}
	for _, p := range lists {
		req := baseRequest(t)
		setNil(reflect.ValueOf(&req).Elem(), p)
		if err := req.Validate(); !errors.Is(err, workspace.ErrInvalid) || !strings.Contains(err.Error(), "request."+p) {
			t.Fatalf("nil %s: got %v, want ErrInvalid naming request.%s", p, err, p)
		}
		env := fake.New()
		r, err := env.Port(workspace.Port{}).Prepare(ctx, req)
		s, _ := env.Receipt(workspace.BindingKey(req.Instance))
		if err == nil || r.Outcome == workspace.OutcomeReady || s.Outcome == workspace.OutcomeReady {
			t.Fatalf("nil %s: Prepare returned %q (stored %q) err=%v, want a refusal", p, r.Outcome, s.Outcome, err)
		}
	}

	// No writable paths at all is an empty list, not a missing one.
	req := baseRequest(t)
	req.Writable = []string{}
	r, err := fake.New().Port(workspace.Port{}).Prepare(ctx, req)
	if err != nil || r.Outcome != workspace.OutcomeReady {
		t.Fatalf("empty writable: got %v %v, want ready", r.Outcome, err)
	}
	b, _ := json.Marshal(r)
	back, err := workspace.Decode(b)
	if err != nil {
		t.Fatalf("empty writable: the ready receipt does not decode: %v", err)
	}
	if err := back.Admit(req.Invocation, req.Owner, req.Instance, r.Allocation.Workspace); err != nil {
		t.Fatalf("empty writable: the decoded receipt is not admitted: %v", err)
	}

	// Over the decoder's size cap: valid in shape, refused by the request's
	// own validation, and never stored or returned as anything ready.
	big := baseRequest(t)
	for i := 0; len(big.Writable)*16 < runner.MaxDocumentBytes; i++ {
		big.Writable = append(big.Writable, "build/output/cache/item-"+strconv.Itoa(i))
	}
	if err := big.Validate(); !errors.Is(err, workspace.ErrInvalid) || !strings.Contains(err.Error(), "size cap") {
		t.Fatalf("oversized request: Validate got %v, want ErrInvalid naming the size cap", err)
	}
	env := fake.New()
	r, err = env.Port(workspace.Port{}).Prepare(ctx, big)
	s, stored := env.Receipt(workspace.BindingKey(big.Instance))
	if !errors.Is(err, workspace.ErrInvalid) || r.Outcome == workspace.OutcomeReady || stored {
		t.Fatalf("oversized receipt: got %q (stored %v %q) %v, want ErrInvalid and nothing stored or ready", r.Outcome, stored, s.Outcome, err)
	}

	// Just under the cap as a request, over it as a receipt: the store choke
	// point refuses it, so nothing is stored and nothing ready is returned.
	near := nearCap(t)
	env = fake.New()
	r, err = env.Port(workspace.Port{}).Prepare(ctx, near)
	s, stored = env.Receipt(workspace.BindingKey(near.Instance))
	if !errors.Is(err, workspace.ErrInvalid) || r.Outcome == workspace.OutcomeReady || stored {
		t.Fatalf("near-cap request: got %q (stored %v %q) %v, want ErrInvalid and nothing stored or ready", r.Outcome, stored, s.Outcome, err)
	}

	// Admit refuses a ready-looking receipt Decode would refuse, whoever
	// built the value.
	ok, err := fake.New().Port(workspace.Port{}).Prepare(ctx, baseRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	ok.Inputs = near
	if err := ok.Admit(near.Invocation, near.Owner, near.Instance, ok.Allocation.Workspace); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("over-cap ready receipt: Admit got %v, want ErrInvalid", err)
	}

	bs := baseRequest(t)
	bs.Writable = []string{`..\x`}
	if err := bs.Validate(); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("backslash writable path: got %v, want ErrInvalid", err)
	}
}

// nearCap returns a valid request whose encoding is within 60 bytes under
// the decoder's size cap, so any receipt that records it is over the cap.
func nearCap(t *testing.T) workspace.Request {
	t.Helper()
	req := baseRequest(t)
	req.Writable = []string{}
	b, _ := json.Marshal(req)
	// Each "build/item-NNNNNN" adds 20 bytes: 17, two quotes and a comma.
	for i := 0; len(b)+20*(i+1) <= runner.MaxDocumentBytes-20; i++ {
		req.Writable = append(req.Writable, fmt.Sprintf("build/item-%06d", i))
	}
	b, _ = json.Marshal(req)
	if n := len(b); n > runner.MaxDocumentBytes-20 || n <= runner.MaxDocumentBytes-60 {
		t.Fatalf("nearCap: built %d bytes, want within 60 under %d", n, runner.MaxDocumentBytes)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("nearCap: the request is not valid: %v", err)
	}
	return req
}

// TestObserveStoreFault: when an observation cannot be made and the store
// cannot record the unknown outcome either, the caller gets an adapter
// failure, never only a hold that hides a store still reading ready; a
// receipt returned beside an error never reads ready; and when Revisions
// fails, the store's own failure is not dropped.
// regression: F-observe-put-error
func TestObserveStoreFault(t *testing.T) {
	req := baseRequest(t)
	env := fake.New()
	p := env.Port(workspace.Port{})
	r, err := p.Prepare(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	env.Forget(r.Allocation.Workspace)
	env.FailPut = true
	_, err = p.Prepare(ctx, req)
	if !errors.Is(err, workspace.ErrAdapter) || errors.Is(err, workspace.ErrHold) {
		t.Fatalf("unobservable workspace and failing store: got %v, want ErrAdapter", err)
	}
	if s, _ := env.Receipt(workspace.BindingKey(req.Instance)); s.Outcome != workspace.OutcomeReady {
		t.Fatalf("control: the failing store changed the stored receipt to %q", s.Outcome)
	}

	// An observed ready receipt the store cannot record is returned with
	// ErrAdapter, and never reads ready beside that error.
	env = fake.New()
	p = env.Port(workspace.Port{})
	if _, err := p.Prepare(ctx, req); err != nil {
		t.Fatal(err)
	}
	env.FailPut = true
	r, err = p.Prepare(ctx, req)
	if !errors.Is(err, workspace.ErrAdapter) || r.Outcome == workspace.OutcomeReady {
		t.Fatalf("ready observation and failing store: got %q %v, want ErrAdapter and not ready", r.Outcome, err)
	}

	// Revisions failing: the unknown outcome is recorded; when the store
	// cannot record it either, the store's failure is what the caller gets.
	for _, failPut := range []bool{false, true} {
		env = fake.New()
		p = env.Port(workspace.Port{})
		if _, err := p.Prepare(ctx, req); err != nil {
			t.Fatal(err)
		}
		env.FailRevisions, env.FailPut = true, failPut
		_, err = p.Prepare(ctx, req)
		s, _ := env.Receipt(workspace.BindingKey(req.Instance))
		switch {
		case !errors.Is(err, workspace.ErrAdapter):
			t.Fatalf("revisions fault (failing store %v): got %v, want ErrAdapter", failPut, err)
		case !failPut && (!errors.Is(err, fake.ErrRevisions) || s.Outcome != workspace.OutcomeUnknown):
			t.Fatalf("revisions fault: got %v (stored %q), want the Revisions failure and unknown stored", err, s.Outcome)
		case failPut && !errors.Is(err, fake.ErrFault):
			t.Fatalf("revisions fault and failing store: got %v, want the store's failure", err)
		}
	}
}

// TestRestoreGuardNoReceipt: restoring files into the binding's workspace
// holds while it has unpublished files, whatever the store records of it:
// no receipt at all, or an intent receipt with no allocation.
// regression: F-restore-over-unpublished
func TestRestoreGuardNoReceipt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		intent bool
		pres   workspace.Presence
	}{
		{"no receipt", false, workspace.PresencePresent},
		{"intent receipt", true, workspace.PresencePresent},
		{"no receipt, unknown files", false, workspace.PresenceUnknown},
		{"control: clean workspace", false, workspace.PresenceNone},
	} {
		env := fake.New()
		p := env.Port(workspace.Port{Continuity: runner.Preservation{
			Files: runner.SupportYes, Conversation: runner.SupportNo, ProcessMemory: runner.SupportNo,
			Descendants: runner.SupportNo, PendingInteractions: runner.SupportNo,
		}})
		a := baseRequest(t)
		if _, err := p.Prepare(ctx, a); err != nil {
			t.Fatal(err)
		}
		b := a
		b.Instance.ID = "w2"
		b.Restore = &workspace.Restore{Source: a.Instance, Needs: runner.PreservationNeed{Files: true}}
		key := workspace.BindingKey(b.Instance)
		prep, _ := env.Preparation(ctx, b.Instance.Cell, b.Environment)
		ws, _ := env.Workspace(ctx, key, prep)
		env.SetUnpublished(ws, tc.pres)
		if tc.intent {
			env.SetReceipt(key, workspace.Receipt{Version: workspace.Version, Inputs: b,
				Outcome: workspace.OutcomeUnknown, Validation: workspace.Validation{Validator: "workspace.Match", Observer: "fake"}})
		}
		_, err := p.Prepare(ctx, b)
		if tc.pres == workspace.PresenceNone {
			if err != nil || env.Restores != 1 {
				t.Fatalf("%s: got %v restores=%d, want a restoration", tc.name, err, env.Restores)
			}
			continue
		}
		if holdReason(t, err) != workspace.HoldUnpublished || env.Restores != 0 {
			t.Fatalf("%s: restores=%d, want a hold and no copy", tc.name, env.Restores)
		}
	}
}

// TestRestoreRebind: re-preparing one binding with a different restoration
// source restores again and records the new source, never the old one beside
// the new request; restoring over unpublished files holds.
// regression: F-restore-rebind-stale
func TestRestoreRebind(t *testing.T) {
	env := fake.New()
	p := env.Port(workspace.Port{Continuity: runner.Preservation{
		Files: runner.SupportYes, Conversation: runner.SupportNo, ProcessMemory: runner.SupportNo,
		Descendants: runner.SupportNo, PendingInteractions: runner.SupportNo,
	}})
	a := baseRequest(t)
	c := a
	c.Instance.ID = "w3"
	for _, r := range []workspace.Request{a, c} {
		if _, err := p.Prepare(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	b := a
	b.Instance.ID = "w2"
	b.Restore = &workspace.Restore{Source: a.Instance, Needs: runner.PreservationNeed{Files: true}}
	r, err := p.Prepare(ctx, b)
	if err != nil || r.Restored == nil || r.Restored.Source != a.Instance || env.Restores != 1 {
		t.Fatalf("first restoration: got %v %+v restores=%d", err, r.Restored, env.Restores)
	}

	b.Restore = &workspace.Restore{Source: c.Instance, Needs: runner.PreservationNeed{Files: true}}
	r, err = p.Prepare(ctx, b)
	if err != nil || r.Restored == nil || r.Restored.Source != c.Instance || env.Restores != 2 {
		t.Fatalf("changed source: got %v %+v restores=%d, want restored from %s", err, r.Restored, env.Restores, c.Instance.ID)
	}

	env.SetUnpublished(r.Allocation.Workspace, workspace.PresencePresent)
	b.Restore = &workspace.Restore{Source: a.Instance, Needs: runner.PreservationNeed{Files: true}}
	if _, err := p.Prepare(ctx, b); holdReason(t, err) != workspace.HoldUnpublished || env.Restores != 2 {
		t.Fatalf("restore over unpublished files: restores=%d, want a hold and no copy", env.Restores)
	}
}
