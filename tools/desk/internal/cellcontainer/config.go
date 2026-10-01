// Package cellcontainer implements the local Docker runtime used by cellctl.
package cellcontainer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	Schema     string          `json:"schema"`
	DockerHost string          `json:"docker_host"`
	Image      string          `json:"image"`
	Platform   string          `json:"platform"`
	Cells      map[string]Cell `json:"cells"`
}
type Cell struct {
	Repo     string          `json:"repo"`
	Incoming string          `json:"incoming"`
	HostLock string          `json:"host_lock,omitempty"`
	Roles    map[string]Role `json:"roles"`
}
type Role struct {
	Harness          string            `json:"harness"`
	Models           map[string]string `json:"models"`
	Volume           string            `json:"volume"`
	Config           string            `json:"config"`
	AppKey           string            `json:"app_key"`
	ClaudeToken      string            `json:"claude_token,omitempty"`
	StartupAction    string            `json:"startup_action,omitempty"`
	Sandbox          string            `json:"codex_sandbox,omitempty"`
	BoundaryApproval string            `json:"container_boundary_approved_by,omitempty"`
	// A migration can explicitly identify one already-running, unlabelled container.
	// Its image, mounts, runtime restrictions and requested model are still checked.
	AdoptID string `json:"adopt_container_id,omitempty"`
}

