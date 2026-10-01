package deskkit

import "testing"

func TestIsAbsFor(t *testing.T) {
	for _, tc := range []struct {
		path       string
		posix, win bool
	}{
		{"/a", true, false}, {`C:\a`, false, true}, {"C:/a", false, true}, {`C:rel`, false, false},
		{`\rooted`, false, false}, {"a/b", false, false}, {"", false, false},
		{`\\srv\share\a`, false, true}, {"//srv/share/a", true, true},
		{`\\?\UNC\srv\share\a`, false, true}, {`\\?\C:\a`, false, true}, {`\\.\pipe\x`, false, true},
	} {
		for _, goos := range []string{"linux", "darwin", "windows"} {
			t.Run(goos+"/"+tc.path, func(t *testing.T) {
				want := tc.posix
				if goos == "windows" {
					want = tc.win
				}
				if got := IsAbsFor(goos, tc.path); got != want {
					t.Fatalf("IsAbsFor(%q,%q)=%v want %v", goos, tc.path, got, want)
				}
			})
		}
	}
	// CELL_ROOTS entries split at '=' only; neither drive colon nor backslash is a separator.
	entry := `example/repo=C:\a`
	if !IsAbsFor("windows", entry[len("example/repo="):]) {
		t.Fatal("drive path in root entry rejected")
	}
}
