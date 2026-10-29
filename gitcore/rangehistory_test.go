package gitcore

import (
	"github.com/medici-finance/assay/gitcore/gittest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCommitRangeParity(t *testing.T) {
	f := gittest.NewFixture(t)
	base := mustGitOutput(t, f, "rev-parse", "HEAD")
	f.CommitFile(t, "main.txt", "main\n", "main change")
	f.Git("branch", "side", base)
	f.Git("checkout", "side")
	f.CommitFile(t, "side.txt", "side\n", "fix #72\n\nbody")
	f.Git("checkout", "-")
	if _, err := f.Git("merge", "--no-ff", "side", "-m", "landing"); err != nil {
		t.Fatal(err)
	}
	tip := mustGitOutput(t, f, "rev-parse", "HEAD")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{base, tip}, {tip, base}, {tip, tip}, {base, base}} {
		rows, err := r.CommitRange(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, row := range rows {
			got = append(got, row.ID)
			msg := mustGitOutput(t, f, "show", "-s", "--format=%B", row.ID)
			if strings.TrimSpace(row.Message) != msg {
				t.Fatalf("message %s=%q want %q", row.ID, row.Message, msg)
			}
			parents := strings.Fields(mustGitOutput(t, f, "show", "-s", "--format=%P", row.ID))
			if !reflect.DeepEqual(row.Parents, parents) && len(row.Parents)+len(parents) != 0 {
				t.Fatalf("parents %v want %v", row.Parents, parents)
			}
		}
		wantText, err := f.Git("rev-list", pair[0]+".."+pair[1])
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Fields(wantText)
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("range %s..%s=%v want %v", pair[0], pair[1], got, want)
		}
	}
}
func TestCommitRangeRefusesNamesAndMissingObjects(t *testing.T) {
	f := gittest.NewFixture(t)
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	id := mustGitOutput(t, f, "rev-parse", "HEAD")
	for _, pair := range [][2]string{{"HEAD", id}, {id, "HEAD"}, {strings.Repeat("1", 40), id}, {id, strings.Repeat("1", 40)}} {
		if rows, err := r.CommitRange(pair[0], pair[1]); err == nil || rows != nil {
			t.Fatalf("uncertain range became success: %v %v", rows, err)
		}
	}
}

func TestCommitRangeCacheCopiesAndGap(t *testing.T) {
	f := gittest.NewFixture(t)
	base := mustGitOutput(t, f, "rev-parse", "HEAD")
	f.CommitFile(t, "range.txt", "change\n", "range change")
	tip := mustGitOutput(t, f, "rev-parse", "HEAD")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.CommitRange(base, tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(first[0].Parents) != 1 {
		t.Fatal(first)
	}
	first[0].Parents[0] = "corrupted"
	again, err := r.CommitRange(base, tip)
	if err != nil || again[0].Parents[0] != base {
		t.Fatalf("caller mutated shared history: %v %v", again, err)
	}
	again[0].Parents[0] = "second corruption"
	third, err := r.CommitRange(base, tip)
	if err != nil || third[0].Parents[0] != base {
		t.Fatalf("cache hit leaked shared history: %v %v", third, err)
	}
	if err := os.Remove(filepath.Join(f.Dir, ".git", "objects", base[:2], base[2:])); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fresh.CommitRange(tip, tip); err == nil || got != nil {
		t.Fatalf("missing excluded parent became success: %v %v", got, err)
	}
}
func TestCommitRangeAbbreviationNamespaces(t *testing.T) {
	f := gittest.NewFixture(t)
	tip := mustGitOutput(t, f, "rev-parse", "HEAD")
	short := tip[:7]
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if why, err := r.NativeAbbreviationReason(short); err != nil || why != "" {
		t.Fatalf("ordinary abbreviation %q %v", why, err)
	}
	for _, namespace := range []string{"refs/heads/", "refs/tags/", "refs/remotes/", "refs/"} {
		name := namespace + short
		if _, err := f.Git("update-ref", name, tip); err != nil {
			t.Fatal(err)
		}
		if why, err := r.NativeAbbreviationReason(short); err != nil || why == "" {
			t.Fatalf("alias %s lost: %q %v", name, why, err)
		}
		if _, err := f.Git("update-ref", "-d", name); err != nil {
			t.Fatal(err)
		}
	}
}
