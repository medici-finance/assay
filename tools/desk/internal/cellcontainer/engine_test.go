package cellcontainer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Config, *Plan) {
	t.Helper()
	d := t.TempDir()
	for _, v := range []string{"config", "incoming"} {
		if err := os.Mkdir(filepath.Join(d, v), 0700); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(d, "key.pem")
	if err := os.WriteFile(key, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	c := &Config{Schema: "cell-containers-v1", DockerHost: "unix:///tmp/example-docker.sock", Image: "sha256:" + strings.Repeat("a", 64), Cells: map[string]Cell{"sample": {Repo: "example-org/example-repo", Incoming: filepath.Join(d, "incoming"), Roles: map[string]Role{"desk": {Harness: "codex", Models: map[string]string{"codex": "test-model"}, Volume: "sample-desk", Config: filepath.Join(d, "config"), AppKey: key, StartupAction: "handoff"}}}}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	p, err := c.Plan("sample", "desk", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return c, p
}
func inspection(p *Plan) *Inspection {
	s := &Inspection{ID: strings.Repeat("b", 64), Name: "/" + p.Name, Image: p.Image}
	s.State.Running = true
	s.State.Status = "running"
	s.Config.User = "501:501"
	s.Config.Tty = true
	s.Config.OpenStdin = true
	s.Config.Cmd = []string{p.Action}
	s.Config.Labels = map[string]string{"io.assay.cell": p.CellName, "io.assay.role": p.RoleKey}
	for k, v := range p.Env {
		s.Config.Env = append(s.Config.Env, k+"="+v)
	}
	s.HostConfig.ReadonlyRootfs = true
	s.HostConfig.CapDrop = []string{"ALL"}
	s.HostConfig.SecurityOpt = []string{"no-new-privileges"}
	s.HostConfig.PidsLimit = 256
	s.HostConfig.Memory = 4 * 1024 * 1024 * 1024
	s.HostConfig.NanoCpus = 2 * 1000 * 1000 * 1000
	s.HostConfig.NetworkMode = p.Network
	s.HostConfig.Tmpfs = map[string]string{"/tmp": "rw,nosuid,nodev,size=512m,mode=1777"}
	s.NetworkSettings.Networks = map[string]json.RawMessage{p.Network: json.RawMessage(`{}`)}
	// Marshal through Docker's wire format instead of repeating the anonymous mount type.
	var mounts []map[string]any
	for _, m := range p.Mounts {
		mounts = append(mounts, map[string]any{"Type": m.Type, "Name": m.Source, "Source": m.Source, "Destination": m.Target, "RW": !m.ReadOnly})
	}
	b, _ := json.Marshal(mounts)
	_ = json.Unmarshal(b, &s.Mounts)
	return s
}
func fakeEngine(t *testing.T, s *Inspection) (Engine, *[][]string) {
	t.Helper()
	calls := [][]string{}
	e := Engine{Output: func(a ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), a...))
		switch strings.Join(a[:2], " ") {
		case "container ls":
			if s == nil {
				return nil, nil
			}
			return []byte(s.ID + "\n"), nil
		case "container inspect":
			return json.Marshal([]*Inspection{s})
		case "image inspect", "volume inspect", "network create":
			return []byte("[]"), nil
		case "network ls":
			return nil, nil
		}
		t.Fatalf("unexpected command: %v", a)
		return nil, nil
	}, Foreground: func(a ...string) error { calls = append(calls, append([]string(nil), a...)); return nil }}
	return e, &calls
}
func TestReconnectPreservesRunningContainer(t *testing.T) {
	_, p := fixture(t)
	s := inspection(p)
	e, calls := fakeEngine(t, s)
	if err := e.Run(p); err != nil {
		t.Fatal(err)
	}
	got := (*calls)[len(*calls)-1]
	want := []string{"attach", "--sig-proxy=false", s.ID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, a := range *calls {
		if a[0] == "run" || a[0] == "stop" {
			t.Fatal("reconnect mutated existing container")
		}
	}
}
func TestInspectionFailuresNeverLaunch(t *testing.T) {
	_, p := fixture(t)
	for _, stage := range []string{"list", "inspect", "invalid-json", "ambiguous"} {
		t.Run(stage, func(t *testing.T) {
			e := Engine{Output: func(a ...string) ([]byte, error) {
				if a[1] == "ls" {
					if stage == "list" {
						return nil, errors.New("daemon unavailable")
					}
					if stage == "ambiguous" {
						return []byte("bad-id"), nil
					}
					return []byte(strings.Repeat("b", 64)), nil
				}
				if stage == "invalid-json" {
					return []byte("not-json"), nil
				}
				return nil, errors.New("inspect denied")
			}, Foreground: func(...string) error { t.Fatal("launched after failed inspection"); return nil }}
			if err := e.Run(p); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}
func TestReconnectRefusesChangedRuntime(t *testing.T) {
	cases := map[string]func(*Inspection){
		"model":            func(s *Inspection) { s.Config.Env = append(s.Config.Env, "CELL_MODEL=different") },
		"harness":          func(s *Inspection) { s.Config.Env = append(s.Config.Env, "CELL_HARNESS=claude") },
		"owner":            func(s *Inspection) { s.Config.Labels["io.assay.cell"] = "other" },
		"image":            func(s *Inspection) { s.Image = "sha256:" + strings.Repeat("c", 64) },
		"volume":           func(s *Inspection) { s.Mounts[0].Name = "other-workspace" },
		"writable-key":     func(s *Inspection) { s.Mounts[len(s.Mounts)-1].RW = true },
		"extra-mount":      func(s *Inspection) { s.Mounts = append(s.Mounts, s.Mounts[0]) },
		"privileged":       func(s *Inspection) { s.HostConfig.Privileged = true },
		"extra-capability": func(s *Inspection) { s.HostConfig.CapAdd = []string{"SYS_ADMIN"} },
		"extra-network":    func(s *Inspection) { s.NetworkSettings.Networks["other"] = json.RawMessage(`{}`) },
		"stopped":          func(s *Inspection) { s.State.Running = false; s.State.Status = "exited" },
		"paused":           func(s *Inspection) { s.State.Paused = true },
		"unlabelled":       func(s *Inspection) { s.Config.Labels = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, p := fixture(t)
			s := inspection(p)
			mutate(s)
			e, _ := fakeEngine(t, s)
			e.Foreground = func(...string) error { t.Fatal("attached to incompatible container"); return nil }
			if err := e.Run(p); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}
func TestLegacyAdoptionRequiresExactIDAndRuntime(t *testing.T) {
	_, p := fixture(t)
	s := inspection(p)
	s.Config.Labels = nil
	p.AdoptID = s.ID
	if err := p.Validate(s); err != nil {
		t.Fatal(err)
	}
	s.HostConfig.Privileged = true
	if err := p.Validate(s); err == nil {
		t.Fatal("adoption bypassed isolation")
	}
	s.HostConfig.Privileged = false
	p.AdoptID = strings.Repeat("c", 64)
	if err := p.Validate(s); err == nil {
		t.Fatal("adopted wrong ID")
	}
}
func TestFreshStartPreservesIsolationAndCredentials(t *testing.T) {
	_, p := fixture(t)
	e, calls := fakeEngine(t, nil)
	if err := e.Run(p); err != nil {
		t.Fatal(err)
	}
	a := strings.Join((*calls)[len(*calls)-1], " ")
	for _, s := range []string{"--pull never", "--read-only", "--cap-drop ALL", "--security-opt no-new-privileges", "--pids-limit 256", "--memory 4g", "--cpus 2", "dst=/run/secrets/app.pem,readonly", "io.assay.cell=sample", "CELL_MODEL=test-model"} {
		if !strings.Contains(a, s) {
			t.Errorf("missing %q", s)
		}
	}
	if !strings.HasSuffix(a, p.Image+" handoff") {
		t.Fatal(a)
	}
}
func TestPausedDoesNotMountForgeKey(t *testing.T) {
	c, _ := fixture(t)
	cell := c.Cells["sample"]
	r := cell.Roles["desk"]
	r.StartupAction = "paused"
	cell.Roles["desk"] = r
	c.Cells["sample"] = cell
	p, err := c.Plan("sample", "desk", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(p.RunArgs(), " "), "app.pem") {
		t.Fatal("paused mounts forge key")
	}
}
func TestSecretsMustBePrivateRegularFiles(t *testing.T) {
	_, p := fixture(t)
	path := p.Mounts[len(p.Mounts)-1].Source
	_ = os.Chmod(path, 0644)
	if err := p.CheckFiles(); err == nil {
		t.Fatal("accepted public key file")
	}
	_ = os.Chmod(path, 0600)
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	p.Mounts[len(p.Mounts)-1].Source = link
	if err := p.CheckFiles(); err == nil {
		t.Fatal("accepted symlink")
	}
}
func TestNativeConfigRejectsRemoteEngineAndDuplicateVolumes(t *testing.T) {
	c, _ := fixture(t)
	c.DockerHost = "tcp://localhost:2375"
	if err := c.Validate(); err == nil {
		t.Fatal("accepted remote engine")
	}
	c.DockerHost = "unix:///tmp/docker.sock"
	cell := c.Cells["sample"]
	cell.Roles["worker"] = cell.Roles["desk"]
	c.Cells["sample"] = cell
	if err := c.Validate(); err == nil {
		t.Fatal("accepted shared volume")
	}
}

func TestMissingWorkspaceNeverCreatesAnEmptyVolume(t *testing.T) {
	_, p := fixture(t)
	e, _ := fakeEngine(t, nil)
	original := e.Output
	e.Output = func(a ...string) ([]byte, error) {
		if a[0] == "volume" {
			return nil, errors.New("missing volume")
		}
		return original(a...)
	}
	e.Foreground = func(...string) error { t.Fatal("started without existing workspace"); return nil }
	if err := e.Run(p); err == nil {
		t.Fatal("expected volume refusal")
	}
}
func TestUnknownConfigurationKeyRefused(t *testing.T) {
	c, _ := fixture(t)
	b, _ := json.Marshal(c)
	b = append([]byte(`{"typo_security_key":true,`), b[1:]...)
	path := filepath.Join(t.TempDir(), "cells.json")
	_ = os.WriteFile(path, b, 0600)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown key silently ignored")
	}
}
