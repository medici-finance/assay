package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func resolve(t *testing.T, env map[string]string, file KnobFile) *Resolved {
	t.Helper()
	r, err := Resolve(env, file, Table())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return r
}

func get(t *testing.T, r *Resolved, name, role string) Value {
	t.Helper()
	v, err := r.Get(name, role)
	if err != nil {
		t.Fatalf("Get(%s, %q): %v", name, role, err)
	}
	return v
}

// TestBudgetsAndRatesDefaultOff: with nothing set, every budget and rate resolves to 0 from the
// default layer and reports Off.
func TestBudgetsAndRatesDefaultOff(t *testing.T) {
	r := resolve(t, nil, KnobFile{})
	n := 0
	for _, k := range Table() {
		if k.Class != ClassBudget && k.Class != ClassRate {
			continue
		}
		n++
		v := get(t, r, k.Name, "worker")
		if v.Value != 0 || !v.Off() || v.Layer != LayerDefault {
			t.Errorf("%s: %s; want 0 (off) from the default layer", k.Name, v)
		}
	}
	if n != 5 {
		t.Fatalf("found %d budget/rate knobs, want 5", n)
	}
	if v := get(t, r, "write.breaker.trip", ""); v.Off() {
		t.Errorf("a fault guard reported Off: %s", v)
	}
}

// TestBudgetCountedPerRole: budgets are read and counted per role. A role-specific value applies
// to that role only, each role has its own counter key, and a budget cannot be read or counted
// without a role.
func TestBudgetCountedPerRole(t *testing.T) {
	r := resolve(t, map[string]string{"ASSAY_DESK_ISSUE_FILE_RATE__WORKER": "3"},
		KnobFile{Knobs: map[string]string{"issue.file.rate": "10"}, Roles: map[string]map[string]string{"pr-review": {"issue.file.rate": "1"}}})
	for role, want := range map[string]int64{"worker": 3, "pr-review": 1, "verify": 10} {
		if v := get(t, r, "issue.file.rate", role); v.Value != want {
			t.Errorf("%s: %s; want %d", role, v, want)
		}
	}
	a, err := CounterKey("issue.file.rate", "worker")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CounterKey("issue.file.rate", "pr-review")
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !strings.Contains(a, "worker") || !strings.Contains(b, "pr-review") {
		t.Errorf("counter keys %q and %q are not distinct per role", a, b)
	}
	if _, err := CounterKey("issue.file.rate", ""); err == nil {
		t.Error("a counter key without a role was issued")
	}
	if _, err := r.Get("issue.file.rate", ""); err == nil {
		t.Error("a budget was read without a role")
	}
	if _, err := r.Get("timeout.forge", "worker"); err == nil {
		t.Error("a knob that is not per role was read for a role")
	}
	for _, k := range Table() {
		if (k.Class == ClassBudget || k.Class == ClassRate) && !k.PerRole {
			t.Errorf("%s knob %s is not counted per role", k.Class, k.Name)
		}
	}
}

// TestEnvOverridesConfigOverridesDefault: env beats the config file, which beats the default,
// and the winner's provenance names its layer and source.
func TestEnvOverridesConfigOverridesDefault(t *testing.T) {
	file := KnobFile{Source: "cell.yaml", Knobs: map[string]string{"issue.file.rate": "0", "timeout.forge": "45s", "review.round_cap": "5"}}
	env := map[string]string{"ASSAY_DESK_ISSUE_FILE_RATE": "10", "ASSAY_DESK_REVIEW_ROUND_CAP": "4"}
	r := resolve(t, env, file)

	v := get(t, r, "issue.file.rate", "worker")
	if v.Value != 10 || v.Layer != LayerEnv || v.Source != "ASSAY_DESK_ISSUE_FILE_RATE" {
		t.Errorf("env did not win: %s", v)
	}
	if got, want := v.String(), "issue.file.rate[worker] = 10 (env ASSAY_DESK_ISSUE_FILE_RATE; config 0; default 0)"; got != want {
		t.Errorf("provenance\n got %s\nwant %s", got, want)
	}
	if v := get(t, r, "review.round_cap", ""); v.Value != 4 || v.Layer != LayerEnv {
		t.Errorf("env did not beat config: %s", v)
	}
	if v := get(t, r, "timeout.forge", ""); v.Duration() != 45*time.Second || v.Layer != LayerConfig || v.Source != "cell.yaml knobs.timeout.forge" {
		t.Errorf("config did not beat default: %s", v)
	}
	v = get(t, r, "lease.ttl", "")
	if v.Duration() != 2*time.Hour || v.Layer != LayerDefault {
		t.Errorf("default did not apply: %s", v)
	}
	if got, want := v.String(), "lease.ttl = 2h (env unset; config unset; default)"; got != want {
		t.Errorf("provenance\n got %s\nwant %s", got, want)
	}
	// Within a layer, a per-role value beats the value for every role; across layers, a
	// lower-precedence per-role value never beats a higher layer.
	r = resolve(t, map[string]string{"ASSAY_DESK_POOL_WIDTH": "4"},
		KnobFile{Knobs: map[string]string{"pool.width": "2"}, Roles: map[string]map[string]string{"worker": {"pool.width": "8"}}})
	if v := get(t, r, "pool.width", "worker"); v.Value != 4 || v.Layer != LayerEnv {
		t.Errorf("config per-role beat env: %s", v)
	}
	if got, want := get(t, r, "pool.width", "worker").String(), "pool.width[worker] = 4 (env ASSAY_DESK_POOL_WIDTH; config[worker] 8; config 2; default 1)"; got != want {
		t.Errorf("provenance\n got %s\nwant %s", got, want)
	}
	r = resolve(t, map[string]string{"ASSAY_DESK_POOL_WIDTH": "4", "ASSAY_DESK_POOL_WIDTH__PR_REVIEW": "6"}, KnobFile{})
	if v := get(t, r, "pool.width", "pr-review"); v.Value != 6 || v.Source != "ASSAY_DESK_POOL_WIDTH__PR_REVIEW" {
		t.Errorf("env per-role did not beat env global: %s", v)
	}
}

