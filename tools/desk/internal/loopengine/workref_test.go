package loopengine

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"testing"
)

// regression: #5704
func TestLocalSubjectHold(t *testing.T) {
	dir := t.TempDir()
	for _, value := range []string{"stream:b-08@2", "stream:m-08@2"} {
		ok, err := Claim(Config{ClaimsDir: dir, RunnerID: "worker"}, Item{ID: value})
		if ok || err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("unqualified local subject claimed: %s %v", value, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("subject hold wrote local claim: %v %v", entries, err)
	}
}
