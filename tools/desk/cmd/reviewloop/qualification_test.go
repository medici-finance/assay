package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// TestQualificationProbe is a test-binary adapter, not another planner. The harness
// owns the literal expectations; this adapter only calls and projects production code.
func TestQualificationProbe(t *testing.T) {
	q := reviewtest.Read(t)
	if strings.HasPrefix(q.Case, "snapshot") {
		snapshotProbe(t, q)
		return
	}
	a, p := filepath.Join(q.Root, "actions.json"), filepath.Join(q.Root, "prs.json")
	for path, data := range map[string][]byte{a: q.Actions, p: q.PRs} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	err := cmdPlan([]string{"--actions", a, "--prs", p, "--now", q.Now}, &output)
	r := reviewtest.Response{Output: output.String(), Exit: deskkit.ExitCodeOf(err), HTTPComplete: true, Depth: map[string]int{}}
	// Internal observations retain full SHAs and duplicate-sweep semantics that the
	// human-facing text abbreviates. The oracle checks Output independently as well.
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

type rejectNetwork struct{ calls int }

func (r *rejectNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, errors.New("offline planner tried network")
}
func snapshotProbe(t *testing.T, q reviewtest.Request) {
	t.Helper()
	trap := &rejectNetwork{}
	old := http.DefaultTransport
	http.DefaultTransport = trap
	t.Cleanup(func() { http.DefaultTransport = old })
	file := filepath.Join(q.Root, "snapshot.json")
	if err := os.WriteFile(file, q.Snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	args := []string{"--snapshot", file, "--selection", q.Selection}
	if q.Case != "snapshot-no-now" {
		args = append(args, "--now", q.Now)
	}
	if q.Case == "snapshot-conflict" {
		args = append(args, "--actions", filepath.Join(q.Root, "does-not-exist"))
	}
	err := cmdPlan(args, &out)
	reviewtest.Write(t, reviewtest.Response{Output: out.String(), Exit: deskkit.ExitCodeOf(err), Facts: map[string]string{"networkAttempts": fmt.Sprint(trap.calls)}, HTTPComplete: trap.calls == 0})
}
