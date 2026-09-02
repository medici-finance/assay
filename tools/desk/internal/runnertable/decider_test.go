package runnertable

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestDeciderRefusesAtBoot covers the decider contract: a missing or
// containment-incompatible decider entry refuses at boot, and a valid
// pinned+contained entry loads clean.
func TestDeciderRefusesAtBoot(t *testing.T) {
	t.Run("missing entirely refuses", func(t *testing.T) {
		if _, err := LoadDecider(envGetter(map[string]string{})); err == nil {
			t.Fatal("a missing decider entry must refuse at boot")
		}
	})

	t.Run("not declared contained refuses", func(t *testing.T) {
		_, err := LoadDecider(envGetter(map[string]string{
			envDeciderKey: `{"cmd":["x"],"pin":"1"}`,
		}))
		if err == nil || !strings.Contains(err.Error(), "contained") {
			t.Fatalf("an entry not declared contained must refuse, got: %v", err)
		}
		if !deskkit.IsRefused(err) {
			t.Fatalf("uncontained decider must map to exit 5 (refused), got code %d", deskkit.ExitCodeOf(err))
		}
	})

	t.Run("missing pin refuses even when contained", func(t *testing.T) {
		_, err := LoadDecider(envGetter(map[string]string{
			envDeciderKey: `{"cmd":["x"],"contained":true}`,
		}))
		if err == nil || !strings.Contains(err.Error(), "version pin") {
			t.Fatalf("a decider entry with no pin must refuse, got: %v", err)
		}
	})

	t.Run("valid pinned contained entry loads clean", func(t *testing.T) {
		d, err := LoadDecider(envGetter(map[string]string{
			envDeciderKey: `{"cmd":["npx","claude-agent-acp"],"model":"decider-model","pin":"0.4.1","contained":true}`,
		}))
		if err != nil {
			t.Fatalf("a valid contained+pinned entry must load clean, got: %v", err)
		}
		if d.Pin != "0.4.1" || !d.Contained || d.Model != "decider-model" {
			t.Fatalf("decider entry mis-loaded: %+v", d)
		}
	})
}
