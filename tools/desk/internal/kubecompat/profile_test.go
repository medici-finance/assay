package kubecompat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testProfile() *Profile {
	p := &Profile{Schema: ProfileName, Cell: "example", Namespace: "assay-desks", Kubeconfig: "/fixture/kubeconfig", Context: "example", Endpoint: "https://api.example.invalid:6443", TLSServerName: "api.example.invalid", CASHA256: strings.Repeat("a", 64), Custody: Custody{"role-custodian", "role-custodian", "role-custodian", "role-custodian", "excluded"}, Roles: map[string]Role{}}
	for _, name := range Roles {
		r := Role{StatefulSet: name, Pod: name + "-0", Container: name, Image: "ghcr.io/medici-finance/assay/" + name + ":candidate@sha256:" + strings.Repeat("b", 64), Agent: "/work/agent/claude", VersionManifest: "/work/agent/package.json", Workdir: "/work/repository", AgentSHA256: strings.Repeat("c", 64), AgentVersion: "2.0.0", Model: "sonnet", Instructions: "/work/role.md", InstructionsSHA256: strings.Repeat("d", 64), InitializationID: strings.Repeat("e", 64)}
		r.Cell = p.Cell
		r.InitialPrompt = RolePrompt(name)
		r.LaunchID = r.LaunchDigest(name)
		p.Roles[name] = r
	}
	return p
}
func identity(p *Profile, role string) Identity {
	return Identity{Cell: p.Cell, Role: role, ConfigDigest: strings.Repeat("f", 64), Namespace: p.Namespace, StatefulSetUID: "workload", PodUID: "pod-first", Container: role, ContainerID: "containerd://first", RestartCount: 0, Image: p.Roles[role].Image}
}

// regression: #4759
func TestKubeCompatIdentity(t *testing.T) {
	p := testProfile()
	if e := p.Validate("example"); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Profile){func(p *Profile) { p.Endpoint = "" }, func(p *Profile) { p.Endpoint = "http://api.example.invalid" }, func(p *Profile) { p.CASHA256 = "" }, func(p *Profile) { p.Custody.Logs = "shared" }, func(p *Profile) { r := p.Roles["worker-desk"]; r.Model = "different"; p.Roles["worker-desk"] = r }, func(p *Profile) { p.Namespace = "foreign/namespace" }} {
		q := testProfile()
		mutate(q)
		if e := q.Validate("example"); e == nil {
			t.Fatal("invalid target/custody/launch accepted")
		}
	}
	if _, _, e := Load("", "example"); e == nil {
		t.Fatal("ambient profile accepted")
	}
	want := identity(p, "worker-desk")
	if e := want.Valid(p, want.ConfigDigest, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Identity){func(i *Identity) { i.Cell = "foreign" }, func(i *Identity) { i.Namespace = "foreign" }, func(i *Identity) { i.Role = "the-desk" }, func(i *Identity) { i.StatefulSetUID = "other" }, func(i *Identity) { i.PodUID = "other" }, func(i *Identity) { i.Container = "other" }, func(i *Identity) { i.ContainerID = "containerd://new" }, func(i *Identity) { i.RestartCount++ }, func(i *Identity) { i.Image = "other" }, func(i *Identity) { i.ConfigDigest = "other" }} {
		got := want
		mutate(&got)
		if want.Same(got) == nil {
			t.Fatal("execution tuple drift accepted")
		}
	}
	for _, raw := range []string{`{"schema":"one","schema":"two"}`, `{"Schema":"one"}`, `{"schema":"one"} {}`} {
		var v struct {
			Schema string `json:"schema"`
		}
		if StrictJSON([]byte(raw), &v) == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
}

type fakeEffects struct {
	s        Snapshot
	calls    []string
	pending  bool
	stopped  bool
	fault    string
	attaches int
	launchID string
}

