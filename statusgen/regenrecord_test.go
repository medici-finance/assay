package main

// Tests for the board regen job's status-historian wiring. history.go's
// header says the --record pass is wired into main's regen CI only, after
// STATUS.md regenerates, and that docs/streams/.history.jsonl is git-tracked.
// Until the regen job ran it, the log never existed in this repository, so
// every Age / Waiting cell on STATUS.md read "—" and --bottleneck had no
// dwell to read. These tests pin the wiring on the live workflow and its
// staged twin: the record runs after the write (and again on a push retry),
// the commit step lands the log in the same bot commit as STATUS.md, and the
// push trigger excludes the log so that commit cannot retrigger the workflow.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// regenRecordWorkflows are the copies whose regen job records the historian.
// The tools/ci-load/activation/ copy predates the wiring and is stale (#1187).
var regenRecordWorkflows = []string{
	"../.github/workflows/assay-statusgen.yml",
	"../ci/staged-workflows/assay-statusgen.yml",
}

const (
	regenCmd       = `"${RUNNER_TEMP}/statusgen" --root .`
	regenRecordCmd = regenCmd + ` --write-and-record --acceptance-forge`
	historyExclude = `      - "!docs/streams/.history.jsonl"`
)

// regenRecordProblems reports why a board workflow would not record and
// commit the status historian from its regen job, or nil when it would.
func regenRecordProblems(wf string) []string {
	var p []string
	job := jobBlock(wf, "  regen:")
	if job == "" {
		return []string{"no regen job"}
	}
	// A maintainer can promote the one-pass command independently. Both forms
	// must finish the board and its histories once per attempt, in order.
	want := []string{regenCmd + " --acceptance-forge", regenCmd + " --record --acceptance-forge"}
	if strings.Contains(job, regenRecordCmd) {
		want = []string{regenRecordCmd}
	}
	initial, retry, _ := strings.Cut(job, "git reset --hard origin/main\n")
	for _, step := range []string{initial, retry} {
		var calls []string
		for _, line := range strings.Split(step, "\n") {
			if line = strings.TrimSpace(line); strings.HasPrefix(line, regenCmd) {
				calls = append(calls, line)
			}
		}
		if !slices.Equal(calls, want) {
			p = append(p, "initial regeneration and retry must each write and record once")
		}
	}
	if !strings.Contains(job, "then git add -- docs/streams/.history.jsonl; fi") {
		p = append(p, "the commit step does not add docs/streams/.history.jsonl")
	}
	if !strings.Contains(job, `git status --porcelain -- STATUS.md docs/streams/.history.jsonl`) {
		p = append(p, "the commit guard does not see a history-only change")
	}
	on := wf[:strings.Index(wf, "\njobs:")]
	push := on[strings.Index(on, "\n  push:"):]
	if k := strings.Index(push, "\n  schedule:"); k >= 0 {
		push = push[:k]
	}
	if !strings.Contains(push, `      - "docs/**"`+"\n") || !strings.Contains(push, historyExclude+"\n") ||
		strings.Index(push, historyExclude) < strings.Index(push, `      - "docs/**"`) {
		p = append(p, "the push trigger does not exclude docs/streams/.history.jsonl after docs/**, so the regen commit retriggers the workflow")
	}
	return p
}

func TestBoardRegenRecordsHistory(t *testing.T) {
	for _, path := range regenRecordWorkflows {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if path == regenRecordWorkflows[1] && !strings.Contains(string(raw), regenRecordCmd) {
			t.Errorf("%s: staged writer lacks --write-and-record", path)
		}
		for _, why := range regenRecordProblems(string(raw)) {
			t.Errorf("%s: %s", path, why)
		}
	}
}

