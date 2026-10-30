package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func coreMetadataReferenceParity(t *testing.T) {
	root, _ := staleHistoryFixture(t)
	runGit(t, root, "add", ".")
	if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte("board\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "STATUS.md")
	runGit(t, root, "commit", "-qm", "board")
	wantHeartbeat := parityGitBytes(t, root, "log", "-1", "--format=%h %cI", "--", "STATUS.md")
	gotHeartbeat, err := gitLastPathHeartbeatOutput(root)
	if err != nil || !bytes.Equal(gotHeartbeat, wantHeartbeat) {
		t.Fatalf("heartbeat %q want %q: %v", gotHeartbeat, wantHeartbeat, err)
	}
	// A zero committer offset renders by Git version, never by host time zone.
	for _, stamp := range []string{"2026-08-01T00:00:00+0000", "2026-08-02T00:00:00+0530"} {
		t.Setenv("GIT_COMMITTER_DATE", stamp)
		if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte(stamp+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "commit", "-qam", "board "+stamp)
		want := parityGitBytes(t, root, "log", "-1", "--format=%h %cI", "--", "STATUS.md")
		got, err := gitLastPathHeartbeatOutput(root)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("committer offset %s: %q want %q: %v", stamp, got, want, err)
		}
	}
	for _, width := range []string{"12", "no", "0", "auto"} {
		runGit(t, root, "config", "core.abbrev", width)
		want, wantErr := exec.Command("git", "-C", root, "log", "-1", "--format=%h %cI", "--", "STATUS.md").Output()
		got, err := gitLastPathHeartbeatOutput(root)
		if (err == nil) != (wantErr == nil) || !bytes.Equal(got, want) {
			t.Fatalf("configured abbreviation %s: %q want %q: %v", width, got, want, err)
		}
	}
	for _, rel := range []string{"seed.txt", ".", "absent"} {
		want := parityGitBytes(t, root, "log", "-1", "--format=%ct", "--", rel)
		got, err := gitLastPathTimeOutput(root, rel)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s: %q want %q: %v", rel, got, want, err)
		}
	}
	runGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/heads/main")
	want := parityGitBytes(t, root, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
	native := func(args ...string) ([]byte, error) {
		t.Fatalf("ordinary snapshot started native: %v", args)
		return nil, nil
	}
	got, err := staleFailReferenceOutput(root, native)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("snapshot: %q want %q: %v", got, want, err)
	}
	// Actual upper calls cannot quietly retain their former Git process.
	if err := os.Chtimes(root, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	expected, err := gitLastPathTimeOutput(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(expected)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	// Only the effective-configuration observation may still start Git.
	script := fmt.Sprintf("#!/bin/sh\n[ \"$3\" = config ] && exec %s \"$@\"\nexit 93\n", strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+filepath.Dir(realGit))
	if got := gitLastTouch(root, "."); got.Unix() != sec {
		t.Fatalf("upper last-touch still uses native/mtime: %v want %d", got, sec)
	}
	if got, err := gitLastPathTimeOutput(root, "."); err != nil || len(got) == 0 {
		t.Fatalf("metadata still launched native: %q %v", got, err)
	}
	if got, err := staleFailReferenceOutput(root, native); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("snapshot still launched native: %q %v", got, err)
	}
}

