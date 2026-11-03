package schema

import (
	"strings"
	"testing"
)

const validDoc = `
cells:
  - name: assay
    repos:
      - example-org/toolkit
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat, glm-5.2]
        app: assay-reviewer-app
      - role: metrics
        schedule: "0 * * * *"
        models: []
        app: assay-desk-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`

func TestParse_Valid(t *testing.T) {
	f, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Cells) != 1 || f.Cells[0].Name != "assay" {
		t.Fatalf("unexpected result: %+v", f)
	}
}

func TestParse_UnknownField(t *testing.T) {
	doc := validDoc + "    unknown_field: nope\n"
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unknown field, got nil")
	}
}

func TestParse_MissingModelsForAIRole(t *testing.T) {
	doc := `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: []
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for empty model ladder on a non-metrics role")
	}
}

func TestParse_UnknownRole(t *testing.T) {
	doc := `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: inbound
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-desk-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unrecognised loop role")
	}
}

func TestParse_IncompleteTriad(t *testing.T) {
	doc := `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an incomplete notification triad")
	}
}

func TestParse_BadSilenceThreshold(t *testing.T) {
	doc := `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-reviewer-app
    silence_threshold: not-a-duration
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unparseable silence_threshold")
	}
}

func TestParse_NoCells(t *testing.T) {
	if _, err := Parse([]byte("cells: []\n")); err == nil {
		t.Fatal("expected an error for zero cells")
	}
}

// ── environments: dev-only by construction (the service contract) ────────────
//
// Every test below is named *Environment* so `go test ./config/... -run
// 'Environment' -count=1` selects exactly this block.
//
// The negative cases are the point of the file. A validator is only worth the
// bytes if it REFUSES, so each rule gets a case that would pass without it,
// and the fixture files under ../../testdata/ carry the same four cases in
// the form the CLI rows (Verify rows 3-5) exercise.

// envDoc renders a cells.yaml document with one environment block spliced in,
// so a case can change exactly one field and nothing else. Keeping the
// surrounding cell constant is what makes a failure attributable.
func envDoc(env string) string {
	return `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
    environments:
` + env
}

const validEnvBlock = `      - name: tracker-dev
        kind: k8s
        tier: dev
        cluster: fixture-cluster
        namespaces: [example-dev-app, example-dev-canton]
        access: [read]
        admin_pod: {namespace: example-dev-app, workload: deploy/example-admin}
        runner_pool: example-verify-tracker
`

func TestEnvironment_Valid(t *testing.T) {
	f, err := Parse([]byte(envDoc(validEnvBlock)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	envs := f.Cells[0].Environments
	if len(envs) != 1 {
		t.Fatalf("expected 1 environment, got %d", len(envs))
	}
	e := envs[0]
	if e.Tier != EnvTierDev {
		t.Errorf("tier: got %q want %q", e.Tier, EnvTierDev)
	}
	if len(e.Namespaces) != 2 {
		t.Errorf("namespaces: got %v want exactly the two tracker dev namespaces", e.Namespaces)
	}
	if len(e.Access) != 1 || e.Access[0] != AccessRead {
		t.Errorf("access: got %v want [read] — exec-admin-pod is a separately signed-off phase 2", e.Access)
	}
	if e.AdminPod == nil || e.AdminPod.Namespace != "example-dev-app" {
		t.Errorf("admin_pod: got %+v want namespace example-dev-app", e.AdminPod)
	}
}

// A cell with no environments at all stays valid: absence means "this cell
// has no verify pool, its cluster rows stay could-not-check", which is the
// state every cell but `assay` is in. If this test ever fails, someone has
// made `environments:` required and silently forced every future cell to
// declare cluster reach it does not want.
func TestEnvironment_OptionalWhenAbsent(t *testing.T) {
	f, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Cells[0].Environments) != 0 {
		t.Fatalf("expected no environments, got %+v", f.Cells[0].Environments)
	}
}

func TestEnvironment_RejectsProdNamespace(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-app, example-prod-app]
        access: [read]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "example-prod-app")
}

