package humanid

import "testing"

// regression: #3669
func TestStartupPolicy(t *testing.T) {
	var p policy
	if p.get() != "" {
		t.Fatal("unconfigured policy enabled")
	}
	for _, invalid := range []string{" ", "-operator", "operator-", "operator/name", "operator[bot]", "operator name", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if err := p.configure(invalid); err == nil {
			t.Fatalf("accepted invalid principal %q", invalid)
		}
		if p.get() != "" {
			t.Fatal("invalid initialization changed policy")
		}
	}
	if err := p.configure("operator"); err != nil {
		t.Fatal(err)
	}
	if p.get() != "operator" {
		t.Fatal("explicit principal lost")
	}
	for _, replacement := range []string{"", "other-user", "operator"} {
		if err := p.configure(replacement); err == nil {
			t.Fatal("reconfiguration accepted")
		}
		if p.get() != "operator" {
			t.Fatal("reconfiguration changed original principal")
		}
	}
	var disabled policy
	if err := disabled.configure(""); err != nil {
		t.Fatal(err)
	}
	if err := disabled.configure("operator"); err == nil || disabled.get() != "" {
		t.Fatal("disabled policy was replaceable")
	}
}

// regression: #3669
func TestUnconfiguredMatchesNobody(t *testing.T) {
	for _, login := range []string{"", " ", "operator", "other-user"} {
		if Matches(login) {
			t.Fatalf("unconfigured policy authorized %q", login)
		}
	}
}
