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
		Labels         map[string]string
	}
	HostConfig struct {
		ReadonlyRootfs, Privileged    bool
		CapDrop, CapAdd, SecurityOpt  []string
		NetworkMode, PidMode, IpcMode string
		Binds, Devices                []json.RawMessage
		PidsLimit, Memory, NanoCpus   int64
		Tmpfs                         map[string]string
	}
	Mounts []struct {
		Type, Name, Source, Destination string
		RW                              bool
	}
	NetworkSettings struct{ Networks map[string]json.RawMessage }
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
	if len(rows) != 1 || rows[0].ID != ids[0] {
		return nil, fmt.Errorf("Docker inspection identity changed")
	}
	return &rows[0], nil
}

// Validate checks actual runtime settings even for labelled containers. An exact
// migration ID admits an unlabelled pilot container, never a differently labelled one.
func (p *Plan) Validate(s *Inspection) error {
	fail := func(what string) error {
		return fmt.Errorf("container %s %s; refusing to attach or stop it", p.Name, what)
	}
	if s.Name != "/"+p.Name || s.Image != p.Image {
		return fail("name/image differs from configuration")
	}
	labels := s.Config.Labels
	labelled := labels["io.assay.cell"] == p.CellName && labels["io.assay.role"] == p.RoleKey
	adopted := labels["io.assay.cell"] == "" && labels["io.assay.role"] == "" && p.AdoptID != "" && s.ID == p.AdoptID
	if !labelled && !adopted {
		return fail("ownership is unverified (legacy migration requires adopt_container_id)")
	}
	env := map[string]string{}
	for _, v := range s.Config.Env {
		k, v, ok := strings.Cut(v, "=")
		if ok {
			env[k] = v
		}
	}
	for k, v := range p.Env {
		if env[k] != v {
			return fail("configuration differs at " + k + " (restart explicitly to change it)")
		}
	}
	if !slices.Equal(s.Config.Cmd, []string{p.Action}) || s.Config.User != "501:501" || !s.Config.Tty || !s.Config.OpenStdin {
		return fail("entrypoint arguments/user/terminal differs")
	}
	h := s.HostConfig
	if !h.ReadonlyRootfs || h.Privileged || len(h.CapAdd) > 0 || !slices.Equal(h.CapDrop, []string{"ALL"}) || len(h.Devices) > 0 || len(h.Binds) > 0 || h.PidsLimit != 256 || h.Memory != 4*1024*1024*1024 || h.NanoCpus != 2*1000*1000*1000 || h.PidMode != "" || h.IpcMode == "host" {
		return fail("runtime isolation differs")
	}
	if len(h.SecurityOpt) != 1 || (h.SecurityOpt[0] != "no-new-privileges" && h.SecurityOpt[0] != "no-new-privileges=true") {
		return fail("security options differ")
	}
	if len(h.Tmpfs) != 1 || h.Tmpfs["/tmp"] != "rw,nosuid,nodev,size=512m,mode=1777" {
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
func (e Engine) EnsureNetwork(p *Plan) error {
	b, err := e.Output("network", "ls", "--filter", "name=^"+p.Network+"$", "--format", "{{.Name}}")
	if err != nil {
		return err
	}
	names := strings.Fields(string(b))
	if len(names) == 0 {
		_, err = e.Output("network", "create", "--label", "assay.product-cell="+p.CellName, p.Network)
		return err
	}
	if len(names) != 1 || names[0] != p.Network {
		return fmt.Errorf("ambiguous container network")
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
