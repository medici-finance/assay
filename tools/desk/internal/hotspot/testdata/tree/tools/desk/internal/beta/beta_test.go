package beta

import "testing"

func TestBeta(t *testing.T) {
	if Beta(1) != 1 {
		t.Fatal("beta")
	}
}
