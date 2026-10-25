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

// regression: #4360 (review F1)
func TestV4HistoryContinuesAfterOversizedLine(t *testing.T) {
	c := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}, LookupEnv: func(k string) (string, bool) { return "1", k == perfrec.EnvVar }})
	p := perfrec.Probe{}
	r := perfrec.BeginSelected(p, p.Mark(), nil, c).Finish("tool", "query", "", 0, "")
	r.Install, _ = perfrec.NewID()
	v4, err := perfrec.ToV4(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(v4)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{perfrec.MaxV4LineBytes + 16, 3 * perfrec.MaxV4LineBytes} {
		for _, ending := range []string{"\n", "\r\n"} {
			t.Run(fmt.Sprintf("%d/%q", size, ending), func(t *testing.T) {
				good := append(append([]byte{}, raw...), '\n')
				data := append([]byte{}, good...)
				// The tail looks like a valid record; it is still part of the oversized line.
				data = append(data, bytes.Repeat([]byte(" "), size)...)
				data = append(data, raw...)
				data = append(data, ending...)
				for i := 0; i < 3; i++ {
					data = append(data, good...)
				}
				path := filepath.Join(t.TempDir(), "day.jsonl")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				records, skipped, err := readPerfDayFile(path)
				if err != nil || len(records) != 4 || skipped != 1 {
					t.Fatalf("records=%d skipped=%d err=%v; want all 4 valid records and one skipped line", len(records), skipped, err)
				}
				// A final oversized line without LF is also exactly one skip.
				data = append(data, bytes.Repeat([]byte("x"), size)...)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				records, skipped, err = readPerfDayFile(path)
				if err != nil || len(records) != 4 || skipped != 2 {
					t.Fatalf("unterminated oversized tail: records=%d skipped=%d err=%v", len(records), skipped, err)
				}
			})
		}
	}
}
