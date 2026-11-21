//go:build darwin || linux

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
)

// The synthetic API's scale/attach effects reach an actual retained PID 1, via
// a local fixture runtime. The image digest in the Kubernetes identity remains
// the published candidate's; the executable launcher bytes come from its commit.
type interactionFixture struct {
	host, image, name, dir, owner, role, source, mutation string
	binding                                               kc.Role
}

func fixtureFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFixtureFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func shaString(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func (f *interactionFixture) docker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"--host", f.host}, args...)...)
	var diagnostic strings.Builder
	cmd.Stderr = &diagnostic
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("offline docker %s: %w: %s", args[0], err, diagnostic.String())
	}
	return strings.TrimSpace(string(out)), nil
}

func prepareInteraction(t *testing.T, role string, binding kc.Role, mutation string) *interactionFixture {
	t.Helper()
	host := os.Getenv("ASSAY_TEST_DOCKER_HOST")
	socket := strings.TrimPrefix(host, "unix://")
	if socket == host || !filepath.IsAbs(socket) {
		t.Fatal("could-not-check: set ASSAY_TEST_DOCKER_HOST to an explicit local Unix socket")
	}
	image := os.Getenv("ASSAY_TEST_FIXTURE_IMAGE")
	if image == "" {
		image = "python:3.13-slim"
	} else if !regexp.MustCompile("^sha256:[a-f0-9]{64}$").MatchString(image) {
		t.Fatal("could-not-check: ASSAY_TEST_FIXTURE_IMAGE requires an immutable local image ID")
	}
	dir := t.TempDir()
	f := &interactionFixture{role: role, host: host, image: image, dir: dir, mutation: mutation,
		name: fmt.Sprintf("assay-offline-%s-%d-%d", role, os.Getpid(), time.Now().UnixNano())}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var err error
	f.image, err = f.docker(ctx, "image", "inspect", image, "--format", "{{.Id}}")
	if err != nil {
		t.Fatal("could-not-check: cached offline fixture runtime required", err)
	}
	if err := f.selectMountOwner(ctx); err != nil {
		t.Fatal("could-not-check: private fixture mount ownership", err)
	}
	root := filepath.Join("..", "..", "..", "..")
	component := filepath.Join(root, "containers/k8s/components/compatibility-cellctl")
	var candidate struct {
		Source string `json:"source_revision"`
		Record string `json:"record_sha256"`
		Images map[string]string
		State  string
	}
	if err := json.Unmarshal(fixtureFile(t, filepath.Join(component, "candidate.json")), &candidate); err != nil {
		t.Fatal(err)
	}
	record := fixtureFile(t, filepath.Join(component, "candidate-release-images.json"))
	var release struct {
		Source string `json:"source_revision"`
		Images []struct{ Name, Ref string }
	}
	if err := json.Unmarshal(record, &release); err != nil {
		t.Fatal(err)
	}
	if candidate.State != "published" || !regexp.MustCompile("^[a-f0-9]{40}$").MatchString(candidate.Source) || shaString(record) != candidate.Record || release.Source != candidate.Source {
		t.Fatal("candidate source/release record mismatch")
	}
	f.source = candidate.Source
	matched := false
	for _, image := range release.Images {
		if image.Name == role && image.Ref == candidate.Images[role] && regexp.MustCompile("@sha256:[a-f0-9]{64}$").MatchString(image.Ref) {
			binding.Image = image.Ref
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("candidate image missing from release record", role)
	}
	launcher, err := exec.Command("git", "show", f.source+":containers/k8s/compatibility/desk-session.py").Output()
	if err != nil {
		t.Fatal("could-not-check: candidate launcher source unavailable", err)
	}
	writeFixtureFile(t, filepath.Join(dir, "desk-session.py"), launcher, 0644)
	agent := fixtureFile(t, filepath.Join("testdata", "interaction", "claude"))
	// Use the same pinned executable and package layout as the shipped profile.
	repo := filepath.Join(dir, "repository")
	agentRel := ".npm-global/bin/claude"
	versionRel := ".npm-global/lib/node_modules/@anthropic-ai/claude-code/package.json"
	for _, path := range []string{repo, filepath.Dir(filepath.Join(dir, agentRel)), filepath.Dir(filepath.Join(dir, versionRel))} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixtureFile(t, filepath.Join(dir, agentRel), agent, 0755)
	writeFixtureFile(t, filepath.Join(dir, versionRel), []byte(`{"name":"@anthropic-ai/claude-code","version":"fixture-1"}`), 0644)
	instructions := []byte("Offline " + role + " fixture. Summarize the seeded work items.\n")
	writeFixtureFile(t, filepath.Join(dir, "role.md"), instructions, 0644)
	writeFixtureFile(t, filepath.Join(repo, "input.txt"), []byte("collect "+role+" work\ncheck "+role+" result\n"), 0644)
	binding.Agent = "/work/" + agentRel
	binding.VersionManifest = "/work/" + versionRel
	binding.Workdir = "/work/repository"
	binding.Model = "offline-fixture-" + role
	binding.AgentVersion = "fixture-1"
	binding.AgentSHA256 = shaString(agent)
	binding.Instructions = "/work/role.md"
	binding.InstructionsSHA256 = shaString(instructions)
	binding.LaunchID = binding.LaunchDigest(role)
	f.binding = binding
	descriptor, err := json.Marshal(binding.LaunchDescriptor(role))
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(dir, "profile.json"), descriptor, 0644)
	if _, err = f.docker(ctx, append(f.runBase(false), "--rm", f.image, "-I", "/work/desk-session.py", "--prepare", "--profile", "/work/profile.json")...); err != nil {
		t.Fatal(err)
	}
	return f
}

