package cellcontainer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Engine is injected in tests. All commands target the explicitly configured local
// socket; a failed listing/inspection is never interpreted as an absent container.
type Engine struct {
	Output     func(...string) ([]byte, error)
	Foreground func(...string) error
}

func CleanEnv() []string {
	return []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "TERM=" + os.Getenv("TERM"), "KUBECONFIG=/dev/null"}
}
func Docker(host string) Engine {
	return Engine{
		Output: func(args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "docker", append([]string{"--host", host}, args...)...)
			cmd.Env = CleanEnv()
			b, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(string(b)))
			}
			return b, nil
		},
		Foreground: func(args ...string) error {
			cmd := exec.Command("docker", append([]string{"--host", host}, args...)...)
			cmd.Env = CleanEnv()
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		},
	}
}

type Inspection struct {
	ID          string `json:"Id"`
	Name, Image string
	State       struct {
		Status                      string
		Running, Paused, Restarting bool
	}
	Config struct {
		User           string
		Tty, OpenStdin bool
		Env            []string
		Cmd            []string
		Entrypoint     []string
		Labels         map[string]string
	}
	HostConfig struct {
		ReadonlyRootfs, Privileged, PublishAllPorts bool
		CapDrop, CapAdd, SecurityOpt, GroupAdd      []string
		NetworkMode, PidMode, IpcMode, UsernsMode   string
		Binds, Devices, DeviceRequests              []json.RawMessage
		PortBindings                                map[string]json.RawMessage
		PidsLimit, Memory, NanoCpus                 int64
		Tmpfs                                       map[string]string
	}
	Mounts []struct {
		Type, Name, Source, Destination string
		RW                              bool
	}
	NetworkSettings struct{ Networks map[string]json.RawMessage }
	// ImageConfig is the container image's own configuration, read by Inspect, so that
	// Validate can tell image defaults from settings added at launch.
	ImageConfig ImageConfig `json:"-"`
}

// ImageConfig holds the image defaults a container inherits when nothing overrides them.
type ImageConfig struct {
	Env        []string
	Entrypoint []string
}

func (e Engine) Inspect(p *Plan) (*Inspection, error) {
	b, err := e.Output("container", "ls", "--all", "--no-trunc", "--filter", "name=^/"+p.Name+"$", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(b))
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) != 1 || !idRE.MatchString(ids[0]) {
		return nil, fmt.Errorf("ambiguous container identity for %s", p.Name)
	}
	b, err = e.Output("container", "inspect", ids[0])
	if err != nil {
		return nil, err
	}
	var rows []Inspection
	if err = json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("invalid Docker inspection: %w", err)
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("Docker inspection returned %d containers", len(rows))
	}
	if rows[0].ID != ids[0] {
		return nil, fmt.Errorf("Docker inspection identity changed")
	}
	if !imageRE.MatchString(rows[0].Image) {
		return nil, fmt.Errorf("container %s has no immutable image ID", p.Name)
	}
	b, err = e.Output("image", "inspect", rows[0].Image)
	if err != nil {
		return nil, err
	}
	var images []struct{ Config ImageConfig }
	if err = json.Unmarshal(b, &images); err != nil {
		return nil, fmt.Errorf("invalid Docker image inspection: %w", err)
	}
	if len(images) != 1 {
		return nil, fmt.Errorf("Docker image inspection returned %d images", len(images))
	}
	rows[0].ImageConfig = images[0].Config
	return &rows[0], nil
}

// Running finds the role's container and validates it against the harness and model it was
// actually launched with, so a per-launch desk --harness/--model override can still be reported
// and stopped. Ownership, image, mounts and runtime isolation are checked exactly as for attach;
// only the selection keys are read from the container. Attach keeps the strict Validate.
func (c *Config) Running(e Engine, name, key string) (*Plan, *Inspection, error) {
	s, err := e.Inspect(&Plan{Name: ContainerName(name, key)})
	if err != nil || s == nil {
		return nil, nil, err
	}
	env := envMap(s.Config.Env)
	p, err := c.Plan(name, key, env["CELL_HARNESS"], env["CELL_MODEL"])
	if err != nil {
		return nil, nil, fmt.Errorf("container %s: %w; refusing to stop it", ContainerName(name, key), err)
	}
	if err = p.Validate(s); err != nil {
		return nil, nil, err
	}
	return p, s, nil
}

func envMap(entries []string) map[string]string {
	env := map[string]string{}
	for _, v := range entries {
		if k, v, ok := strings.Cut(v, "="); ok {
			env[k] = v
		}
	}
	return env
}

