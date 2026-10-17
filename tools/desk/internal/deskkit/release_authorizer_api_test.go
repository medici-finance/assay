package deskkit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Execute the real resolve shell with a curl replacement, not a second copy of
// its predicates. GitHub's commit-PR list omits merged_by; the detail supplies it.
func TestReleaseTagPushAuthorizerAPI(t *testing.T) {
	for _, workflow := range []string{
		filepath.Join("..", "..", "..", "..", ".github", "workflows", "release.yml"),
		stagedWorkflowPath("release.yml"),
	} {
		t.Run(workflow, func(t *testing.T) {
			skipIfFixtureAbsent(t, workflow, "release workflow is not published")
			raw, err := os.ReadFile(workflow)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Jobs map[string]struct{ Steps []struct{ ID, Run string } }
			}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			var script string
			for _, step := range doc.Jobs["resolve"].Steps {
				if step.ID == "tag" {
					script = step.Run
				}
			}
			if script == "" {
				t.Fatal("resolve tag shell absent")
			}
			valid := map[string]any{"number": 123, "merged_at": "2026-10-16T00:00:00Z", "merge_commit_sha": "test-merge-sha", "title": "release: v1.2.3", "body": "Release notes\nRELEASE: v1.2.3\n", "merged_by": map[string]any{"login": "release-driver"}}
			listPR := make(map[string]any)
			for k, v := range valid {
				if k != "merged_by" {
					listPR[k] = v
				}
			}
			for _, tc := range []struct {
				name, status, raw string
				mutate            func(map[string]any)
				pass              bool
			}{
				{name: "list_without_merged_by_detail_with_authorizer", pass: true},
				{name: "detail_HTTP_failure", status: "503"},
				{name: "detail_malformed_JSON", raw: "{"},
				{name: "detail_wrong_shape", raw: "[]"},
				{name: "detail_unmerged", mutate: func(p map[string]any) { p["merged_at"] = nil }},
				{name: "detail_wrong_number", mutate: func(p map[string]any) { p["number"] = 124 }},
				{name: "detail_wrong_SHA", mutate: func(p map[string]any) { p["merge_commit_sha"] = "other-merge-sha" }},
				{name: "detail_wrong_title", mutate: func(p map[string]any) { p["title"] = "release: v1.2.4" }},
				{name: "detail_padded_title", mutate: func(p map[string]any) { p["title"] = " release: v1.2.3 " }},
				{name: "detail_wrong_body", mutate: func(p map[string]any) { p["body"] = "RELEASE: v1.2.4" }},
				{name: "detail_missing_merger", mutate: func(p map[string]any) { delete(p, "merged_by") }},
				{name: "detail_empty_login", mutate: func(p map[string]any) { p["merged_by"] = map[string]any{"login": ""} }},
				{name: "detail_newline_login", mutate: func(p map[string]any) { p["merged_by"] = map[string]any{"login": "driver\nauthorizer=other"} }},
				{name: "detail_trailing_newline_login", mutate: func(p map[string]any) { p["merged_by"] = map[string]any{"login": "driver\n"} }},
				{name: "detail_carriage_return_login", mutate: func(p map[string]any) { p["merged_by"] = map[string]any{"login": "driver\r"} }},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					detail := make(map[string]any)
					for k, v := range valid {
						detail[k] = v
					}
					if tc.mutate != nil {
						tc.mutate(detail)
					}
					write := func(name string, data []byte, mode os.FileMode) {
						t.Helper()
						if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
							t.Fatal(err)
						}
					}
					listJSON, _ := json.Marshal([]any{listPR})
					detailJSON, _ := json.Marshal(detail)
					if tc.raw != "" {
						detailJSON = []byte(tc.raw)
					}
					write("list.json", listJSON, 0600)
					write("detail.json", detailJSON, 0600)
					write("resolve.sh", []byte(script), 0600)
					write("curl", []byte(`#!/usr/bin/env python3
import os, pathlib, sys
args = sys.argv[1:]
out = args[args.index("-o") + 1]
url = args[-1]
root = pathlib.Path(os.environ["FIXTURE_DIR"])
with (root / "requests").open("a") as log:
    log.write(url + "\n")
if url.endswith("/commits/test-merge-sha/pulls"):
    name, status = "list.json", "200"
elif url.endswith("/pulls/123"):
    name, status = "detail.json", os.environ.get("DETAIL_STATUS", "200")
else:
    sys.exit("unexpected API URL: " + url)
pathlib.Path(out).write_bytes((root / name).read_bytes())
print(status, end="")
`), 0700)
					status := tc.status
					if status == "" {
						status = "200"
					}
					cmd := exec.Command("bash", filepath.Join(dir, "resolve.sh"))
					cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_DIR="+dir, "DETAIL_STATUS="+status, "RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+filepath.Join(dir, "outputs"), "EVENT=push", "REF_NAME=v1.2.3", "SHA=test-merge-sha", "REPO=example/project", "GH_TOKEN=offline-fixture", "KUBECONFIG=/dev/null")
					out, runErr := cmd.CombinedOutput()
					outputs, _ := os.ReadFile(filepath.Join(dir, "outputs"))
					if tc.pass {
						if runErr != nil {
							t.Fatalf("valid detail rejected: %v\n%s", runErr, out)
						}
						if !strings.Contains(string(outputs), "authorizer=release-driver (merged release PR #123)\n") {
							t.Fatalf("missing detail authorizer: %s", outputs)
						}
						calls, _ := os.ReadFile(filepath.Join(dir, "requests"))
						if strings.Count(string(calls), "\n") != 2 {
							t.Fatalf("expected list and detail lookup, got %s", calls)
						}
					} else {
						if runErr == nil {
							t.Fatalf("invalid detail accepted: %s\n%s", outputs, out)
						}
						if strings.Contains(string(outputs), "authorizer=") || strings.Contains(string(outputs), "tag=") {
							t.Fatalf("refusal emitted release authority: %s", outputs)
						}
					}
				})
			}
		})
	}
}
