package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
)

func commsFixture(t *testing.T) (*Cell, *deskCommsConfig, string) {
	t.Helper()
	c := codexEnvironmentCell(t)
	c.Name = "example"
	c.Kind = "house"
	c.Env.Put("ROLES", strings.Join(knownRoles, " "))
	dir := custodytest.PrivateTempDir(t)
	key := filepath.Join(dir, "signing.key")
	trust := filepath.Join(dir, "trust.json")
	for _, p := range []string{key, trust} {
		if err := os.WriteFile(p, []byte("fixture-custody-content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	endpoint := filepath.Join(dir, "gateway.sock")
	if runtime.GOOS == "windows" {
		endpoint = `\\.\pipe\assay-comms-test`
	}
	cfg := &deskCommsConfig{Cell: c.Name, Mode: "interim", Repo: "example/repo", Socket: endpoint, QueueDir: filepath.Join(dir, "queue"), TrustStore: trust, SigningKeys: map[string]string{}, Decider: json.RawMessage(`{"cmd":["fixture-reader"],"model":"fixture","pin":"1","isolate":true,"contained":true}`)}
	for _, role := range knownRoles {
		cfg.SigningKeys[role] = key
	}
	path := filepath.Join(dir, "comms.json")
	c.Env.Put("CELL_COMMS_CONFIG", path)
	writeCommsFixture(t, path, cfg)
	return c, cfg, path
}

func writeCommsFixture(t *testing.T, path string, cfg *deskCommsConfig) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDeskCommsRoleContext(t *testing.T) {
	c, cfg, _ := commsFixture(t)
	for _, role := range knownRoles {
		env, err := c.deskCommsEnv(role, []string{"DESK_CELL=other", "desk_role=other", "DESK_COMMS_KEY=wrong", "DESK_COMMS_GATEWAY=wrong", "GH_TOKEN=secret-not-on-argv"})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"DESK_CELL=example", "DESK_ROLE=" + role, "DESK_COMMS_GATEWAY=" + cfg.Socket, "DESK_COMMS_KEY=" + cfg.SigningKeys[role]} {
			if !strings.Contains(strings.Join(env, "\n"), want) {
				t.Fatalf("missing %s", want)
			}
		}
		if strings.Contains(strings.Join(env, "\n"), "=other") {
			t.Fatal("parent identity survived")
		}
		args, err := c.codexEnvironmentArgs(env)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, "\n")
		if !strings.Contains(joined, `shell_environment_policy.set.DESK_ROLE="`+role+`"`) {
			t.Fatal("Codex command context lost role")
		}
		if strings.Contains(joined, "secret-not-on-argv") || strings.Contains(joined, "fixture-custody-content") {
			t.Fatal("secret copied onto argv")
		}
	}
}

func TestDeskCommsPreflightRefusesIncompleteConfig(t *testing.T) {
	for _, kind := range []string{"wrong-cell", "unknown-mode", "full", "missing-role", "no-decider", "uncontained", "missing-key", "unknown-field", "extra-object", "relative-custody", "not-house"} {
		t.Run(kind, func(t *testing.T) {
			c, cfg, path := commsFixture(t)
			switch kind {
			case "wrong-cell":
				cfg.Cell = "other"
			case "unknown-mode":
				cfg.Mode = "maybe"
			case "full":
				cfg.Mode = "full"
			case "missing-role":
				delete(cfg.SigningKeys, "worker-desk")
			case "no-decider":
				cfg.Decider = nil
			case "uncontained":
				cfg.Decider = json.RawMessage(`{"cmd":["fixture"],"pin":"1","isolate":true,"contained":false}`)
			case "missing-key":
				cfg.SigningKeys["the-desk"] = filepath.Join(filepath.Dir(path), "absent")
			case "relative-custody":
				cfg.TrustStore = "relative.json"
			case "not-house":
				c.Kind = "product"
			}
			writeCommsFixture(t, path, cfg)
			if kind == "unknown-field" {
				raw, _ := os.ReadFile(path)
				raw = append(raw[:len(raw)-1], []byte(`,"typo":true}`)...)
				os.WriteFile(path, raw, 0600)
			}
			if kind == "extra-object" {
				f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				f.WriteString("{}")
				f.Close()
			}
			if _, err := c.loadDeskComms(); err == nil {
				t.Fatal("bad comms configuration accepted")
			}
		})
	}
}

// The fixture itself must load, or every refusal case above passes vacuously.
func TestDeskCommsFixtureLoads(t *testing.T) {
	c, _, _ := commsFixture(t)
	if cfg, err := c.loadDeskComms(); err != nil || cfg == nil {
		t.Fatalf("positive control: valid comms configuration refused: %v", err)
	}
}

func TestCommsBinaryCheck(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "commsgw")
	if err := os.WriteFile(exe, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := commsBinary(exe); err != nil {
		t.Fatalf("present binary refused: %v", err)
	}
	if err := commsBinary(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("absent binary accepted")
	}
	if err := commsBinary(dir); err == nil {
		t.Fatal("directory accepted as a binary")
	}
	if runtime.GOOS != "windows" {
		plain := filepath.Join(dir, "plain")
		if err := os.WriteFile(plain, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := commsBinary(plain); err == nil {
			t.Fatal("non-executable file accepted as a binary")
		}
	}
}

func TestDeskCommsDisabledDoesNotInheritAnotherCell(t *testing.T) {
	c, cfg, path := commsFixture(t)
	for _, configured := range []bool{true, false} {
		if configured {
			cfg.Mode = "disabled"
			writeCommsFixture(t, path, cfg)
		} else {
			c.Env.Put("CELL_COMMS_CONFIG", "")
		}
		env, err := c.deskCommsEnv("worker-desk", []string{"DESK_CELL=wrong", "DESK_ROLE=wrong", "DESK_COMMS_KEY=wrong", "PATH=preserved"})
		if err != nil || len(env) != 1 || env[0] != "PATH=preserved" {
			t.Fatalf("disabled env=%v err=%v", env, err)
		}
	}
}

func TestCommsServiceUsesOneConfig(t *testing.T) {
	c, cfg, _ := commsFixture(t)
	t.Setenv("ASSAY_COMMS_LISTEN", "0.0.0.0:1234")
	t.Setenv("GH_TOKEN", "ambient-secret")
	cfg.ClaudeConfigDir = filepath.Join(t.TempDir(), "claude account")
	env := strings.Join(c.commsServiceEnv(cfg), "\n")
	for _, want := range []string{"ASSAY_COMMS_LOCAL_ONLY=1", "ASSAY_COMMS_CELL=example", "ASSAY_COMMS_REPO=example/repo", "CLAUDE_CONFIG_DIR=" + cfg.ClaudeConfigDir, "ASSAY_COMMS_QUEUE_DIR=" + cfg.QueueDir} {
		if !strings.Contains(env, want) {
			t.Fatal(want)
		}
	}
	if strings.Contains(env, "ambient-secret") || strings.Contains(env, "0.0.0.0") {
		t.Fatal("ambient service config survived")
	}
}

func TestCommsServiceHelper(t *testing.T) {
	mode := os.Getenv("CELL_COMMS_TEST_HELPER")
	if mode == "" {
		return
	}
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "fail" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(7)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestCommsServiceFailureCancelsPeer(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	result := runCommsPair(ctx, [][]string{{exe, "-test.run=^TestCommsServiceHelper$", "--", "wait"}, {exe, "-test.run=^TestCommsServiceHelper$", "--", "fail"}}, append(os.Environ(), "CELL_COMMS_TEST_HELPER=1"))
	if result.ExitCode != 7 || result.Err == nil || result.Uncertain || time.Since(start) > 4*time.Second {
		t.Fatalf("failed service did not cleanly stop its peer: %+v", result)
	}
}