var Roles = map[string]string{"desk": "the-desk", "worker": "worker-desk", "reviewer": "pr-review-desk", "verifier": "verify-desk", "intake-loop": "intake-desk", "issue-loop": "intake-desk"}
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var volumeRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
var imageRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var idRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var repoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func absolute(p string) bool { return filepath.IsAbs(p) && !strings.ContainsAny(p, ",\n\r\x00") }
func Load(path string) (*Config, error) {
	if !absolute(path) {
		return nil, fmt.Errorf("container config must be an absolute path")
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("container config must be a regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&c); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("container config has trailing data")
	}
	if err = c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
func (c *Config) Validate() error {
	if c.Schema != "cell-containers-v1" {
		return fmt.Errorf("unsupported container configuration schema")
	}
	if !strings.HasPrefix(c.DockerHost, "unix:///") || !absolute(strings.TrimPrefix(c.DockerHost, "unix://")) {
		return fmt.Errorf("docker_host must name an explicit local Unix socket")
	}
	if !imageRE.MatchString(c.Image) {
		return fmt.Errorf("container image must be an immutable sha256 image ID")
	}
	if c.Platform == "" {
		c.Platform = "linux/amd64"
	}
	volumes := map[string]bool{}
	for name, cell := range c.Cells {
		if !nameRE.MatchString(name) || !repoRE.MatchString(cell.Repo) || !absolute(cell.Incoming) {
			return fmt.Errorf("invalid cell name, repository or incoming path for %q", name)
		}
		if cell.HostLock != "" && !absolute(cell.HostLock) {
			return fmt.Errorf("host_lock must be absolute")
		}
		for role, r := range cell.Roles {
			if Roles[role] == "" || (r.Harness != "codex" && r.Harness != "claude") {
				return fmt.Errorf("invalid role or harness for %s/%s", name, role)
			}
			if !volumeRE.MatchString(r.Volume) || volumes[r.Volume] {
				return fmt.Errorf("each role needs a distinct valid volume: %s/%s", name, role)
			}
			volumes[r.Volume] = true
			if !absolute(r.Config) || !absolute(r.AppKey) || (r.ClaudeToken != "" && !absolute(r.ClaudeToken)) {
				return fmt.Errorf("role mount paths must be absolute")
			}
			if r.StartupAction != "" && r.StartupAction != "paused" && r.StartupAction != "handoff" {
				return fmt.Errorf("startup_action must be paused or handoff")
			}
			if r.StartupAction == "handoff" && role != "desk" {
				return fmt.Errorf("handoff is coordinator-only")
			}
			if r.Sandbox != "" && r.Sandbox != "workspace-write" && r.Sandbox != "container" {
				return fmt.Errorf("invalid codex_sandbox")
			}
			if r.Sandbox == "container" && r.BoundaryApproval == "" {
				return fmt.Errorf("container-only sandbox needs an approval record")
			}
			if r.AdoptID != "" && !idRE.MatchString(r.AdoptID) {
				return fmt.Errorf("adopt_container_id must be a full container ID")
			}
			for h, m := range r.Models {
				if (h != "codex" && h != "claude") || m == "" || strings.ContainsAny(m, "\r\n\x00") {
					return fmt.Errorf("invalid model pin")
				}
			}
		}
	}
	return nil
}
func (c *Config) RoleKey(name, role string) (string, error) {
	cell, ok := c.Cells[name]
	if !ok {
		return "", fmt.Errorf("cell %q is absent from container config", name)
	}
	key := ""
	for k := range cell.Roles {
		if Roles[k] == role {
			if key != "" {
				return "", fmt.Errorf("role %s is ambiguous; configure only one intake role per cell", role)
			}
			key = k
		}
	}
	if key == "" {
		return "", fmt.Errorf("role %s is not configured", role)
	}
	return key, nil
}

type Mount struct {
	Type, Source, Target string
	ReadOnly             bool
}
type Plan struct {
	Name, CellName, RoleKey, Harness, Model, Action, Image, Host, Platform, Network, HostLock, AdoptID string
	Env                                                                                                map[string]string
	Mounts                                                                                             []Mount
}

func (c *Config) Plan(name, key, harness, model string) (*Plan, error) {
	cell, ok := c.Cells[name]
	if !ok {
		return nil, fmt.Errorf("unknown container cell %q", name)
	}
	r, ok := cell.Roles[key]
	if !ok {
		return nil, fmt.Errorf("unknown role %q", key)
	}
	if harness == "" {
		harness = r.Harness
	}
	if harness != "codex" && harness != "claude" {
		return nil, fmt.Errorf("unsupported harness")
	}
	if model == "" {
		model = r.Models[harness]
	}
	if model == "" || strings.ContainsAny(model, "\r\n\x00") {
		return nil, fmt.Errorf("a model pin is required")
	}
	action := r.StartupAction
	if action == "" {
		action = "paused"
	}
	sandbox := r.Sandbox
	if sandbox == "" {
		sandbox = "workspace-write"
	}
	p := &Plan{Name: "assay-" + name + "-" + key, CellName: name, RoleKey: key, Harness: harness, Model: model, Action: action, Image: c.Image, Host: c.DockerHost, Platform: c.Platform, Network: "assay-product-" + name, AdoptID: r.AdoptID}
	if key == "desk" {
		p.HostLock = cell.HostLock
	}
	p.Mounts = []Mount{{"volume", r.Volume, "/work", false}, {"bind", r.Config, "/cell-config", true}, {"bind", cell.Incoming, "/incoming", true}}
	p.Env = map[string]string{"CELL_REPO": cell.Repo, "CELL_ROLE": key, "CELL_HARNESS": harness, "CELL_MODEL": model, "DESK_ROOTS": cell.Repo + "=/work/repo", "DESK_LOOP": Roles[key], "DESK_SESSION": "container-" + name + "-" + key, "ASSAY_DESK": Roles[key], "CODEX_EXECUTION_BOUNDARY": sandbox}
	if action == "handoff" {
		p.Mounts = append(p.Mounts, Mount{"bind", r.AppKey, "/run/secrets/app.pem", true})
		p.Env[strings.ToUpper(strings.ReplaceAll(name+"_"+key, "-", "_"))+"_PEM"] = "/run/secrets/app.pem"
		p.Env[strings.ToUpper(strings.ReplaceAll(key, "-", "_"))+"_PEM"] = "/run/secrets/app.pem"
	}
	if harness == "claude" {
		if r.ClaudeToken == "" {
			return nil, fmt.Errorf("Claude requires its own configured model token")
		}
		p.Mounts = append(p.Mounts, Mount{"bind", r.ClaudeToken, "/run/secrets/model-token", true})
	}
	return p, nil
}

// CheckFiles never reads credential contents or contacts Docker/the forge.
func (p *Plan) CheckFiles() error {
	for _, m := range p.Mounts {
		if m.Type != "bind" {
			continue
		}
		st, err := os.Lstat(m.Source)
		if err != nil {
			return err
		}
		if strings.HasPrefix(m.Target, "/run/secrets/") {
			if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
				return fmt.Errorf("credential must be a private regular file: %s", m.Source)
			}
		} else if !st.IsDir() {
			return fmt.Errorf("mount must be a directory: %s", m.Source)
		}
	}
	return nil
}
func (p *Plan) RunArgs() []string {
	a := []string{"run", "--rm", "--sig-proxy=false", "--pull", "never", "--platform", p.Platform, "--read-only", "--user", "501:501", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--memory", "4g", "--cpus", "2", "--tmpfs", "/tmp:rw,nosuid,nodev,size=512m,mode=1777", "--name", p.Name, "--network", p.Network, "--label", "io.assay.cell=" + p.CellName, "--label", "io.assay.role=" + p.RoleKey, "-it"}
	for _, m := range p.Mounts {
		s := "type=" + m.Type + ",src=" + m.Source + ",dst=" + m.Target
		if m.ReadOnly {
			s += ",readonly"
		}
		a = append(a, "--mount", s)
	}
	keys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a = append(a, "--env", k+"="+p.Env[k])
	}
	return append(a, p.Image, p.Action)
}
