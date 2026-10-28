// Package testfixture supplies a real HTTPS/SPDY fake API for offline tests only.
package testfixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	apps "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type RoleState struct {
	UID, ContainerID string
	Restart          int32
	Replicas         int32
	Started          int
	Present, Ready   bool
	Termination      *core.ContainerStateTerminated
}
type Server struct {
	mu                         sync.Mutex
	HTTP                       *httptest.Server
	Profile                    *kc.Profile
	ProfilePath                string
	Roles                      map[string]*RoleState
	Paths                      []string
	Frames, Resizes            int
	ReplaceOnUpgrade, Redirect bool
	AttachRedirect             string
	LastResize                 kc.Size
	Fault                      string
	Calls                      int
	Files                      map[string]string
}

func New(t testing.TB) *Server {
	t.Helper()
	dir := t.TempDir()
	s := &Server{Roles: map[string]*RoleState{}, Files: map[string]string{}}
	p := &kc.Profile{Schema: kc.ProfileName, Cell: "example", Namespace: "assay-desks", Kubeconfig: filepath.Join(dir, "kubeconfig.json"), Context: "fixture", TLSServerName: "example.com", Custody: kc.Custody{Operator: "synthetic-role-custodian", Terminal: "role-custodian", Logs: "role-custodian", Nodes: "role-custodian", DownstreamLogs: "excluded"}, Roles: map[string]kc.Role{}}
	for _, role := range kc.Roles {
		v := kc.Role{StatefulSet: role, Pod: role + "-0", Container: role, Image: "ghcr.io/medici-finance/assay/" + role + ":fixture@sha256:" + strings.Repeat("a", 64), Agent: "/work/fixture-agent", VersionManifest: "/work/package.json", Workdir: "/work/repository", AgentSHA256: strings.Repeat("b", 64), AgentVersion: "2.0.0", Model: "synthetic-no-model", Instructions: "/work/role.md", InstructionsSHA256: strings.Repeat("c", 64), InitializationID: strings.Repeat("d", 64)}
		v.Cell = p.Cell
		v.InitialPrompt = kc.RolePrompt(role)
		v.LaunchID = v.LaunchDigest(role)
		p.Roles[role] = v
		s.Roles[role] = &RoleState{UID: "pod-" + role, ContainerID: "containerd://" + role, Ready: true}
	}
	s.Profile = p
	s.HTTP = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.HTTP.Close)
	p.Endpoint = s.HTTP.URL
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.HTTP.Certificate().Raw})
	sum := sha256.Sum256(ca)
	p.CASHA256 = hex.EncodeToString(sum[:])
	config := clientcmdapi.NewConfig()
	config.Clusters["fixture"] = &clientcmdapi.Cluster{Server: p.Endpoint, TLSServerName: p.TLSServerName, CertificateAuthorityData: ca}
	config.Contexts[p.Context] = &clientcmdapi.Context{Cluster: "fixture", AuthInfo: "fixture", Namespace: p.Namespace}
	config.AuthInfos["fixture"] = &clientcmdapi.AuthInfo{Token: "synthetic-offline-fixture"}
	raw, e := clientcmd.Write(*config)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p.Kubeconfig, raw, 0600); e != nil {
		t.Fatal(e)
	}
	s.ProfilePath = filepath.Join(dir, "profile.json")
	s.Save(t)
	return s
}
func (s *Server) Save(t testing.TB) {
	t.Helper()
	raw, e := json.Marshal(s.Profile)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.ProfilePath, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func (s *Server) Counts() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	starts := 0
	for _, r := range s.Roles {
		starts += r.Started
	}
	return starts, s.Frames, s.Resizes
}
func (s *Server) Change(role string, fn func(*RoleState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.Roles[role])
}
func (s *Server) podSpec(role string) core.PodSpec {
	r := s.Profile.Roles[role]
	no := false
	return core.PodSpec{AutomountServiceAccountToken: &no, Containers: []core.Container{{Name: role, Image: r.Image, Args: []string{"/usr/local/bin/desk-session"}, Stdin: true, TTY: true, Env: []core.EnvVar{{Name: "ASSAY_DESK", Value: role}, {Name: "ASSAY_COMPAT_LAUNCH_ID", Value: r.LaunchID}, {Name: "ASSAY_COMPAT_INIT_ID", Value: r.InitializationID}}, ReadinessProbe: &core.Probe{ProbeHandler: core.ProbeHandler{Exec: &core.ExecAction{Command: []string{"/usr/local/bin/desk-session", "--ready"}}}}, TerminationMessagePath: "/dev/termination-log", TerminationMessagePolicy: core.TerminationMessageReadFile}}}
}
func (s *Server) serve(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	s.Paths = append(s.Paths, req.Method+" "+req.URL.RequestURI())
	s.Calls++
	if s.Redirect {
		s.mu.Unlock()
		http.Redirect(w, req, "https://outside.example.invalid", http.StatusFound)
		return
	}
	if s.Fault == "partition" {
		s.mu.Unlock()
		http.Error(w, "synthetic partition", 503)
		return
	}
	path := req.URL.Path
	parts := strings.Split(path, "/")
	role := ""
	for _, name := range kc.Roles {
		if strings.Contains(path, "/statefulsets/"+name) || strings.Contains(path, "/pods/"+name+"-0") {
			role = name
			break
		}
	}
	if role == "" || !strings.Contains(path, "/namespaces/"+s.Profile.Namespace+"/") {
		s.mu.Unlock()
		http.Error(w, "foreign named operation", 403)
		return
	}
	r := s.Roles[role]
	uid := types.UID("sts-" + role)
	if strings.HasSuffix(path, "/attach") {
		if s.AttachRedirect != "" {
			destination := s.AttachRedirect
			s.mu.Unlock()
			http.Redirect(w, req, destination, http.StatusFound)
			return
		}
		if req.Method != "POST" || req.URL.Query().Get("container") != role || req.URL.Query().Get("tty") != "true" {
			s.mu.Unlock()
			http.Error(w, "wrong attach", 403)
			return
		}
		if s.ReplaceOnUpgrade {
			r.UID = "replacement-pod"
			r.ContainerID = "containerd://replacement"
		}
		s.mu.Unlock()
		s.attach(w, req, role)
		return
	}
	var object any
	if strings.HasSuffix(path, "/scale") {
		if req.Method == "PUT" {
			var scale autoscaling.Scale
			if json.NewDecoder(req.Body).Decode(&scale) != nil || scale.UID != uid || (scale.Spec.Replicas != 0 && scale.Spec.Replicas != 1) {
				s.mu.Unlock()
				http.Error(w, "wrong scale", 409)
				return
			}
			r.Replicas = scale.Spec.Replicas
			if r.Replicas == 1 && !r.Present {
				r.Started++
				r.Present = true
				r.Termination = nil
				r.Ready = true
				r.UID = fmt.Sprintf("pod-%s-%d", role, r.Started)
				r.ContainerID = fmt.Sprintf("containerd://%s-%d", role, r.Started)
			}
			if r.Replicas == 0 {
				r.Present = false
			}
		}
		object = &autoscaling.Scale{TypeMeta: meta.TypeMeta{APIVersion: "autoscaling/v1", Kind: "Scale"}, ObjectMeta: meta.ObjectMeta{Name: role, Namespace: s.Profile.Namespace, UID: uid, ResourceVersion: "1"}, Spec: autoscaling.ScaleSpec{Replicas: r.Replicas}}
	} else if len(parts) > 2 && parts[2] == "apps" || strings.Contains(path, "/statefulsets/") {
		object = &apps.StatefulSet{TypeMeta: meta.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"}, ObjectMeta: meta.ObjectMeta{Name: role, Namespace: s.Profile.Namespace, UID: uid}, Spec: apps.StatefulSetSpec{Replicas: &r.Replicas, Template: core.PodTemplateSpec{Spec: s.podSpec(role)}}}
	} else {
		if !r.Present {
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(meta.Status{Status: "Failure", Reason: meta.StatusReasonNotFound, Code: 404})
			return
		}
		yes := true
		cs := core.ContainerStatus{Name: role, ContainerID: r.ContainerID, RestartCount: r.Restart, ImageID: "docker-pullable://" + s.Profile.Roles[role].Image, Ready: r.Ready, State: core.ContainerState{Running: &core.ContainerStateRunning{}}}
		if r.Termination != nil {
			cs.State = core.ContainerState{Terminated: r.Termination}
		}
		annotations := map[string]string{}
		if s.Fault == "busy" {
			annotations["assay.dev/work-state"] = "busy"
		}
		if s.Fault == "unknown-work" {
			annotations["assay.dev/work-state"] = "unmeasured"
		}
		object = &core.Pod{TypeMeta: meta.TypeMeta{APIVersion: "v1", Kind: "Pod"}, ObjectMeta: meta.ObjectMeta{Annotations: annotations, Name: role + "-0", Namespace: s.Profile.Namespace, UID: types.UID(r.UID), OwnerReferences: []meta.OwnerReference{{Kind: "StatefulSet", Name: role, UID: uid, Controller: &yes}}}, Spec: s.podSpec(role), Status: core.PodStatus{Phase: core.PodRunning, ContainerStatuses: []core.ContainerStatus{cs}}}
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(object)
}
func (s *Server) attach(w http.ResponseWriter, req *http.Request, role string) {
	if _, e := httpstream.Handshake(req, w, []string{"v4.channel.k8s.io"}); e != nil {
		return
	}
	type received struct {
		stream httpstream.Stream
		reply  <-chan struct{}
	}
	incoming := make(chan received, 8)
	conn := spdy.NewResponseUpgrader().UpgradeResponse(w, req, func(st httpstream.Stream, reply <-chan struct{}) error { incoming <- received{st, reply}; return nil })
	if conn == nil {
		return
	}
	defer conn.Close()
	expected := 2 // error + resize
	if req.URL.Query().Get("stdin") == "true" {
		expected++
	}
	if req.URL.Query().Get("stdout") == "true" {
		expected++
	}
	streams := map[string]httpstream.Stream{}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for len(streams) < expected {
		select {
		case pair := <-incoming:
			select {
			case <-pair.reply:
			case <-timer.C:
				return
			}
			streams[pair.stream.Headers().Get(core.StreamType)] = pair.stream
		case <-timer.C:
			return
		case <-req.Context().Done():
			return
		}
	}
	if resize := streams[core.StreamTypeResize]; resize != nil {
		go func() {
			var size kc.Size
			if json.NewDecoder(resize).Decode(&size) == nil {
				s.mu.Lock()
				s.Resizes++
				s.LastResize = size
				s.mu.Unlock()
			}
		}()
	}
	if output := streams[core.StreamTypeStdout]; output != nil {
		_, _ = output.Write([]byte("synthetic retained terminal\n"))
	}
	if input := streams[core.StreamTypeStdin]; input != nil {
		data := make([]byte, 4096)
		n, _ := input.Read(data)
		if n > 0 {
			s.mu.Lock()
			s.Frames++
			r := s.Roles[role]
			frame := string(data[:n])
			if strings.HasPrefix(frame, "\x1dASSAY_STOP ") {
				if s.Fault == "absent" {
					r.Present = false
				} else if s.Fault == "changed" {
					r.UID = "changed"
				} else if s.Fault != "no-ack" {
					message, _ := json.Marshal(map[string]any{"schema": kc.ProfileName, "pod_uid": r.UID, "launch_id": s.Profile.Roles[role].LaunchID, "clean": true, "group_empty": s.Fault != "survivor"})
					r.Termination = &core.ContainerStateTerminated{ContainerID: r.ContainerID, ExitCode: 0, Message: string(message)}
					r.Ready = false
				}
			} else {
				s.Files[role] = frame
			}
			s.mu.Unlock()
		}
	}
	if status := streams[core.StreamTypeError]; status != nil {
		raw, _ := json.Marshal(meta.Status{Status: meta.StatusSuccess})
		_, _ = status.Write(raw)
		_ = status.Close()
	}
}

func (s *Server) Input(role string) string { s.mu.Lock(); defer s.mu.Unlock(); return s.Files[role] }
func (s *Server) Resize() kc.Size          { s.mu.Lock(); defer s.mu.Unlock(); return s.LastResize }
func (s *Server) ContactCount() int        { s.mu.Lock(); defer s.mu.Unlock(); return s.Calls }
