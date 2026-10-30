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
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte("#!/bin/sh\nexit 93\n"), 0755); err != nil {
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
	for _, key := range []string{"i18n.logOutputEncoding", "i18n.commitEncoding"} {
		t.Run(key, func(t *testing.T) {
			root, _ := staleHistoryFixture(t)
			runGit(t, root, "config", key, "UTF-8")
			real, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			shim := t.TempDir()
			trace := filepath.Join(shim, "calls")
			script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\n' \"$*\" >> %s\nexec %s \"$@\"\n", strconv.Quote(trace), strconv.Quote(real))
			if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
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
