package cellcontainer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A role secret is a read-only file bind plus one path variable for the
// worker and verifier containers only; any other role holding a binding refuses its plan.
// regression: #6065
func TestRoleSecretPlanHoldersOnly(t *testing.T) {
	c, _ := fixture(t)
	cell := c.Cells["sample"]
	base := cell.Roles["desk"]
	base.StartupAction = ""
	for _, key := range []string{"worker", "verifier", "reviewer", "intake-loop"} {
		r := base
		r.Volume = "sample-" + key
		cell.Roles[key] = r
	}
	c.Cells["sample"] = cell
	src := filepath.Join(t.TempDir(), "fake.secret")
	if err := os.WriteFile(src, []byte("plainly-fake-words"), 0o600); err != nil {
		t.Fatal(err)
	}
	bind := []SecretBind{{Service: "fake-svc", Source: src, EnvName: "ASSAY_ROLE_SECRET_FAKE_SVC_FILE"}}
	if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"sample": {"worker-desk": bind, "verify-desk": bind}}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"worker", "verifier", "reviewer", "intake-loop", "desk"} {
		p, err := c.Plan("sample", key, "", "")
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		var mounted bool
		for _, m := range p.Mounts {
			if strings.HasPrefix(m.Target, RoleSecretTargetPrefix) {
				mounted = true
				if !m.ReadOnly || m.Source != src || m.Target != "/run/secrets/role-fake-svc" {
					t.Errorf("%s: secret mount %+v is not the read-only file bind", key, m)
				}
			}
		}
		v, hasEnv := p.Env["ASSAY_ROLE_SECRET_FAKE_SVC_FILE"]
		holder := key == "worker" || key == "verifier"
		if mounted != holder || hasEnv != holder {
			t.Errorf("%s: mounted=%t env=%t, want %t", key, mounted, hasEnv, holder)
		}
		if holder && v != "/run/secrets/role-fake-svc" {
			t.Errorf("%s: variable carries %q, not the mount path", key, v)
		}
		for k := range p.Env {
			if !holder && strings.HasPrefix(k, "ASSAY_ROLE_SECRET_") {
				t.Errorf("%s carries %s", key, k)
			}
		}
		for _, a := range p.RunArgs() {
			if strings.Contains(a, "plainly-fake-words") {
				t.Fatalf("%s: run arguments carry the value", key)
			}
		}
		if err := p.CheckFiles(); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	// A binding to a non-holding role refuses that role's plan rather than being dropped.
	if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"sample": {"pr-review-desk": bind}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Plan("sample", "reviewer", "", ""); err == nil || !strings.Contains(err.Error(), "may not hold a role secret") {
		t.Fatalf("reviewer plan with a binding: %v", err)
	}
	// A secret source is a protected path: no directory mount may contain it.
	if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"sample": {"worker-desk": {{Service: "fake-svc", Source: filepath.Join(base.Config, "fake.secret"), EnvName: "ASSAY_ROLE_SECRET_FAKE_SVC_FILE"}}}}); err == nil {
		t.Fatal("a secret inside a mounted config directory was accepted")
	}
	// Malformed bindings never install.
	for _, b := range []SecretBind{{Service: "Bad", Source: src, EnvName: "ASSAY_ROLE_SECRET_BAD_FILE"}, {Service: "ok", Source: "relative", EnvName: "ASSAY_ROLE_SECRET_OK_FILE"}, {Service: "ok", Source: src, EnvName: "OTHER"}} {
		if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"sample": {"worker-desk": {b}}}); err == nil {
			t.Errorf("malformed binding %+v accepted", b)
		}
	}
	if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"absent": {"worker-desk": bind}}); err == nil {
		t.Error("binding for an undefined cell accepted")
	}
	// A group-readable secret fails the launch-time file check.
	if err := c.SetRoleSecrets(map[string]map[string][]SecretBind{"sample": {"worker-desk": bind}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(src, 0o640); err != nil {
		t.Fatal(err)
	}
	p, err := c.Plan("sample", "worker", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CheckFiles(); err == nil {
		t.Error("group-readable secret passed CheckFiles")
	}
}
