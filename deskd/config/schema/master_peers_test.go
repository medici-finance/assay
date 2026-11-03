package schema

import "testing"

// the service contract: `master:` (a config KEY, never a heuristic) and `peers:`
// on Cell. See cells.go Cell.MasterRoleOrDefault / validateMasterAndPeers.

const masterPeersBaseDoc = `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-reviewer-app
      - role: dispatch
        schedule: "*/5 * * * *"
        models: [deepseek-chat]
        app: assay-desk-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
`

func TestMaster_AbsentDefaultsToDispatch(t *testing.T) {
	f, err := Parse([]byte(masterPeersBaseDoc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := f.Cells[0].MasterRoleOrDefault(); got != RoleDispatch {
		t.Fatalf("MasterRoleOrDefault() = %q, want %q (backward-compat default for the two dogfood cells)", got, RoleDispatch)
	}
}

func TestMaster_ExplicitRoleThatIsConfigured(t *testing.T) {
	doc := masterPeersBaseDoc + "    master: review\n"
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := f.Cells[0].MasterRoleOrDefault(); got != RoleReview {
		t.Fatalf("MasterRoleOrDefault() = %q, want review (an explicit master: names the master, never a heuristic)", got)
	}
}

func TestMaster_RejectsRoleNotConfigured(t *testing.T) {
	doc := masterPeersBaseDoc + "    master: ops\n"
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("expected an error: master: ops names no loop role this cell configures, and must be refused, never silently defaulted")
	}
}

func TestPeers_ValidHTTPSAndLoopback(t *testing.T) {
	doc := masterPeersBaseDoc + `    peers:
      - {cell: tracker, url: "https://tracker.example.com"}
      - {cell: other, url: "http://127.0.0.1:8797"}
`
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Cells[0].Peers) != 2 {
		t.Fatalf("Peers = %+v, want 2 entries", f.Cells[0].Peers)
	}
}

func TestPeers_RejectsPlaintextNonLoopbackURL(t *testing.T) {
	doc := masterPeersBaseDoc + `    peers:
      - {cell: tracker, url: "http://tracker.example.com"}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error: a peer url must be https, or http only against a loopback host — never a plaintext network address")
	}
}

func TestPeers_RejectsSelfPeer(t *testing.T) {
	doc := masterPeersBaseDoc + `    peers:
      - {cell: assay, url: "https://self.example.com"}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error: a cell cannot peer with itself")
	}
}

func TestPeers_RejectsDuplicatePeerCell(t *testing.T) {
	doc := masterPeersBaseDoc + `    peers:
      - {cell: tracker, url: "https://a.example.com"}
      - {cell: tracker, url: "https://b.example.com"}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error: duplicate peer cell name")
	}
}

func TestPeers_RejectsEmptyCellName(t *testing.T) {
	doc := masterPeersBaseDoc + `    peers:
      - {cell: "", url: "https://a.example.com"}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error: peer cell name is required")
	}
}

func TestMasterAndPeers_AbsentIsValid(t *testing.T) {
	if _, err := Parse([]byte(masterPeersBaseDoc)); err != nil {
		t.Fatalf("absence of master/peers must be valid (both optional): %v", err)
	}
}
