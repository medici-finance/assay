package cellcontainer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	// Image defaults are inherited unless the launch overrides them.
	s.ImageConfig = ImageConfig{Env: []string{"PATH=/usr/local/bin:/usr/bin:/bin"}, Entrypoint: []string{"/usr/local/bin/cell-entrypoint"}}
	s.Config.Env = append(s.Config.Env, s.ImageConfig.Env...)
	s.Config.Entrypoint = s.ImageConfig.Entrypoint
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
		case "image inspect":
			if s == nil {
				return []byte(`[{"Config":{}}]`), nil
			}
			return json.Marshal([]map[string]ImageConfig{{"Config": s.ImageConfig}})
		case "volume inspect", "network create":
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
	listed := strings.Repeat("b", 64)
	row := func(id, image string) []byte {
		b, _ := json.Marshal([]map[string]string{{"Id": id, "Image": image}})
		return b
	}
	_, p := fixture(t)
	type reply func() ([]byte, error)
	cases := map[string]struct {
		list, inspect, image reply
		want                 string
	}{
		"list":             {list: func() ([]byte, error) { return nil, errors.New("daemon unavailable") }, want: "daemon unavailable"},
		"ambiguous":        {list: func() ([]byte, error) { return []byte("bad-id"), nil }, want: "ambiguous container identity"},
		"inspect":          {inspect: func() ([]byte, error) { return nil, errors.New("inspect denied") }, want: "inspect denied"},
		"invalid-json":     {inspect: func() ([]byte, error) { return []byte("not-json"), nil }, want: "invalid Docker inspection"},
		"identity-changed": {inspect: func() ([]byte, error) { return row(strings.Repeat("c", 64), p.Image), nil }, want: "identity changed"},
		"two-rows": {inspect: func() ([]byte, error) {
			return json.Marshal([]map[string]string{{"Id": listed, "Image": p.Image}, {"Id": listed, "Image": p.Image}})
		}, want: "returned 2 containers"},
		"mutable-image":  {inspect: func() ([]byte, error) { return row(listed, "example:latest"), nil }, want: "no immutable image ID"},
		"image-inspect":  {image: func() ([]byte, error) { return nil, errors.New("image unavailable") }, want: "image unavailable"},
		"image-rows":     {image: func() ([]byte, error) { return []byte("[]"), nil }, want: "returned 0 images"},
		"image-two-rows": {image: func() ([]byte, error) { return []byte(`[{"Config":{}},{"Config":{}}]`), nil }, want: "returned 2 images"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := Engine{Output: func(a ...string) ([]byte, error) {
				switch {
				case a[1] == "ls" && tc.list != nil:
					return tc.list()
				case a[1] == "ls":
					return []byte(listed), nil
				case a[0] == "container" && tc.inspect != nil:
					return tc.inspect()
				case a[0] == "container":
					return row(listed, p.Image), nil
				case tc.image != nil:
					return tc.image()
				}
				return []byte(`[{"Config":{}}]`), nil
			}, Foreground: func(...string) error { t.Fatal("launched after failed inspection"); return nil }}
			err := e.Run(p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want refusal containing %q, got %v", tc.want, err)
			}
		})
	}
}

