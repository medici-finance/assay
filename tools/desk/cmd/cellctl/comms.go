package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/runnertable"
)

// The operator-owned manifest binds a cell to the existing gateway and drain.
// It contains paths and a pinned reader entry, never inline signing credentials.
type deskCommsConfig struct {
	Cell            string            `json:"cell"`
	Mode            string            `json:"mode"`
	Repo            string            `json:"repo"`
	Socket          string            `json:"socket"`
	QueueDir        string            `json:"queue_dir"`
	TrustStore      string            `json:"trust_store"`
	SigningKeys     map[string]string `json:"signing_keys"`
	Decider         json.RawMessage   `json:"decider"`
	ClaudeConfigDir string            `json:"claude_config_dir,omitempty"`
}

func privateCommsFile(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("comms custody path must be absolute")
	}
	_, fi, err := deskkit.LstatCustody(path, deskkit.CustodyNoLinks)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("comms custody path must be a regular file")
	}
	return deskkit.VerifyCustodyOwnerOnly(path, fi)
}

func privateCommsDir(path string) error {
	_, fi, err := deskkit.LstatCustody(path, deskkit.CustodyNoLinks)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("comms storage must be a directory")
	}
	if runtime.GOOS == "windows" {
		return deskkit.VerifyCustodyOwnerOnly(path, fi)
	}
	if fi.Mode().Perm() != 0700 {
		return fmt.Errorf("comms directory must have mode 0700: %s", path)
	}
	return nil
}

func (c *Cell) loadDeskComms() (*deskCommsConfig, error) {
	path := c.Env.Get("CELL_COMMS_CONFIG")
	if path == "" {
		return nil, nil
	}
	if c.Kind != "house" {
		return nil, fmt.Errorf("host comms requires a house cell")
	}
	if err := privateCommsFile(path); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var cfg deskCommsConfig
	if err := d.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("comms config: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("comms config must contain one JSON object")
	}
	if cfg.Cell != c.Name {
		return nil, fmt.Errorf("comms config belongs to a different cell")
	}
	if cfg.Mode == "disabled" {
		return nil, nil
	}
	if cfg.Mode != "interim" {
		return nil, fmt.Errorf("comms mode must be disabled or interim; autonomous session firing is not supported")
	}
	if parts := strings.Split(cfg.Repo, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("comms repo must name the quarantine issue repository as owner/repo")
	}
	if err := commstransport.Validate(cfg.Socket); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cfg.QueueDir) {
		return nil, fmt.Errorf("comms queue_dir must be absolute")
	}
	if err := privateCommsFile(cfg.TrustStore); err != nil {
		return nil, err
	}
	for role, path := range cfg.SigningKeys {
		if !valueIn(role, knownRoles) {
			return nil, fmt.Errorf("comms config has unknown signing role %q", role)
		}
		if err := privateCommsFile(path); err != nil {
			return nil, fmt.Errorf("comms signing key for %s: %w", role, err)
		}
	}
	for _, role := range c.upRoles(false) {
		if cfg.SigningKeys[role] == "" {
			return nil, fmt.Errorf("comms signing key missing for %s", role)
		}
	}
	if len(cfg.Decider) == 0 {
		return nil, fmt.Errorf("comms requires a pinned contained decider; otherwise every message would be held")
	}
	if _, err := runnertable.LoadDecider(func(k string) string {
		if k == runnertable.EnvDeciderKey {
			return string(cfg.Decider)
		}
		return ""
	}); err != nil {
		return nil, err
	}
	if cfg.ClaudeConfigDir != "" && !filepath.IsAbs(cfg.ClaudeConfigDir) {
		return nil, fmt.Errorf("comms claude_config_dir must be absolute")
	}
	return &cfg, nil
}

// Drop ambient identity before composing this role's context. A parent desk's
// key or role must never leak into a different cell/role launch.
func (c *Cell) deskCommsEnv(role string, env []string) ([]string, error) {
	cfg, err := c.loadDeskComms()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(key), "DESK_COMMS_") || strings.EqualFold(key, "DESK_CELL") || strings.EqualFold(key, "DESK_ROLE") {
			continue
		}
		out = append(out, kv)
	}
	if cfg == nil {
		return out, nil
	}
	key := cfg.SigningKeys[role]
	if key == "" {
		return nil, fmt.Errorf("comms signing key missing for %s", role)
	}
	return append(out, "DESK_CELL="+c.Name, "DESK_ROLE="+role,
		"DESK_COMMS_GATEWAY="+cfg.Socket, "DESK_COMMS_KEY="+key), nil
}

func (c *Cell) commsServiceEnv(cfg *deskCommsConfig) []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "ASSAY_COMMS_") || strings.HasPrefix(key, "ASSAY_RUNNER_") || key == "GH_TOKEN" || key == "GITHUB_TOKEN" || key == "GITLAB_TOKEN" {
			continue
		}
		env = append(env, kv)
	}
	values := map[string]string{
		"HOME": c.Home, "USERPROFILE": c.Home, "ASSAY_CONFIG_HOME": c.Config,
		"PATH":      deskToolsBin(c.Env) + string(filepath.ListSeparator) + c.Env.Get("PATH"),
		"DESK_LOOP": "the-desk", "DESK_SESSION": c.Name + "-comms",
		"ASSAY_COMMS_GATEWAY_ENABLE": "1", "ASSAY_COMMS_CELL": c.Name,
		"ASSAY_COMMS_QUEUE_DIR": cfg.QueueDir, "ASSAY_COMMS_SOCKET": cfg.Socket,
		"ASSAY_COMMS_LOCAL_ONLY":  "1",
		"ASSAY_COMMS_TRUST_STORE": cfg.TrustStore,
		"ASSAY_RUNNER_DECIDER":    string(cfg.Decider), "ASSAY_COMMS_REPO": cfg.Repo,
	}
	if cfg.ClaudeConfigDir != "" {
		values["CLAUDE_CONFIG_DIR"] = cfg.ClaudeConfigDir
	}
	for k, v := range values {
		env = envSet(env, k, v)
	}
	return env
}

