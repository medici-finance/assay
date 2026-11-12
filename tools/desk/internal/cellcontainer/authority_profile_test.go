package cellcontainer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

func authorityConfig(kind celllaunch.AuthorityKind) AuthorityConfig {
	role := "worker"
	if kind == celllaunch.AuthorityProducer {
		role = ""
	}
	return AuthorityConfig{Schema: AuthorityProfileSchema, Host: "unix:///fixture-owned.sock", Engine: "fixture-engine", DockerBinary: "/fixture/docker", DockerConfig: "/fixture/empty-cli", Image: "sha256:" + strings.Repeat("a", 64), Platform: "linux/arm64", Owner: "fixture-owner", Cell: "fixture-cell", LaunchID: "caller-a", Role: role, Kind: kind, Volumes: AuthorityVolumes{Private: "fixture-control", Current: "fixture-current", Bootstrap: "fixture-bootstrap", Grants: []AuthorityGrantVolume{{LaunchID: "caller-a", Volume: "fixture-grant-a", Slot: 0}, {LaunchID: "caller-b", Volume: "fixture-grant-b", Slot: 1}}}, Limits: AuthorityRuntimeLimits{Pids: 32, Memory: 64 * 1024 * 1024, NanoCPUs: 500_000_000, TmpfsBytes: 8 * 1024 * 1024, CommandTimeout: 20 * time.Second, StopGrace: 2 * time.Second}}
}

// This is a runtime-validator fixture, not a real engine or custody witness.
func authorityInspection(p *AuthorityPlan) *Inspection {
	s := &Inspection{ID: strings.Repeat("b", 64), Name: "/" + p.name, Image: p.config.Image}
	s.State.Running = true
	s.State.Status = "running"
	s.State.Pid = 1234
	s.Config.User = ContainerUser
	s.Config.Cmd = []string{"standby"}
	s.Config.Entrypoint = []string{AuthorityEntrypoint}
	s.Config.Env = []string{authorityImagePath}
	s.Config.Labels = map[string]string{"io.assay.authority.owner": p.config.Owner, "io.assay.authority.cell": p.config.Cell, "io.assay.authority.kind": string(p.config.Kind), "io.assay.authority.launch": p.config.LaunchID, "io.assay.authority.plan": p.digest}
	s.ImageConfig = ImageConfig{User: ContainerUser, Env: []string{authorityImagePath}, Entrypoint: []string{AuthorityEntrypoint}}
	h := &s.HostConfig
	h.ReadonlyRootfs = true
	h.CapDrop = []string{"ALL"}
	h.SecurityOpt = []string{"no-new-privileges"}
	h.PidsLimit = p.config.Limits.Pids
	h.Memory = p.config.Limits.Memory
	h.NanoCpus = p.config.Limits.NanoCPUs
	h.NetworkMode = "none"
	h.Tmpfs = map[string]string{"/tmp": p.restrictions().tmpfs}
	s.NetworkSettings.Networks = map[string]json.RawMessage{"none": json.RawMessage(`{}`)}
	var mounts []map[string]any
	for _, m := range p.mounts {
		mounts = append(mounts, map[string]any{"Type": m.Type, "Name": m.Source, "Source": m.Source, "Destination": m.Target, "RW": !m.ReadOnly})
	}
	b, _ := json.Marshal(mounts)
	json.Unmarshal(b, &s.Mounts)
	return s
}

func TestAuthorityProfileMountMatrix(t *testing.T) {
	for _, kind := range []celllaunch.AuthorityKind{celllaunch.AuthorityProducer, celllaunch.AuthorityQuery, celllaunch.AuthorityCaller, celllaunch.AuthoritySibling} {
		t.Run(string(kind), func(t *testing.T) {
			p, e := NewAuthorityPlan(authorityConfig(kind))
			if e != nil {
				t.Fatal(e)
			}
			want := map[string]bool{}
			switch kind {
			case celllaunch.AuthorityProducer:
				want = map[string]bool{"/authority-control": false, "/authority-current": false, "/authority-bootstrap": false, "/authority-grants/slot-0": false, "/authority-grants/slot-1": false}
			case celllaunch.AuthorityQuery:
				want = map[string]bool{"/authority-current": true, "/authority-bootstrap": true}
			case celllaunch.AuthorityCaller:
				want = map[string]bool{"/authority-grant": true}
			}
			if len(want) != len(p.mounts) {
				t.Fatal("closed mount population changed")
			}
			for _, m := range p.mounts {
				ro, ok := want[m.Target]
				if !ok || m.Type != "volume" || m.ReadOnly != ro {
					t.Fatal("unexpected mount view")
				}
			}
			if e = p.Validate(authorityInspection(p)); e != nil {
				t.Fatal("positive validator", e)
			}
			args := strings.Join(p.createArgs(), " ")
			for _, bad := range []string{"--privileged", " -it ", "type=bind", "docker.sock", "--publish", "--env"} {
				if strings.Contains(args, bad) {
					t.Fatal("forbidden runtime capability")
				}
			}
		})
	}
}

func TestAuthorityProfileRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Inspection)
	}{
		{"writable-root", func(s *Inspection) { s.HostConfig.ReadonlyRootfs = false }},
		{"tty", func(s *Inspection) { s.Config.Tty = true }},
		{"stdin", func(s *Inspection) { s.Config.OpenStdin = true }},
		{"key-environment", func(s *Inspection) { s.Config.Env = append(s.Config.Env, "UNPLANNED=value") }},
		{"image-volume", func(s *Inspection) {
			s.ImageConfig.Volumes = map[string]json.RawMessage{"/unplanned": json.RawMessage(`{}`)}
		}},
		{"image-cmd", func(s *Inspection) { s.ImageConfig.Cmd = []string{"unplanned"} }},
		{"owner", func(s *Inspection) { s.Config.Labels["io.assay.authority.owner"] = "foreign" }},
		{"network", func(s *Inspection) { s.HostConfig.NetworkMode = "host" }},
		{"capability", func(s *Inspection) { s.HostConfig.CapAdd = []string{"SYS_ADMIN"} }},
		{"pid-sharing", func(s *Inspection) { s.HostConfig.PidMode = "host" }},
		{"extra-control-mount", func(s *Inspection) {
			s.Mounts = append(s.Mounts, s.Mounts[0])
			s.Mounts[len(s.Mounts)-1].Destination = "/authority-control"
		}},
		{"writable-current", func(s *Inspection) { s.Mounts[0].RW = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, e := NewAuthorityPlan(authorityConfig(celllaunch.AuthorityQuery))
			if e != nil {
				t.Fatal(e)
			}
			s := authorityInspection(p)
			tc.change(s)
			if p.Validate(s) == nil {
				t.Fatal("profile refusal absent")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*AuthorityConfig)
	}{
		{"ambient-host", func(c *AuthorityConfig) { c.Host = "" }},
		{"remote-engine", func(c *AuthorityConfig) { c.Host = "tcp://service.example:2375" }},
		{"unknown-kind", func(c *AuthorityConfig) { c.Kind = "unknown" }},
		{"unknown-role", func(c *AuthorityConfig) { c.Role = "unknown" }},
		{"missing-image", func(c *AuthorityConfig) { c.Image = "" }},
		{"missing-budget", func(c *AuthorityConfig) { c.Limits.Memory = 0 }},
		{"duplicate-volume", func(c *AuthorityConfig) { c.Volumes.Current = c.Volumes.Private }},
		{"unknown-slot", func(c *AuthorityConfig) { c.Volumes.Grants[0].Slot = AuthorityGrantSlots }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := authorityConfig(celllaunch.AuthorityQuery)
			tc.change(&c)
			if _, e := NewAuthorityPlan(c); e == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestAuthorityComponentPermit(t *testing.T) {
	p, e := NewAuthorityPlan(authorityConfig(celllaunch.AuthorityCaller))
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.Intent(strings.Repeat("b", 64))
	if e != nil {
		t.Fatal(e)
	}
	permit, e := celllaunch.NewAuthorityPermit(s)
	if e != nil {
		t.Fatal(e)
	}
	o := celllaunch.ContainerObservation{State: celllaunch.Owned, Scope: celllaunch.Scope{Cell: s.Cell, Role: string(s.Kind), LaunchID: s.LaunchID, OS: "linux"}, Identity: s.Container}
	if permit.Check(s) != nil || s.Match(o) != nil {
		t.Fatal("positive binding")
	}
	for _, state := range []celllaunch.ObservationState{celllaunch.Unknown, celllaunch.Missing, celllaunch.Foreign} {
		copy := o
		copy.State = state
		if s.Match(copy) == nil {
			t.Fatal("non-owned state accepted")
		}
	}
	for _, field := range []string{"id", "engine", "policy", "cell", "kind"} {
		copy := o
		switch field {
		case "id":
			copy.Identity.ID = strings.Repeat("c", 64)
		case "engine":
			copy.Identity.Engine = "foreign"
		case "policy":
			copy.Identity.PolicyDigest = strings.Repeat("c", 64)
		case "cell":
			copy.Scope.Cell = "foreign"
		case "kind":
			copy.Scope.Role = "producer"
		}
		if s.Match(copy) == nil {
			t.Fatal("observation mismatch accepted")
		}
	}
	s.Container.ID = strings.Repeat("c", 64)
	if permit.Check(s) == nil {
		t.Fatal("permit followed changed intent")
	}
}