// Each case changes one setting and must be refused for that setting, so every guard in
// Validate has a test that fails when that guard alone is removed.
func TestReconnectRefusesChangedRuntime(t *testing.T) {
	empty := json.RawMessage(`{}`)
	cases := map[string]struct {
		mutate func(*Plan, *Inspection)
		want   string
	}{
		"model":            {func(_ *Plan, s *Inspection) { s.Config.Env = append(s.Config.Env, "CELL_MODEL=different") }, "differs at CELL_MODEL"},
		"harness":          {func(_ *Plan, s *Inspection) { s.Config.Env = append(s.Config.Env, "CELL_HARNESS=claude") }, "differs at CELL_HARNESS"},
		"name":             {func(_ *Plan, s *Inspection) { s.Name = "/other" }, "name/image differs"},
		"image":            {func(_ *Plan, s *Inspection) { s.Image = "sha256:" + strings.Repeat("c", 64) }, "name/image differs"},
		"owner":            {func(_ *Plan, s *Inspection) { s.Config.Labels["io.assay.cell"] = "other" }, "ownership is unverified"},
		"unlabelled":       {func(_ *Plan, s *Inspection) { s.Config.Labels = nil }, "ownership is unverified"},
		"labelled-adopted": {func(p *Plan, s *Inspection) { s.Config.Labels["io.assay.cell"] = "other"; p.AdoptID = s.ID }, "ownership is unverified"},
		"extra-env":        {func(_ *Plan, s *Inspection) { s.Config.Env = append(s.Config.Env, "EXTRA_SETTING=1") }, "unplanned setting EXTRA_SETTING"},
		"image-env":        {func(_ *Plan, s *Inspection) { s.Config.Env = append(s.Config.Env, "PATH=/elsewhere") }, "unplanned setting PATH"},
		"cmd":              {func(_ *Plan, s *Inspection) { s.Config.Cmd = []string{"sh"} }, "entrypoint arguments differ"},
		"entrypoint":       {func(_ *Plan, s *Inspection) { s.Config.Entrypoint = []string{"/bin/sh"} }, "entrypoint differs from the image"},
		"root-user":        {func(_ *Plan, s *Inspection) { s.Config.User = "0:0" }, "user differs"},
		"image-user":       {func(_ *Plan, s *Inspection) { s.Config.User = "" }, "user differs"},
		"no-tty":           {func(_ *Plan, s *Inspection) { s.Config.Tty = false }, "terminal settings differ"},
		"no-stdin":         {func(_ *Plan, s *Inspection) { s.Config.OpenStdin = false }, "terminal settings differ"},
		"writable-root":    {func(_ *Plan, s *Inspection) { s.HostConfig.ReadonlyRootfs = false }, "root filesystem is writable"},
		"privileged":       {func(_ *Plan, s *Inspection) { s.HostConfig.Privileged = true }, "is privileged"},
		"extra-capability": {func(_ *Plan, s *Inspection) { s.HostConfig.CapAdd = []string{"SYS_ADMIN"} }, "adds capabilities"},
		"kept-capability":  {func(_ *Plan, s *Inspection) { s.HostConfig.CapDrop = nil }, "does not drop all capabilities"},
		"device":           {func(_ *Plan, s *Inspection) { s.HostConfig.Devices = []json.RawMessage{empty} }, "has devices"},
		"device-request":   {func(_ *Plan, s *Inspection) { s.HostConfig.DeviceRequests = []json.RawMessage{empty} }, "requests devices"},
		"bind":             {func(_ *Plan, s *Inspection) { s.HostConfig.Binds = []json.RawMessage{json.RawMessage(`"/srv:/srv"`)} }, "unplanned bind mounts"},
		"group":            {func(_ *Plan, s *Inspection) { s.HostConfig.GroupAdd = []string{"0"} }, "adds groups"},
		"port":             {func(_ *Plan, s *Inspection) { s.HostConfig.PortBindings = map[string]json.RawMessage{"22/tcp": empty} }, "publishes ports"},
		"publish-all":      {func(_ *Plan, s *Inspection) { s.HostConfig.PublishAllPorts = true }, "publishes ports"},
		"pids":             {func(_ *Plan, s *Inspection) { s.HostConfig.PidsLimit = 0 }, "process limit differs"},
		"memory":           {func(_ *Plan, s *Inspection) { s.HostConfig.Memory = 0 }, "memory limit differs"},
		"cpus":             {func(_ *Plan, s *Inspection) { s.HostConfig.NanoCpus = 0 }, "CPU limit differs"},
		"pid-namespace":    {func(_ *Plan, s *Inspection) { s.HostConfig.PidMode = "host" }, "shares a process namespace"},
		"ipc-host":         {func(_ *Plan, s *Inspection) { s.HostConfig.IpcMode = "host" }, "shares an IPC namespace"},
		"ipc-container":    {func(_ *Plan, s *Inspection) { s.HostConfig.IpcMode = "container:" + strings.Repeat("d", 64) }, "shares an IPC namespace"},
		"userns":           {func(_ *Plan, s *Inspection) { s.HostConfig.UsernsMode = "host" }, "user namespace mode differs"},
		"security-opt":     {func(_ *Plan, s *Inspection) { s.HostConfig.SecurityOpt = nil }, "security options differ"},
		"tmpfs":            {func(_ *Plan, s *Inspection) { s.HostConfig.Tmpfs["/tmp"] = "rw,size=512m" }, "temporary filesystem differs"},
		"network-mode":     {func(_ *Plan, s *Inspection) { s.HostConfig.NetworkMode = "host" }, "network differs"},
		"extra-network":    {func(_ *Plan, s *Inspection) { s.NetworkSettings.Networks["other"] = empty }, "network differs"},
		"volume":           {func(_ *Plan, s *Inspection) { s.Mounts[0].Name = "other-workspace" }, "mount differs at /work"},
		"writable-key":     {func(_ *Plan, s *Inspection) { s.Mounts[len(s.Mounts)-1].RW = true }, "mount differs at /run/secrets/app.pem"},
		"duplicate-mount":  {func(_ *Plan, s *Inspection) { s.Mounts = append(s.Mounts, s.Mounts[0]) }, "duplicate mount target"},
		"missing-mount":    {func(_ *Plan, s *Inspection) { s.Mounts = s.Mounts[:len(s.Mounts)-1] }, "mount count differs"},
		"extra-mount": {func(_ *Plan, s *Inspection) {
			s.Mounts = append(s.Mounts, s.Mounts[len(s.Mounts)-1])
			s.Mounts[len(s.Mounts)-1].Destination = "/srv"
		}, "mount count differs"},
		"stopped":    {func(_ *Plan, s *Inspection) { s.State.Running = false; s.State.Status = "exited" }, "explicit recovery"},
		"paused":     {func(_ *Plan, s *Inspection) { s.State.Paused = true }, "explicit recovery"},
		"restarting": {func(_ *Plan, s *Inspection) { s.State.Restarting = true }, "explicit recovery"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, p := fixture(t)
			s := inspection(p)
			if err := p.Validate(s); err != nil {
				t.Fatalf("unchanged fixture refused: %v", err)
			}
			tc.mutate(p, s)
			e, _ := fakeEngine(t, s)
			e.Foreground = func(...string) error { t.Fatal("attached to incompatible container"); return nil }
			err := e.Run(p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want refusal containing %q, got %v", tc.want, err)
			}
		})
	}
}

