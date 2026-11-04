package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func authorityRepoText(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("../../../../..", path))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

// The source requirements remain separate caller classes. The actual positive
// below reuses the real comms Ed25519 signer through Controller, not a port/mock.
func TestForgeLocalAuthorityCompatibility(t *testing.T) {
	for path, tokens := range map[string][]string{
		"docs/proposals/non-ci-architecture/21-cell-operator-surface.md":            {"mTLS client certificates for machine callers", "signed tokens for lighter clients"},
		"docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md": {"Local agent / CLI", "Trusted launcher supplies a short-lived signed capability over a protected per-cell channel", "same-UID peers are not distinct role identities", "Never derive role from an HTTP header or agent-chosen environment value"},
		"spec/operator-incarnation-v1.md":                                           {"snapshot or backup restore never presents operator-control state as", "trustworthy record outside the", "rotates both the incarnation identity and the key reference"},
		"spec/forge-local-authority-v1.md":                                          {"Whole-VM rollback and production restoration remain held", "original exclusive", "network none", "56", "monotonic"},
	} {
		text := authorityRepoText(t, path)
		for _, token := range tokens {
			if !strings.Contains(text, token) {
				t.Fatalf("actual compatibility requirement drift in %s: %s", path, token)
			}
		}
	}
	f := newAuthorityFixture(t, nil)
	r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
	if e != nil || r.Caller().ID != "fixture-caller" || f.verifier.CheckRelease(context.Background(), r) != nil {
		t.Fatal("actual selected primitive/producer compatibility failed")
	}
	if Operation != "mirror-query" || Profile != "linux-named-volume-v1" {
		t.Fatal("local operation/profile silently changed")
	}
}

func TestForgeLocalAuthorityCI(t *testing.T) {
	type workflowDoc struct {
		On          map[string]any    `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Runner string            `yaml:"runs-on"`
			If     string            `yaml:"if"`
			Needs  any               `yaml:"needs"`
			Env    map[string]string `yaml:"env"`
			Steps  []struct {
				Uses string            `yaml:"uses"`
				With map[string]any    `yaml:"with"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	var workflow workflowDoc
	raw := authorityRepoText(t, ".github/workflows/ci.yml")
	if yaml.Unmarshal([]byte(raw), &workflow) != nil {
		t.Fatal("actual CI YAML invalid")
	}
	if _, ok := workflow.On["pull_request"]; !ok {
		t.Fatal("authority CI unreachable on PR")
	}
	if _, ok := workflow.On["push"]; !ok {
		t.Fatal("authority CI unreachable on main")
	}
	job, ok := workflow.Jobs["forge-local-authority"]
	if !ok || job.Runner != "ubuntu-24.04" || job.If != "" || workflow.Permissions["contents"] != "read" {
		t.Fatal("qualified hosted fixture job missing/gated/permissions changed")
	}
	var reference workflowDoc
	if yaml.Unmarshal([]byte(authorityRepoText(t, ".github/workflows/assay-workflow.yml")), &reference) != nil {
		t.Fatal("canonical pinned workflow source invalid")
	}
	var pin map[string]string
	for _, step := range reference.Jobs["test"].Steps {
		if step.Env["GO_SHA256"] != "" {
			if pin != nil {
				t.Fatal("ambiguous canonical toolchain pin")
			}
			pin = step.Env
		}
	}
	if pin == nil || pin["GO_VERSION"] == "" || len(pin["GO_SHA256"]) != 64 {
		t.Fatal("canonical toolchain pin absent")
	}
	checkoutSafe, installSafe := false, false
	closureSafe := false
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			checkoutSafe = step.With["persist-credentials"] == false
		}
		if step.Env["GO_SHA256"] != "" {
			checksum, extract := strings.Index(step.Run, "sha256sum -c -"), strings.Index(step.Run, "tar -C")
			installSafe = step.Env["GO_VERSION"] == pin["GO_VERSION"] && step.Env["GO_SHA256"] == pin["GO_SHA256"] && checksum >= 0 && extract > checksum && strings.Contains(step.Run, "set -euo pipefail")
		}
		if strings.Contains(step.Run, "list -mod=readonly -deps -test ./internal/forgeservice/auth ./internal/forgeservice/auth/testfixture") {
			closureSafe = step.Env["GOAUTH"] == "off" && step.Env["GOPROXY"] == "https://proxy.golang.org" && step.Env["GOSUMDB"] == "sum.golang.org"
		}
	}
	if !checkoutSafe {
		t.Fatal("new authority checkout persists credentials")
	}
	if !installSafe {
		t.Fatal("new authority toolchain checksum pin omitted or used after extraction")
	}
	if !closureSafe {
		t.Fatal("new hosted offline test closure not prepared from pinned public dependencies")
	}
	var script string
	for _, step := range job.Steps {
		script += "\n" + step.Run
	}
	for _, token := range []string{"git worktree add --detach", "qualify_fixture.py", "--endpoint", "unix:///var/run/docker.sock", "--platform linux/amd64", "--module-cache", "--go-cache"} {
		if !strings.Contains(script, token) {
			t.Fatalf("actual owned fixture entrypoint missing: %s", token)
		}
	}
	aggregate := workflow.Jobs["build-test"]
	found := false
	if needs, ok := aggregate.Needs.([]any); ok {
		for _, need := range needs {
			found = found || need == "forge-local-authority"
		}
	}
	if !found {
		t.Fatal("required build-test omits authority fixture")
	}
	var guard string
	for _, step := range aggregate.Steps {
		guard += "\n" + step.Run
		for _, value := range step.Env {
			guard += "\n" + value
		}
	}
	if !strings.Contains(guard, "needs.forge-local-authority.result") || !strings.Contains(guard, `"$AUTHORITY" = "success"`) {
		t.Fatal("required gate permits missing/skipped authority qualification")
	}
	runner := authorityRepoText(t, "tools/desk/internal/forgeservice/auth/testdata/qualify_fixture.py")
	for _, token := range []string{"--- PASS: TestForgeLocalAuthorityCustody ", "--- SKIP:", "PASS: local authority production controls killed; exact bytes restored", "--network=none", "--pull=false", "--builder=default", "cleanup-image-absence", "final-head"} {
		if !strings.Contains(runner, token) {
			t.Fatalf("qualification fails open or lacks source/cleanup guard: %s", token)
		}
	}
}
