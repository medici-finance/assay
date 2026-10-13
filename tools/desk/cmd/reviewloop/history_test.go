package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func planFixture(t *testing.T, count int) []string {
	t.Helper()
	var actions, prs []string
	for i := 0; i < count; i++ {
		action := "NEEDS-REVIEW"
		if i%2 == 1 {
			action = "FLIP"
		}
		actions = append(actions, fmt.Sprintf(`{"repo":"example-org/example-repo","number":%d,"action":%q}`, 42+i, action))
		prs = append(prs, fmt.Sprintf(`{"repo":"example-org/example-repo","number":%d,"headSHA":"aaaaaaaaaaaabbbbbbbbbbbb"}`, 42+i))
	}
	a := fmt.Sprintf(`{"asOf":%q,"scope":{"repos":["example-org/example-repo"],"count":1},"prPopulation":{"complete":true},"rows":[%s]}`, fixedNow, strings.Join(actions, ","))
	return []string{"--actions", writeTemp(t, "a.json", a), "--prs", writeTemp(t, "p.json", `{"prs":[`+strings.Join(prs, ",")+`]}`), "--now", fixedNow}
}

func historyDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir, err := deskkit.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// regression: #2793
func TestPlanHistoryIndependent(t *testing.T) {
	args := planFixture(t, 4)
	var want strings.Builder
	historyDir(t)
	if err := cmdPlan(args, &want); err != nil {
		t.Fatal(err)
	}
	for _, result := range []string{"ok", "noop", "dryrun", "refused", "unverifiable"} {
		for _, verb := range []string{"review", "review:correctness:approve", "review:security:approve", "review:correctness:request-changes", "ready"} {
			t.Run(result+"/"+verb, func(t *testing.T) {
				dir := historyDir(t)
				var data []byte
				for n := 42; n < 46; n++ {
					head := "aaaaaaaaaaaabbbbbbbbbbbb"
					b, err := json.Marshal(deskkit.Entry{Tool: "deskpost", Repo: "example-org/example-repo", PR: &n, HeadSHA: &head, Verb: verb, Result: result, BodyDigest: "different-review-body"})
					if err != nil {
						t.Fatal(err)
					}
					data = append(data, append(b, '\n')...)
				}
				if err := os.WriteFile(filepath.Join(dir, "audit.jsonl.2026-08-12"), data, 0600); err != nil {
					t.Fatal(err)
				}
				entries, err := deskkit.LoadEntries()
				if err != nil || len(entries) != 4 {
					t.Fatalf("invalid producer-shaped fixture: entries=%d err=%v", len(entries), err)
				}
				for _, dry := range []bool{false, true} {
					call := append([]string(nil), args...)
					if dry {
						call = append(call, "--dry-run")
					}
					var got strings.Builder
					if err := cmdPlan(call, &got); err != nil {
						t.Fatal(err)
					}
					if got.String() != want.String() {
						t.Fatalf("historical %s %s changed the board plan (dry-run=%v)", verb, result, dry)
					}
				}
			})
		}
	}
	for _, mode := range []string{"missing", "corrupt", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			dir := historyDir(t)
			path := filepath.Join(dir, "audit.jsonl")
			switch mode {
			case "corrupt":
				if err := os.WriteFile(path, []byte("{broken\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			var got strings.Builder
			if err := cmdPlan(args, &got); err != nil {
				t.Fatal(err)
			}
			if got.String() != want.String() {
				t.Fatalf("%s history changed the board plan", mode)
			}
		})
	}
}
