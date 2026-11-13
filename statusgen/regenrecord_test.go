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
	"os"
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
	regenWriteCmd  = `"${RUNNER_TEMP}/statusgen" --root . --acceptance-forge`
	regenRecordCmd = `"${RUNNER_TEMP}/statusgen" --root . --record --acceptance-forge`
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
	// Every write in the job (the first run and each push retry) is followed
	// directly by a record against the same tree.
	writes := strings.Count(job, regenWriteCmd+"\n")
	if writes == 0 {
		p = append(p, "regen job runs no statusgen write")
	}
	for i, rest := 0, job; i < writes; i++ {
		at := strings.Index(rest, regenWriteCmd+"\n")
		rest = rest[at+len(regenWriteCmd)+1:]
		next := strings.TrimLeft(rest, " ")
		for strings.HasPrefix(next, "#") {
			next = strings.TrimLeft(next[strings.Index(next, "\n")+1:], " ")
		}
		if !strings.HasPrefix(next, regenRecordCmd+"\n") {
			p = append(p, "a statusgen write in the regen job is not followed by --record")
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
		for _, why := range regenRecordProblems(string(raw)) {
			t.Errorf("%s: %s", path, why)
		}
	}
}

// TestBoardRegenRecordGuardFires is the negative control: each of the four
// properties, removed from the live file, must be reported.
func TestBoardRegenRecordGuardFires(t *testing.T) {
	raw, err := os.ReadFile(regenRecordWorkflows[0])
	if err != nil {
		t.Fatal(err)
	}
	wf := string(raw)
	if p := regenRecordProblems(wf); len(p) != 0 {
		t.Fatalf("baseline is not clean: %v", p)
	}
	mutants := map[string][2]string{
		"retry without record": {"            " + regenWriteCmd + "\n            " + regenRecordCmd + "\n", "            " + regenWriteCmd + "\n"},
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