func coreStaleFailMessageParity(t *testing.T) {
	root, tip := staleHistoryFixture(t)
	tree := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", tip+"^{tree}")))
	base := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", tip+"^1")))
	// Explicit criss-cross parents: two merges share both incomparable bases.
	commit := func(message string, parents ...string) string {
		args := []string{"commit-tree", tree, "-m", message}
		for _, p := range parents {
			args = append(args, "-p", p)
		}
		return strings.TrimSpace(string(parityGitBytes(t, root, args...)))
	}
	a := commit("left\n\nfix #74", base)
	b := commit("right\n\nfix #75", base)
	x := commit("cross one", a, b)
	y := commit("cross two", b, a)
	z := commit("land cross", x, y)
	for _, pair := range [][2]string{{base, tip}, {x, z}, {a, y}, {tip, tip}} {
		rows, err := ordinaryGitCore(root).repo.CommitRange(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		want := []string{}
		for _, row := range rows {
			got = append(got, row.ID+"\x00"+row.Message+"\n")
		}
		ids := strings.Fields(string(parityGitBytes(t, root, "log", pair[0]+".."+pair[1], "--format=%H")))
		for _, id := range ids {
			want = append(want, id+"\x00"+string(parityGitBytes(t, root, "show", "-s", "--format=%B", id)))
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("message multiset %v: got %q want %q", pair, got, want)
		}
	}
}

func TestStaleFailNativeEncodingKeys(t *testing.T) {
	for key, value := range map[string]string{"i18n.logOutputEncoding": "UTF-8", "i18n.commitEncoding": "UTF-8", "log.showSignature": "true"} {
		t.Run(key, func(t *testing.T) {
			root, _ := staleHistoryFixture(t)
			runGit(t, root, "config", key, value)
			trace := gitTraceShim(t)
			end := beginGitReadSession()
			defer end()
			_, ok := newestFailFix(root, &BriefFile{Issues: []int{72}}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
			data, err := os.ReadFile(trace)
			if !ok || err != nil || !bytes.Contains(data, []byte("..")) {
				t.Fatalf("%s did not retain native range: %v %v %s", key, ok, err, data)
			}
		})
	}
}

func TestReferenceSnapshotHistoryIndependent(t *testing.T) {
	for _, profile := range []string{"replace", "graft", "both"} {
		t.Run(profile, func(t *testing.T) {
			root, tip := staleHistoryFixture(t)
			base := strings.TrimSpace(string(parityGitBytes(t, root, "rev-parse", tip+"^1")))
			if profile != "graft" {
				runGit(t, root, "update-ref", "refs/replace/"+tip, base)
			}
			if profile != "replace" {
				if err := os.WriteFile(filepath.Join(root, ".git", "info", "grafts"), []byte(tip+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want := parityGitBytes(t, root, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
			calls := 0
			native := func(args ...string) ([]byte, error) {
				calls++
				return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
			}
			end := beginGitReadSession()
			defer end()
			got, err := staleFailReferenceOutput(root, native)
			if err != nil || calls != 0 || !bytes.Equal(got, want) {
				t.Fatalf("reference-only profile %s changed bytes or started native: calls=%d %q want %q %v", profile, calls, got, want, err)
			}
		})
	}
}

// lastPathFollowFixture: a side branch changes STATUS.md and is merged with
// -s ours, so default simplification and follow mode select different commits.
func lastPathFollowFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "checkout", "-qb", "main")
	runGit(t, root, "config", "core.abbrev", "7")
	at := func(sec string) []string {
		return []string{"GIT_AUTHOR_DATE=@" + sec + " +0000", "GIT_COMMITTER_DATE=@" + sec + " +0000"}
	}
	mustWrite(t, filepath.Join(root, "STATUS.md"), "A\n")
	runGit(t, root, "add", "STATUS.md")
	runGitEnv(t, root, at("1700000000"), "commit", "-qm", "c1")
	runGit(t, root, "checkout", "-qb", "side")
	mustWrite(t, filepath.Join(root, "STATUS.md"), "B\n")
	runGitEnv(t, root, at("1700000200"), "commit", "-qam", "s1")
	runGit(t, root, "checkout", "-q", "main")
	mustWrite(t, filepath.Join(root, "o.txt"), "o\n")
	runGit(t, root, "add", "o.txt")
	runGitEnv(t, root, at("1700000100"), "commit", "-qm", "m1")
	runGitEnv(t, root, at("1700000300"), "merge", "-q", "-s", "ours", "-m", "ours", "side")
	return root
}

// regression: #4846 F4, log.follow selected another commit
func coreLastPathFollowParity(t *testing.T) {
	root := lastPathFollowFixture(t)
	global := filepath.Join(t.TempDir(), "global")
	seen := map[string]bool{}
	// Global scope and a valueless (implicit true) boolean are both effective.
	for _, cfg := range []string{"", "[log]\n\tfollow\n", "[log]\n\tfollow = false\n", "[log]\n\tfollow = true\n"} {
		if err := os.WriteFile(global, []byte(cfg), 0644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_CONFIG_GLOBAL", global)
		want := parityGitBytes(t, root, "log", "-1", "--format=%ct", "--", "STATUS.md")
		seen[string(want)] = true
		got, err := gitLastPathTimeOutput(root, "STATUS.md")
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("config %q: last-path time %q want %q: %v", cfg, got, want, err)
		}
		want = parityGitBytes(t, root, "log", "-1", "--format=%h %cI", "--", "STATUS.md")
		got, err = gitLastPathHeartbeatOutput(root)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("config %q: heartbeat %q want %q: %v", cfg, got, want, err)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("fixture does not separate follow from default simplification: %v", seen)
	}
}

// regression: #4846 F4/A2, config and pathspec env profiles
func TestLastPathNativeProfiles(t *testing.T) {
	cases := []struct{ name, key, value, env string }{
		{"follow-local", "log.follow", "true", ""},
		{"show-signature", "log.showSignature", "true", ""},
		{"log-output-encoding", "i18n.logOutputEncoding", "UTF-8", ""},
		{"commit-encoding", "i18n.commitEncoding", "UTF-8", ""},
		{"icase-pathspecs", "", "", "GIT_ICASE_PATHSPECS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := lastPathFollowFixture(t)
			if tc.key != "" {
				runGit(t, root, "config", tc.key, tc.value)
			}
			if tc.env != "" {
				t.Setenv(tc.env, "1")
			}
			trace := gitTraceShim(t)
			wantTime := parityGitBytes(t, root, "log", "-1", "--format=%ct", "--", "STATUS.md")
			wantBeat := parityGitBytes(t, root, "log", "-1", "--format=%h %cI", "--", "STATUS.md")
			if err := os.Remove(trace); err != nil {
				t.Fatal(err)
			}
			gotTime, err := gitLastPathTimeOutput(root, "STATUS.md")
			if err != nil || !bytes.Equal(gotTime, wantTime) {
				t.Fatalf("time %q want %q: %v", gotTime, wantTime, err)
			}
			gotBeat, err := gitLastPathHeartbeatOutput(root)
			if err != nil || !bytes.Equal(gotBeat, wantBeat) {
				t.Fatalf("heartbeat %q want %q: %v", gotBeat, wantBeat, err)
			}
			data, _ := os.ReadFile(trace)
			for _, argv := range []string{"log -1 --format=%ct -- STATUS.md", "log -1 --format=%h %cI -- STATUS.md"} {
				if !bytes.Contains(data, []byte(argv)) {
					t.Fatalf("native %q not retained:\n%s", argv, data)
				}
			}
		})
	}
}

// The effective-configuration observation is made once per root in a run.
func TestLastPathConfigObservedOnce(t *testing.T) {
	root := lastPathFollowFixture(t)
	trace := gitTraceShim(t)
	end := beginGitReadSession()
	defer end()
	for i := 0; i < 3; i++ {
		if got, err := gitLastPathTimeOutput(root, "STATUS.md"); err != nil || string(got) != "1700000000\n" {
			t.Fatalf("time %q: %v", got, err)
		}
	}
	data, _ := os.ReadFile(trace)
	if n := bytes.Count(data, []byte(" config ")); n != 1 || bytes.Contains(data, []byte(" log ")) {
		t.Fatalf("want one config observation and no native log, got %d:\n%s", n, data)
	}
}

// gitTraceShim puts a recording git first on PATH and returns its trace file.
func gitTraceShim(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	trace := filepath.Join(shim, "calls")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %s\nexec %s \"$@\"\n", strconv.Quote(trace), strconv.Quote(real))
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	return trace
}
