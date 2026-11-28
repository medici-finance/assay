package workspace_test

import (
	"encoding/json"
	"errors"
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
// whose receipt would exceed the decoder's size cap is never ready; and a
// backslash path is not a clean relative path.
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

	// Over the decoder's size cap: valid in shape, never a ready receipt.
	big := baseRequest(t)
	for i := 0; len(big.Writable)*16 < runner.MaxDocumentBytes; i++ {
		big.Writable = append(big.Writable, "build/output/cache/item-"+strconv.Itoa(i))
	}
	env := fake.New()
	r, err = env.Port(workspace.Port{}).Prepare(ctx, big)
	s, _ := env.Receipt(workspace.BindingKey(big.Instance))
	if !errors.Is(err, workspace.ErrInvalid) || s.Outcome == workspace.OutcomeReady {
		t.Fatalf("oversized receipt: got %q (stored %q) %v, want ErrInvalid and nothing ready", r.Outcome, s.Outcome, err)
	}

	bs := baseRequest(t)
	bs.Writable = []string{`..\x`}
	if err := bs.Validate(); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("backslash writable path: got %v, want ErrInvalid", err)
	}
}

// TestObserveStoreFault: when an observation cannot be made and the store
// cannot record the unknown outcome either, the caller gets an adapter
// failure, never only a hold that hides a store still reading ready.
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
