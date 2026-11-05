// Package native is the effectful typed Kubernetes adapter. Its constructors do
// no discovery or network I/O and accept no ambient credential or target source.
package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

type Client struct {
	p      *kc.Profile
	digest string
	config *rest.Config
	apps   *appsclient.AppsV1Client
	core   *coreclient.CoreV1Client
}

func fileData(data []byte, path string) ([]byte, error) {
	if len(data) > 0 && path != "" {
		return nil, errors.New("ambiguous credential source")
	}
	if path != "" {
		if !filepath.IsAbs(path) {
			return nil, errors.New("credential path must be explicit and absolute")
		}
		f, e := os.Open(path)
		if e != nil {
			return nil, errors.New("selected credential file unavailable")
		}
		defer f.Close()
		data, e = io.ReadAll(io.LimitReader(f, 1048577))
		if e != nil {
			return nil, errors.New("selected credential file unreadable")
		}
	}
	if len(data) > 1048576 {
		return nil, errors.New("selected credential data exceeds bound")
	}
	return data, nil
}

// Config parses only the explicitly selected file and snapshots static credentials.
// In particular it never calls BuildConfigFromFlags or InClusterConfig.
func Config(p *kc.Profile) (*rest.Config, error) {
	if e := p.Validate(p.Cell); e != nil {
		return nil, e
	}
	raw, e := fileData(nil, p.Kubeconfig)
	if e != nil {
		return nil, e
	}
	conf, e := clientcmd.Load(raw)
	if e != nil {
		return nil, errors.New("selected kubeconfig is malformed")
	}
	contextRow, ok := conf.Contexts[p.Context]
	if !ok || contextRow.Namespace != p.Namespace || len(contextRow.Extensions) > 0 {
		return nil, errors.New("selected context or namespace differs")
	}
	cluster, ok := conf.Clusters[contextRow.Cluster]
	if !ok || cluster.Server != p.Endpoint || cluster.InsecureSkipTLSVerify || cluster.TLSServerName != p.TLSServerName || cluster.ProxyURL != "" || len(cluster.Extensions) > 0 {
		return nil, errors.New("selected API/TLS identity differs")
	}
	ca, e := fileData(cluster.CertificateAuthorityData, cluster.CertificateAuthority)
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(ca)
	if len(ca) == 0 || hex.EncodeToString(sum[:]) != p.CASHA256 {
		return nil, errors.New("selected CA pin differs")
	}
	auth, ok := conf.AuthInfos[contextRow.AuthInfo]
	if !ok || auth.Exec != nil || auth.AuthProvider != nil || auth.Username != "" || auth.Password != "" || auth.Impersonate != "" || auth.ImpersonateUID != "" || len(auth.ImpersonateGroups) > 0 || len(auth.ImpersonateUserExtra) > 0 || len(auth.Extensions) > 0 {
		return nil, errors.New("only explicit static compatibility credentials are supported")
	}
	cert, e := fileData(auth.ClientCertificateData, auth.ClientCertificate)
	if e != nil {
		return nil, e
	}
	key, e := fileData(auth.ClientKeyData, auth.ClientKey)
	if e != nil {
		return nil, e
	}
	token := auth.Token
	if auth.TokenFile != "" {
		if token != "" {
			return nil, errors.New("ambiguous static token")
		}
		data, e := fileData(nil, auth.TokenFile)
		if e != nil {
			return nil, e
		}
		token = strings.TrimSpace(string(data))
	}
	hasCert := len(cert) > 0 && len(key) > 0
	if (len(cert) > 0) != (len(key) > 0) || hasCert == (token != "") {
		return nil, errors.New("exactly one static credential is required")
	}
	cfg := &rest.Config{Host: p.Endpoint, BearerToken: token, TLSClientConfig: rest.TLSClientConfig{ServerName: p.TLSServerName, CAData: ca, CertData: cert, KeyData: key}, Timeout: 15 * time.Second, UserAgent: "assay-cell-kube-compat-v1", QPS: 5, Burst: 10}
	cfg.ContentType = "application/json"
	cfg.AcceptContentTypes = "application/json"
	cfg.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper { return &boundTransport{p.Endpoint, rt} }
	return cfg, nil
}

