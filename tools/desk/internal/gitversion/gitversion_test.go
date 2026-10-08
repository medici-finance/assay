package gitversion

import "testing"

func TestParseGitVersion(t *testing.T) {
	cases := []struct {
		in                  string
		major, minor, patch int
	}{
		{"git version 2.39.5\n", 2, 39, 5},
		{"git version 2.47.3", 2, 47, 3},
		{"git version 2.50.1 (Apple Git-155)", 2, 50, 1},
		{"git version 2.45.2.windows.1", 2, 45, 2},
		{"git version 3.0", 3, 0, 0},
	}
	for _, c := range cases {
		v, err := ParseGitVersion(c.in)
		if err != nil {
			t.Fatalf("ParseGitVersion(%q): %v", c.in, err)
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.patch {
			t.Fatalf("ParseGitVersion(%q) = %s, want %d.%d.%d", c.in, v, c.major, c.minor, c.patch)
		}
	}
	for _, bad := range []string{"", "hub version 2.14.2", "git version two"} {
		if _, err := ParseGitVersion(bad); err == nil {
			t.Fatalf("ParseGitVersion(%q): want an error, got none", bad)
		}
	}
}

func TestGitVersionAtLeast(t *testing.T) {
	cases := []struct {
		v            GitVersion
		major, minor int
		want         bool
	}{
		{GitVersion{Major: 2, Minor: 39, Patch: 5}, 2, 40, false},
		{GitVersion{Major: 2, Minor: 40}, 2, 40, true},
		{GitVersion{Major: 2, Minor: 45, Patch: 9}, 2, 46, false},
		{GitVersion{Major: 2, Minor: 47, Patch: 3}, 2, 46, true},
		{GitVersion{Major: 3, Minor: 0}, 2, 46, true},
		{GitVersion{Major: 1, Minor: 99}, 2, 0, false},
	}
	for _, c := range cases {
		if got := c.v.AtLeast(c.major, c.minor); got != c.want {
			t.Fatalf("%s.AtLeast(%d, %d) = %v, want %v", c.v, c.major, c.minor, got, c.want)
		}
	}
}

// The installed git must parse: a host whose git the guard cannot read would otherwise turn
// every version-gated test into a silent skip.
func TestInstalledGitVersionParses(t *testing.T) {
	v := InstalledGitVersion(t)
	if v.Major < 2 {
		t.Fatalf("installed git %s is older than any supported git", v)
	}
}
