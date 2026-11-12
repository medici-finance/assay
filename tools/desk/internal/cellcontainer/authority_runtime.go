package cellcontainer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

// AuthorityRuntime has no injected engine or admission callback. Its public
// construction accepts only an already validated immutable closed plan.
type AuthorityRuntime struct{ plan *AuthorityPlan }
type AuthorityHandle struct {
	runtime *AuthorityRuntime
	id      string
	intent  celllaunch.AuthorityLaunch
	hostPID atomic.Int64
}

func (h *AuthorityHandle) Intent() celllaunch.AuthorityLaunch {
	if h == nil {
		return celllaunch.AuthorityLaunch{}
	}
	return h.intent
}

// Configuration returns detached operator metadata, not signing or current authority.
func (r *AuthorityRuntime) Configuration() AuthorityConfig {
	if r == nil || r.plan == nil {
		return AuthorityConfig{}
	}
	c := r.plan.config
	c.Volumes.Grants = append([]AuthorityGrantVolume(nil), c.Volumes.Grants...)
	return c
}

func (h *AuthorityHandle) HostPID() int64 {
	if h == nil {
		return 0
	}
	return h.hostPID.Load()
}

func NewAuthorityRuntime(p *AuthorityPlan) (*AuthorityRuntime, error) {
	if p == nil {
		return nil, ErrAuthorityProfile
	}
	if _, e := NewAuthorityPlan(p.config); e != nil {
		return nil, ErrAuthorityProfile
	}
	return &AuthorityRuntime{plan: p}, nil
}

func (r *AuthorityRuntime) command(ctx context.Context, args []string, input []byte) (*exec.Cmd, error) {
	if r == nil || r.plan == nil || ctx == nil || ctx.Err() != nil || len(input) > celllaunch.MaxRecordBytes {
		return nil, ErrAuthorityProfile
	}
	c := r.plan.config
	if !validAuthorityToolPath(c.DockerBinary) || !validAuthorityToolPath(c.DockerConfig) {
		return nil, ErrAuthorityProfile
	}
	// A dedicated empty CLI configuration prevents ambient registry/auth helpers.
	st, e := os.Lstat(c.DockerConfig)
	if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		return nil, ErrAuthorityProfile
	}
	p := filepath.Join(c.DockerConfig, "config.json")
	st, e = os.Lstat(p)
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		return nil, ErrAuthorityProfile
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, ErrAuthorityProfile
	}
	b, e := io.ReadAll(io.LimitReader(f, 129))
	ce := f.Close()
	if e != nil || ce != nil || !bytes.Equal(bytes.TrimSpace(b), []byte("{}")) {
		return nil, ErrAuthorityProfile
	}
	cmd := exec.CommandContext(ctx, c.DockerBinary, append([]string{"--config", c.DockerConfig, "--host", c.Host}, args...)...)
	cmd.Env = []string{"PATH=" + filepath.Dir(c.DockerBinary), "DOCKER_CONFIG=" + c.DockerConfig, "KUBECONFIG=/dev/null"}
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = io.Discard
	return cmd, nil
}

func (r *AuthorityRuntime) output(ctx context.Context, args []string, input []byte) ([]byte, error) {
	if r == nil || r.plan == nil || ctx == nil {
		return nil, ErrAuthorityProfile
	}
	call, cancel := context.WithTimeout(ctx, r.plan.config.Limits.CommandTimeout)
	defer cancel()
	cmd, e := r.command(call, args, input)
	if e != nil {
		return nil, e
	}
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		return nil, ErrAuthorityProfile
	}
	if cmd.Start() != nil {
		return nil, ErrAuthorityProfile
	}
	b, re := io.ReadAll(io.LimitReader(pipe, celllaunch.MaxRecordBytes+1))
	if re != nil || len(b) > celllaunch.MaxRecordBytes {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, ErrAuthorityProfile
	}
	if cmd.Wait() != nil || call.Err() != nil {
		return nil, ErrAuthorityProfile
	}
	return b, nil
}

func (r *AuthorityRuntime) engine(ctx context.Context) Engine {
	return Engine{Output: func(args ...string) ([]byte, error) { return r.output(ctx, args, nil) }}
}

func (r *AuthorityRuntime) checkEngine(ctx context.Context) error {
	b, e := r.output(ctx, []string{"info", "--format", "{{.ID}}"}, nil)
	if e != nil || strings.TrimSpace(string(b)) != r.plan.config.Engine {
		return ErrAuthorityProfile
	}
	return nil
}

