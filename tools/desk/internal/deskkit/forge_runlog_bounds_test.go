package deskkit

// Run-log BOUND tests (review finding runlog-bounds-unpinned). Each bound the backends apply —
// the parts per run, the archive download, the bytes read per part, the tail kept per part, and
// GitLab's job-list pagination — is driven past its edge with a lowered value through the same
// code path production runs (runLog with explicit bounds), so removing any of them goes red.

import (
	"strings"
	"testing"
)

// lowBounds returns defaultRunLogBounds with the given edits.
func lowBounds(edit func(*runLogBounds)) runLogBounds {
	b := defaultRunLogBounds
	edit(&b)
	return b
}

// wantCouldNotCheck fails unless err is a could-not-check whose text names every fragment.
func wantCouldNotCheck(t *testing.T, err error, parts []RunLogPart, frags ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want could-not-check, got %d part(s) as success", len(parts))
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("want could-not-check (exit %d), got exit %d: %v", ExitUnverifiable, ExitCodeOf(err), err)
	}
	for _, f := range frags {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("refusal does not name %q: %v", f, err)
		}
	}
}

// TestRunLogBoundsDefaults: production's bounds are the documented constants.
func TestRunLogBoundsDefaults(t *testing.T) {
	want := runLogBounds{partCap: 4 << 20, readCap: 64 << 20, maxParts: 200, archiveCap: 64 << 20, perPage: 100, maxPages: 25}
	if defaultRunLogBounds != want {
		t.Fatalf("defaultRunLogBounds = %+v, want %+v", defaultRunLogBounds, want)
	}
	if RunLogPartCap != want.partCap || RunLogReadCap != want.readCap || RunLogMaxParts != want.maxParts ||
		runLogArchiveCap != want.archiveCap || gitlabPerPage != want.perPage || gitlabMaxCIPage != want.maxPages {
		t.Fatalf("a run-log constant drifted from the documented bound")
	}
}

func ghJobsArchive(t *testing.T, n int) []byte {
	var files [][2]string
	for i := 0; i < n; i++ {
		files = append(files, [2]string{"job" + string(rune('a'+i)) + ".txt", "log\n"})
	}
	return zipOf(t, files)
}

// TestRunLogGithubPartBound: the job count is bounded — at the bound it reads, past it it refuses.
func TestRunLogGithubPartBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.maxParts = 2 })
	if parts, err := ghRunLogStub(t, ghJobsArchive(t, 2)).runLog(rlRepo, RunRef{ID: "501"}, lim); err != nil || len(parts) != 2 {
		t.Fatalf("at the bound: %d part(s), err %v", len(parts), err)
	}
	parts, err := ghRunLogStub(t, ghJobsArchive(t, 3)).runLog(rlRepo, RunRef{ID: "501"}, lim)
	wantCouldNotCheck(t, err, parts, "more than 2 jobs")
	// Step files do not count against the bound: two jobs with many steps each read fine.
	steps := zipOf(t, [][2]string{{"0_a.txt", "a\n"}, {"1_b.txt", "b\n"}, {"a/1_x.txt", "a\n"}, {"a/2_y.txt", "a\n"}, {"b/1_x.txt", "b\n"}})
	if parts, err := ghRunLogStub(t, steps).runLog(rlRepo, RunRef{ID: "501"}, lim); err != nil || len(parts) != 2 {
		t.Fatalf("step files spent the job bound: %d part(s), err %v", len(parts), err)
	}
}

// TestRunLogGithubArchiveBound: the archive download is bounded — an archive of exactly the cap
// reads, one byte more refuses.
func TestRunLogGithubArchiveBound(t *testing.T) {
	arc := ghJobsArchive(t, 1)
	at := lowBounds(func(b *runLogBounds) { b.archiveCap = int64(len(arc)) })
	if _, err := ghRunLogStub(t, arc).runLog(rlRepo, RunRef{ID: "501"}, at); err != nil {
		t.Fatalf("an archive at the cap must read: %v", err)
	}
	over := lowBounds(func(b *runLogBounds) { b.archiveCap = int64(len(arc)) - 1 })
	parts, err := ghRunLogStub(t, arc).runLog(rlRepo, RunRef{ID: "501"}, over)
	wantCouldNotCheck(t, err, parts, "log archive", "exceeds")
}

// TestRunLogGithubReadBound: the bytes read per job are bounded, summed across a job's step
// files — past the bound the job is refused by name, never cut to a slice.
func TestRunLogGithubReadBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.readCap = 100 })
	at := zipOf(t, [][2]string{{"0_build.txt", strings.Repeat("a", 100)}})
	if _, err := ghRunLogStub(t, at).runLog(rlRepo, RunRef{ID: "501"}, lim); err != nil {
		t.Fatalf("a job log at the read bound must read: %v", err)
	}
	over := zipOf(t, [][2]string{{"0_build.txt", strings.Repeat("a", 101)}})
	parts, err := ghRunLogStub(t, over).runLog(rlRepo, RunRef{ID: "501"}, lim)
	wantCouldNotCheck(t, err, parts, `"0_build.txt"`, "exceeds 100 bytes")
	joined := zipOf(t, [][2]string{{"build/1_a.txt", strings.Repeat("a", 60)}, {"build/2_b.txt", strings.Repeat("b", 60)}})
	parts, err = ghRunLogStub(t, joined).runLog(rlRepo, RunRef{ID: "501"}, lim)
	wantCouldNotCheck(t, err, parts, `"build"`, "exceeds 100 bytes")
}

