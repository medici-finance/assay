package trust

import (
	"testing"
	"time"
)

func TestIdentityProfiles(t *testing.T) {
	for _, bot := range []bool{false, true} {
		for _, p := range []Profile{ObservedID, Content, NumericID} {
			if Admit(true, true, 3003, 9999, bot, p) {
				t.Fatalf("wrong supplied ID admitted: bot=%v profile=%v", bot, p)
			}
			if !Admit(true, true, 3003, 3003, bot, p) {
				t.Fatalf("matching ID refused: bot=%v profile=%v", bot, p)
			}
			if Admit(true, true, 0, 3003, bot, p) {
				t.Fatal("unpinned identity admitted with an ID")
			}
		}
	}
	if Admit(true, true, 3003, 0, false, Content) || Admit(true, true, 3003, 0, true, NumericID) {
		t.Fatal("strict profile accepted missing ID")
	}
	if !Admit(true, true, 3003, 0, true, Content) || !Admit(true, true, 3003, 0, false, ObservedID) {
		t.Fatal("explicit legacy profile lost")
	}
	if Admit(false, true, 3003, 3003, true, Content) || Admit(true, false, 3003, 3003, true, Content) {
		t.Fatal("unconfigured or untrusted admitted")
	}
}
func TestBlessingEdits(t *testing.T) {
	at := time.Unix(100, 0)
	event := Event{Authority: true, Trusted: true, Created: at}
	if !Blessed(time.Time{}, []Event{event}) {
		t.Fatal("valid blessing refused")
	}
	if Blessed(at, []Event{event}) || Blessed(time.Time{}, []Event{event, {Created: at}}) {
		t.Fatal("same-time untrusted edit admitted")
	}
	if Blessed(time.Time{}, []Event{event, {Created: at.Add(-time.Second), Edited: at.Add(time.Second)}}) {
		t.Fatal("post-blessing edit admitted")
	}
}
