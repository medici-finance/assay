package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	q "github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

func callerPreparationProbe(t *testing.T, request q.Request) {
	t.Helper()
	_, f, stamp, labels, args := prepareDispatchFixture(t)
	observed, err := rf.DecodeObservation(request.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range observed.Result.Rows {
		if row.Identity.Number == request.Size {
			f.head = row.Identity.Head
			f.number = row.Identity.Number
			found = true
		}
	}
	stamp.number = f.number
	if !found {
		t.Fatal("selected subject absent from watcher handoff")
	}
	path := filepath.Join(t.TempDir(), "watcher.json")
	if err := os.WriteFile(path, request.Snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	args[0] = strings.Replace(args[0], "pr-77", "pr-"+strconv.Itoa(f.number), 1)
	for i := 1; i < len(args); i++ {
		if args[i-1] == "--observation" {
			args[i] = path
		}
		if args[i-1] == "--pr" {
			args[i] = strconv.Itoa(f.number)
		}
	}
	if strings.HasSuffix(request.Case, "-moved") {
		f.moved = "head"
	}
	rc, stderr := runCapturingStderr(t, args)
	output := ""
	if rc == 0 {
		b, err := os.ReadFile(emittedPathFor(promptFileOf(t, args)))
		if err != nil {
			t.Fatal(err)
		}
		output = string(b)
	}
	q.Write(t, q.Response{Exit: rc, Output: output, Facts: map[string]string{"head": f.head, "prReads": fmt.Sprint(f.prReads), "stamps": fmt.Sprint(len(stamp.requests)), "queueLabels": fmt.Sprint(len(*labels)), "diagnostic": stderr}, Attempts: f.transport.Snapshot(), HTTPComplete: true})
}
