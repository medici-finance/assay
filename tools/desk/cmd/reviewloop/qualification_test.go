package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// TestQualificationProbe is a test-binary adapter, not another planner. The harness
// owns the literal expectations; this adapter only calls and projects production code.
func TestQualificationProbe(t *testing.T) {
	q := reviewtest.Read(t)
	a, p := filepath.Join(q.Root, "actions.json"), filepath.Join(q.Root, "prs.json")
	for path, data := range map[string][]byte{a: q.Actions, p: q.PRs} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := cmdPlan([]string{"--actions", a, "--prs", p, "--now", q.Now}, io.Discard)
	r := reviewtest.Response{Exit: deskkit.ExitCodeOf(err), HTTPComplete: true, Depth: map[string]int{}}
	b, readErr := ReadBoard(q.Actions, q.PRs)
	if readErr == nil {
		now, e := time.Parse(time.RFC3339, q.Now)
		if e != nil {
			t.Fatal(e)
		}
		r.Idle = Idle(b, now).State.String()
		for _, row := range b.Rows {
			r.Rows = append(r.Rows, reviewtest.Row{ID: row.ID(), Head: row.Head, Action: row.Action, Disposition: row.Rule.Disposition.String()})
		}
		for k, v := range b.CountByDisposition() {
			r.Depth[k.String()] = v
		}
		// Duplicate observations must coalesce without losing their measured count.
		vs := Coalesce([]*Board{b, b})
		r.Facts = map[string]string{"proposals": fmt.Sprint(len(vs))}
		for _, v := range vs {
			r.Facts[fmt.Sprintf("proposal/%d", v.Key.PR)] = fmt.Sprintf("%s:%s:%v:%d", v.Key.Head, v.Key.Verb, v.Suppressed, v.Observations)
		}
	}
	reviewtest.Write(t, r)
}
