package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2971 — a cell-scoped stop flag shows on the board banner, and one that
// halts nothing (here: a cell name outside [a-z0-9-]) is tagged INERT rather than listed as if
// it were armed.
func TestBannerShowsCellStopFlags(t *testing.T) {
	h := Header{StopFlags: []deskkit.ActiveStopFlag{
		{Name: "STOP.cell.a", Reason: "drain a"},
		{Name: "STOP.cell.Bad_Name", Reason: "present (no reason) — INERT: \"Bad_Name\" fails the cell-name rule [a-z0-9-], so no session can carry it and this flag halts nothing", Inert: true},
	}}
	var b bytes.Buffer
	printBanners(&b, h)
	out := b.String()
	t.Log(out)
	if !strings.Contains(out, "WARNING: STOP flag active — STOP.cell.a: drain a") {
		t.Fatalf("live cell flag not bannered as STOP:\n%s", out)
	}
	if !strings.Contains(out, "WARNING: INERT STOP flag active — STOP.cell.Bad_Name:") {
		t.Fatalf("invalid-cell-name flag not tagged INERT:\n%s", out)
	}
}
