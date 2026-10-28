// Package kubecompat implements the explicitly selected trusted compatibility profile.
// It does not own work acceptance, credential grants or managed-candidate isolation.
package kubecompat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

const ProfileName = "cell-kube-compat-v1"

var Roles = []string{"the-desk", "intake-desk", "worker-desk", "pr-review-desk", "verify-desk"}
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var hashRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var imageRE = regexp.MustCompile(`^ghcr\.io/medici-finance/assay/[a-z-]+:[a-zA-Z0-9._-]+@sha256:[a-f0-9]{64}$`)

// Profile contains no credential value; Kubeconfig is an explicit custodian-owned file.
type Profile struct {
	Schema        string          `json:"schema"`
	Cell          string          `json:"cell"`
	Namespace     string          `json:"namespace"`
	Kubeconfig    string          `json:"kubeconfig"`
	Context       string          `json:"context"`
	Endpoint      string          `json:"endpoint"`
	TLSServerName string          `json:"tls_server_name"`
	CASHA256      string          `json:"ca_sha256"`
	Custody       Custody         `json:"custody"`
	Roles         map[string]Role `json:"roles"`
}
type Custody struct {
	Operator       string `json:"operator"`
	Terminal       string `json:"terminal"`
	Logs           string `json:"logs"`
	Nodes          string `json:"nodes"`
	DownstreamLogs string `json:"downstream_logs"`
}
type Role struct {
	Cell               string `json:"cell"`
	InitialPrompt      string `json:"initial_prompt"`
	Agent              string `json:"agent"`
	VersionManifest    string `json:"version_manifest"`
	Workdir            string `json:"workdir"`
	StatefulSet        string `json:"statefulset"`
	Pod                string `json:"pod"`
	Container          string `json:"container"`
	Image              string `json:"image"`
	LaunchID           string `json:"launch_id"`
	AgentSHA256        string `json:"agent_sha256"`
	AgentVersion       string `json:"agent_version"`
	Model              string `json:"model"`
	Instructions       string `json:"instructions"`
	InstructionsSHA256 string `json:"instructions_sha256"`
	InitializationID   string `json:"initialization_id"`
}

