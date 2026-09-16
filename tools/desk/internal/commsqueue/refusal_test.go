package commsqueue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// refusal_test.go — the shared refusal line's own contract (#1165): it
// round-trips through journal.log next to the other kinds, it refuses to
// write a line the sweep would have to read as corrupt, and it can never
// carry a payload.

func TestAppendRefusalRoundTrip(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 9, 15, 21, 0, 0, 0, time.UTC)
	rec := RefusalRecord{
		Time: at, Kind: JournalKindRefused, ID: "m-1", Cell: "cell-a", Refusal: RefusalLaneDenied,
		From: comms.SenderID{Cell: "cell-a", Role: "the-desk"}, To: comms.Lane{Cell: "cell-a", Role: "janitor"},
		Verb: "handoff", RawDigest: "abc",
	}
	// A landed line first, so the refusal line is proven to APPEND to the
	// same journal rather than replace it.
	if err := AppendJournal(root, "2026-09-15T20:59:00Z landed id=m-0 from=cell-a/the-desk to=cell-a/worker-desk verb=handoff (x)"); err != nil {
		t.Fatal(err)
	}
	if err := AppendRefusal(root, rec); err != nil {
		t.Fatalf("AppendRefusal: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "journal.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "2026-09-15T20:59:00Z landed") {
		t.Fatalf("refusal must append after the existing line, got %q", lines)
	}
	var back RefusalRecord
	if err := json.Unmarshal([]byte(lines[1]), &back); err != nil {
		t.Fatalf("refusal line does not decode: %v\n%s", err, lines[1])
	}
	if !reflect.DeepEqual(back, rec) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", back, rec)
	}
	if back.SenderKey() != "cell-a/the-desk" || back.LaneKey() != "cell-a/janitor" {
		t.Fatalf("keys: sender=%q lane=%q", back.SenderKey(), back.LaneKey())
	}
}

func TestAppendRefusalRefusesUnjournalableRecords(t *testing.T) {
	root := t.TempDir()
	good := RefusalRecord{Time: time.Now(), Kind: JournalKindRefused, ID: "x", Refusal: RefusalReplay}
	cases := map[string]RefusalRecord{
		"wrong kind":   {Time: good.Time, Kind: "landed", ID: "x", Refusal: RefusalReplay},
		"unknown kind": {Time: good.Time, Kind: JournalKindRefused, ID: "x", Refusal: "made-up"},
		"empty id":     {Time: good.Time, Kind: JournalKindRefused, ID: "", Refusal: RefusalReplay},
	}
	for name, rec := range cases {
		if err := AppendRefusal(root, rec); err == nil {
			t.Errorf("%s: AppendRefusal must refuse, wrote a line the sweep would read as corrupt", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "journal.log")); !os.IsNotExist(err) {
		t.Fatalf("no refused record may have touched the journal, stat err=%v", err)
	}
	if err := AppendRefusal(root, good); err != nil {
		t.Fatalf("the well-formed control must write: %v", err)
	}
}

func TestRefusalRecordUnattributedKeys(t *testing.T) {
	var rec RefusalRecord
	if rec.SenderKey() != "(unattributed)" || rec.LaneKey() != "(unattributed)" {
		t.Fatalf("empty from/to must key as unattributed, got sender=%q lane=%q", rec.SenderKey(), rec.LaneKey())
	}
}

// TestRefusalRecordNeverCarriesPayload pins the ruling by construction: the
// struct has no payload / assertion / detail field, and a marshalled record
// contains none of those keys — a future field addition here is a ruling
// change and must change this test on purpose.
func TestRefusalRecordNeverCarriesPayload(t *testing.T) {
	typ := reflect.TypeOf(RefusalRecord{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, banned := range []string{"payload", "assertion", "sig", "detail", "body", "raw"} {
			if name == banned || (banned == "raw" && name == "raw") {
				t.Fatalf("RefusalRecord must not carry a %q field (found %s)", banned, typ.Field(i).Name)
			}
		}
	}
	rec := RefusalRecord{Time: time.Now(), Kind: JournalKindRefused, ID: "x", Refusal: RefusalOther, RawDigest: "d"}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{`"payload"`, `"assertion"`, `"sig"`, `"detail"`} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("marshalled refusal line carries %s: %s", banned, raw)
		}
	}
}
