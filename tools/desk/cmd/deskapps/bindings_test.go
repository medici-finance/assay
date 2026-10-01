package main

import (
	"strings"
	"testing"
)

// TestBindingsWritten — Verify row 9. The team tier's bindings write REVIEWER_APP and
// WORKER_APP (among every role) to `<prefix>-act`, and READ_APP to `<prefix>-read`.
func TestBindingsWritten(t *testing.T) {
	setupTest(t)

	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range specs {
		if err := writeBindings(spec); err != nil {
			t.Fatalf("writeBindings(%s): %v", spec.Name, err)
		}
	}

	content := readAppsEnv()
	t.Logf("apps.env after team-tier bindings:\n%s", content)

	for _, want := range []string{
		"REVIEWER_APP=example-act",
		"WORKER_APP=example-act",
		"VERIFIER_APP=example-act",
		"DESK_APP=example-act",
		"ISSUE_LOOP_APP=example-act",
		"INTAKE_LOOP_APP=example-act",
		"READ_APP=example-read",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("apps.env missing %q:\n%s", want, content)
		}
	}
}

// TestBindingsWrittenFamily pins the family-tier per-role bindings: each role's App is its
// own `<prefix>-<role>-app`, and there is no READ_APP line (family has no read/act split).
func TestBindingsWrittenFamily(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("family", "assay")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range specs {
		if err := writeBindings(spec); err != nil {
			t.Fatal(err)
		}
	}
	content := readAppsEnv()
	for _, want := range []string{
		"REVIEWER_APP=assay-reviewer-app",
		"WORKER_APP=assay-worker-app",
		"VERIFIER_APP=assay-verifier-app",
		"DESK_APP=assay-desk-app",
		"ISSUE_LOOP_APP=assay-issue-loop-app",
		"INTAKE_LOOP_APP=assay-intake-loop-app",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("apps.env missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "READ_APP=") {
		t.Fatalf("family tier must not write a READ_APP line:\n%s", content)
	}
}

// TestMergeAppsEnvPreservesUnrelatedKeys — a fresh write never clobbers a key it did not
// come to update (a hand-edited or prior-run line survives).
func TestMergeAppsEnvPreservesUnrelatedKeys(t *testing.T) {
	setupTest(t)
	if err := mergeAppsEnv(map[string]string{"SOME_OTHER_APP_ID": "12345"}); err != nil {
		t.Fatal(err)
	}
	if err := writeBindings(AppSpec{Name: "example-act", Roles: []string{"reviewer"}}); err != nil {
		t.Fatal(err)
	}
	content := readAppsEnv()
	if !strings.Contains(content, "SOME_OTHER_APP_ID=12345") {
		t.Fatalf("unrelated key was dropped:\n%s", content)
	}
	if !strings.Contains(content, "REVIEWER_APP=example-act") {
		t.Fatalf("new binding missing:\n%s", content)
	}
}
