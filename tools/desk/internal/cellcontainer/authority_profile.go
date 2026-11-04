package cellcontainer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

var ErrAuthorityProfile = errors.New("local authority: protected container profile unavailable")

const AuthorityProfileSchema = "cell-local-authority-v1"
const AuthorityEntrypoint = "/authority-helper"
const authorityImagePath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const AuthorityGrantSlots = 8

type AuthorityGrantVolume struct {
	LaunchID, Volume string
	Slot             int
}
type AuthorityVolumes struct {
	Private, Current, Bootstrap string
	Grants                      []AuthorityGrantVolume
}
type AuthorityRuntimeLimits struct {
	Pids, Memory, NanoCPUs, TmpfsBytes int64
	CommandTimeout, StopGrace          time.Duration
}
type AuthorityConfig struct {
	Schema, Host, Engine, DockerBinary, DockerConfig, Image, Platform string
	Owner, Cell, LaunchID, Role                                       string
	Kind                                                              celllaunch.AuthorityKind
	Volumes                                                           AuthorityVolumes
	Limits                                                            AuthorityRuntimeLimits
}

// AuthorityPlan has a closed role/mount/command vocabulary. It is deliberately
// separate from the legacy interactive harness Plan; no legacy refusal is relaxed.
type AuthorityPlan struct {
	config       AuthorityConfig
	name, digest string
	mounts       []Mount
}

func NewAuthorityPlan(c AuthorityConfig) (*AuthorityPlan, error) {
	l := c.Limits
	if c.Schema != AuthorityProfileSchema || !strings.HasPrefix(c.Host, "unix:///") || !absolute(strings.TrimPrefix(c.Host, "unix://")) || !validModel(c.Engine) || len(c.Engine) > 256 || !absolute(c.DockerBinary) || !absolute(c.DockerConfig) || !imageRE.MatchString(c.Image) || (c.Platform != "linux/amd64" && c.Platform != "linux/arm64") || !nameRE.MatchString(c.Owner) || !nameRE.MatchString(c.Cell) || !nameRE.MatchString(c.LaunchID) || len(c.Owner) > 96 || len(c.Cell) > 48 || len(c.LaunchID) > 48 || l.Pids <= 0 || l.Pids > PidsLimit || l.Memory <= 0 || l.Memory > MemoryBytes || l.NanoCPUs <= 0 || l.NanoCPUs > NanoCPUs || l.TmpfsBytes <= 0 || l.TmpfsBytes > 512*1024*1024 || l.CommandTimeout <= 0 || l.StopGrace < time.Second || l.StopGrace > l.CommandTimeout || l.StopGrace%time.Second != 0 {
		return nil, ErrAuthorityProfile
	}
	switch c.Kind {
	case celllaunch.AuthorityProducer:
		if c.Role != "" {
			return nil, ErrAuthorityProfile
		}
	case celllaunch.AuthorityQuery, celllaunch.AuthorityCaller, celllaunch.AuthoritySibling:
		if Roles[c.Role] == "" {
			return nil, ErrAuthorityProfile
		}
	default:
		return nil, ErrAuthorityProfile
	}
	c.Volumes.Grants = append([]AuthorityGrantVolume(nil), c.Volumes.Grants...)
	sort.Slice(c.Volumes.Grants, func(i, j int) bool { return c.Volumes.Grants[i].Slot < c.Volumes.Grants[j].Slot })
	if len(c.Volumes.Grants) > AuthorityGrantSlots {
		return nil, ErrAuthorityProfile
	}
	seen := map[string]bool{}
	slots := map[int]bool{}
	launches := map[string]bool{}
	volume := func(v string) bool {
		if !volumeRE.MatchString(v) || len(v) > 128 || seen[v] {
			return false
		}
		seen[v] = true
		return true
	}
	if !volume(c.Volumes.Private) || !volume(c.Volumes.Current) || !volume(c.Volumes.Bootstrap) || len(c.Volumes.Grants) == 0 {
		return nil, ErrAuthorityProfile
	}
	var mounts []Mount
	switch c.Kind {
	case celllaunch.AuthorityProducer:
		mounts = []Mount{{"volume", c.Volumes.Private, "/authority-private", false}, {"volume", c.Volumes.Current, "/authority-current", false}, {"volume", c.Volumes.Bootstrap, "/authority-bootstrap", false}}
	case celllaunch.AuthorityQuery:
		mounts = []Mount{{"volume", c.Volumes.Current, "/authority-current", true}, {"volume", c.Volumes.Bootstrap, "/authority-bootstrap", true}}
	case celllaunch.AuthorityCaller, celllaunch.AuthoritySibling:
	}
	matchedCaller := false
	for _, g := range c.Volumes.Grants {
		if !nameRE.MatchString(g.LaunchID) || len(g.LaunchID) > 48 || !volume(g.Volume) || g.Slot < 0 || g.Slot >= AuthorityGrantSlots || slots[g.Slot] || launches[g.LaunchID] {
			return nil, ErrAuthorityProfile
		}
		slots[g.Slot], launches[g.LaunchID] = true, true
		if c.Kind == celllaunch.AuthorityProducer {
			mounts = append(mounts, Mount{"volume", g.Volume, AuthorityGrantTarget(g.Slot), false})
		}
		if c.Kind == celllaunch.AuthorityCaller && g.LaunchID == c.LaunchID {
			matchedCaller = true
			mounts = append(mounts, Mount{"volume", g.Volume, "/authority-grant", true})
		}
	}
	if c.Kind == celllaunch.AuthorityCaller && !matchedCaller {
		return nil, ErrAuthorityProfile
	}
	name := "assay-authority-" + c.Cell + "-" + string(c.Kind)
	if c.Kind != celllaunch.AuthorityProducer {
		name += "-" + c.LaunchID
	}
	b, e := json.Marshal(c)
	if e != nil {
		return nil, ErrAuthorityProfile
	}
	sum := sha256.Sum256(b)
	return &AuthorityPlan{config: c, name: name, digest: hex.EncodeToString(sum[:]), mounts: mounts}, nil
}