type boundTransport struct {
	endpoint string
	inner    http.RoundTripper
}

func (b *boundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, _ := url.Parse(b.endpoint)
	if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host || req.URL.User != nil {
		return nil, errors.New("request escaped explicit API target")
	}
	return b.inner.RoundTrip(req)
}
func New(p *kc.Profile, digest string) (*Client, error) {
	cfg, e := Config(p)
	if e != nil {
		return nil, e
	}
	return NewConfigured(p, digest, cfg)
}

// NewConfigured is for native fake-API fixtures. Production composes only Config.
func NewConfigured(p *kc.Profile, digest string, cfg *rest.Config) (*Client, error) {
	hc, e := rest.HTTPClientFor(cfg)
	if e != nil {
		return nil, errors.New("selected API transport unavailable")
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("API redirects are forbidden") }
	apps, e := appsclient.NewForConfigAndClient(cfg, hc)
	if e != nil {
		return nil, e
	}
	corec, e := coreclient.NewForConfigAndClient(cfg, hc)
	if e != nil {
		return nil, e
	}
	return &Client{p, digest, cfg, apps, corec}, nil
}
func (c *Client) Observe(ctx context.Context, role string) (kc.Snapshot, error) {
	r, ok := c.p.Roles[role]
	if !ok {
		return kc.Snapshot{}, errors.New("foreign role")
	}
	sts, e := c.apps.StatefulSets(c.p.Namespace).Get(ctx, r.StatefulSet, meta.GetOptions{})
	if e != nil {
		return kc.Snapshot{}, errors.New("named workload unavailable")
	}
	s := kc.Snapshot{StatefulSetUID: string(sts.UID)}
	if sts.Spec.Replicas == nil || (*sts.Spec.Replicas != 0 && *sts.Spec.Replicas != 1) || sts.Namespace != c.p.Namespace || sts.Name != r.StatefulSet || sts.UID == "" {
		return s, errors.New("named workload identity differs")
	}
	s.Replicas = *sts.Spec.Replicas
	if e = checkPod(sts.Spec.Template.Spec, role, r); e != nil {
		return s, e
	}
	pod, e := c.core.Pods(c.p.Namespace).Get(ctx, r.Pod, meta.GetOptions{})
	if apierrors.IsNotFound(e) {
		return s, nil
	}
	if e != nil {
		return s, errors.New("named Pod unavailable")
	}
	if pod.Namespace != c.p.Namespace || pod.Name != r.Pod || pod.UID == "" {
		return s, errors.New("named Pod identity differs")
	}
	owned := false
	for _, o := range pod.OwnerReferences {
		if o.Controller != nil && *o.Controller && o.Kind == "StatefulSet" && o.Name == r.StatefulSet && o.UID == sts.UID {
			owned = true
		}
	}
	if !owned {
		return s, errors.New("Pod does not belong to named workload")
	}
	if e = checkPod(pod.Spec, role, r); e != nil {
		return s, e
	}
	workState := pod.Annotations["assay.dev/work-state"]
	s.Busy = workState == "busy"
	s.WorkUnknown = workState != "" && workState != "busy" && workState != "idle-reconciled"
	s.PodPresent = true
	s.Identity = kc.Identity{Cell: c.p.Cell, Role: role, ConfigDigest: c.digest, Namespace: c.p.Namespace, StatefulSetUID: string(sts.UID), PodUID: string(pod.UID), Container: r.Container, Image: r.Image}
	if len(pod.Status.ContainerStatuses) == 0 && pod.Status.Phase == core.PodPending {
		s.Pending = true
		return s, nil
	}
	if len(pod.Status.ContainerStatuses) != 1 {
		return s, errors.New("container identity unavailable")
	}
	cs := pod.Status.ContainerStatuses[0]
	if cs.State.Waiting != nil && cs.ContainerID == "" && cs.RestartCount == 0 && pod.Status.Phase == core.PodPending {
		s.Pending = true
		return s, nil
	}
	imageDigest := r.Image[strings.LastIndex(r.Image, "@")+1:]
	if cs.Name != r.Container || cs.ContainerID == "" || cs.RestartCount < 0 || !strings.HasSuffix(cs.ImageID, "@"+imageDigest) {
		return s, errors.New("running container/image identity differs")
	}
	s.Identity = kc.Identity{Cell: c.p.Cell, Role: role, ConfigDigest: c.digest, Namespace: c.p.Namespace, StatefulSetUID: string(sts.UID), PodUID: string(pod.UID), Container: r.Container, ContainerID: cs.ContainerID, RestartCount: cs.RestartCount, Image: r.Image}
	s.Running = cs.State.Running != nil && cs.Ready && pod.DeletionTimestamp == nil
	term := cs.State.Terminated
	if term == nil {
		term = cs.LastTerminationState.Terminated
	}
	if term != nil {
		var message struct {
			Schema     string `json:"schema"`
			PodUID     string `json:"pod_uid"`
			LaunchID   string `json:"launch_id"`
			Clean      bool   `json:"clean"`
			GroupEmpty bool   `json:"group_empty"`
		}
		if json.Unmarshal([]byte(term.Message), &message) == nil && message.Schema == kc.ProfileName {
			s.Termination = &kc.Termination{ContainerID: term.ContainerID, PodUID: message.PodUID, LaunchID: message.LaunchID, Clean: message.Clean, GroupEmpty: message.GroupEmpty, ExitCode: term.ExitCode}
		}
	}
	return s, nil
}
func checkPod(p core.PodSpec, role string, r kc.Role) error {
	if p.AutomountServiceAccountToken == nil || *p.AutomountServiceAccountToken || p.HostPID || p.HostIPC || p.HostNetwork || p.ShareProcessNamespace != nil && *p.ShareProcessNamespace || len(p.InitContainers) > 0 || len(p.EphemeralContainers) > 0 || len(p.Containers) != 1 {
		return errors.New("Pod isolation differs")
	}
	v := p.Containers[0]
	if v.Name != role || v.Image != r.Image || len(v.Command) != 0 || len(v.Args) != 1 || v.Args[0] != "/usr/local/bin/desk-session" || !v.Stdin || !v.TTY || v.StdinOnce {
		return errors.New("pinned agent launch differs")
	}
	if v.ReadinessProbe == nil || v.ReadinessProbe.Exec == nil || len(v.ReadinessProbe.Exec.Command) != 2 || v.ReadinessProbe.Exec.Command[0] != "/usr/local/bin/desk-session" || v.ReadinessProbe.Exec.Command[1] != "--ready" || v.ReadinessProbe.HTTPGet != nil || v.ReadinessProbe.TCPSocket != nil || v.ReadinessProbe.GRPC != nil || v.TerminationMessagePath != "/dev/termination-log" || v.TerminationMessagePolicy != core.TerminationMessageReadFile {
		return errors.New("exact agent readiness/termination protocol missing")
	}
	env := map[string]string{}
	for _, e := range v.Env {
		if e.ValueFrom == nil {
			env[e.Name] = e.Value
		}
	}
	if env["ASSAY_DESK"] != role || env["ASSAY_COMPAT_LAUNCH_ID"] != r.LaunchID || env["ASSAY_COMPAT_INIT_ID"] != r.InitializationID {
		return errors.New("pinned launch identity differs")
	}
	return nil
}
func (c *Client) Scale(ctx context.Context, role, uid string, replicas int32) error {
	r, ok := c.p.Roles[role]
	if !ok || (replicas != 0 && replicas != 1) {
		return errors.New("unsupported named scale")
	}
	scale, e := c.apps.StatefulSets(c.p.Namespace).GetScale(ctx, r.StatefulSet, meta.GetOptions{})
	if e != nil {
		return errors.New("scale could not be measured")
	}
	if scale.UID != types.UID(uid) {
		return errors.New("workload changed before scale")
	}
	scale.Spec.Replicas = replicas
	_, e = c.apps.StatefulSets(c.p.Namespace).UpdateScale(ctx, r.StatefulSet, scale, meta.UpdateOptions{})
	if e != nil {
		return errors.New("scale outcome unknown; held")
	}
	return nil
}

