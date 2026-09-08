package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// acks_preserve_test.go — deskroster shares the roster beacon file with the `deskack` verb,
// which appends receipt records under the `acks` key. deskroster
// must round-trip that data through its OWN rewrites and must not delete a beacon that
// still holds receipts.

// TestSetPreservesAcks — registering PR work (which rewrites the beacon) must not drop the
// acks another writer already put on the file.
func TestSetPreservesAcks(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-preserve")
	t.Setenv("DESK_LOOP", "worker-desk")

	// A receipt landed first (as deskack would write it).
	if _, err := deskkit.AppendAck("s-preserve", deskkit.AckRecord{Role: "worker-desk", Restatement: "a receipt"}); err != nil {
		t.Fatalf("AppendAck: %v", err)
	}
	// deskroster then registers PR work on the SAME beacon.
	if err := cmdSet([]string{"--repo", "tracker", "--pr", "7", "--what", "brief y", "--session", "s-preserve"}); err != nil {
		t.Fatalf("cmdSet: %v", err)
	}

	path, _ := deskkit.AckBeaconPath("s-preserve")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon: %v", err)
	}
	var obj struct {
		OpenWork []WorkEntry         `json:"open_work"`
		Acks     []deskkit.AckRecord `json:"acks"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if len(obj.Acks) != 1 || obj.Acks[0].Restatement != "a receipt" {
		t.Fatalf("deskroster's set dropped the acks: %s", data)
	}
	if len(obj.OpenWork) != 1 {
		t.Fatalf("the PR work entry was not registered: %s", data)
	}
}

// TestDropKeepsAcksOnlyBeacon — dropping the last PR entry must NOT delete a beacon that
// still holds receipts.
func TestDropKeepsAcksOnlyBeacon(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "s-acksonly")

	// Seed a beacon with one PR entry AND an acks array.
	writeTestBeacon(t, home, Beacon{
		Session:  "s-acksonly",
		OpenWork: []WorkEntry{{Repo: "tracker", PR: 3, What: "w"}},
		Acks:     json.RawMessage(`[{"ts":"2026-09-06T00:00:00Z","role":"worker-desk","restatement":"kept"}]`),
	})

	if err := cmdDrop([]string{"--repo", "tracker", "--pr", "3", "--session", "s-acksonly"}); err != nil {
		t.Fatalf("cmdDrop: %v", err)
	}

	path, _ := deskkit.AckBeaconPath("s-acksonly")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("beacon was deleted despite holding receipts: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj struct {
		Acks []deskkit.AckRecord `json:"acks"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if len(obj.Acks) != 1 || obj.Acks[0].Restatement != "kept" {
		t.Fatalf("acks lost when the last PR entry was dropped: %s", data)
	}
}