// The daemon's user namespace can map the native host UID differently. Stat
// the empty mount itself, read-only, before writing any fixture files.
func (f *interactionFixture) selectMountOwner(ctx context.Context) error {
	args := append(f.runBase(true), "--rm", f.image, "-I", "-c",
		"import os; s = os.stat('/work'); print(f'{s.st_uid}:{s.st_gid}')")
	owner, err := f.docker(ctx, args...)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`^[0-9]{1,10}:[0-9]{1,10}$`).MatchString(owner) {
		return fmt.Errorf("invalid fixture owner %q", owner)
	}
	f.owner = owner
	return nil
}

func (f *interactionFixture) runBase(probe bool) []string {
	mount := "type=bind,src=" + f.dir + ",dst=/work"
	args := []string{"run", "--pull=never", "--network=none", "--cap-drop=ALL", "--security-opt", "no-new-privileges", "--entrypoint", "python3"}
	if probe {
		mount += ",readonly"
	} else {
		args = append(args, "--user", f.owner)
	}
	return append(args, "--mount", mount)
}

// regression: #7356
func TestKubeCompatPrivateFixtureOptions(t *testing.T) {
	f := interactionFixture{dir: "/private-fixture", owner: "1000:1000"}
	option := func(args []string, key string) string {
		if i := slices.Index(args, key); i >= 0 && i+1 < len(args) {
			return args[i+1]
		}
		return ""
	}
	probe := f.runBase(true)
	workload := f.runBase(false)
	if option(probe, "--mount") != "type=bind,src=/private-fixture,dst=/work,readonly" ||
		option(workload, "--mount") != "type=bind,src=/private-fixture,dst=/work" ||
		option(workload, "--user") != f.owner {
		t.Fatalf("mount ownership or write boundary changed: probe=%v workload=%v", probe, workload)
	}
	for _, args := range [][]string{probe, workload} {
		if !slices.Contains(args, "--cap-drop=ALL") || !slices.Contains(args, "--network=none") || !slices.Contains(args, "--pull=never") || option(args, "--security-opt") != "no-new-privileges" {
			t.Fatalf("offline fixture isolation changed: %v", args)
		}
	}
}

func (f *interactionFixture) start() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := f.binding
	args := append(f.runBase(false), "-dit", "--name", f.name, "-e", "ASSAY_DESK="+f.role, "-e", "ASSAY_POD_UID=pod-"+f.role+"-1", "-e", "ASSAY_COMPAT_INIT_ID="+r.InitializationID, "-e", "ASSAY_COMPAT_LAUNCH_ID="+r.LaunchID, f.image, "-I", "/work/desk-session.py", "--profile", "/work/profile.json")
	if _, err := f.docker(ctx, args...); err != nil {
		return err
	}
	for {
		data, _ := os.ReadFile(filepath.Join(f.dir, ".assay-compat", "state.json"))
		var state struct{ State string }
		if json.Unmarshal(data, &state) == nil && state.State == "running" {
			if _, err := os.Stat(filepath.Join(f.dir, "repository", "calls.jsonl")); err == nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			logs, _ := f.docker(context.Background(), "logs", "--tail", "4", f.name)
			return fmt.Errorf("candidate did not start synthetic agent: %s", logs)
		case <-time.After(25 * time.Millisecond):
		}
	}
	if f.mutation == "duplicate-start" {
		_, err := f.docker(ctx, "exec", "-w", r.Workdir, "-e", "FIXTURE_DUPLICATE=1", "-e", "ASSAY_CELL_ROLE="+f.role, "-e", "DESK_LOOP="+f.role, f.name, r.Agent, "--model", r.Model, "--append-system-prompt-file", r.Instructions, r.InitialPrompt)
		return err
	}
	return nil
}

// Only this fixture's named container is attachable. The socket has to be
// explicitly supplied; neither a Docker context nor a kubeconfig is consulted.
func (f *interactionFixture) terminal(ctx context.Context, role string, input io.Reader, output io.Writer, resize io.Reader) error {
	if role != f.role || input == nil || output == nil || resize == nil {
		return fmt.Errorf("unexpected fixture attach for %q", role)
	}
	dial := &net.Dialer{Timeout: 3 * time.Second}
	socket := strings.TrimPrefix(f.host, "unix://")
	conn, err := dial.DialContext(ctx, "unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	stopped := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopped()
	req, err := http.NewRequestWithContext(ctx, "POST", "http://fixture/containers/"+f.name+"/attach?stdin=1&stdout=1&stderr=1&stream=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	if err = req.Write(conn); err != nil {
		return err
	}
	buffered := bufio.NewReader(conn)
	response, err := http.ReadResponse(buffered, req)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("fixture attach: %s", response.Status)
	}
	if f.mutation == "model-on-attach" {
		if _, err = io.WriteString(conn, "work\n"); err != nil {
			return err
		}
	}
	results := make(chan error, 3)
	go func() { _, err := io.Copy(conn, input); results <- err }()
	go func() { _, err := io.Copy(output, buffered); results <- err }()
	go func() {
		tr := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) { return dial.DialContext(c, "unix", socket) }}
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
		decoder := json.NewDecoder(resize)
		for {
			var size kc.Size
			if err := decoder.Decode(&size); err != nil {
				return
			}
			target := "http://fixture/containers/" + f.name + "/resize?h=" + strconv.Itoa(int(size.Height)) + "&w=" + strconv.Itoa(int(size.Width))
			req, err := http.NewRequestWithContext(ctx, "POST", target, nil)
			if err != nil {
				results <- err
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				results <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("fixture resize: %s", resp.Status)
				return
			}
		}
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-results:
		return err
	}
}