func (r *AuthorityRuntime) checkImage(ctx context.Context) error {
	b, e := r.output(ctx, []string{"image", "inspect", r.plan.config.Image}, nil)
	if e != nil {
		return e
	}
	var rows []struct {
		ID, Os, Architecture string
		Config               ImageConfig
	}
	if json.Unmarshal(b, &rows) != nil || len(rows) != 1 {
		return ErrAuthorityProfile
	}
	s := rows[0]
	c := r.plan.config
	if s.ID != c.Image || s.Os+"/"+s.Architecture != c.Platform || s.Config.User != ContainerUser || len(s.Config.Volumes) != 0 || len(s.Config.Cmd) != 0 || !slices.Equal(s.Config.Entrypoint, []string{AuthorityEntrypoint}) || !slices.Equal(s.Config.Env, []string{authorityImagePath}) {
		return ErrAuthorityProfile
	}
	return nil
}

// PrepareVolumes is an explicit operator/fixture act, never runtime startup DDL
// or credential provisioning. Existing names must carry the exact owned label.
func (r *AuthorityRuntime) PrepareVolumes(ctx context.Context) error {
	if e := r.checkEngine(ctx); e != nil {
		return e
	}
	for _, name := range r.plan.volumeNames() {
		b, e := r.output(ctx, []string{"volume", "ls", "--filter", "name=^" + name + "$", "--format", "{{.Name}}"}, nil)
		if e != nil {
			return e
		}
		names := strings.Fields(string(b))
		if len(names) == 0 {
			if _, e = r.output(ctx, []string{"volume", "create", "--label", "io.assay.authority.owner=" + r.plan.config.Owner, name}, nil); e != nil {
				return e
			}
		} else if len(names) != 1 || names[0] != name {
			return ErrAuthorityProfile
		}
		if e = r.checkVolume(ctx, name); e != nil {
			return e
		}
	}
	return nil
}

func (r *AuthorityRuntime) checkVolume(ctx context.Context, name string) error {
	b, e := r.output(ctx, []string{"volume", "inspect", name}, nil)
	if e != nil {
		return e
	}
	var rows []struct {
		Name   string
		Labels map[string]string
	}
	if json.Unmarshal(b, &rows) != nil || len(rows) != 1 || rows[0].Name != name || rows[0].Labels["io.assay.authority.owner"] != r.plan.config.Owner {
		return ErrAuthorityProfile
	}
	return nil
}

func (r *AuthorityRuntime) Create(ctx context.Context) (*AuthorityHandle, error) {
	if e := r.checkEngine(ctx); e != nil {
		return nil, e
	}
	if e := r.checkImage(ctx); e != nil {
		return nil, e
	}
	for _, v := range r.plan.volumeNames() {
		if e := r.checkVolume(ctx, v); e != nil {
			return nil, e
		}
	}
	e := r.engine(ctx)
	prior, err := e.Inspect(&Plan{Name: r.plan.name})
	if err != nil || prior != nil {
		return nil, ErrAuthorityProfile
	}
	b, err := r.output(ctx, r.plan.createArgs(), nil)
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(string(b))
	if !idRE.MatchString(id) {
		return nil, ErrAuthorityProfile
	}
	intent, err := r.plan.Intent(id)
	if err != nil {
		return nil, err
	}
	h := &AuthorityHandle{runtime: r, id: id, intent: intent}
	if _, err = r.inspect(ctx, h, false); err != nil {
		return h, err
	}
	if _, err = r.output(ctx, []string{"container", "start", id}, nil); err != nil {
		return h, err
	}
	if _, err = r.Observe(ctx, h); err != nil {
		return h, err
	}
	return h, nil
}

// ownedInspection is only an object-ownership check for cleanup of a candidate
// we created, including one whose runtime qualification failed. It grants no use.
func (r *AuthorityRuntime) ownedInspection(ctx context.Context, h *AuthorityHandle) (*Inspection, error) {
	if h == nil || h.runtime != r || !idRE.MatchString(h.id) {
		return nil, ErrAuthorityProfile
	}
	if e := r.checkEngine(ctx); e != nil {
		return nil, e
	}
	s, e := r.engine(ctx).Inspect(&Plan{Name: r.plan.name})
	if e != nil || s == nil || s.ID != h.id || !r.plan.owns(s) || ctx.Err() != nil {
		return nil, ErrAuthorityProfile
	}
	return s, nil
}
func (r *AuthorityRuntime) inspect(ctx context.Context, h *AuthorityHandle, running bool) (*Inspection, error) {
	s, e := r.ownedInspection(ctx, h)
	if e != nil {
		return nil, e
	}
	if r.plan.Validate(s) != nil {
		return nil, ErrAuthorityProfile
	}
	if running && (!s.State.Running || s.State.Paused || s.State.Restarting) {
		return nil, ErrAuthorityProfile
	}
	return s, nil
}