// Down and status read the per-launch selection from the container itself, so a desk
// --model/--harness override can be reported and stopped. Everything else stays strict.
func TestRunningAcceptsPerLaunchSelectionOnly(t *testing.T) {
	c, registered := fixture(t)
	launched, err := c.Plan("sample", "desk", "", "override-model")
	if err != nil {
		t.Fatal(err)
	}
	s := inspection(launched)
	if err = registered.Validate(s); err == nil || !strings.Contains(err.Error(), "differs at CELL_MODEL") {
		t.Fatalf("attach must keep the strict model match, got %v", err)
	}
	e, _ := fakeEngine(t, s)
	p, got, err := c.Running(e, "sample", "desk")
	if err != nil {
		t.Fatalf("running override launch refused: %v", err)
	}
	if got.ID != s.ID || p.Model != "override-model" {
		t.Fatalf("got %s %s", got.ID, p.Model)
	}
	s.HostConfig.ReadonlyRootfs = false
	if _, _, err = c.Running(e, "sample", "desk"); err == nil || !strings.Contains(err.Error(), "root filesystem is writable") {
		t.Fatalf("running check skipped isolation: %v", err)
	}
	s.HostConfig.ReadonlyRootfs = true
	s.Config.Env = append(s.Config.Env, "CELL_MODEL=bad\x01model")
	if _, _, err = c.Running(e, "sample", "desk"); err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("running check accepted an invalid selection: %v", err)
	}
	none, _ := fakeEngine(t, nil)
	if p, s, err := c.Running(none, "sample", "desk"); p != nil || s != nil || err != nil {
		t.Fatalf("absent container: %v %v %v", p, s, err)
	}
}

