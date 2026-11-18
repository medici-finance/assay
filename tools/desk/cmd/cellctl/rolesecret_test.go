package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
)

// The synthetic value every role-secret fixture file holds: plainly fake words, never a
// credential. No output, receipt or error may carry it.
const rsFakeValue = "plainly-fake-words-not-a-credential"

// rsFixture is a native container cell whose configuration defines the coordinator, worker,
// verifier and reviewer containers, plus one owner-only secret file outside every checkout. The
// ACL probe is stubbed to "no list" (TestRoleSecretACLProbeRealFile exercises the real one).
func rsFixture(t *testing.T) (c *Cell, root, secret string) {
	t.Helper()
	root = t.TempDir()
	for _, d := range []string{"config", "incoming"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(root, "key.pem")
	if err := os.WriteFile(key, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	role := func(volume string) cellcontainer.Role {
		return cellcontainer.Role{Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: volume, Config: filepath.Join(root, "config"), AppKey: key}
	}
	cfg := cellcontainer.Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]cellcontainer.Cell{"sample": {
		Repo: "example-org/example-repo", Incoming: filepath.Join(root, "incoming"),
		Roles: map[string]cellcontainer.Role{"desk": role("sample-desk"), "worker": role("sample-worker"), "verifier": role("sample-verifier"), "reviewer": role("sample-reviewer")},
	}}}
	b, _ := json.Marshal(cfg)
	config := filepath.Join(root, "containers.json")
	if err := os.WriteFile(config, b, 0o600); err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(root, "cells")
	t.Setenv("CELLS_ROOT", cells)
	for _, k := range []string{"DRY_RUN", "CELL_CONTAINER_CONFIG", "CELL_CONTAINER_LAUNCHER", "CELL_MODEL_POLICY", "CELL_PROVIDER", envRoleSecrets, envRoleSecretSurface} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	newNativeContainer(cells, "sample", "example-org/example-repo", config, "the-desk worker-desk verify-desk pr-review-desk", "")
	secretDir := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(secretDir, 0o700); err != nil {
		t.Fatal(err)
	}
	secret = filepath.Join(secretDir, "fake-svc.secret")
	if err := os.WriteFile(secret, []byte(rsFakeValue), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := roleSecretACLProbe
	roleSecretACLProbe = func(string) (bool, error) { return false, nil }
	t.Cleanup(func() { roleSecretACLProbe = orig })
	return loadCell("sample"), root, secret
}

// rsBind appends role-secret lines to the cell's own cell.env and reloads the cell, so each key
// is sourced from cell.env exactly as an operator's would be.
func rsBind(t *testing.T, lines ...string) *Cell {
	t.Helper()
	path := filepath.Join(os.Getenv("CELLS_ROOT"), "sample", "cell.env")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return loadCell("sample")
}

func rsValid(secret string) []string {
	return []string{envRoleSecretSurface + "=container", envRoleSecrets + "=example-org/example-repo:worker-desk:fake-svc=" + secret + ",example-org/example-repo:verify-desk:fake-svc=" + secret}
}

// rsCheck runs the role-secret rows of `check` and returns whether they passed and what they printed.
func rsCheck(t *testing.T, c *Cell) (bool, string) {
	t.Helper()
	k := &checker{ok: true}
	out := captureStdout(t, func() { c.checkRoleSecrets(k) })
	if strings.Contains(out, rsFakeValue) {
		t.Fatalf("check output carries the secret value: %s", out)
	}
	return k.ok, out
}

func rsSecretMount(p *cellcontainer.Plan) (cellcontainer.Mount, bool) {
	for _, m := range p.Mounts {
		if strings.HasPrefix(m.Target, cellcontainer.RoleSecretTargetPrefix) {
			return m, true
		}
	}
	return cellcontainer.Mount{}, false
}

// Only worker-desk and verify-desk containers carry a role secret, as a
// read-only bind plus one variable naming its path; a binding to any other role refuses parse
// and launch rather than being dropped.
// regression: #6065
func TestRoleSecretScope(t *testing.T) {
	_, _, secret := rsFixture(t)
	c := rsBind(t, rsValid(secret)...)
	if ok, out := rsCheck(t, c); !ok {
		t.Fatalf("a valid container-surface binding failed check:\n%s", out)
	}
	for _, role := range []string{"the-desk", "worker-desk", "verify-desk", "pr-review-desk"} {
		p := c.nativeContainerPlan(role, "", "")
		holder := role == "worker-desk" || role == "verify-desk"
		m, mounted := rsSecretMount(p)
		v, hasEnv := p.Env["ASSAY_ROLE_SECRET_FAKE_SVC_FILE"]
		if mounted != holder || hasEnv != holder {
			t.Errorf("%s: mounted=%t env=%t, want %t", role, mounted, hasEnv, holder)
			continue
		}
		for k := range p.Env {
			if !holder && strings.HasPrefix(k, "ASSAY_ROLE_SECRET_") {
				t.Errorf("%s carries %s", role, k)
			}
		}
		if holder && (!m.ReadOnly || m.Source != secret || m.Target != "/run/secrets/role-fake-svc" || v != m.Target) {
			t.Errorf("%s: mount %+v env %q is not the read-only path-only binding", role, m, v)
		}
		if err := p.CheckFiles(); err != nil {
			t.Errorf("%s: %v", role, err)
		}
	}

	// A binding to the reviewer (or intake, or the coordinator) refuses everywhere.
	for _, bad := range []string{"pr-review-desk", "intake-desk", "the-desk"} {
		_, _, secret := rsFixture(t)
		c := rsBind(t, envRoleSecretSurface+"=container", envRoleSecrets+"=example-org/example-repo:"+bad+":fake-svc="+secret)
		if _, err := c.roleSecretBindings(); err == nil || !strings.Contains(err.Error(), "only worker-desk and verify-desk") {
			t.Errorf("%s binding parsed: %v", bad, err)
		}
		if ok, _ := rsCheck(t, c); ok {
			t.Errorf("%s binding passed check", bad)
		}
		assertDies(t, bad+" binding: launch gate", c.roleSecretLaunchGate)
		assertDies(t, bad+" binding: worker plan", func() { c.nativeContainerPlan("worker-desk", "", "") })
	}

	// Two services that spell the same variable for one role refuse.
	_, _, secret = rsFixture(t)
	c = rsBind(t, envRoleSecretSurface+"=container", envRoleSecrets+"=example-org/example-repo:worker-desk:a-b="+secret+",example-org/example-repo:worker-desk:a.b="+secret)
	if _, err := c.roleSecretBindings(); err == nil || !strings.Contains(err.Error(), "same variable") {
		t.Errorf("duplicate variable accepted: %v", err)
	}

	// A malformed entry never echoes what follows its `=`.
	_, _, _ = rsFixture(t)
	c = rsBind(t, envRoleSecretSurface+"=container", envRoleSecrets+"=example-org/example-repo:worker-desk="+rsFakeValue)
	if _, err := c.roleSecretBindings(); err == nil || strings.Contains(err.Error(), rsFakeValue) {
		t.Errorf("malformed entry: %v", err)
	}

	// The keys bind only from cell.env: the launching shell cannot bind a secret to a cell.
	_, _, secret = rsFixture(t)
	t.Setenv(envRoleSecretSurface, "container")
	t.Setenv(envRoleSecrets, "example-org/example-repo:worker-desk:fake-svc="+secret)
	c = loadCell("sample")
	if _, err := c.roleSecretBindings(); err == nil || !strings.Contains(err.Error(), "read only from the cell's own cell.env") {
		t.Errorf("a process-environment binding was accepted: %v", err)
	}
	assertDies(t, "process-environment binding", func() { c.nativeContainerPlan("worker-desk", "", "") })
}

// Every launch-time file check fails closed, and the dry run names the
// binding by path only.
// regression: #6065
func TestRoleSecretCheck(t *testing.T) {
	cases := map[string]func(t *testing.T, secret string) (path string){
		"group-readable": func(t *testing.T, secret string) string {
			if err := os.Chmod(secret, 0o640); err != nil {
				t.Fatal(err)
			}
			return secret
		},
		"world-readable": func(t *testing.T, secret string) string {
			if err := os.Chmod(secret, 0o604); err != nil {
				t.Fatal(err)
			}
			return secret
		},
		"foreign-owned": func(t *testing.T, secret string) string {
			orig := roleSecretOwnerUID
			roleSecretOwnerUID = func() int { return os.Getuid() + 4242 }
			t.Cleanup(func() { roleSecretOwnerUID = orig })
			return secret
		},
		"symlink": func(t *testing.T, secret string) string {
			link := filepath.Join(filepath.Dir(secret), "link.secret")
			if err := os.Symlink(secret, link); err != nil {
				t.Fatal(err)
			}
			return link
		},
		"directory": func(t *testing.T, secret string) string {
			d := filepath.Join(filepath.Dir(secret), "dir.secret")
			if err := os.Mkdir(d, 0o700); err != nil {
				t.Fatal(err)
			}
			return d
		},
		"inside-a-git-checkout": func(t *testing.T, secret string) string {
			repo := filepath.Join(filepath.Dir(secret), "checkout")
			sub := filepath.Join(repo, "deep")
			if err := os.MkdirAll(sub, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: elsewhere\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(sub, "fake-svc.secret")
			if err := os.WriteFile(p, []byte(rsFakeValue), 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		},
		"group-writable-directory": func(t *testing.T, secret string) string {
			if err := os.Chmod(filepath.Dir(secret), 0o720); err != nil {
				t.Fatal(err)
			}
			return secret
		},
		"access-control-list": func(t *testing.T, secret string) string {
			roleSecretACLProbe = func(p string) (bool, error) { return p == secret, nil }
			return secret
		},
		"directory-access-control-list": func(t *testing.T, secret string) string {
			roleSecretACLProbe = func(p string) (bool, error) { return p != secret, nil }
			return secret
		},
		"unreadable-access-control-list": func(t *testing.T, secret string) string {
			roleSecretACLProbe = func(string) (bool, error) { return false, errors.New("probe failed") }
			return secret
		},
		"missing": func(t *testing.T, secret string) string {
			return filepath.Join(filepath.Dir(secret), "absent.secret")
		},
	}
	// Each case refuses for its own reason, so disabling one predicate cannot hide behind another.
	reasons := map[string]string{
		"group-readable":                 "mode 0640 is readable or writable beyond its owner",
		"world-readable":                 "mode 0604 is readable or writable beyond its owner",
		"foreign-owned":                  "is owned by uid",
		"symlink":                        "is a symlink",
		"directory":                      "is not a regular file",
		"inside-a-git-checkout":          "lies inside a git checkout or worktree",
		"group-writable-directory":       "is writable beyond its owner",
		"access-control-list":            "carries an access-control list; remove it",
		"directory-access-control-list":  "its directory carries an access-control list",
		"unreadable-access-control-list": "probe failed",
		"missing":                        "does not exist",
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, secret := rsFixture(t)
			path := mutate(t, secret)
			c := rsBind(t, envRoleSecretSurface+"=container", envRoleSecrets+"=example-org/example-repo:worker-desk:fake-svc="+path)
			ok, out := rsCheck(t, c)
			if ok || !strings.Contains(out, "MISS  role secrets: refused") {
				t.Fatalf("check passed:\n%s", out)
			}
			if want, has := reasons[name]; !has || !strings.Contains(out, want) {
				t.Fatalf("refused, but not for %q:\n%s", want, out)
			}
			assertDies(t, name+": worker plan", func() { c.nativeContainerPlan("worker-desk", "", "") })
		})
	}

	t.Run("project-mismatch", func(t *testing.T) {
		_, _, secret := rsFixture(t)
		c := rsBind(t, envRoleSecretSurface+"=container", envRoleSecrets+"=example-org/another-repo:worker-desk:fake-svc="+secret)
		if ok, out := rsCheck(t, c); ok || !strings.Contains(out, "this container cell serves example-org/example-repo") {
			t.Fatalf("check passed:\n%s", out)
		}
	})

	t.Run("dry-run-names-path-only", func(t *testing.T) {
		_, _, secret := rsFixture(t)
		c := rsBind(t, rsValid(secret)...)
		c.Env.putFrom("DRY_RUN", "1", layerProcess)
		out := captureStdout(t, func() { c.nativeContainer("desk", "worker-desk", "--harness", "codex", "--model", "test-model") })
		if strings.Contains(out, rsFakeValue) {
			t.Fatalf("dry run carries the value:\n%s", out)
		}
		want := "[role-secret] role=worker-desk project=example-org/example-repo service=fake-svc path=" + secret + " mount=/run/secrets/role-fake-svc readonly=true env=ASSAY_ROLE_SECRET_FAKE_SVC_FILE"
		if !strings.Contains(out, want) || !strings.Contains(out, `dst=/run/secrets/role-fake-svc\,readonly`) {
			t.Fatalf("dry run does not name the binding:\n%s", out)
		}
		out = captureStdout(t, func() { c.nativeContainer("desk", "pr-review-desk", "--harness", "codex", "--model", "test-model") })
		if strings.Contains(out, "role-secret") || strings.Contains(out, "ASSAY_ROLE_SECRET_") {
			t.Fatalf("reviewer dry run carries a binding:\n%s", out)
		}
	})
}

// The real ACL probe reads a plain owner-only file as carrying no list.
// regression: #6065
func TestRoleSecretACLProbeRealFile(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the ACL probe refuses on this platform")
	}
	p := filepath.Join(t.TempDir(), "plain.secret")
	if err := os.WriteFile(p, []byte(rsFakeValue), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{p, filepath.Dir(p)} {
		has, err := probeACL(path)
		if err != nil || has {
			t.Fatalf("probeACL(%s) = %t, %v", path, has, err)
		}
	}
	if _, err := probeACL(filepath.Join(filepath.Dir(p), "absent")); err == nil {
		t.Fatal("probe of a missing path reported no error")
	}
}

// A removed file refuses the next launch, and each launch's receipt names
// role, service and path, never the value or a hash of it; a receipt that cannot be written
// refuses the launch before the engine runs.
// regression: #6065
func TestRoleSecretRevoke(t *testing.T) {
	_, _, secret := rsFixture(t)
	c := rsBind(t, rsValid(secret)...)
	roleSecretNow = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	t.Cleanup(func() { roleSecretNow = time.Now })

	p, cfg := c.nativeContainerPlanCfg("worker-desk", "", "")
	if err := c.writeRoleSecretReceipts(cfg, p); err != nil {
		t.Fatal(err)
	}
	// A non-holder writes nothing.
	rp, rcfg := c.nativeContainerPlanCfg("pr-review-desk", "", "")
	if err := c.writeRoleSecretReceipts(rcfg, rp); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(c.Dir, "state", roleSecretReceipts)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(rsFakeValue))
	if strings.Contains(string(raw), rsFakeValue) || strings.Contains(string(raw), hex.EncodeToString(sum[:])) {
		t.Fatalf("receipt carries the value or its hash: %s", raw)
	}
	var lines []roleSecretReceipt
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		var r roleSecretReceipt
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("receipt line %q: %v", sc.Text(), err)
		}
		lines = append(lines, r)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d receipts, want 1 (the worker's): %s", len(lines), raw)
	}
	r := lines[0]
	if r.Cell != "sample" || r.Role != "worker-desk" || r.Project != "example-org/example-repo" || r.Service != "fake-svc" || r.Path != secret || r.LaunchedAt != "2026-01-02T03:04:05Z" || r.FileMtime == "" {
		t.Fatalf("receipt %+v", r)
	}
	for _, want := range []struct {
		path string
		mode os.FileMode
	}{{path, 0o600}, {filepath.Dir(path), 0o700}} {
		st, err := os.Stat(want.path)
		if err != nil || st.Mode().Perm() != want.mode {
			t.Fatalf("%s: mode %v (%v), want %04o", want.path, st.Mode().Perm(), err, want.mode)
		}
	}

	// Revocation: the file is removed, and the next launch refuses.
	if err := os.Remove(secret); err != nil {
		t.Fatal(err)
	}
	if ok, out := rsCheck(t, c); ok || !strings.Contains(out, "does not exist") {
		t.Fatalf("check passed for a removed file:\n%s", out)
	}
	assertDies(t, "launch after the file was removed", func() { c.nativeContainerPlan("worker-desk", "", "") })
	if err := c.writeRoleSecretReceipts(cfg, p); err == nil {
		t.Fatal("a receipt was written for a removed file")
	}

	// A receipt that cannot be written refuses the launch before any engine call.
	if err := os.WriteFile(secret, []byte(rsFakeValue), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(c.Dir, "state")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Dir, "state"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.Dir, "run"), 0o700); err != nil {
		t.Fatal(err)
	}
	var engineCalls int
	defer func(orig func(string) cellcontainer.Engine) { nativeEngine = orig }(nativeEngine)
	nativeEngine = func(string) cellcontainer.Engine {
		engineCalls++
		fail := errors.New("the engine must not be reached")
		return cellcontainer.Engine{Output: func(...string) ([]byte, error) { return nil, fail }, Foreground: func(...string) error { return fail }}
	}
	assertDies(t, "launch with an unwritable receipt", func() {
		cmdContainerRun([]string{"sample", "worker-desk", "codex", "test-model"})
	})
	if engineCalls != 0 {
		t.Fatalf("the engine was reached %d times", engineCalls)
	}
	if _, err := os.Stat(filepath.Join(c.Dir, "run", "worker.lock")); err != nil {
		t.Fatalf("the refusal came before the lifetime lock, not at the receipt: %v", err)
	}
}