func AuthorityGrantTarget(slot int) string { return "/authority-grants/slot-" + strconv.Itoa(slot) }
func (p *AuthorityPlan) Name() string      { return p.name }
func (p *AuthorityPlan) Digest() string    { return p.digest }
func (p *AuthorityPlan) Intent(id string) (celllaunch.AuthorityLaunch, error) {
	c := p.config
	s := celllaunch.AuthorityLaunch{Schema: "cell-authority-launch-v1", LaunchID: c.LaunchID, Cell: c.Cell, Role: c.Role, Kind: c.Kind, Container: celllaunch.ContainerIdentity{Endpoint: c.Host, Engine: c.Engine, ID: id, Cell: c.Cell, Role: string(c.Kind), PolicyDigest: p.digest}}
	if s.Validate() != nil {
		return celllaunch.AuthorityLaunch{}, ErrAuthorityProfile
	}
	return s, nil
}

func (p *AuthorityPlan) restrictions() runtimeRestrictions {
	l := p.config.Limits
	return runtimeRestrictions{l.Pids, l.Memory, l.NanoCPUs, fmt.Sprintf("rw,nosuid,nodev,size=%d,mode=1777", l.TmpfsBytes), "none"}
}

func (p *AuthorityPlan) owns(s *Inspection) bool {
	c := p.config
	return s != nil && idRE.MatchString(s.ID) && s.Name == "/"+p.name && s.Image == c.Image && s.Config.Labels["io.assay.authority.owner"] == c.Owner && s.Config.Labels["io.assay.authority.cell"] == c.Cell && s.Config.Labels["io.assay.authority.kind"] == string(c.Kind) && s.Config.Labels["io.assay.authority.launch"] == c.LaunchID && s.Config.Labels["io.assay.authority.plan"] == p.digest
}

func (p *AuthorityPlan) Validate(s *Inspection) error {
	if !p.owns(s) || s.Config.User != ContainerUser || s.Config.Tty || s.Config.OpenStdin || !slices.Equal(s.Config.Cmd, []string{"standby"}) || !slices.Equal(s.Config.Entrypoint, []string{AuthorityEntrypoint}) || !slices.Equal(s.ImageConfig.Entrypoint, []string{AuthorityEntrypoint}) || s.ImageConfig.User != ContainerUser || len(s.ImageConfig.Volumes) != 0 || len(s.ImageConfig.Cmd) != 0 || !slices.Equal(s.ImageConfig.Env, []string{authorityImagePath}) || !slices.Equal(s.Config.Env, []string{authorityImagePath}) {
		return ErrAuthorityProfile
	}
	if isolationFailure(s, p.restrictions()) != "" || mountFailure(s, p.mounts) != "" {
		return ErrAuthorityProfile
	}
	return nil
}

func (p *AuthorityPlan) createArgs() []string {
	c, l := p.config, p.config.Limits
	args := []string{"container", "create", "--pull", "never", "--platform", c.Platform, "--read-only", "--user", ContainerUser, "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", strconv.FormatInt(l.Pids, 10), "--memory", strconv.FormatInt(l.Memory, 10), "--cpus", fmt.Sprintf("%d.%09d", l.NanoCPUs/1_000_000_000, l.NanoCPUs%1_000_000_000), "--tmpfs", "/tmp:" + p.restrictions().tmpfs, "--network", "none", "--name", p.name}
	for _, label := range []string{"owner=" + c.Owner, "cell=" + c.Cell, "kind=" + string(c.Kind), "launch=" + c.LaunchID, "plan=" + p.digest} {
		args = append(args, "--label", "io.assay.authority."+label)
	}
	for _, m := range p.mounts {
		v := "type=volume,src=" + m.Source + ",dst=" + m.Target
		if m.ReadOnly {
			v += ",readonly"
		}
		args = append(args, "--mount", v)
	}
	return append(args, c.Image, "standby")
}

func (p *AuthorityPlan) volumeNames() []string {
	out := make([]string, 0, len(p.mounts))
	for _, m := range p.mounts {
		out = append(out, m.Source)
	}
	return out
}

func validAuthorityToolPath(p string) bool {
	return filepath.IsAbs(p) && filepath.Clean(p) == p && !strings.ContainsAny(p, "\x00\r\n")
}