// The positive `-dev` rule on its own: a namespace with no dev segment is not
// a dev namespace, even though it names nothing forbidden either.
func TestEnvironment_RejectsNamespaceWithoutDevSegment(t *testing.T) {
	block := `      - name: loops
        kind: k8s
        tier: dev
        namespaces: [example-loops]
        access: [read]
        runner_pool: example-verify-loops
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "example-loops")
}

// The denylist earning its keep: this namespace SATISFIES the positive
// `-dev` rule and is still refused. Without the second rule it would be
// declarable, which is the whole reason both are applied.
func TestEnvironment_RejectsDevPrefixedProdNamespace(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-prod-mirror]
        access: [read]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "example-dev-prod-mirror")
}

func TestEnvironment_RejectsProdTier(t *testing.T) {
	block := `      - name: tracker-prod
        kind: k8s
        tier: prod
        namespaces: [example-dev-app]
        access: [read]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "tier")
}

func TestEnvironment_RejectsUngrantableAccessLevel(t *testing.T) {
	for _, level := range []string{"cluster-admin", "write", "admin", "secrets"} {
		block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-app]
        access: [read, ` + level + `]
        runner_pool: example-verify-tracker
`
		err := parseErr(t, envDoc(block))
		assertMentions(t, err, "access")
		assertMentions(t, err, level)
	}
}

// An entry scoped by cluster alone. RBAC binds namespaces, and dev/prod are
// sibling namespaces in ONE cluster, so an empty namespace list must fail
// closed rather than read as "unrestricted".
func TestEnvironment_RejectsBareContextWithNoNamespaces(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        cluster: fixture-cluster
        namespaces: []
        access: [read]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "namespaces")
}

func TestEnvironment_RejectsAdminPodOutsideDeclaredNamespaces(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-app]
        access: [read, exec-admin-pod]
        admin_pod: {namespace: example-dev-canton, workload: deploy/example-admin}
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "admin_pod")
}

func TestEnvironment_RejectsUnsupportedKind(t *testing.T) {
	block := `      - name: tracker-dev
        kind: vm
        tier: dev
        namespaces: [example-dev-app]
        access: [read]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "kind")
}

func TestEnvironment_RequiresRunnerPool(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-app]
        access: [read]
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "runner_pool")
}

// Two cells may not share a verify pool: one pool is one identity, and the
// pool's RBAC would then be the union of both cells' declarations with
// nothing in the config recording that.
func TestEnvironment_RejectsSharedRunnerPoolAcrossCells(t *testing.T) {
	doc := `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops: [{role: review, schedule: "*/10 * * * *", models: [deepseek-chat], app: assay-reviewer-app}]
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
    environments:
      - {name: tracker-dev, kind: k8s, tier: dev, namespaces: [example-dev-app], access: [read], runner_pool: example-verify-tracker}
  - name: other
    repos: [example-org/other]
    streams_root: docs/streams
    loops: [{role: review, schedule: "*/10 * * * *", models: [deepseek-chat], app: assay-reviewer-app}]
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
    environments:
      - {name: other-dev, kind: k8s, tier: dev, namespaces: [other-dev-app], access: [read], runner_pool: example-verify-tracker}
`
	err := parseErr(t, doc)
	assertMentions(t, err, "runner_pool")
}

// ── the canton exec gate (the operator namespace isolation policy, the authorization contract) ──

