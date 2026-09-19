// state.go — the per-App state record `apps.state.json` (design.md §4, schema
// deskapps-state-v1). It is the ONLY thing `deskapps resume`/`status` (briefs 03/04) read;
// this brief owns writing it.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const stateSchema = "deskapps-state-v1"

// AppRowState is one App's state in the design.md §4 machine:
// pending → posted → keyed → avatar_ok → installed → verified, with posted → paused (the
// throttle) and paused → pending on resume.
type AppRowState string

const (
	StatePending   AppRowState = "pending"
	StatePosted    AppRowState = "posted"
	StatePaused    AppRowState = "paused"
	StateKeyed     AppRowState = "keyed"
	StateAvatarOK  AppRowState = "avatar_ok"
	StateInstalled AppRowState = "installed"
	StateVerified  AppRowState = "verified"
)

// AppRow is one App's row in apps.state.json.
type AppRow struct {
	App             string      `json:"app"`
	Tier            string      `json:"tier"`
	Owner           string      `json:"owner"`
	OwnerKind       string      `json:"owner_kind"` // "org" or "me"
	Roles           []string    `json:"roles"`
	ReadOnly        bool        `json:"read_only"`
	State           AppRowState `json:"state"`
	StateNonce      string      `json:"state_nonce"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	AppID           string      `json:"app_id,omitempty"`
	ClientID        string      `json:"client_id,omitempty"`
	AvatarConfirmed bool        `json:"avatar_confirmed"`
	Detail          string      `json:"detail,omitempty"`
}

// StateFile is the whole apps.state.json document.
type StateFile struct {
	Schema string   `json:"schema"`
	Apps   []AppRow `json:"apps"`
}

func statePath() string {
	return deskkit.ConfigHomeWritePath("apps.state.json")
}

// loadState reads apps.state.json, or returns a fresh empty document if it does not exist
// yet — a first run is not an error.
func loadState() (*StateFile, error) {
	b, err := os.ReadFile(statePath())
	if os.IsNotExist(err) {
		return &StateFile{Schema: stateSchema}, nil
	}
	if err != nil {
		return nil, err
	}
	var sf StateFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return nil, err
	}
	return &sf, nil
}

// saveState writes apps.state.json 0600 (it is machine state, not a secret, but it carries
// per-App identifiers so it follows the same custody mode as the rest of the credential
// plane).
func saveState(sf *StateFile) error {
	sf.Schema = stateSchema
	b, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	p := statePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// rowByApp returns the row named app, or nil.
func (sf *StateFile) rowByApp(app string) *AppRow {
	for i := range sf.Apps {
		if sf.Apps[i].App == app {
			return &sf.Apps[i]
		}
	}
	return nil
}

// rowByNonce returns the row whose state_nonce equals nonce, or nil. This is the ONE lookup
// a callback is trusted through: an unknown nonce means "no pending row issued this state",
// which is the single-point-of-failure line in the brief — the record-side match is the
// third, independent layer behind the state check and the loopback bind.
func (sf *StateFile) rowByNonce(nonce string) *AppRow {
	if nonce == "" {
		return nil
	}
	for i := range sf.Apps {
		if sf.Apps[i].StateNonce == nonce {
			return &sf.Apps[i]
		}
	}
	return nil
}

// specFor returns the AppSpec named app, or the zero value if absent.
func specFor(specs []AppSpec, app string) AppSpec {
	for _, s := range specs {
		if s.Name == app {
			return s
		}
	}
	return AppSpec{}
}
