package deskkit

import "testing"

// regression: #2732
func TestRemoteUserinfoEmailBoundary(t *testing.T) {
	cases := []struct {
		name, text string
		refuse     bool
	}{
		{"scp", "git@github.com:example-org/project.git", false},
		{"ssh", "ssh://git@github.com/example-org/project.git", false},
		{"https", "https://fixture-user@github.com/example-org/project.git", false},
		{"password-userinfo", "https://fixture-user:" + "fixture-token" + "@github.com/example-org/project.git", false},
		{"plain-address", obEmailOutside, true},
		{"space-after-remote", "https://github.com/example-org/project.git " + obEmailOutside, true},
		{"comma-after-remote", "https://github.com/example-org/project.git," + obEmailOutside, true},
		{"semicolon-after-remote", "https://github.com/example-org/project.git;" + obEmailOutside, true},
		{"path-address-with-colon", "https://github.com/example-org/" + obEmailOutside + ":notes", true},
		{"root-url-comma-address", "https://github.com," + obEmailOutside, true},
		{"path-address", "https://github.com/example-org/" + obEmailOutside, true},
		{"query-address", "https://github.com/example-org/project.git?contact=" + obEmailOutside, true},
		{"fragment-address", "https://github.com/example-org/project.git#" + obEmailOutside, true},
		{"double-at-unreserved", obEmailOutside + "@github.com:example-org/project.git", true},
		{"synthetic-double-at", "git@example.invalid@github.com:example-org/project.git", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found := false
			for _, f := range personalDataScan(c.text) {
				if f.rule == RulePIIEmail {
					found = true
				}
			}
			if found != c.refuse {
				t.Fatalf("pii.email refused=%v want=%v", found, c.refuse)
			}
		})
	}
}