// The lazy way to write the ruling in — append one word to the existing
// entry's access list — must be refused, because that entry also spans the
// canton namespace.
func TestEnvironment_RejectsExecOnCantonNamespace(t *testing.T) {
	block := `      - name: tracker-dev
        kind: k8s
        tier: dev
        namespaces: [example-dev-app, example-dev-canton]
        access: [read, exec-admin-pod]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "exec-admin-pod")
	assertMentions(t, err, "example-dev-canton")
	assertMentions(t, err, "namespace isolation")
}

// The gate is on the NAMESPACE, not on the environment's name — renaming the
// entry must not buy exec into a canton namespace.
func TestEnvironment_RejectsExecOnCantonEvenWhenAloneInTheEntry(t *testing.T) {
	block := `      - name: tracker-dev-canton
        kind: k8s
        tier: dev
        namespaces: [example-dev-canton]
        access: [read, exec-admin-pod]
        runner_pool: example-verify-tracker
`
	err := parseErr(t, envDoc(block))
	assertMentions(t, err, "example-dev-canton")
}

// Exec on a non-canton dev namespace is exactly what the ruling grants, so it
// must be ACCEPTED. Without this case the rule above could be over-broad
// (refusing all exec everywhere) and still look correct.
func TestEnvironment_AcceptsExecOnDevAppNamespace(t *testing.T) {
	block := `      - name: tracker-dev-app
        kind: k8s
        tier: dev
        namespaces: [example-dev-app]
        access: [read, exec-admin-pod]
        admin_pod: {namespace: example-dev-app, workload: deploy/example-admin}
        runner_pool: example-verify-tracker
`
	f, err := Parse([]byte(envDoc(block)))
	if err != nil {
		t.Fatalf("exec on a dev app namespace is the granted case, got: %v", err)
	}
	if got := f.Cells[0].Environments[0].Access; len(got) != 2 {
		t.Errorf("access = %v, want [read exec-admin-pod]", got)
	}
}

// One cell, two environments, ONE identity: the shape the ruling produces.
// Before the ruling this was rejected, because runner_pool uniqueness was
// enforced globally rather than across cells.
func TestEnvironment_AcceptsOnePoolAcrossTwoEnvironmentsInOneCell(t *testing.T) {
	block := `      - name: tracker-dev-app
        kind: k8s
        tier: dev
        namespaces: [example-dev-app]
        access: [read, exec-admin-pod]
        runner_pool: example-verify-tracker
      - name: tracker-dev-canton
        kind: k8s
        tier: dev
        namespaces: [example-dev-canton]
        access: [read]
        runner_pool: example-verify-tracker
`
	f, err := Parse([]byte(envDoc(block)))
	if err != nil {
		t.Fatalf("one cell may serve several environments from one pool: %v", err)
	}
	if n := len(f.Cells[0].Environments); n != 2 {
		t.Fatalf("expected 2 environments, got %d", n)
	}
}

func TestEnvironment_RejectsUnknownField(t *testing.T) {
	block := validEnvBlock + "        privileged: true\n"
	parseErr(t, envDoc(block))
}

// The four committed CLI fixtures (Verify rows 3-5 plus the bare-context
// case) are exercised here too, so a fixture that stops being red is caught
// by `go test` and not only by someone re-running the CLI rows by hand.
func TestEnvironment_CommittedRedFixturesAreRejected(t *testing.T) {
	for _, name := range []string{
		"env-prod-namespace.yaml",
		"env-tier-prod.yaml",
		"env-access-cluster-admin.yaml",
		"env-bare-context.yaml",
		"env-exec-canton.yaml",
	} {
		if _, err := Load("../../testdata/" + name); err == nil {
			t.Errorf("%s: expected the fixture to be REJECTED, got nil error — the fixture has stopped being red", name)
		}
	}
}

// The adopter-specific real-config assertion remains enforced in its original
// source repository. It is outside this reusable module: no adopter cells.yaml
// is transferred. The private handoff pins the retained function and its hash.

func parseErr(t *testing.T, doc string) error {
	t.Helper()
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	return err
}

func assertMentions(t *testing.T, err error, want string) {
	t.Helper()
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not mention %q", err.Error(), want)
	}
}

// TestParse_CellNameWithSlashRejected — review pr192-S2: a desk id is
// "<cell>/<role>" (or "<cell>/custom/<id>"), and both the terminal gate's
// scope (layer c) and the registry's cell check (layer d) read the cell as
// everything before the FIRST slash. A cell name containing "/" would make
// that reading ambiguous — with cells "a" and "a/b" configured, cell a's
// answerer would pass both layers on "a/b/review" — so the schema refuses it.
func TestParse_CellNameWithSlashRejected(t *testing.T) {
	for _, name := range []string{"assay/b", "assay/", "/assay"} {
		doc := strings.Replace(validDoc, "  - name: assay\n", "  - name: "+name+"\n", 1)
		if doc == validDoc {
			t.Fatal("the fixture substitution did not apply")
		}
		err := parseErr(t, doc)
		assertMentions(t, err, `must not contain "/"`)
	}
}