func TestNetworkMustCarryCellLabel(t *testing.T) {
	_, p := fixture(t)
	for listing, want := range map[string]string{
		"":                                  "",
		p.Network + "\tsample\n":            "",
		p.Network + "\t\n":                  "not labelled for cell sample",
		p.Network + "\tother\n":             "not labelled for cell sample",
		p.Network + "\tsample\nx\tsample\n": "ambiguous container network",
		"assay-product-other\tsample\n":     "ambiguous container network",
	} {
		created := false
		e := Engine{Output: func(a ...string) ([]byte, error) {
			if a[1] == "create" {
				created = true
				if want := []string{"network", "create", "--label", "assay.product-cell=sample", p.Network}; !slices.Equal(a, want) {
					t.Fatalf("network created as %q, want %q", a, want)
				}
			}
			return []byte(listing), nil
		}}
		err := e.EnsureNetwork(p)
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatalf("%q: want %q, got %v", listing, want, err)
		}
		if created != (listing == "") {
			t.Fatalf("%q: created=%v", listing, created)
		}
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

// The launch argv is the only control on a fresh start, so it is pinned exactly.
func TestRunArgsAreExact(t *testing.T) {
	_, p := fixture(t)
	want := []string{"run", "--rm", "--sig-proxy=false", "--pull", "never", "--platform", "linux/amd64", "--read-only", "--user", "501:501", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--memory", "4g", "--cpus", "2", "--tmpfs", "/tmp:rw,nosuid,nodev,size=512m,mode=1777", "--name", "assay-sample-desk", "--network", "assay-product-sample", "--label", "io.assay.cell=sample", "--label", "io.assay.role=desk", "-it",
		"--mount", "type=volume,src=sample-desk,dst=/work", "--mount", "type=bind,src=" + p.Mounts[1].Source + ",dst=/cell-config,readonly", "--mount", "type=bind,src=" + p.Mounts[2].Source + ",dst=/incoming,readonly", "--mount", "type=bind,src=" + p.Mounts[3].Source + ",dst=/run/secrets/app.pem,readonly",
		"--env", "ASSAY_DESK=the-desk", "--env", "CELL_HARNESS=codex", "--env", "CELL_MODEL=test-model", "--env", "CELL_REPO=example-org/example-repo", "--env", "CELL_ROLE=desk", "--env", "CODEX_EXECUTION_BOUNDARY=workspace-write", "--env", "DESK_LOOP=the-desk", "--env", "DESK_PEM=/run/secrets/app.pem", "--env", "DESK_ROOTS=example-org/example-repo=/work/repo", "--env", "DESK_SESSION=container-sample-desk", "--env", "SAMPLE_DESK_PEM=/run/secrets/app.pem",
		p.Image, "handoff"}
	if got := p.RunArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("launch argv changed:\n got %q\nwant %q", got, want)
	}
}

