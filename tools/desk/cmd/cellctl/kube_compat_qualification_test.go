package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/testfixture"
)

// Bind the fixture's advertised image and the on-disk CLI profile to the same
// published role pin. The HTTPS API simulates what runs; it never pulls an image.
func restartWorld(t *testing.T) (*cliWorld, *testfixture.Server) {
	t.Helper()
	w, s := kubeWorld(t)
	raw, err := os.ReadFile("../../../../containers/k8s/components/compatibility-cellctl/candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	var candidate struct{ Images map[string]string }
	if err := json.Unmarshal(raw, &candidate); err != nil {
		t.Fatal(err)
	}
	for _, role := range kc.Roles {
		profile := s.Profile.Roles[role]
		profile.Image = candidate.Images[role]
		s.Profile.Roles[role] = profile
	}
	s.Save(t)
	return w, s
}

// regression: #7347
func TestKubeCompatControlledRestart(t *testing.T) {
	const role = "worker-desk"
	w, s := restartWorld(t)
	run := func(args ...string) {
		t.Helper()
		if got := w.run(t, nil, args...); got.Code != 0 {
			t.Fatal(got)
		}
	}
	receipt := func(state string) kc.Identity {
		t.Helper()
		r, err := kc.ReadReceipt(filepath.Join(w.cellDir, "run", "kubecompat", role+".json"))
		if err != nil || r.State != state {
			t.Fatalf("want %s receipt: %+v (%v)", state, r, err)
		}
		return r.Identity
	}
	run("desk", "example", role)
	before := receipt("running")
	run("desk", "example", role)
	run("attach", "example", role)
	run("attach", "example", role)
	starts, _, _ := s.Counts()
	if starts != 1 || before != receipt("running") {
		t.Fatal("duplicate start, reconnect launch or stop on stream close", starts)
	}
	if code, output := kubeTTY(t, w, 3*time.Second, "down", "example", role); code != 0 {
		t.Fatalf("controlled stop: %d %s", code, output)
	}
	if got := receipt("clean-stopped"); got != before {
		t.Fatal("stop receipt belongs to a different execution", got)
	}
	run("desk", "example", role)
	after := receipt("running")
	starts, _, _ = s.Counts()
	if starts != 2 || before.PodUID == after.PodUID || before.ContainerID == after.ContainerID || after.RestartCount != 0 {
		t.Fatal("expected exactly one new execution after explicit start", starts, before, after)
	}
	if before.ConfigDigest != after.ConfigDigest || before.StatefulSetUID != after.StatefulSetUID || before.Image != after.Image {
		t.Fatal("restart changed the bound workload, profile or image", before, after)
	}
	binary, err := os.ReadFile(cellctlBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	observed, err := json.Marshal([]kc.Identity{before, after})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cellctl-sha256=%x; execution-identities=%s", sha256.Sum256(binary), observed)
}

// Ordinary observation and receipt faults must refuse both a new client stream
// and a replacement start. Only the synthetic loopback fixture is reachable.
// regression: #7347
func TestKubeCompatRestartHolds(t *testing.T) {
	const role = "worker-desk"
	for _, name := range []string{"missing-receipt", "stale-receipt", "changed-pod", "same-pod-new-container", "unknown-stop", "wrong-candidate-image"} {
		t.Run(name, func(t *testing.T) {
			w, s := restartWorld(t)
			if got := w.run(t, nil, "desk", "example", role); got.Code != 0 {
				t.Fatal(got)
			}
			path := filepath.Join(w.cellDir, "run", "kubecompat", role+".json")
			wantAttach, wantStart := "execution changed", "execution changed"
			switch name {
			case "missing-receipt":
				wantAttach, wantStart = "no measured current execution", "existing execution is unknown"
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "stale-receipt":
				r, err := kc.ReadReceipt(path)
				if err != nil {
					t.Fatal(err)
				}
				r.Identity.PodUID = "previous-pod"
				if err := kc.WriteReceipt(path, *r); err != nil {
					t.Fatal(err)
				}
			case "changed-pod":
				s.Change(role, func(r *testfixture.RoleState) { r.UID = "new-pod" })
			case "same-pod-new-container":
				s.Change(role, func(r *testfixture.RoleState) { r.ContainerID = "containerd://new"; r.Restart++ })
			case "unknown-stop":
				wantAttach, wantStart = "no measured current execution", "unobserved execution"
				s.Change(role, func(_ *testfixture.RoleState) { s.Fault = "absent" })
				if code, _ := kubeTTY(t, w, time.Second, "down", "example", role); code == 0 {
					t.Fatal("absence was accepted as clean termination")
				}
			case "wrong-candidate-image":
				// The configured and Pod-spec images match. Only observed container status differs.
				wantAttach, wantStart = "execution could not be measured", "execution could not be measured"
				s.Change(role, func(r *testfixture.RoleState) {
					r.ObservedImageID = "docker-pullable://" + s.Profile.Roles["intake-desk"].Image
				})
			}
			if got := w.run(t, nil, "attach", "example", role); got.Code == 0 {
				t.Fatal("expected attach refusal", wantAttach, got)
			}
			// Attach's CLI returns the refusal code; status prints the measured reason.
			state := w.run(t, nil, "status", "example")
			if !strings.Contains(state.Stdout, role+": held/unmeasured; "+wantAttach) {
				t.Fatal("expected held role status", wantAttach, state)
			}
			if got := w.run(t, nil, "desk", "example", role); got.Code == 0 || !strings.Contains(got.Stderr+got.Stdout, wantStart) {
				t.Fatal("expected start refusal", wantStart, got)
			}
			starts, _, _ := s.Counts()
			if starts != 1 {
				t.Fatal("replacement started", starts)
			}
		})
	}
}
