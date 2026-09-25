package samplepkg

// TestSomething is TestCountsFixture's refusals-dimension decoy: a call to Refused inside
// a _test.go file must NOT be counted (a counter that counted this would pass row 2's
// exact-count assertion only by accident, and fail as soon as a real test called Refused
// to assert a refusal — see brief-03's Review note).
func TestSomething() {
	_ = Refused("decoy: _test.go refusal — NOT counted")
}
