package cellop

import (
	"fmt"
	"testing"
)

// The confirm word is pinned to the digest's first 8 bytes modulo the list length, steps past a
// word of the plan's id, and changes when those bytes change (spec/cell-op-v1.md section 3).
// regression: #6605
func TestConfirmWordPinned(t *testing.T) {
	zeros := fmt.Sprintf("%064x", 0)
	at := func(i int) string { return fmt.Sprintf("%016x%048x", i, 0) }
	nAdj := len(idAdjectives)
	for _, c := range []struct{ d, id, want string }{
		{zeros, "4f1c2a9e7b3d5c8e0a6f2b1d", "able"},
		{zeros, "calm-heron", "able"},
		{at(1), "calm-heron", "agile"},
		{at(nAdj), "calm-heron", "falcon"},
		{at(nAdj + len(idNouns)), "calm-heron", "able"},
		// stepping past a word of the id, once and twice, and across the end of the list
		{zeros, "able-falcon", "agile"},
		{zeros, "able-agile", "airy"},
		{at(nAdj), "brave-falcon", "heron"},
		{at(nAdj + len(idNouns) - 1), "calm-horizon", "able"},
	} {
		got, err := confirmWord(c.d, c.id)
		if err != nil || got != c.want {
			t.Errorf("confirmWord(%s…, %s) = %q, %v; want %q", c.d[:16], c.id, got, err, c.want)
		}
	}
	if _, err := confirmWord("not-a-digest", "calm-heron"); err == nil {
		t.Error("a malformed digest selected a word")
	}
}