// TestOutOfBoundsRefusedNamingLayer: an out-of-bounds value is refused, not clamped, and the
// refusal names the knob, the value and the layer it came from.
func TestOutOfBoundsRefusedNamingLayer(t *testing.T) {
	cases := []struct {
		env       map[string]string
		file      KnobFile
		knob, val string
		layer     Layer
		source    string
	}{
		{env: map[string]string{"ASSAY_DESK_ISSUE_FILE_RATE": "-1"}, knob: "issue.file.rate", val: "-1", layer: LayerEnv, source: "ASSAY_DESK_ISSUE_FILE_RATE"},
		{env: map[string]string{"ASSAY_DESK_FORGE_PAGE_SIZE": "500"}, knob: "forge.page_size", val: "500", layer: LayerEnv, source: "ASSAY_DESK_FORGE_PAGE_SIZE"},
		{file: KnobFile{Source: "cell.yaml", Knobs: map[string]string{"timeout.hook": "1h"}}, knob: "timeout.hook", val: "1h", layer: LayerConfig, source: "cell.yaml knobs.timeout.hook"},
		{file: KnobFile{Source: "cell.yaml", Roles: map[string]map[string]string{"worker": {"pool.width": "99"}}}, knob: "pool.width", val: "99", layer: LayerConfig, source: "cell.yaml roles.worker.pool.width"},
		{env: map[string]string{"ASSAY_DESK_WRITE_BREAKER_TRIP": "0"}, knob: "write.breaker.trip", val: "0", layer: LayerEnv, source: "ASSAY_DESK_WRITE_BREAKER_TRIP"},
	}
	for _, c := range cases {
		_, err := Resolve(c.env, c.file, Table())
		if err == nil {
			t.Errorf("%s=%s from %s was accepted", c.knob, c.val, c.layer)
			continue
		}
		var be *BoundsError
		if !errors.As(err, &be) {
			t.Errorf("%s: %v is not a BoundsError", c.knob, err)
			continue
		}
		if be.Knob != c.knob || be.Value != c.val || be.Layer != c.layer || be.Source != c.source {
			t.Errorf("refusal %+v; want knob %s value %s layer %s source %s", be, c.knob, c.val, c.layer, c.source)
		}
		for _, want := range []string{c.knob, c.val, string(c.layer)} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q does not name %q", err, want)
			}
		}
	}
	// A shadowed invalid value is still refused: the config file is checked even when env wins.
	_, err := Resolve(map[string]string{"ASSAY_DESK_REVIEW_ROUND_CAP": "3"}, KnobFile{Knobs: map[string]string{"review.round_cap": "50"}}, Table())
	var be *BoundsError
	if !errors.As(err, &be) || be.Layer != LayerConfig {
		t.Errorf("a shadowed out-of-bounds config value was not refused: %v", err)
	}
}

