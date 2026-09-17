package stopsignal

import "testing"

func TestShouldStopWhenArmedAndWorking(t *testing.T) {
	if !ShouldStop(true, "working") {
		t.Fatal("an armed flag on a working run must signal stop")
	}
}

func TestShouldStopFalseWhenNotArmed(t *testing.T) {
	if ShouldStop(false, "working") {
		t.Fatal("an unarmed flag must never signal stop")
	}
}

func TestShouldStopFalseWhenHandedOff(t *testing.T) {
	if ShouldStop(true, "handed-off") {
		t.Fatal("a handed-off run has nothing left to cooperatively stop")
	}
}

func TestShouldStopFalseWhenBlocked(t *testing.T) {
	if ShouldStop(true, "blocked") {
		t.Fatal("a blocked run is already halted; it is not the state this signal targets")
	}
}