// gateStreams performs a post-upgrade exact execution check before allowing any
// user byte in or out. Kubernetes attach has no UID/containerID request precondition.
// It does not auto-reconnect. A name replacement during upgrade yields zero user I/O.
type gateStreams struct {
	once   sync.Once
	check  func() error
	err    error
	cancel context.CancelFunc
}

func (g *gateStreams) allow() error {
	g.once.Do(func() {
		g.err = g.check()
		if g.err != nil {
			g.cancel()
		}
	})
	return g.err
}

type gateRead struct {
	g *gateStreams
	r io.Reader
}

func (g gateRead) Read(p []byte) (int, error) {
	if e := g.g.allow(); e != nil {
		return 0, e
	}
	return g.r.Read(p)
}

type gateWrite struct {
	g *gateStreams
	w io.Writer
}

func (g gateWrite) Write(p []byte) (int, error) {
	if e := g.g.allow(); e != nil {
		return 0, e
	}
	return g.w.Write(p)
}

type sizes struct {
	ctx   context.Context
	input <-chan kc.Size
	gate  *gateStreams
}

func (q sizes) Next() *remotecommand.TerminalSize {
	if e := q.gate.allow(); e != nil {
		return nil
	}
	select {
	case <-q.ctx.Done():
		return nil
	case s, ok := <-q.input:
		if !ok {
			return nil
		}
		return &remotecommand.TerminalSize{Width: s.Width, Height: s.Height}
	}
}
func (c *Client) Attach(ctx context.Context, role string, want kc.Identity, t kc.Terminal) error {
	if e := want.Valid(c.p, c.digest, role); e != nil {
		return e
	}
	before, e := c.Observe(ctx, role)
	if e != nil || !before.Running {
		return errors.New("attach execution unavailable")
	}
	if e = want.Same(before.Identity); e != nil {
		return e
	}
	cfg := rest.CopyConfig(c.config)
	cfg.Timeout = 0
	wrapper, upgrader, e := spdy.RoundTripperFor(cfg)
	if e != nil {
		return errors.New("attach transport unavailable")
	}
	r := c.p.Roles[role]
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	options := &core.PodAttachOptions{Container: r.Container, Stdin: t.In != nil, Stdout: t.Out != nil, Stderr: false, TTY: true}
	req := c.core.RESTClient().Post().Namespace(c.p.Namespace).Resource("pods").Name(r.Pod).SubResource("attach").VersionedParams(options, runtime.NewParameterCodec(scheme))
	executor, e := remotecommand.NewSPDYExecutorRejectRedirects(wrapper, upgrader, http.MethodPost, req.URL())
	if e != nil {
		return errors.New("attach transport unavailable")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	gate := &gateStreams{cancel: cancel, check: func() error {
		observed, e := c.Observe(streamCtx, role)
		if e != nil || !observed.Running {
			return errors.New("execution unavailable after attach upgrade")
		}
		return want.Same(observed.Identity)
	}}
	opts := remotecommand.StreamOptions{Tty: true}
	if t.In != nil {
		opts.Stdin = gateRead{gate, t.In}
	}
	if t.Out != nil {
		opts.Stdout = gateWrite{gate, t.Out}
	}
	if t.Sizes != nil {
		opts.TerminalSizeQueue = sizes{streamCtx, t.Sizes, gate}
	}
	if e = executor.StreamWithContext(streamCtx, opts); e != nil {
		return errors.New("attach ended or identity changed; reconnect requires a fresh exact check")
	}
	if gate.err != nil {
		return gate.err
	}
	return nil
}

// Assert the SDK adapter implements only the module-local lifecycle boundary.
var _ kc.Effects = (*Client)(nil)