// TestRunLogGithubTailBound: the text kept per job is the cap's worth of its END.
func TestRunLogGithubTailBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.partCap = 8 })
	arc := zipOf(t, [][2]string{{"0_build.txt", "12345678"}, {"1_test.txt", strings.Repeat("a", 50) + "FAILURE!"}})
	parts, err := ghRunLogStub(t, arc).runLog(rlRepo, RunRef{ID: "501"}, lim)
	if err != nil || len(parts) != 2 {
		t.Fatalf("%d part(s), err %v", len(parts), err)
	}
	if parts[0].Text != "12345678" || parts[0].Truncated {
		t.Fatalf("a log at the cap is whole: %+v", parts[0])
	}
	if parts[1].Text != "FAILURE!" || !parts[1].Truncated {
		t.Fatalf("an over-cap log keeps its last 8 bytes, flagged: %+v", parts[1])
	}
}

// TestRunLogGitlabJobBound: the job count is bounded on GitLab too.
func TestRunLogGitlabJobBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.maxParts = 2 })
	jobs, traces := glJobs(2)
	if parts, err := (&glRunLogStub{jobs: jobs, traces: traces}).forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim); err != nil || len(parts) != 2 {
		t.Fatalf("at the bound: %d part(s), err %v", len(parts), err)
	}
	jobs, traces = glJobs(3)
	parts, err := (&glRunLogStub{jobs: jobs, traces: traces}).forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim)
	wantCouldNotCheck(t, err, parts, "more than 2 jobs")
}

// TestRunLogGitlabPaged: a job list longer than one page is walked to its end (X-Next-Page),
// so a pipeline is never reduced to its first page of jobs.
func TestRunLogGitlabPaged(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.perPage = 2 })
	jobs, traces := glJobs(5)
	s := &glRunLogStub{jobs: jobs, traces: traces}
	parts, err := s.forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim)
	if err != nil {
		t.Fatalf("RunLog: %v", err)
	}
	if len(parts) != 5 || parts[4].Name != "job-4" || parts[4].Text != "log of job-4\n" || s.pages != 3 {
		t.Fatalf("want all 5 jobs over 3 pages, got %d part(s) over %d page(s)", len(parts), s.pages)
	}
}

// TestRunLogGitlabPageBound: a job list with more pages than the walk allows refuses rather
// than returning the pages it did read.
func TestRunLogGitlabPageBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.perPage = 2; b.maxPages = 2 })
	jobs, traces := glJobs(4)
	if parts, err := (&glRunLogStub{jobs: jobs, traces: traces}).forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim); err != nil || len(parts) != 4 {
		t.Fatalf("exactly the page bound: %d part(s), err %v", len(parts), err)
	}
	jobs, traces = glJobs(5)
	parts, err := (&glRunLogStub{jobs: jobs, traces: traces}).forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim)
	wantCouldNotCheck(t, err, parts, "more job pages")
}

// TestRunLogGitlabReadBound: the bytes read per trace are bounded — streamed, and refused by
// job name past the bound.
func TestRunLogGitlabReadBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.readCap = 100 })
	s := &glRunLogStub{jobs: []map[string]any{{"id": 70, "name": "build", "status": "failed"}},
		traces: map[int]string{70: strings.Repeat("a", 100)}}
	if _, err := s.forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim); err != nil {
		t.Fatalf("a trace at the read bound must read: %v", err)
	}
	s.traces[70] = strings.Repeat("a", 101)
	parts, err := s.forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim)
	wantCouldNotCheck(t, err, parts, `"build"`, "exceeds 100 bytes")
}

// TestRunLogGitlabTailBound: the text kept per trace is the cap's worth of its END.
func TestRunLogGitlabTailBound(t *testing.T) {
	lim := lowBounds(func(b *runLogBounds) { b.partCap = 8 })
	s := &glRunLogStub{jobs: []map[string]any{{"id": 70, "name": "build", "status": "failed"}},
		traces: map[int]string{70: strings.Repeat("a", 50) + "FAILURE!"}}
	parts, err := s.forge(t).runLog(rlRepo, RunRef{ID: "5"}, lim)
	if err != nil || len(parts) != 1 || parts[0].Text != "FAILURE!" || !parts[0].Truncated {
		t.Fatalf("want the last 8 bytes flagged truncated, got %+v err %v", parts, err)
	}
}