// LaunchDescriptor is the exact image-owned launch contract. The hash uses sorted
// JSON object keys, shared with the Python launcher's canonical encoding.
func (r Role) LaunchDescriptor(role string) map[string]string {
	return map[string]string{"schema": ProfileName, "cell": r.Cell, "role": role, "initialization_id": r.InitializationID, "agent": r.Agent, "agent_sha256": r.AgentSHA256, "agent_version": r.AgentVersion, "version_manifest": r.VersionManifest, "model": r.Model, "instructions": r.Instructions, "instructions_sha256": r.InstructionsSHA256, "workdir": r.Workdir, "initial_prompt": r.InitialPrompt}
}
func RolePrompt(role string) string {
	return "Read /opt/assay/plugin/skills/" + role + "/SKILL.md and run that role under its existing STOP, claim and human gates."
}
func (r Role) LaunchDigest(role string) string {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(r.LaunchDescriptor(role))
	raw := bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func StrictJSON(raw []byte, target any) error {
	// Token walk rejects duplicate members before encoding/json can discard them.
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s := k.(string)
				if seen[s] {
					return errors.New("duplicate JSON member")
				}
				seen[s] = true
				if e = walk(); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = walk(); e != nil {
					return e
				}
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	if e := walk(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(target); e != nil {
		return e
	}
	// Exact member names: encoding/json alone would accept case variants.
	var supplied, decoded any
	if e := json.Unmarshal(raw, &supplied); e != nil {
		return e
	}
	out, e := json.Marshal(target)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(out, &decoded); e != nil {
		return e
	}
	a, _ := json.Marshal(supplied)
	b, _ := json.Marshal(decoded)
	if !bytes.Equal(a, b) {
		return errors.New("JSON must use the exact contract members and values")
	}
	return nil
}
func Load(path, cell string) (*Profile, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", errors.New("explicit absolute compatibility profile required")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, "", errors.New("compatibility profile unavailable")
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 65537))
	if e != nil || len(raw) > 65536 {
		return nil, "", errors.New("compatibility profile exceeds bound")
	}
	var p Profile
	if e = StrictJSON(raw, &p); e != nil {
		return nil, "", fmt.Errorf("compatibility profile: %w", e)
	}
	if e = p.Validate(cell); e != nil {
		return nil, "", e
	}
	raw, _ = json.Marshal(p)
	h := sha256.Sum256(raw)
	return &p, hex.EncodeToString(h[:]), nil
}
func (p Profile) Validate(cell string) error {
	bad := func() error {
		return errors.New("invalid explicit compatibility target, role, image or custody declaration")
	}
	u, e := url.Parse(p.Endpoint)
	if p.Schema != ProfileName || p.Cell != cell || !nameRE.MatchString(p.Cell) || !nameRE.MatchString(p.Namespace) || !filepath.IsAbs(p.Kubeconfig) || p.Context == "" || e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || p.TLSServerName == "" || !hashRE.MatchString(p.CASHA256) {
		return bad()
	}
	if p.Custody.Operator == "" || p.Custody.Terminal != "role-custodian" || p.Custody.Logs != "role-custodian" || p.Custody.Nodes != "role-custodian" || (p.Custody.DownstreamLogs != "excluded" && p.Custody.DownstreamLogs != "role-custodian") {
		return bad()
	}
	if len(p.Roles) != len(Roles) {
		return bad()
	}
	seen := map[string]bool{}
	for _, name := range Roles {
		r, ok := p.Roles[name]
		if !ok || !nameRE.MatchString(r.StatefulSet) || r.Pod != r.StatefulSet+"-0" || r.Container != name || seen[r.StatefulSet] || !imageRE.MatchString(r.Image) || !hashRE.MatchString(r.LaunchID) || !hashRE.MatchString(r.AgentSHA256) || !hashRE.MatchString(r.InstructionsSHA256) || r.Cell != p.Cell || r.InitialPrompt != RolePrompt(name) || r.AgentVersion == "" || !filepath.IsAbs(r.Agent) || !filepath.IsAbs(r.VersionManifest) || !filepath.IsAbs(r.Workdir) || r.LaunchID != r.LaunchDigest(name) || r.Model == "" || !filepath.IsAbs(r.Instructions) || !hashRE.MatchString(r.InitializationID) {
			return bad()
		}
		seen[r.StatefulSet] = true
	}
	return nil
}

// Identity is an observed execution, never an authorization token.
type Identity struct {
	Cell           string `json:"cell"`
	Role           string `json:"role"`
	ConfigDigest   string `json:"config_digest"`
	Namespace      string `json:"namespace"`
	StatefulSetUID string `json:"statefulset_uid"`
	PodUID         string `json:"pod_uid"`
	Container      string `json:"container"`
	ContainerID    string `json:"container_id"`
	RestartCount   int32  `json:"restart_count"`
	Image          string `json:"image"`
}

func (i Identity) Valid(p *Profile, digest, role string) error {
	r, ok := p.Roles[role]
	if !ok || i.Cell != p.Cell || i.Role != role || i.ConfigDigest != digest || i.Namespace != p.Namespace || i.StatefulSetUID == "" || i.PodUID == "" || i.Container != r.Container || i.ContainerID == "" || i.RestartCount < 0 || i.Image != r.Image {
		return errors.New("execution identity differs or is incomplete")
	}
	return nil
}
func (i Identity) Same(other Identity) error {
	if i != other {
		return errors.New("execution changed; held for custodian reconciliation")
	}
	return nil
}

type Receipt struct {
	Schema   string   `json:"schema"`
	Identity Identity `json:"identity"`
	State    string   `json:"state"`
}

func ReadReceipt(path string) (*Receipt, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if len(raw) > 16384 {
		return nil, errors.New("receipt exceeds bound")
	}
	var r Receipt
	if e = StrictJSON(raw, &r); e != nil {
		return nil, e
	}
	if r.Schema != ProfileName || (r.State != "running" && r.State != "clean-stopped" && r.State != "held") {
		return nil, errors.New("invalid receipt")
	}
	return &r, nil
}
func WriteReceipt(path string, r Receipt) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".observation-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(raw)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(tmp, path)
	}
	if e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
