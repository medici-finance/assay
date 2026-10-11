package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellreader"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type commsInitOptions struct {
	ReaderCmd, Model, Effort, Pin, Repo string
	ReaderContained                     bool
}

func writeCommsPrivate(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if e := f.Sync(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}

func (c *Cell) initComms(o commsInitOptions) error {
	if c.Kind != "house" {
		return fmt.Errorf("comms init requires a house cell")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("comms init private provisioning currently requires macOS or Linux")
	}
	if c.Env.Get("DRY_RUN") != "1" {
		lock, err := acquireCellLock(c, ".comms-init.lock")
		if err != nil {
			return err
		}
		defer func() { deskkit.UnlockFile(lock); lock.Close() }()
	}
	selected := c.Env.Get("CELL_COMMS_CONFIG")
	path := selected
	if path == "" {
		path = filepath.Join(c.Dir, "comms", "comms.json")
	}
	if path != "" {
		if _, err := os.Lstat(path); err == nil {
			old := c.Env.Get("CELL_COMMS_CONFIG")
			c.Env.Put("CELL_COMMS_CONFIG", path)
			cfg, err := c.loadDeskComms()
			c.Env.Put("CELL_COMMS_CONFIG", old)
			if err != nil {
				return err
			}
			if cfg == nil {
				return fmt.Errorf("existing comms configuration is disabled; select interim mode deliberately")
			}
			if _, err = c.commsCommands(); err != nil {
				return err
			}
			if selected == "" && c.Env.Get("DRY_RUN") != "1" {
				applyEnvKVs(c.Env, filepath.Join(c.Dir, "cell.env"), false, []string{"CELL_COMMS_CONFIG=" + path})
				c.Env.Put("CELL_COMMS_CONFIG", path)
			}
			fmt.Printf("[comms] existing configuration valid; reused %s\n", path)
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if _, err := c.commsCommands(); err != nil {
		return err
	}
	repo := o.Repo
	if repo == "" {
		repo = c.Env.Get("CELL_REPO_SLUG")
	}
	if repo == "" {
		for _, entry := range rootEntries(c.Env.Get("CELL_ROOTS")) {
			if filepath.Clean(entry[1]) == filepath.Clean(c.Repo) {
				repo = entry[0]
				break
			}
		}
	}
	if parts := strings.Split(repo, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("comms init needs --repo owner/repo or a primary CELL_ROOTS entry")
	}
	root := filepath.Join(c.Dir, "comms")
	workspace := filepath.Join(root, "reader-workspace")
	var command []string
	var authSource, readerHome string
	model, effort, pin := o.Model, o.Effort, o.Pin
	if o.ReaderCmd != "" {
		if err := json.Unmarshal([]byte(o.ReaderCmd), &command); err != nil || len(command) == 0 || command[0] == "" {
			return fmt.Errorf("--reader-cmd must be a nonempty JSON argv")
		}
		if model == "" || pin == "" || !o.ReaderContained {
			return fmt.Errorf("external ACP reader requires --model, --reader-pin and --reader-contained attestation")
		}
	} else {
		policy, _, err := c.cellModelPolicy()
		if err != nil {
			return err
		}
		requested := model
		if requested == "" {
			requested = "fast"
		}
		if policy != nil {
			res, err := policy.Resolve("worker-desk", "codex", requested, "codex")
			if err != nil {
				return err
			}
			model = res.Model
			if effort == "" {
				effort = res.Effort
			}
		} else {
			if model == "" {
				model = c.Env.Get("TIER_MODEL_FAST_CODEX")
			}
			if effort == "" {
				effort = "low"
			}
		}
		binary, err := exec.LookPath("codex")
		if err != nil {
			return fmt.Errorf("native reader requires codex on PATH: %w", err)
		}
		binary, err = filepath.EvalSymlinks(binary)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(binary)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		version, err := exec.Command(binary, "--version").Output()
		if err != nil {
			return err
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		home := c.Env.Get("CODEX_HOME")
		if home == "" {
			home = filepath.Join(hostHome(c.Env), ".codex")
		}
		authSource = filepath.Join(home, "auth.json")
		if err := privateCommsFile(authSource); err != nil {
			return fmt.Errorf("reader needs an existing private Codex auth.json login: %w", err)
		}
		readerHome = filepath.Join(root, "codex-home")
		command = []string{self, "comms-reader", "--codex", binary, "--sha256", hex.EncodeToString(sum[:]), "--model", model, "--effort", effort, "--home", readerHome, "--workspace", workspace}
		pin = "cellctl-contained-reader-v1+" + strings.TrimSpace(string(version))
	}
	if effort != "" && !valueIn(effort, []string{"minimal", "low", "medium", "high", "xhigh", "max"}) {
		return fmt.Errorf("invalid reader effort")
	}
	if model == "" || isFloatingAlias(policyBase(model)) {
		return fmt.Errorf("comms reader needs an exact model pin")
	}
	if c.Env.Get("DRY_RUN") == "1" {
		fmt.Printf("[dry-run] comms init: private seed, trust store, five role bindings and manifest under %s; model=%s; no credentials created\n", root, model)
		return nil
	}
	if err := os.MkdirAll(root, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := privateCommsDir(root); err != nil {
		return err
	}
	if err := os.MkdirAll(workspace, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := privateCommsDir(workspace); err != nil {
		return err
	}
	if readerHome != "" {
		if err := os.MkdirAll(readerHome, 0700); err != nil {
			return err
		}
		if err := privateCommsDir(readerHome); err != nil {
			return err
		}
		auth := filepath.Join(readerHome, "auth.json")
		if fi, err := os.Lstat(auth); err == nil {
			target, e := os.Readlink(auth)
			if e != nil || fi.Mode()&os.ModeSymlink == 0 || target != authSource {
				return fmt.Errorf("reader auth link differs from the existing operator login")
			}
		} else if os.IsNotExist(err) {
			if _, err = newLinker().link(authSource, auth); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	seedPath := filepath.Join(root, "signing.key")
	var seed []byte
	if b, err := readPrivateComms(seedPath); err == nil {
		seed, err = hex.DecodeString(strings.TrimSpace(string(b)))
		if err != nil || len(seed) != ed25519.SeedSize {
			return fmt.Errorf("existing signing seed malformed")
		}
	} else if os.IsNotExist(err) {
		seed = make([]byte, ed25519.SeedSize)
		if _, err = rand.Read(seed); err != nil {
			return err
		}
		if err = writeCommsPrivate(seedPath, []byte(hex.EncodeToString(seed)+"\n")); err != nil {
			return err
		}
	} else {
		return err
	}
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	trustPath := filepath.Join(root, "trust.json")
	trust := map[string]string{c.Name: base64.StdEncoding.EncodeToString(public)}
	if b, err := readPrivateComms(trustPath); err == nil {
		var existing map[string]string
		if json.Unmarshal(b, &existing) != nil || len(existing) != 1 || existing[c.Name] != trust[c.Name] {
			return fmt.Errorf("existing trust store does not match this cell's signing seed")
		}
	} else if os.IsNotExist(err) {
		b, _ := json.Marshal(trust)
		if err = writeCommsPrivate(trustPath, append(b, '\n')); err != nil {
			return err
		}
	} else {
		return err
	}
	queue := filepath.Join(root, "queue")
	if err := os.MkdirAll(queue, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := privateCommsDir(queue); err != nil {
		return err
	}
	keys := map[string]string{}
	for _, role := range knownRoles {
		keys[role] = seedPath
	}
	decider, _ := json.Marshal(map[string]any{"cmd": command, "model": model, "pin": pin, "isolate": true, "contained": true})
	cfg := deskCommsConfig{Cell: c.Name, Mode: "interim", Repo: repo, Socket: filepath.Join(root, "gateway.sock"), QueueDir: queue, TrustStore: trustPath, SigningKeys: keys, Decider: decider}
	manifest := filepath.Join(root, "comms.json")
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := writeCommsPrivate(manifest, append(b, '\n')); err != nil {
		return err
	}
	c.Env.Put("CELL_COMMS_CONFIG", manifest)
	if _, err := c.loadDeskComms(); err != nil {
		return err
	}
	applyEnvKVs(c.Env, filepath.Join(c.Dir, "cell.env"), false, []string{"CELL_COMMS_CONFIG=" + manifest})
	fmt.Printf("[comms] initialized %s; keys stay private; reader=%s; start with cellctl up %s\n", c.Name, model, c.Ref)
	return nil
}

func cmdCommsInit(cell string, v *cli.Values) {
	c := loadCell(cell)
	if err := c.initComms(commsInitOptions{v.String("reader-cmd"), v.String("model"), v.String("effort"), v.String("reader-pin"), v.String("repo"), v.Bool("reader-contained")}); err != nil {
		die("comms init: %v", err)
	}
}

func cmdCommsReader(args []string) {
	o := cellreader.Options{}
	for len(args) > 0 {
		if len(args) < 2 {
			die("comms-reader: option requires a value")
		}
		key, value := args[0], args[1]
		args = args[2:]
		switch key {
		case "--codex":
			o.Binary = value
		case "--sha256":
			o.SHA256 = value
		case "--model":
			o.Model = value
		case "--effort":
			o.Effort = value
		case "--home":
			o.Home = value
		case "--workspace":
			o.Workspace = value
		default:
			die("comms-reader: unknown option %s", key)
		}
	}
	if err := cellreader.Serve(context.Background(), o, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "contained reader unavailable")
		exitWith(6)
	}
}

func readPrivateComms(path string) ([]byte, error) {
	if err := privateCommsFile(path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
