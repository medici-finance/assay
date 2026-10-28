package native_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/native"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/testfixture"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func runtimeFor(t *testing.T, s *testfixture.Server) *kc.Runtime {
	t.Helper()
	p, d, e := kc.Load(s.ProfilePath, "example")
	if e != nil {
		t.Fatal(e)
	}
	effect, e := native.New(p, d)
	if e != nil {
		t.Fatal(e)
	}
	return &kc.Runtime{Profile: p, Digest: d, CellDir: t.TempDir(), Effects: effect, Poll: time.Millisecond}
}

// regression: #4759
func TestKubeCompatIdentity(t *testing.T) {
	s := testfixture.New(t)
	rt := runtimeFor(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e := rt.Start(ctx, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	sizes := make(chan kc.Size, 1)
	sizes <- kc.Size{Width: 99, Height: 33}
	close(sizes)
	if e := rt.Attach(ctx, "worker-desk", kc.Terminal{In: strings.NewReader("retained input"), Out: &output, Sizes: sizes}); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(output.String(), "retained terminal") {
		t.Fatal("native stdout was not streamed")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, _, count := s.Counts()
		if count > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if s.Input("worker-desk") != "retained input" || s.Resize() != (kc.Size{Width: 99, Height: 33}) {
		t.Fatal("positive native stdin/resize not delivered", s.Input("worker-desk"), s.Resize())
	}
	starts, _, _ := s.Counts()
	if starts != 1 {
		t.Fatal("attach started another execution")
	}
	s.Change("worker-desk", func(r *testfixture.RoleState) { r.ContainerID = "containerd://replacement"; r.Restart++ })
	if e := rt.Attach(ctx, "worker-desk", kc.Terminal{In: strings.NewReader("must-not-reach")}); e == nil {
		t.Fatal("changed container accepted")
	}
	for _, mutation := range []string{"exec", "auth-provider", "proxy", "insecure", "CA", "TLS", "context", "namespace"} {
		t.Run(mutation, func(t *testing.T) {
			s := testfixture.New(t)
			raw, _ := os.ReadFile(s.Profile.Kubeconfig)
			cfg, e := clientcmd.Load(raw)
			if e != nil {
				t.Fatal(e)
			}
			switch mutation {
			case "exec":
				cfg.AuthInfos["fixture"].Exec = &clientcmdapi.ExecConfig{Command: "must-not-run"}
			case "auth-provider":
				cfg.AuthInfos["fixture"].AuthProvider = &clientcmdapi.AuthProviderConfig{Name: "must-not-run"}
			case "proxy":
				cfg.Clusters["fixture"].ProxyURL = "https://outside.example.invalid"
			case "insecure":
				cfg.Clusters["fixture"].InsecureSkipTLSVerify = true
			case "CA":
				s.Profile.CASHA256 = strings.Repeat("0", 64)
			case "TLS":
				cfg.Clusters["fixture"].TLSServerName = "outside.example.invalid"
			case "context":
				s.Profile.Context = "absent"
			case "namespace":
				cfg.Contexts["fixture"].Namespace = "foreign"
			}
			raw, e = clientcmd.Write(*cfg)
			if e != nil {
				t.Fatal(e)
			}
			_ = os.WriteFile(s.Profile.Kubeconfig, raw, 0600)
			if _, e = native.New(s.Profile, strings.Repeat("a", 64)); e == nil {
				t.Fatal("unsupported explicit target accepted")
			}
			if s.Calls != 0 {
				t.Fatal("invalid target caused operation contact")
			}
		})
	}
}

// regression: #4759
func TestKubeCompatUpgradeRace(t *testing.T) {
	s := testfixture.New(t)
	rt := runtimeFor(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, e := rt.Start(ctx, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	s.ReplaceOnUpgrade = true
	var out bytes.Buffer
	sizes := make(chan kc.Size, 1)
	sizes <- kc.Size{Width: 80, Height: 24}
	close(sizes)
	if e := rt.Attach(ctx, "worker-desk", kc.Terminal{In: strings.NewReader("must-not-reach"), Out: &out, Sizes: sizes}); e == nil {
		t.Fatal("replacement during native upgrade accepted")
	}
	_, frames, resizes := s.Counts()
	if frames != 0 || resizes != 0 || out.Len() != 0 {
		t.Fatal("user I/O reached replacement", frames, resizes, out.String())
	}
}

// regression: #4759
func TestKubeCompatNativeStop(t *testing.T) {
	s := testfixture.New(t)
	rt := runtimeFor(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, e := rt.Start(ctx, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	if e := rt.Stop(ctx, "worker-desk", true); e != nil {
		t.Fatal(e)
	}
	if _, e := rt.Start(ctx, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	starts, _, _ := s.Counts()
	if starts != 2 {
		t.Fatal("new clean Pod was not exactly one invocation", starts)
	}
}

// regression: #4759
func TestKubeCompatRedirect(t *testing.T) {
	s := testfixture.New(t)
	rt := runtimeFor(t, s)
	s.Redirect = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := rt.Start(ctx, "worker-desk"); e == nil {
		t.Fatal("redirect accepted")
	}
	if s.Calls != 1 {
		t.Fatal("redirect was followed or retried", s.Calls)
	}
}

// regression: #4759
func TestKubeCompatAttachRedirect(t *testing.T) {
	s := testfixture.New(t)
	rt := runtimeFor(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := rt.Start(ctx, "worker-desk"); e != nil {
		t.Fatal(e)
	}
	s.AttachRedirect = s.HTTP.URL + "/redirect-must-not-be-followed"
	before := s.ContactCount()
	if e := rt.Attach(ctx, "worker-desk", kc.Terminal{In: strings.NewReader("must-not-reach")}); e == nil {
		t.Fatal("attach redirect accepted")
	}
	// Current()+Attach() perform two exact-name preflight pairs; only the fifth
	// request may be the rejected attach upgrade. No redirected operation follows.
	if got := s.ContactCount() - before; got != 5 {
		t.Fatal("SPDY redirect followed or retried", got)
	}
	_, frames, resizes := s.Counts()
	if frames != 0 || resizes != 0 {
		t.Fatal("redirect received user I/O")
	}
}