// Validate checks actual runtime settings even for labelled containers. An exact
// migration ID admits an unlabelled pilot container, never a differently labelled one.
// Each refusal names the one setting that differs.
func (p *Plan) Validate(s *Inspection) error {
	fail := func(what string) error {
		return fmt.Errorf("container %s %s; refusing to attach or stop it", p.Name, what)
	}
	if s.Name != "/"+p.Name || s.Image != p.Image {
		return fail("name/image differs from configuration")
	}
	labels := s.Config.Labels
	labelled := labels["io.assay.cell"] == p.CellName && labels["io.assay.role"] == p.RoleKey
	unlabelled := labels["io.assay.cell"] == "" && labels["io.assay.role"] == ""
	adopted := unlabelled && p.AdoptID != "" && s.ID == p.AdoptID
	if !labelled && !adopted {
		return fail("ownership is unverified (legacy migration requires adopt_container_id)")
	}
	env := envMap(s.Config.Env)
	for k, v := range p.Env {
		if env[k] != v {
			return fail("configuration differs at " + k + " (restart explicitly to change it)")
		}
	}
	image := envMap(s.ImageConfig.Env)
	for k, v := range env {
		if _, planned := p.Env[k]; planned {
			continue
		}
		if iv, ok := image[k]; !ok || iv != v {
			return fail("environment has an unplanned setting " + k)
		}
	}
	h := s.HostConfig
	switch {
	case !slices.Equal(s.Config.Cmd, []string{p.Action}):
		return fail("entrypoint arguments differ")
	case !slices.Equal(s.Config.Entrypoint, s.ImageConfig.Entrypoint):
		return fail("entrypoint differs from the image")
	case s.Config.User != ContainerUser:
		return fail("user differs")
	case !s.Config.Tty || !s.Config.OpenStdin:
		return fail("terminal settings differ")
	case !h.ReadonlyRootfs:
		return fail("root filesystem is writable")
	case h.Privileged:
		return fail("is privileged")
	case len(h.CapAdd) > 0:
		return fail("adds capabilities")
	case !slices.Equal(h.CapDrop, []string{"ALL"}):
		return fail("does not drop all capabilities")
	case len(h.Devices) > 0:
		return fail("has devices")
	case len(h.DeviceRequests) > 0:
		return fail("requests devices (such as GPUs)")
	case len(h.Binds) > 0:
		return fail("has unplanned bind mounts")
	case len(h.GroupAdd) > 0:
		return fail("adds groups")
	case len(h.PortBindings) > 0 || h.PublishAllPorts:
		return fail("publishes ports")
	case h.PidsLimit != PidsLimit:
		return fail("process limit differs")
	case h.Memory != MemoryBytes:
		return fail("memory limit differs")
	case h.NanoCpus != NanoCPUs:
		return fail("CPU limit differs")
	case h.PidMode != "":
		return fail("shares a process namespace")
	case h.IpcMode == "host" || strings.HasPrefix(h.IpcMode, "container:"):
		return fail("shares an IPC namespace")
	case h.UsernsMode != "":
		return fail("user namespace mode differs")
	}
	if len(h.SecurityOpt) != 1 || (h.SecurityOpt[0] != "no-new-privileges" && h.SecurityOpt[0] != "no-new-privileges=true") {
		return fail("security options differ")
	}
	if len(h.Tmpfs) != 1 || h.Tmpfs["/tmp"] != TmpfsOptions {
		return fail("temporary filesystem differs")
	}
	if h.NetworkMode != p.Network || len(s.NetworkSettings.Networks) != 1 {
		return fail("network differs")
	}
	if _, ok := s.NetworkSettings.Networks[p.Network]; !ok {
		return fail("network differs")
	}
	mounts := map[string]Mount{}
	for _, m := range s.Mounts {
		if m.Type == "tmpfs" && m.Destination == "/tmp" {
			continue
		}
		source := m.Source
		if m.Type == "volume" {
			source = m.Name
		}
		if _, ok := mounts[m.Destination]; ok {
			return fail("duplicate mount target")
		}
		mounts[m.Destination] = Mount{m.Type, source, m.Destination, !m.RW}
	}
	if len(mounts) != len(p.Mounts) {
		return fail("mount count differs")
	}
	for _, m := range p.Mounts {
		if mounts[m.Target] != m {
			return fail("mount differs at " + m.Target)
		}
	}
	return nil
}

// EnsureNetwork creates the cell network or accepts an existing one only when it carries this
// cell's label. The network is a naming boundary, not an egress or host-isolation boundary.
func (e Engine) EnsureNetwork(p *Plan) error {
	b, err := e.Output("network", "ls", "--filter", "name=^"+p.Network+"$", "--format", "{{.Name}}\t{{.Label \"assay.product-cell\"}}")
	if err != nil {
		return err
	}
	rows := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(rows) == 1 && rows[0] == "" {
		_, err = e.Output("network", "create", "--label", "assay.product-cell="+p.CellName, p.Network)
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("ambiguous container network")
	}
	name, label, _ := strings.Cut(rows[0], "\t")
	if name != p.Network {
		return fmt.Errorf("ambiguous container network")
	}
	if label != p.CellName {
		return fmt.Errorf("network %s is not labelled for cell %s", p.Network, p.CellName)
	}
	return nil
}
func (e Engine) Check(p *Plan) error {
	if err := p.CheckFiles(); err != nil {
		return err
	}
	if _, err := e.Output("image", "inspect", p.Image); err != nil {
		return err
	}
	// Never implicitly create an empty work volume in place of a missing workspace.
	for _, m := range p.Mounts {
		if m.Type == "volume" {
			if _, err := e.Output("volume", "inspect", m.Source); err != nil {
				return err
			}
		}
	}
	s, err := e.Inspect(p)
	if err != nil {
		return err
	}
	if s != nil {
		return p.Validate(s)
	}
	return nil
}

// Run uses the immutable ID after inspection, so replacement of a container name
// cannot redirect an attach to a different process. Docker's unique name provides
// the final launch race guard. A collision stays a visible error, never a removal.
func (e Engine) Run(p *Plan) error {
	s, err := e.Inspect(p)
	if err != nil {
		return err
	}
	if s != nil {
		if err = p.Validate(s); err != nil {
			return err
		}
		if !s.State.Running || s.State.Paused || s.State.Restarting {
			return fmt.Errorf("container %s is %s; explicit recovery is required", p.Name, s.State.Status)
		}
		return e.Foreground("attach", "--sig-proxy=false", s.ID)
	}
	if err = e.Check(p); err != nil {
		return err
	}
	if err = e.EnsureNetwork(p); err != nil {
		return err
	}
	return e.Foreground(p.RunArgs()...)
}