// TestBoardRegenRecordGuardFires is the negative control: each of the
// properties, removed from the live file, must be reported.
func TestBoardRegenRecordGuardFires(t *testing.T) {
	raw, err := os.ReadFile(regenRecordWorkflows[1])
	if err != nil {
		t.Fatal(err)
	}
	wf := string(raw)
	if p := regenRecordProblems(wf); len(p) != 0 {
		t.Fatalf("baseline is not clean: %v", p)
	}
	mutants := map[string][2]string{
		"retry without record": {"\n            " + regenRecordCmd + "\n", "\n            " + regenCmd + " --acceptance-forge\n"},
		"extra record pass":    {regenRecordCmd + "\n", regenRecordCmd + "\n          " + regenCmd + " --record --acceptance-forge\n"},
		"no history add":       {"then git add -- docs/streams/.history.jsonl; fi", "then :; fi"},
		"guard on STATUS.md":   {"git status --porcelain -- STATUS.md docs/streams/.history.jsonl", "git status --porcelain -- STATUS.md"},
		"no push exclusion":    {historyExclude + "\n", ""},
	}
	for name, m := range mutants {
		if !strings.Contains(wf, m[0]) {
			t.Fatalf("%s: mutation anchor not found", name)
		}
		if len(regenRecordProblems(strings.Replace(wf, m[0], m[1], 1))) == 0 {
			t.Errorf("%s: the guard reported nothing", name)
		}
	}
}

// A real restore episode makes missing or duplicate DORA recording visible.
// The fixture does not use GitHub or require credentials.
func TestWriteAndRecordMatchesSeparatePasses(t *testing.T) {
	fixedNow(t, "2026-08-26T00:00:00Z")
	t.Setenv("GITHUB_REPOSITORY", "example-org/board")
	t.Setenv("STATUSGEN_DORA_WORKFLOW", "ci")
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/repos/example-org/board/actions/runs":
			body = `{"workflow_runs":[{"id":100,"name":"ci","head_sha":"red","status":"completed","conclusion":"failure","updated_at":"2026-08-25T20:00:00Z"},{"id":101,"name":"ci","head_sha":"green","status":"completed","conclusion":"success","updated_at":"2026-08-25T21:30:00Z"}]}`
		case "/repos/example-org/board/pulls":
			body = "[]"
		default:
			return nil, fmt.Errorf("unexpected request %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	separate, combined := t.TempDir(), t.TempDir()
	for _, root := range []string{separate, combined} {
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
	}
	contents := func(root, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var firstHistory, firstDora []byte
	for pass := 1; pass <= 2; pass++ {
		if code := run(separate, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write: exit %d", code)
		}
		if pass == 1 {
			for _, path := range []string{historyRelPath, doraTimingRelPath} {
				if _, err := os.Stat(filepath.Join(separate, path)); !os.IsNotExist(err) {
					t.Fatalf("standalone write recorded %s: %v", path, err)
				}
			}
		}
		if code := run(separate, "record", nil, nil, ""); code != 0 {
			t.Fatalf("record: exit %d", code)
		}
		if code := run(combined, "write-and-record", nil, nil, ""); code != 0 {
			t.Fatalf("write-and-record: exit %d", code)
		}
		for _, path := range []string{"STATUS.md", "docs/streams/FINDINGS.md", historyRelPath, doraTimingRelPath} {
			if !bytes.Equal(contents(separate, path), contents(combined, path)) {
				t.Fatalf("pass %d: %s differs", pass, path)
			}
		}
		history, dora := contents(combined, historyRelPath), contents(combined, doraTimingRelPath)
		if pass == 1 {
			if bytes.Count(history, []byte("\n")) != 2 || bytes.Count(dora, []byte("\n")) != 1 {
				t.Fatal("expected two seeded brief transitions and one restore episode")
			}
			firstHistory, firstDora = history, dora
		} else if !bytes.Equal(history, firstHistory) || !bytes.Equal(dora, firstDora) {
			t.Fatal("retry changed a historian")
		}
	}
}

func TestWriteAndRecordSkipsHistoryOnWriteError(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "STATUS.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "write-and-record", nil, nil, ""); code != 1 {
		t.Fatalf("board write failed, got exit %d", code)
	}
	for _, path := range []string{historyRelPath, doraTimingRelPath} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("failed write recorded %s: %v", path, err)
		}
	}
}
