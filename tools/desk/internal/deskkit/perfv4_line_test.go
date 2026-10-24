package deskkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

// regression: #4360
func TestV4HistoryLineBounds(t *testing.T) {
	t.Setenv(perfrec.EnvVar, "1")
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }})
	p := perfrec.Probe{}
	r := perfrec.BeginSelected(p, p.Mark(), nil, c).Finish("tool", "query", "", 0, "")
	r.Install, _ = perfrec.NewID()
	v4, e := perfrec.ToV4(r)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(v4)
	if e != nil {
		t.Fatal(e)
	}
	for _, ending := range []string{"\n", "\r\n"} {
		for _, size := range []int{perfrec.MaxV4LineBytes - 1, perfrec.MaxV4LineBytes, perfrec.MaxV4LineBytes + 1} {
			t.Run(fmt.Sprintf("%d/%q", size, ending), func(t *testing.T) {
				line := append(append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), size-len(raw)-len(ending))...), []byte(ending)...)
				path := filepath.Join(t.TempDir(), "day.jsonl")
				if e := os.WriteFile(path, line, 0600); e != nil {
					t.Fatal(e)
				}
				rs, skipped, e := readPerfDayFile(path)
				if e != nil {
					t.Fatal(e)
				}
				want := 0
				if size < perfrec.MaxV4LineBytes {
					want = 1
				}
				if len(rs) != want || skipped != 1-want {
					t.Fatalf("actual wire bytes=%d ending=%q records=%d skipped=%d; want records=%d", len(line), ending, len(rs), skipped, want)
				}
			})
		}
	}
}