func TestMalformedAndUnknownSettingsRefused(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		file KnobFile
	}{
		"unknown env":             {env: map[string]string{"ASSAY_DESK_ISSUE_FILE_RAT": "3"}},
		"authority is not a knob": {env: map[string]string{"ASSAY_DESK_ALLOWED_OPERATIONS": "change.merge"}},
		"per-role on global knob": {env: map[string]string{"ASSAY_DESK_TIMEOUT_FORGE__WORKER": "10s"}},
		"bad role suffix":         {env: map[string]string{"ASSAY_DESK_POOL_WIDTH__": "2"}},
		"unknown file knob":       {file: KnobFile{Knobs: map[string]string{"issue.rate": "3"}}},
		"bad file role":           {file: KnobFile{Roles: map[string]map[string]string{"Worker": {"pool.width": "2"}}}},
		"file per-role on global": {file: KnobFile{Roles: map[string]map[string]string{"worker": {"lease.ttl": "1h"}}}},
		"not a number":            {env: map[string]string{"ASSAY_DESK_ISSUE_FILE_RATE": "three"}},
		"bare duration":           {env: map[string]string{"ASSAY_DESK_LEASE_TTL": "20"}},
		"bad duration":            {file: KnobFile{Knobs: map[string]string{"lease.ttl": "soon"}}},
		"renew not under ttl":     {file: KnobFile{Knobs: map[string]string{"lease.ttl": "10m", "lease.renew_every": "10m"}}},
	}
	for name, c := range cases {
		if _, err := Resolve(c.env, c.file, Table()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// Every refusal is reported at once.
	_, err := Resolve(map[string]string{"ASSAY_DESK_NOPE": "1", "ASSAY_DESK_FORGE_PAGE_SIZE": "0"}, KnobFile{Knobs: map[string]string{"nope": "1"}}, Table())
	if err == nil || strings.Count(err.Error(), "\n") != 2 {
		t.Errorf("want three refusals reported together, got %v", err)
	}
	// Variables outside the ASSAY_DESK_ namespace are not the resolver's business.
	resolve(t, map[string]string{"HOME": "/x", "ASSAY_OTHER": "1"}, KnobFile{})
}

func TestDurationsAcceptDays(t *testing.T) {
	r := resolve(t, map[string]string{"ASSAY_DESK_RECORDS_RETENTION": "30d"}, KnobFile{})
	if v := get(t, r, "records.retention", ""); v.Duration() != 30*24*time.Hour {
		t.Errorf("records.retention = %s", v)
	}
}

func TestNarrowOnlyNarrows(t *testing.T) {
	r := resolve(t, map[string]string{"ASSAY_DESK_WRITE_RATE_PER_PR_HOUR": "20"}, KnobFile{})
	cases := []struct {
		knob      string
		role      string
		requested int64
		want      int64
		wantErr   bool
	}{
		{"write.rate.per_pr_hour", "worker", 5, 5, false},
		{"write.rate.per_pr_hour", "worker", 50, 20, false},
		{"write.rate.per_pr_hour", "worker", 0, 0, true}, // cannot switch a limit off
		{"issue.file.rate", "worker", 2, 2, false},       // resolved off: the request applies
		{"pool.width", "worker", 0, 0, false},
		{"pool.width", "worker", 3, 1, false},
		{"timeout.forge", "", 1, 0, true}, // not narrowable
		{"write.rate.per_pr_hour", "worker", -1, 0, true},
	}
	for _, c := range cases {
		got, err := r.Narrow(c.knob, c.role, c.requested)
		if (err != nil) != c.wantErr || (!c.wantErr && got != c.want) {
			t.Errorf("Narrow(%s, %d) = %d, %v; want %d (error %v)", c.knob, c.requested, got, err, c.want, c.wantErr)
		}
	}
}

func TestTableIsWellFormed(t *testing.T) {
	if err := validateTable(Table()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"issue.file.rate", "issue.file.window", "write.rate.per_pr_hour", "write.rate.per_repo_hour",
		"evidence.unnumbered_cap", "autolane.daily_cap", "write.breaker.trip", "write.breaker.cooldown",
		"review.round_cap", "pool.width", "token.github.refresh_margin", "key.github.max_age",
		"token.gitlab.pat_days", "token.gitlab.expiry_warn", "token.gitlab.in_use_window", "lease.ttl",
		"lease.renew_every", "records.retention", "forge.page_size", "timeout.forge", "timeout.callout",
		"timeout.decide", "timeout.hook", "policy.window", "policy.stale_after",
	}
	got := Table()
	if len(got) != len(want) {
		t.Fatalf("table has %d knobs, want %d", len(got), len(want))
	}
	for i, k := range got {
		if k.Name != want[i] {
			t.Errorf("knob %d = %s, want %s", i, k.Name, want[i])
		}
	}
	bad := append(Table(), Knob{Name: "issue.file.rate", Max: 1})
	if validateTable(bad) == nil {
		t.Error("a repeated knob name validated")
	}
	if validateTable([]Knob{{Name: "x", Class: ClassBudget, Default: 1, Max: 2, PerRole: true}}) == nil {
		t.Error("a budget with a non-zero default validated")
	}
	if validateTable([]Knob{{Name: "x", Default: 5, Max: 2}}) == nil {
		t.Error("a default outside its bounds validated")
	}
}

func TestValuesForDisplay(t *testing.T) {
	r := resolve(t, nil, KnobFile{})
	if n := len(r.Values("")); n != 18 {
		t.Errorf("Values(\"\") = %d knobs, want the 18 that are not per role", n)
	}
	if n := len(r.Values("worker")); n != 25 {
		t.Errorf("Values(worker) = %d knobs, want 25", n)
	}
}

func TestEnvFromList(t *testing.T) {
	got := EnvFromList([]string{"ASSAY_DESK_LEASE_TTL=1h", "PATH=/bin", "ASSAY_DESK_X=a=b", "BROKEN"})
	if len(got) != 2 || got["ASSAY_DESK_LEASE_TTL"] != "1h" || got["ASSAY_DESK_X"] != "a=b" {
		t.Errorf("EnvFromList = %v", got)
	}
}