func TestConfigValidateRefusals(t *testing.T) {
	cases := map[string]struct {
		mutate func(c *Config, cell *Cell, r *Role)
		want   string
	}{
		"schema":             {func(c *Config, _ *Cell, _ *Role) { c.Schema = "v0" }, "unsupported container configuration schema"},
		"remote-engine":      {func(c *Config, _ *Cell, _ *Role) { c.DockerHost = "tcp://localhost:2375" }, "explicit local Unix socket"},
		"relative-socket":    {func(c *Config, _ *Cell, _ *Role) { c.DockerHost = "unix://docker.sock" }, "explicit local Unix socket"},
		"mutable-image":      {func(c *Config, _ *Cell, _ *Role) { c.Image = "example:latest" }, "immutable sha256 image ID"},
		"repo":               {func(_ *Config, cell *Cell, _ *Role) { cell.Repo = "not a repo" }, "invalid cell name, repository or incoming path"},
		"incoming":           {func(_ *Config, cell *Cell, _ *Role) { cell.Incoming = "relative" }, "invalid cell name, repository or incoming path"},
		"host-lock":          {func(_ *Config, cell *Cell, _ *Role) { cell.HostLock = "relative.lock" }, "host_lock must be absolute"},
		"harness":            {func(_ *Config, _ *Cell, r *Role) { r.Harness = "other" }, "invalid role or harness"},
		"volume":             {func(_ *Config, _ *Cell, r *Role) { r.Volume = "/host/path" }, "distinct valid volume"},
		"config-path":        {func(_ *Config, _ *Cell, r *Role) { r.Config = "relative" }, "role mount paths must be absolute"},
		"key-path":           {func(_ *Config, _ *Cell, r *Role) { r.AppKey = "relative.pem" }, "role mount paths must be absolute"},
		"token-path":         {func(_ *Config, _ *Cell, r *Role) { r.ClaudeToken = "relative" }, "role mount paths must be absolute"},
		"startup-action":     {func(_ *Config, _ *Cell, r *Role) { r.StartupAction = "shell" }, "startup_action must be paused or handoff"},
		"sandbox":            {func(_ *Config, _ *Cell, r *Role) { r.Sandbox = "danger-full-access" }, "invalid codex_sandbox"},
		"sandbox-approval":   {func(_ *Config, _ *Cell, r *Role) { r.Sandbox = "container" }, "container-only sandbox needs an approval record"},
		"adopt-id":           {func(_ *Config, _ *Cell, r *Role) { r.AdoptID = "abc123" }, "adopt_container_id must be a full container ID"},
		"model-harness":      {func(_ *Config, _ *Cell, r *Role) { r.Models = map[string]string{"other": "m"} }, "invalid model pin harness"},
		"model-empty":        {func(_ *Config, _ *Cell, r *Role) { r.Models = map[string]string{"codex": ""} }, "invalid model pin"},
		"model-newline":      {func(_ *Config, _ *Cell, r *Role) { r.Models = map[string]string{"codex": "a\nb"} }, "invalid model pin"},
		"model-control":      {func(_ *Config, _ *Cell, r *Role) { r.Models = map[string]string{"codex": "a\x01b"} }, "invalid model pin"},
		"model-delete":       {func(_ *Config, _ *Cell, r *Role) { r.Models = map[string]string{"codex": "a\x7fb"} }, "invalid model pin"},
		"config-root":        {func(_ *Config, _ *Cell, r *Role) { r.Config = "/" }, "is the host root"},
		"incoming-root":      {func(_ *Config, cell *Cell, _ *Role) { cell.Incoming = "/" }, "is the host root"},
		"config-holds-key":   {func(_ *Config, _ *Cell, r *Role) { r.AppKey = filepath.Join(r.Config, "nested", "key.pem") }, "contains a protected path"},
		"config-holds-token": {func(_ *Config, _ *Cell, r *Role) { r.ClaudeToken = filepath.Join(r.Config, "token") }, "contains a protected path"},
		"incoming-holds-key": {func(_ *Config, cell *Cell, r *Role) { r.AppKey = filepath.Join(cell.Incoming, "key.pem") }, "contains a protected path"},
		"incoming-holds-socket": {func(c *Config, cell *Cell, _ *Role) {
			c.DockerHost = "unix://" + filepath.Join(cell.Incoming, "engine.sock")
		}, "contains a protected path"},
		"config-holds-socket": {func(c *Config, _ *Cell, r *Role) { c.DockerHost = "unix://" + filepath.Join(r.Config, "engine.sock") }, "contains a protected path"},
		"default-socket-dir":  {func(_ *Config, cell *Cell, _ *Role) { cell.Incoming = "/var/run" }, "contains a protected path"},
		// Paths that do not exist yet are refused lexically, without the filesystem.
		"absent-config-holds-key": {func(_ *Config, _ *Cell, r *Role) {
			r.Config, r.AppKey = "/nonexistent-example/config", "/nonexistent-example/config/nested/key.pem"
		}, "contains a protected path"},
		"absent-incoming-holds-socket": {func(c *Config, cell *Cell, _ *Role) {
			cell.Incoming, c.DockerHost = "/nonexistent-example/incoming", "unix:///nonexistent-example/incoming/engine.sock"
		}, "contains a protected path"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := fixture(t)
			cell := c.Cells["sample"]
			r := cell.Roles["desk"]
			tc.mutate(c, &cell, &r)
			cell.Roles["desk"] = r
			c.Cells["sample"] = cell
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want refusal containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestHandoffIsCoordinatorOnly(t *testing.T) {
	c, _ := fixture(t)
	cell := c.Cells["sample"]
	r := cell.Roles["desk"]
	r.Volume = "sample-worker"
	r.StartupAction = "paused"
	cell.Roles["worker"] = r
	c.Cells["sample"] = cell
	if err := c.Validate(); err != nil {
		t.Fatalf("paused worker refused: %v", err)
	}
	r.StartupAction = "handoff"
	cell.Roles["worker"] = r
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "handoff is coordinator-only") {
		t.Fatalf("worker handoff accepted: %v", err)
	}
}

func TestSandboxApprovalRecordAdmitsContainerBoundary(t *testing.T) {
	c, _ := fixture(t)
	cell := c.Cells["sample"]
	r := cell.Roles["desk"]
	r.Sandbox, r.BoundaryApproval = "container", "recorded decision"
	cell.Roles["desk"] = r
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	p, err := c.Plan("sample", "desk", "", "")
	if err != nil || p.Env["CODEX_EXECUTION_BOUNDARY"] != "container" {
		t.Fatalf("%v %v", err, p)
	}
}

// mountAlias returns a second spelling of dir that a string comparison cannot equate with it.
type mountAlias struct {
	name string
	make func(t *testing.T, dir string) string
}

var mountAliases = []mountAlias{
	// A symlinked parent: Lstat of the mount source still sees a directory.
	{"symlink", func(t *testing.T, dir string) string {
		parent := filepath.Dir(dir) + "-alias"
		if err := os.Symlink(filepath.Dir(dir), parent); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(parent, filepath.Base(dir))
	}},
	// A case-variant spelling names the same directory on a case-insensitive filesystem (the
	// default on macOS); symlink resolution does not fold it, so only file identity sees it.
	{"case-variant", func(t *testing.T, dir string) string {
		alias := filepath.Join(filepath.Dir(dir), strings.ToUpper(filepath.Base(dir)))
		if _, err := os.Stat(alias); err != nil {
			t.Skip("temporary filesystem is case-sensitive")
		}
		return alias
	}},
}

// identityFixture is a paused role whose forge key lives in dir/cfg, plus an alias of dir/cfg.
func identityFixture(t *testing.T, alias mountAlias) (c *Config, real, other, key string) {
	t.Helper()
	c, _ = fixture(t)
	d := t.TempDir()
	real = filepath.Join(d, "cfg")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	key = filepath.Join(real, "key.pem")
	if err := os.WriteFile(key, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	other = alias.make(t, real)
	cell := c.Cells["sample"]
	r := cell.Roles["desk"]
	r.AppKey, r.StartupAction = key, "paused"
	cell.Roles["desk"] = r
	c.Cells["sample"] = cell
	return c, real, other, key
}

func setMounts(c *Config, config, incoming string) {
	cell := c.Cells["sample"]
	r := cell.Roles["desk"]
	if config != "" {
		r.Config = config
	}
	if incoming != "" {
		cell.Incoming = incoming
	}
	cell.Roles["desk"] = r
	c.Cells["sample"] = cell
}

func wantProtected(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "contains a protected path") {
		t.Fatalf("%s: alias of a directory holding a credential accepted: %v", what, err)
	}
}

// Containment compares file identity, so an alternate spelling of a directory that holds a
// credential is refused by every entrypoint: Config.Validate, Plan and CheckFiles.
func TestMountContainmentComparesFileIdentity(t *testing.T) {
	for _, alias := range mountAliases {
		t.Run(alias.name, func(t *testing.T) {
			t.Run("validate-config", func(t *testing.T) {
				c, _, other, _ := identityFixture(t, alias)
				setMounts(c, other, "")
				wantProtected(t, "Config.Validate", c.Validate())
			})
			t.Run("validate-incoming", func(t *testing.T) {
				c, _, other, _ := identityFixture(t, alias)
				setMounts(c, "", other)
				wantProtected(t, "Config.Validate", c.Validate())
			})
			t.Run("plan", func(t *testing.T) {
				c, _, other, _ := identityFixture(t, alias)
				setMounts(c, other, "")
				_, err := c.Plan("sample", "desk", "", "")
				wantProtected(t, "Plan", err)
			})
			t.Run("check-files", func(t *testing.T) {
				c, _, other, key := identityFixture(t, alias)
				p, err := c.Plan("sample", "desk", "", "")
				if err != nil {
					t.Fatal(err)
				}
				p.Mounts[1].Source, p.Protected = other, []string{key}
				wantProtected(t, "CheckFiles", p.CheckFiles())
			})
			t.Run("socket-not-yet-present", func(t *testing.T) {
				c, real, other, _ := identityFixture(t, alias)
				// Move the key out so that only the engine socket's directory is protected here.
				key := filepath.Join(t.TempDir(), "key.pem")
				if err := os.WriteFile(key, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				cell := c.Cells["sample"]
				r := cell.Roles["desk"]
				r.AppKey = key
				cell.Roles["desk"] = r
				c.DockerHost = "unix://" + filepath.Join(other, "engine.sock")
				setMounts(c, "", real)
				wantProtected(t, "Config.Validate", c.Validate())
			})
		})
	}
}

// A credential configured through a symlink is compared at its real location too.
func TestCredentialReachedThroughSymlinkIsProtected(t *testing.T) {
	c, real, _, key := identityFixture(t, mountAliases[0])
	keys := filepath.Join(t.TempDir(), "keys")
	if err := os.Mkdir(keys, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(key, filepath.Join(keys, "app.pem")); err != nil {
		t.Fatal(err)
	}
	p, err := c.Plan("sample", "desk", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p.Mounts[1].Source, p.Protected = real, []string{filepath.Join(keys, "app.pem")}
	wantProtected(t, "CheckFiles", p.CheckFiles())
}

// The host root is refused under any spelling, with or without protected paths.
func TestHostRootAliasRefused(t *testing.T) {
	c, _ := fixture(t)
	root := filepath.Join(t.TempDir(), "root-alias")
	if err := os.Symlink("/", root); err != nil {
		t.Fatal(err)
	}
	setMounts(c, root, "")
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "is the host root") {
		t.Fatalf("Config.Validate accepted an alias of the host root: %v", err)
	}
	if _, err := c.Plan("sample", "desk", "", ""); err == nil || !strings.Contains(err.Error(), "is the host root") {
		t.Fatalf("Plan accepted an alias of the host root: %v", err)
	}
	_, p := fixture(t)
	p.Mounts[1].Source = root + "/." // a trailing dot makes Lstat follow the alias
	p.Protected = nil
	if err := p.CheckFiles(); err == nil || !strings.Contains(err.Error(), "is the host root") {
		t.Fatalf("CheckFiles accepted an alias of the host root: %v", err)
	}
}

// A directory mount may not be, or contain, the operator's home directory.
func TestMountMayNotContainHome(t *testing.T) {
	c, _ := fixture(t)
	t.Setenv("HOME", filepath.Join(c.Cells["sample"].Incoming, "home"))
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "contains a protected path") {
		t.Fatalf("mount containing the home directory accepted: %v", err)
	}
}

func TestPlanRefusesControlBytesInModel(t *testing.T) {
	c, _ := fixture(t)
	for _, m := range []string{"a\x01b", "a\x1bb", "a\x7fb", "a\tb"} {
		if _, err := c.Plan("sample", "desk", "", m); err == nil || !strings.Contains(err.Error(), "control character") {
			t.Fatalf("%q: %v", m, err)
		}
	}
	if _, err := c.Plan("sample", "desk", "", "model-with spaces'and;quotes"); err != nil {
		t.Fatalf("printable model refused: %v", err)
	}
}

// A socket that does not exist yet is compared through its symlink-resolved parent, so a mount
// of a directory above that real parent is refused too.
func TestMissingSocketBehindSymlinkIsProtected(t *testing.T) {
	c, _ := fixture(t)
	d := t.TempDir()
	inner := filepath.Join(d, "outer", "inner")
	if err := os.MkdirAll(inner, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(inner, link); err != nil {
		t.Fatal(err)
	}
	c.DockerHost = "unix://" + filepath.Join(link, "engine.sock")
	setMounts(c, "", filepath.Join(d, "outer"))
	wantProtected(t, "Config.Validate", c.Validate())
}

// check validates an existing container, not only the files, image and volume.
func TestCheckValidatesExistingContainer(t *testing.T) {
	_, p := fixture(t)
	s := inspection(p)
	s.HostConfig.Privileged = true
	e, _ := fakeEngine(t, s)
	if err := e.Check(p); err == nil || !strings.Contains(err.Error(), "is privileged") {
		t.Fatalf("check accepted an incompatible running container: %v", err)
	}
}
