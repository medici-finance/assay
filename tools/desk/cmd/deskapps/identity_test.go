package main

import (
	"errors"
	"testing"
)

// TestGhIdentityRefusesEmptyLogin — A-2. An empty `gh api user` login is refused when the
// identity is resolved, not only at /callback, so the person is told before they click Create.
func TestGhIdentityRefusesEmptyLogin(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		err     error
		wantErr bool
	}{
		{name: "login present", out: `{"login":"the-real-operator"}`},
		{name: "login empty", out: `{"login":""}`, wantErr: true},
		{name: "login absent", out: `{}`, wantErr: true},
		{name: "gh fails", err: errors.New("exit status 1"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := runGH
			t.Cleanup(func() { runGH = old })
			runGH = func(args ...string) ([]byte, error) { return []byte(tc.out), tc.err }
			u, err := ghIdentity()
			if (err != nil) != tc.wantErr {
				t.Fatalf("ghIdentity() err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && u.Login != "" {
				t.Fatalf("a refused identity carried login %q", u.Login)
			}
		})
	}
}
