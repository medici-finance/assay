package caps

import "testing"

func TestAllowedUnderCap(t *testing.T) {
	if !Allowed("fresh", map[string]int{"fresh": 1}, map[string]int{"fresh": 2}) {
		t.Fatal("one in-flight against a cap of two must still admit")
	}
}

func TestBlockedAtCap(t *testing.T) {
	if Allowed("fresh", map[string]int{"fresh": 2}, map[string]int{"fresh": 2}) {
		t.Fatal("a class already at its cap must not admit another")
	}
}

func TestBlockedOverCap(t *testing.T) {
	if Allowed("resume", map[string]int{"resume": 3}, map[string]int{"resume": 2}) {
		t.Fatal("a class already over its cap must not admit another")
	}
}

func TestUncappedClassAlwaysAllowed(t *testing.T) {
	if !Allowed("rework", map[string]int{"rework": 50}, map[string]int{"fresh": 2}) {
		t.Fatal("a class with no configured cap must always be admitted")
	}
}