func cmdComms(cell string, args []string) {
	recovering := len(args) == 2 && args[0] == "recover" && args[1] == "--confirm-stopped"
	if !recovering && (len(args) != 1 || (args[0] != "check" && args[0] != "run")) {
		die("comms <cell> check|run|recover --confirm-stopped")
	}
	c := loadCell(cell)
	cfg, err := c.loadDeskComms()
	if err != nil {
		die("comms: %v", err)
	}
	if cfg == nil {
		die("comms is disabled; configure CELL_COMMS_CONFIG for this cell before starting it")
	}
	var commands [][]string
	for _, name := range []string{"commsgw", "commsloop"} {
		suffix := ""
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		path := filepath.Join(deskToolsBin(c.Env), name+suffix)
		if err := commsBinary(path); err != nil {
			die("comms: %v", err)
		}
		commands = append(commands, []string{path})
	}
	if args[0] == "check" {
		fmt.Printf("[comms] cell=%s mode=interim config=valid binaries=present (gateway reachability and model credentials not probed)\n", c.Name)
		return
	}
	lease, err := cellcadence.Acquire(filepath.Join(c.Dir, "run", "comms"))
	if err != nil {
		die("comms ownership: %v", err)
	}
	defer lease.Close()
	if recovering {
		// Confirmation covers the prior gateway, drain and every owned child.
		if runtime.GOOS != "windows" {
			if fi, err := os.Lstat(cfg.Socket); err == nil {
				if fi.Mode()&os.ModeSocket == 0 {
					die("comms recovery refuses a non-socket endpoint")
				}
				if err := os.Remove(cfg.Socket); err != nil {
					die("comms recovery: %v", err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				die("comms recovery: %v", err)
			}
		}
		if err := os.Remove(filepath.Join(c.Dir, "run", "comms", "checkpoint.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			die("comms recovery: %v", err)
		}
		fmt.Println("[comms] recovery recorded; run the service to start it")
		return
	}
	for _, dir := range []string{cfg.QueueDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			die("comms queue: %v", err)
		}
		if err := privateCommsDir(dir); err != nil {
			die("comms queue custody: %v", err)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.MkdirAll(filepath.Dir(cfg.Socket), 0700); err != nil {
			die("comms socket directory: %v", err)
		}
		if _, err := os.Lstat(cfg.Socket); !errors.Is(err, os.ErrNotExist) {
			die("comms endpoint already exists; inspect the prior gateway before restarting")
		}
	}
	ctx, stop := cellprocess.NotifyContext(context.Background())
	defer stop()
	fmt.Printf("[comms] cell=%s mode=interim endpoint=%s; starting gateway and mailbox drain\n", c.Name, cfg.Socket)
	err = lease.RunInteractive(ctx, c.Name, "comms", func(ctx context.Context) cellcadence.Result {
		result := runCommsPair(ctx, commands, c.commsServiceEnv(cfg))
		removeStoppedEndpoint(result, cfg.Socket)
		return result
	})
	// A joined cancellation can also contain a storage or child-cleanup error.
	// Never turn that uncertainty into a successful shutdown.
	if err != nil {
		die("comms stopped: %v", err)
	}
}

// commsBinary checks that a supervised service is present in the cell's own
// desk-tools directory. The path is fixed by the cell, never searched on PATH.
func commsBinary(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("%s is not executable", path)
	}
	return nil
}

// removeStoppedEndpoint removes the gateway socket once both services have
// certainly stopped. The supervisor kills its children, so a gateway never
// gets to close its own listener here. After an uncertain stop, where a child
// may still be serving, the socket stays for recover --confirm-stopped.
func removeStoppedEndpoint(result cellcadence.Result, socket string) {
	if result.Uncertain || runtime.GOOS == "windows" {
		return
	}
	if fi, err := os.Lstat(socket); err == nil && fi.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(socket)
	}
}

func runCommsPair(ctx context.Context, commands [][]string, env []string) cellcadence.Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan cellcadence.Result, len(commands))
	for _, command := range commands {
		go func(argv []string) {
			code, uncertain, err := cellprocess.Run(ctx, argv, env, "", os.Stdout, os.Stderr)
			results <- cellcadence.Result{ExitCode: code, Uncertain: uncertain, Err: err}
		}(command)
	}
	first := <-results
	if ctx.Err() == nil && first.Err == nil {
		first.Err = fmt.Errorf("comms service exited unexpectedly (exit %d)", first.ExitCode)
	}
	cancel() // a failed/exited gateway must not leave a detached drain (or vice versa).
	for i := 1; i < len(commands); i++ {
		r := <-results
		first.Uncertain = first.Uncertain || r.Uncertain
		if r.Uncertain {
			first.Err = errors.Join(first.Err, r.Err)
		}
	}
	return first
}
