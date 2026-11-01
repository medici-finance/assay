package drainloop

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

type fixtureSource struct {
	current OperatorIncarnation
	err     error
}

func (s fixtureSource) CurrentIncarnation(context.Context, string) (OperatorIncarnation, error) {
	return s.current, s.err
}

// fixtureBoundary is deliberately called directly, without Run or a scheduler.
// It checks the production record predicate before recording a fake effect.
// It does not qualify signatures, real custody, store transactions or providers.
type fixtureBoundary struct {
	source  IncarnationSource
	active  CellActivation
	owned   CellOwnership
	effects int
}

func (p *fixtureBoundary) apply(cap CellOwnership) error {
	if p.source == nil {
		return errors.New("current operator source missing")
	}
	current, err := p.source.CurrentIncarnation(context.Background(), cap.CellID)
	if err != nil {
		return err
	}
	if err := MatchCellFence(current, p.active, p.owned, cap); err != nil {
		return err
	}
	p.effects++
	return nil
}

func cellFenceFixture() fixtureBoundary {
	return fixtureBoundary{
		source: fixtureSource{current: OperatorIncarnation{
			Schema: IncarnationSchema, CellID: "example-cell",
			ID: "incarnation-new", KeyRef: "ref:example-key-new",
		}},
		active: CellActivation{IncarnationID: "incarnation-new",
			KeyRef: "ref:example-key-new", StopReceipt: "ref:group-stopped"},
		owned: CellOwnership{CellID: "example-cell", IncarnationID: "incarnation-new",
			OwnerID: "example-owner", Generation: "generation-7", Backend: "example-backend"},
	}
}

// setCurrent edits the protected current record the fixture source returns.
func setCurrent(p *fixtureBoundary, edit func(*OperatorIncarnation)) {
	current := p.source.(fixtureSource).current
	edit(&current)
	p.source = fixtureSource{current: current}
}

// regression: F-ge16-schema
func TestCellFenceBoundary(t *testing.T) {
	cases := []struct {
		name   string
		change func(*fixtureBoundary, *CellOwnership)
	}{
		{"stale-incarnation", func(p *fixtureBoundary, cap *CellOwnership) {
			// A rollback restores both old ownership and the matching capability.
			// Only protected operator state exposes that they belong to the past.
			p.owned.IncarnationID = "incarnation-old"
			cap.IncarnationID = "incarnation-old"
		}},
		{"stale-generation", func(_ *fixtureBoundary, cap *CellOwnership) { cap.Generation = "generation-6" }},
		{"wrong-owner", func(_ *fixtureBoundary, cap *CellOwnership) { cap.OwnerID = "other-owner" }},
		{"wrong-backend", func(_ *fixtureBoundary, cap *CellOwnership) { cap.Backend = "other-backend" }},
		{"wrong-cell", func(_ *fixtureBoundary, cap *CellOwnership) { cap.CellID = "other-cell" }},
		{"missing-source", func(p *fixtureBoundary, _ *CellOwnership) { p.source = nil }},
		{"unreadable-source", func(p *fixtureBoundary, _ *CellOwnership) {
			p.source = fixtureSource{err: errors.New("protected source unreadable")}
		}},
		{"missing-incarnation", func(p *fixtureBoundary, _ *CellOwnership) { p.source = fixtureSource{} }},
		{"mismatched-activation", func(p *fixtureBoundary, _ *CellOwnership) {
			p.active.IncarnationID = "incarnation-old"
		}},
		{"old-key", func(p *fixtureBoundary, _ *CellOwnership) { p.active.KeyRef = "ref:old-key" }},
		{"missing-activation", func(p *fixtureBoundary, _ *CellOwnership) { p.active = CellActivation{} }},
		{"paused", func(p *fixtureBoundary, _ *CellOwnership) { p.active.Paused = true }},
		{"unconfirmed-stop", func(p *fixtureBoundary, _ *CellOwnership) { p.active.StopReceipt = "" }},
		// Each case below breaks exactly one guard and keeps every other record
		// consistent with it, so no neighbouring guard refuses on its behalf.
		{"unsupported-schema", func(p *fixtureBoundary, _ *CellOwnership) {
			setCurrent(p, func(c *OperatorIncarnation) { c.Schema = "operator-incarnation/v0" })
		}},
		{"empty-current-cell", func(p *fixtureBoundary, cap *CellOwnership) {
			setCurrent(p, func(c *OperatorIncarnation) { c.CellID = "" })
			p.owned.CellID, cap.CellID = "", ""
		}},
		{"empty-current-id", func(p *fixtureBoundary, cap *CellOwnership) {
			setCurrent(p, func(c *OperatorIncarnation) { c.ID = "" })
			p.active.IncarnationID, p.owned.IncarnationID, cap.IncarnationID = "", "", ""
		}},
		{"empty-current-key", func(p *fixtureBoundary, _ *CellOwnership) {
			setCurrent(p, func(c *OperatorIncarnation) { c.KeyRef = "" })
			p.active.KeyRef = ""
		}},
		{"owned-cell-not-current", func(p *fixtureBoundary, cap *CellOwnership) {
			p.owned.CellID, cap.CellID = "other-cell", "other-cell"
		}},
		{"empty-owner", func(p *fixtureBoundary, cap *CellOwnership) {
			p.owned.OwnerID, cap.OwnerID = "", ""
		}},
		{"empty-generation", func(p *fixtureBoundary, cap *CellOwnership) {
			p.owned.Generation, cap.Generation = "", ""
		}},
		{"empty-backend", func(p *fixtureBoundary, cap *CellOwnership) {
			p.owned.Backend, cap.Backend = "", ""
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := cellFenceFixture()
			cap := p.owned
			tc.change(&p, &cap)
			if err := p.apply(cap); err == nil || p.effects != 0 {
				t.Fatalf("held fixture admitted effect: error=%v effects=%d", err, p.effects)
			}
		})
	}
	p := cellFenceFixture()
	if err := p.apply(p.owned); err != nil || p.effects != 1 {
		t.Fatalf("current positive control: error=%v effects=%d", err, p.effects)
	}
}

// regression: F-ge16-schema
func TestIncarnationContractFields(t *testing.T) {
	body, err := os.ReadFile("../spec/operator-incarnation-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	// Each inventory line reads "- `GoName` — `wire_name`; ...".
	var names []string
	inFields := false
	for _, line := range strings.Split(string(body), "\n") {
		if line == "<!-- incarnation-fields:begin -->" {
			inFields = true
			continue
		}
		if line == "<!-- incarnation-fields:end -->" {
			inFields = false
			break
		}
		if inFields && strings.HasPrefix(line, "- `") {
			parts := strings.Split(line, "`")
			if len(parts) < 4 {
				t.Fatalf("inventory line names no wire name: %q", line)
			}
			names = append(names, parts[1]+"="+parts[3])
			if parts[1] == "Schema" && !strings.Contains(line, "`"+IncarnationSchema+"`") {
				t.Fatalf("inventory Schema line does not state %q: %q", IncarnationSchema, line)
			}
		}
	}
	typ := reflect.TypeOf(OperatorIncarnation{})
	var exported []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			wire, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			exported = append(exported, field.Name+"="+wire)
		}
	}
	if !reflect.DeepEqual(names, exported) {
		t.Fatalf("contract fields %v differ from exported type fields %v", names, exported)
	}
}