func (f *fakeEffects) Observe(ctx context.Context, role string) (Snapshot, error) {
	if f.fault == "partition" {
		return Snapshot{}, context.DeadlineExceeded
	}
	if f.pending {
		f.pending = false
		s := f.s
		s.Pending = true
		s.Identity.ContainerID = ""
		s.Running = false
		return s, nil
	}
	s := f.s
	if f.stopped {
		switch f.fault {
		case "absent":
			s.PodPresent = false
		case "changed":
			s.Identity.PodUID = "another"
		case "new-container":
			s.Identity.ContainerID = "containerd://another"
			s.Identity.RestartCount++
		default:
			if f.fault != "no-ack" {
				s.Termination = &Termination{ContainerID: s.Identity.ContainerID, PodUID: s.Identity.PodUID, LaunchID: f.launchID, Clean: true, GroupEmpty: f.fault != "survivor"}
			}
		}
	}
	return s, nil
}
func (f *fakeEffects) Scale(ctx context.Context, role, uid string, n int32) error {
	f.calls = append(f.calls, "scale")
	if n == 1 {
		f.s.PodPresent = true
		f.s.Running = true
		f.pending = true
	}
	if n == 0 {
		f.s.PodPresent = false
		f.s.Replicas = 0
	}
	return nil
}
func (f *fakeEffects) Attach(ctx context.Context, role string, id Identity, term Terminal) error {
	f.attaches++
	if term.In != nil {
		buf := make([]byte, 256)
		n, _ := term.In.Read(buf)
		if strings.HasPrefix(string(buf[:n]), "\x1dASSAY_STOP ") {
			f.stopped = true
		}
	}
	return nil
}

// regression: #4759
func TestKubeCompatPending(t *testing.T) {
	p := testProfile()
	id := identity(p, "worker-desk")
	f := &fakeEffects{s: Snapshot{StatefulSetUID: id.StatefulSetUID, Identity: id}}
	r := &Runtime{p, id.ConfigDigest, t.TempDir(), f, time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, e := r.Start(ctx, "worker-desk")
	if e != nil || got != id || len(f.calls) != 1 {
		t.Fatalf("pending cold start: %+v %v %v", got, e, f.calls)
	}
	if _, e = r.Start(ctx, "worker-desk"); e != nil || len(f.calls) != 1 {
		t.Fatal("repeated start invoked another execution", e)
	}
	f.s.Identity.ContainerID = "containerd://new"
	if _, e = r.Start(ctx, "worker-desk"); e == nil {
		t.Fatal("same-Pod new container admitted")
	}
}

// regression: #4759
func TestKubeCompatStopHolds(t *testing.T) {
	for _, fault := range []string{"partition", "absent", "changed", "new-container", "no-ack", "survivor"} {
		t.Run(fault, func(t *testing.T) {
			p := testProfile()
			id := identity(p, "worker-desk")
			f := &fakeEffects{s: Snapshot{StatefulSetUID: id.StatefulSetUID, PodPresent: true, Running: true, Identity: id}, fault: fault, launchID: p.Roles["worker-desk"].LaunchID}
			r := &Runtime{p, id.ConfigDigest, t.TempDir(), f, time.Millisecond}
			if e := WriteReceipt(r.ReceiptPath("worker-desk"), Receipt{ProfileName, id, "running"}); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if r.Stop(ctx, "worker-desk", true) == nil {
				t.Fatal("uncertain stop passed")
			}
			if len(f.calls) != 0 {
				t.Fatal("scale substituted for termination")
			}
		})
	}
}

// regression: #4759
func TestKubeCompatReceipt(t *testing.T) {
	p := testProfile()
	id := identity(p, "worker-desk")
	path := filepath.Join(t.TempDir(), "run", "worker-desk.json")
	want := Receipt{ProfileName, id, "running"}
	if e := WriteReceipt(path, want); e != nil {
		t.Fatal(e)
	}
	got, e := ReadReceipt(path)
	if e != nil || *got != want {
		t.Fatal(got, e)
	}
	raw, _ := json.Marshal(want)
	raw = append(raw, []byte(` {}`)...)
	_ = os.WriteFile(path, raw, 0600)
	if _, e = ReadReceipt(path); e == nil {
		t.Fatal("corrupt receipt accepted")
	}
}
