package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Workspace receipts are private to each cell. A persistent random metadata token prevents
// a recycled Herdr ID from becoming authority over someone else's workspace.
type herdrWorkspace struct {
	ID         string `json:"workspace_id"`
	Owner      string `json:"owner"`
	DefaultTab string `json:"default_tab,omitempty"`
}

func (c *Cell) herdrDir() string { return filepath.Join(c.Dir, "run", "herdr") }
func (c *Cell) herdrLease() (*os.File, error) {
	dir := c.herdrDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := privateCommsDir(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "owner.lock")
	if _, err := os.Lstat(path); err == nil {
		if err = privateCommsFile(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func (c *Cell) readHerdrWorkspace() (*herdrWorkspace, error) {
	path := filepath.Join(c.herdrDir(), "workspace.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if err := privateCommsFile(path); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var w herdrWorkspace
	if json.Unmarshal(b, &w) != nil || w.Owner == "" {
		return nil, fmt.Errorf("invalid cell Herdr workspace receipt")
	}
	return &w, nil
}
func (c *Cell) saveHerdrWorkspace(w *herdrWorkspace) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	dir := c.herdrDir()
	f, err := os.CreateTemp(dir, "workspace-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "workspace.json"))
}
func verifyHerdrWorkspace(w *herdrWorkspace) (bool, error) {
	// An ID-less receipt is a create that never finished (Herdr refused, or the run died). The
	// owner token is reported only after the ID is saved, under the cell lease, so no live
	// workspace can carry it: the receipt holds no authority and the caller recreates or drops it.
	if w.ID == "" {
		return false, nil
	}
	out, err := exec.Command("herdr", "workspace", "list").Output()
	if err != nil {
		return false, fmt.Errorf("inspect cell Herdr workspace: %w", err)
	}
	var d struct {
		Result struct {
			Workspaces *[]struct {
				ID     string            `json:"workspace_id"`
				Tokens map[string]string `json:"tokens"`
			} `json:"workspaces"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &d) != nil || d.Result.Workspaces == nil {
		return false, fmt.Errorf("invalid Herdr workspace inventory")
	}
	for _, live := range *d.Result.Workspaces {
		if live.ID == w.ID {
			if live.Tokens["cellctl_owner"] != w.Owner {
				return false, fmt.Errorf("cell Herdr workspace ownership changed; refusing control")
			}
			return true, nil
		}
	}
	return false, nil
}
func (c *Cell) ensureHerdrWorkspace() (*herdrWorkspace, error) {
	lock, err := c.herdrLease()
	if err != nil {
		return nil, err
	}
	defer func() { deskkit.UnlockFile(lock); lock.Close() }()
	w, err := c.readHerdrWorkspace()
	if err != nil {
		return nil, err
	}
	if w != nil {
		live, err := verifyHerdrWorkspace(w)
		if err != nil {
			return nil, err
		}
		if live {
			return w, nil
		}
	}
	if !helpHas("report-metadata", "herdr", "workspace") {
		return nil, fmt.Errorf("Herdr requires workspace report-metadata for cell ownership")
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	w = &herdrWorkspace{Owner: hex.EncodeToString(nonce[:])}
	if err = c.saveHerdrWorkspace(w); err != nil {
		return nil, err
	}
	out, err := exec.Command("herdr", "workspace", "create", "--cwd", c.Dir, "--label", c.Name, "--no-focus").Output()
	if err != nil {
		return nil, err
	}
	var d struct {
		Result struct {
			Workspace struct {
				ID string `json:"workspace_id"`
			} `json:"workspace"`
			Tab struct {
				ID string `json:"tab_id"`
			} `json:"tab"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &d) != nil || d.Result.Workspace.ID == "" {
		return nil, fmt.Errorf("Herdr created no identifiable cell workspace")
	}
	w.ID = d.Result.Workspace.ID
	w.DefaultTab = d.Result.Tab.ID
	if err = c.saveHerdrWorkspace(w); err != nil {
		return nil, err
	}
	if err = exec.Command("herdr", "workspace", "report-metadata", w.ID, "--source", "cellctl", "--token", "cellctl_owner="+w.Owner).Run(); err != nil {
		return nil, err
	}
	live, err := verifyHerdrWorkspace(w)
	if err != nil {
		return nil, err
	}
	if !live {
		return nil, fmt.Errorf("new cell Herdr workspace disappeared")
	}
	return w, nil
}
func (c *Cell) closeHerdrWorkspace() error {
	if _, err := os.Lstat(c.herdrDir()); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	lock, err := c.herdrLease()
	if err != nil {
		return err
	}
	defer func() { deskkit.UnlockFile(lock); lock.Close() }()
	w, err := c.readHerdrWorkspace()
	if err != nil || w == nil {
		return err
	}
	live, err := verifyHerdrWorkspace(w)
	if err != nil {
		return err
	}
	if live {
		if err = exec.Command("herdr", "workspace", "close", w.ID).Run(); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(c.herdrDir(), "workspace.json"))
}

func (c *Cell) dropHerdrDefault(w *herdrWorkspace) error {
	if w.DefaultTab == "" {
		return nil
	}
	lock, err := c.herdrLease()
	if err != nil {
		return err
	}
	defer func() { deskkit.UnlockFile(lock); lock.Close() }()
	saved, err := c.readHerdrWorkspace()
	if err != nil {
		return err
	}
	if saved == nil || saved.ID != w.ID || saved.Owner != w.Owner {
		return fmt.Errorf("cell workspace changed during launch")
	}
	if saved.DefaultTab == "" {
		return nil
	}
	live, err := verifyHerdrWorkspace(saved)
	if err != nil {
		return err
	}
	if !live {
		return fmt.Errorf("cell workspace disappeared during launch")
	}
	tab := saved.DefaultTab
	saved.DefaultTab = ""
	if err = c.saveHerdrWorkspace(saved); err != nil {
		return err
	}
	return exec.Command("herdr", "tab", "close", tab).Run()
}