// Observe obtains a fresh actual engine observation; no saved record or caller
// boolean can produce Owned through this construction path.
func (r *AuthorityRuntime) Observe(ctx context.Context, h *AuthorityHandle) (celllaunch.ContainerObservation, error) {
	s, e := r.inspect(ctx, h, true)
	if e != nil {
		return celllaunch.ContainerObservation{State: celllaunch.Unknown}, e
	}
	h.hostPID.Store(s.State.Pid)
	return celllaunch.ContainerObservation{State: celllaunch.Owned, Scope: celllaunch.Scope{Cell: r.plan.config.Cell, Role: string(r.plan.config.Kind), LaunchID: r.plan.config.LaunchID, OS: "linux"}, Identity: h.intent.Container}, nil
}

func (r *AuthorityRuntime) helper(ctx context.Context, h *AuthorityHandle, op string, input []byte) ([]byte, error) {
	if _, e := r.inspect(ctx, h, true); e != nil {
		return nil, e
	}
	return r.output(ctx, []string{"container", "exec", "-i", h.id, AuthorityEntrypoint, op}, input)
}

func (r *AuthorityRuntime) RunProducer(ctx context.Context, h *AuthorityHandle, input []byte) error {
	if r.plan.config.Kind != celllaunch.AuthorityProducer {
		return ErrAuthorityProfile
	}
	if _, e := r.inspect(ctx, h, true); e != nil {
		return e
	}
	cmd, e := r.command(ctx, []string{"container", "exec", "-i", h.id, AuthorityEntrypoint, "producer"}, input)
	if e != nil {
		return e
	}
	cmd.Stdout = io.Discard
	if cmd.Run() != nil && ctx.Err() == nil {
		return ErrAuthorityProfile
	}
	return nil
}
func (r *AuthorityRuntime) Control(ctx context.Context, h *AuthorityHandle, input []byte) ([]byte, error) {
	if r.plan.config.Kind != celllaunch.AuthorityProducer {
		return nil, ErrAuthorityProfile
	}
	return r.helper(ctx, h, "control", input)
}
func (r *AuthorityRuntime) Verify(ctx context.Context, h *AuthorityHandle, input []byte) ([]byte, error) {
	if r.plan.config.Kind != celllaunch.AuthorityQuery {
		return nil, ErrAuthorityProfile
	}
	return r.helper(ctx, h, "verify", input)
}
func (r *AuthorityRuntime) ReadGrant(ctx context.Context, h *AuthorityHandle) ([]byte, error) {
	if r.plan.config.Kind != celllaunch.AuthorityCaller {
		return nil, ErrAuthorityProfile
	}
	return r.helper(ctx, h, "read-grant", nil)
}
func (r *AuthorityRuntime) Probe(ctx context.Context, h *AuthorityHandle, input []byte) ([]byte, error) {
	return r.helper(ctx, h, "probe", input)
}

func (r *AuthorityRuntime) CloseOwned(ctx context.Context, h *AuthorityHandle) error {
	s, e := r.ownedInspection(ctx, h)
	if e != nil {
		return e
	}
	if s.State.Running {
		grace := int64(r.plan.config.Limits.StopGrace / time.Second)
		if _, e = r.output(ctx, []string{"container", "stop", "--time", strconv.FormatInt(grace, 10), h.id}, nil); e != nil {
			return e
		}
	}
	if _, e = r.ownedInspection(ctx, h); e != nil {
		return e
	}
	if _, e = r.output(ctx, []string{"container", "rm", h.id}, nil); e != nil {
		return e
	}
	return nil
}
func (r *AuthorityRuntime) RemoveOwnedVolumes(ctx context.Context) error {
	if e := r.checkEngine(ctx); e != nil {
		return e
	}
	for _, v := range r.plan.volumeNames() {
		if e := r.checkVolume(ctx, v); e != nil {
			return e
		}
		if _, e := r.output(ctx, []string{"volume", "rm", v}, nil); e != nil {
			return e
		}
	}
	return nil
}
