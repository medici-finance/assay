package main

import (
	"os"
	"path/filepath"
	"testing"
)

// regression: #3327
func TestBoardRequiresRoleClaims(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(t.TempDir(), "statusgen")
	script := `#!/bin/sh
case " $* " in *" --role-claims "*) echo '{"claimsKnown":false,"claimsReason":"role identity unavailable","rows":[]}'; exit 0;; esac
echo missing-role-boundary >&2
exit 6
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESK_LOOP", "")
	v, err := nextUpForRoot(bin, root, "example/project")
	if err != nil || v.ClaimsKnown || v.ClaimsReason != "role identity unavailable" {
		t.Fatalf("board lost required role read: %+v %v", v, err)
	}
}