// Every surface but a native container cell binds nothing and refuses, and
// so does a container cell beside any other cell that could reach the file.
// regression: #6065
func TestRoleSecretReadIsolation(t *testing.T) {
	refused := func(t *testing.T, c *Cell, want string) {
		t.Helper()
		ok, out := rsCheck(t, c)
		if ok || !strings.Contains(out, want) {
			t.Fatalf("check did not refuse with %q:\n%s", want, out)
		}
	}
	for name, surface := range map[string]string{"unset": "", "host": "host", "unknown": "vm"} {
		t.Run("surface-"+name, func(t *testing.T) {
			_, _, secret := rsFixture(t)
			lines := []string{envRoleSecrets + "=example-org/example-repo:worker-desk:fake-svc=" + secret}
			if surface != "" {
				lines = append(lines, envRoleSecretSurface+"="+surface)
			}
			c := rsBind(t, lines...)
			refused(t, c, "MISS  role secrets: refused — "+envRoleSecretSurface)
			assertDies(t, "launch gate", c.roleSecretLaunchGate)
			assertDies(t, "worker plan", func() { c.nativeContainerPlan("worker-desk", "", "") })
		})
	}
	t.Run("host-kind", func(t *testing.T) {
		_, _, secret := rsFixture(t)
		c := rsBind(t, rsValid(secret)...)
		for _, kind := range []string{"host", "desktop", "k8s"} {
			c.Kind = kind
			refused(t, c, "CELL_KIND="+kind+" is not a native container cell")
			assertDies(t, kind+" launch gate", c.roleSecretLaunchGate)
		}
		if _, _, err := c.kubeProfileE(); err == nil {
			t.Error("a Kubernetes compatibility profile was built for a cell with role secrets")
		}
	})
	t.Run("external-launcher", func(t *testing.T) {
		_, _, secret := rsFixture(t)
		c := rsBind(t, rsValid(secret)...)
		c.Env.putFrom("CELL_CONTAINER_CONFIG", "", layerCellEnv)
		refused(t, c, "external CELL_CONTAINER_LAUNCHER")
		assertDies(t, "launch gate", c.roleSecretLaunchGate)
	})
	t.Run("acl", func(t *testing.T) {
		_, _, secret := rsFixture(t)
		c := rsBind(t, rsValid(secret)...)
		roleSecretACLProbe = func(string) (bool, error) { return true, nil }
		refused(t, c, "access-control list")
		assertDies(t, "worker plan", func() { c.nativeContainerPlan("worker-desk", "", "") })
	})

	sibling := func(t *testing.T, env string) *Cell {
		t.Helper()
		dir := filepath.Join(os.Getenv("CELLS_ROOT"), "other")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cell.env"), []byte(env), 0o600); err != nil {
			t.Fatal(err)
		}
		return loadCell("sample")
	}
	otherConfig := func(t *testing.T, root string, mutate func(*cellcontainer.Role)) string {
		t.Helper()
		r := cellcontainer.Role{Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: "other-worker", Config: filepath.Join(root, "config"), AppKey: filepath.Join(root, "key.pem")}
		mutate(&r)
		cfg := cellcontainer.Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]cellcontainer.Cell{"other": {Repo: "example-org/other-repo", Incoming: filepath.Join(root, "incoming"), Roles: map[string]cellcontainer.Role{"worker": r}}}}
		b, _ := json.Marshal(cfg)
		p := filepath.Join(root, "other-containers.json")
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, tc := range map[string]struct {
		env  func(root, secret string) string
		want string
	}{
		"host-sibling":      {func(_, _ string) string { return "CELL_KIND=host\n" }, "cell other (kind host) runs windows on the host"},
		"default-kind":      {func(_, _ string) string { return "CELL_REPO=example-org/x\n" }, "cell other (kind k8s)"},
		"external-launcher": {func(_, _ string) string { return "CELL_KIND=container\nCELL_CONTAINER_LAUNCHER=/bin/false\n" }, "cell other runs on an external container launcher"},
		"same-file-binding": {func(root, secret string) string {
			return "CELL_KIND=container\nCELL_CONTAINER_CONFIG=" + filepath.Join(root, "containers.json") + "\n" + envRoleSecrets + "=example-org/example-repo:worker-desk:x=" + secret + "\n"
		}, "cell other binds the same secret file"},
		"unreadable-config": {func(root, _ string) string {
			return "CELL_KIND=container\nCELL_CONTAINER_CONFIG=" + filepath.Join(root, "absent.json") + "\n"
		}, "cell other: its container configuration cannot be read"},
		"same-config-is-ok": {func(root, _ string) string {
			return "CELL_KIND=container\nCELL_CONTAINER_CONFIG=" + filepath.Join(root, "containers.json") + "\n"
		}, ""},
	} {
		t.Run("sibling-"+name, func(t *testing.T) {
			_, root, secret := rsFixture(t)
			rsBind(t, rsValid(secret)...)
			c := sibling(t, tc.env(root, secret))
			ok, out := rsCheck(t, c)
			if tc.want == "" {
				if !ok {
					t.Fatalf("refused:\n%s", out)
				}
				return
			}
			if ok || !strings.Contains(out, tc.want) {
				t.Fatalf("check did not refuse with %q:\n%s", tc.want, out)
			}
			assertDies(t, "worker plan", func() { c.nativeContainerPlan("worker-desk", "", "") })
		})
	}
	t.Run("sibling-shared-volume", func(t *testing.T) {
		_, root, secret := rsFixture(t)
		rsBind(t, rsValid(secret)...)
		cfg := otherConfig(t, root, func(r *cellcontainer.Role) { r.Volume = "sample-worker" })
		refused(t, sibling(t, "CELL_KIND=container\nCELL_CONTAINER_CONFIG="+cfg+"\n"), "shares writable volume sample-worker")
	})
	t.Run("sibling-mounts-the-file", func(t *testing.T) {
		_, root, secret := rsFixture(t)
		rsBind(t, rsValid(secret)...)
		cfg := otherConfig(t, root, func(r *cellcontainer.Role) { r.Config = filepath.Dir(secret) })
		refused(t, sibling(t, "CELL_KIND=container\nCELL_CONTAINER_CONFIG="+cfg+"\n"), "which holds the worker-desk/fake-svc secret file")
	})
	t.Run("sibling-distinct-container-cell-is-ok", func(t *testing.T) {
		_, root, secret := rsFixture(t)
		rsBind(t, rsValid(secret)...)
		cfg := otherConfig(t, root, func(*cellcontainer.Role) {})
		if ok, out := rsCheck(t, sibling(t, "CELL_KIND=container\nCELL_CONTAINER_CONFIG="+cfg+"\n")); !ok {
			t.Fatalf("refused:\n%s", out)
		}
	})
	t.Run("sibling-unreadable", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads a mode-000 file")
		}
		_, _, secret := rsFixture(t)
		rsBind(t, rsValid(secret)...)
		c := sibling(t, "CELL_KIND=host\n")
		envPath := filepath.Join(os.Getenv("CELLS_ROOT"), "other", "cell.env")
		if err := os.Chmod(envPath, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(envPath, 0o600) })
		refused(t, c, "cell other: its cell.env cannot be read")
	})
}

// A host window never inherits a role-secret variable from the launching
// shell.
// regression: #6065
func TestRoleSecretHostWindowUnset(t *testing.T) {
	got := unsetRoleSecretEnv([]string{"PATH=/bin", "ASSAY_ROLE_SECRET_FAKE_SVC_FILE=/x", "assay_role_secret_lower_file=/y", "HOME=/h"})
	if strings.Join(got, ",") != "PATH=/bin,HOME=/h" {
		t.Fatalf("got %q", got)
	}
}
