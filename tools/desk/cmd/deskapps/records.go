// records.go — the two record writes a successful conversion makes: the PEM (0600, never
// printed/logged/served — design.md §1 rule 2) and `apps.env` (App IDs, client id, webhook
// secret, and the `<ROLE>_APP=`/`READ_APP=` bindings brief 01 defined and this brief
// writes). Both are MERGES: a fresh install must not clobber unrelated keys a prior run, or
// a hand edit, already wrote.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// newStateNonce returns a fresh per-row state nonce: cryptographically random and
// unguessable. It is the ONE control that keeps a callback from being accepted by a
// listener that did not issue it (brief 02's single-point-of-failure line); the loopback
// bind and the record-side match (rowByNonce) are the two independent layers behind it.
func newStateNonce() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// mkdirSecure creates dir (and parents) and enforces mode 0700 on dir itself (S-4):
// os.MkdirAll leaves an ALREADY-existing directory's mode untouched, so a credential-plane
// dir a prior run or a hand edit left wider stays wide without this explicit Chmod.
func mkdirSecure(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// writeFileSecure writes data to p and enforces mode 0600 on it (S-4): os.WriteFile applies
// its perm argument only when it CREATES the file, so a pre-existing PEM/apps.env/state file
// left at a looser mode (a hand edit, a restore, an older writer) would be rewritten in place
// and keep that looser mode. The explicit Chmod closes that window.
func writeFileSecure(p string, data []byte) error {
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

// writePEM writes the App's private key 0600 at <config-home>/<app>.pem, byte-equal to the
// conversion's `pem` field. Nothing else in this package holds a PEM past this call — see
// secrets_test.go for the assertion that it never reaches stdout, the audit log, or a
// served page.
func writePEM(app, pem string) error {
	p := deskkit.ConfigHomeWritePath(app + ".pem")
	if err := mkdirSecure(filepath.Dir(p)); err != nil {
		return err
	}
	return writeFileSecure(p, []byte(pem))
}

// appsEnvPath is where apps.env is read from and written to: the head of the
// App-credential search path (deskkit.ConfigHomeWritePath), matching every other writer of
// this file.
func appsEnvPath() string {
	return deskkit.ConfigHomeWritePath("apps.env")
}

// writeAppRecords writes/updates this App's `<PREFIX>_APP_ID`, `<PREFIX>_CLIENT_ID` and
// `<PREFIX>_WEBHOOK_SECRET` lines in apps.env, where <PREFIX> is deskkit.AppEnvPrefix(appName)
// — the same prefix desktoken resolves App IDs and installation IDs under.
func writeAppRecords(appName, appID, clientID, webhookSecret string) error {
	prefix := deskkit.AppEnvPrefix(appName)
	return mergeAppsEnv(map[string]string{
		prefix + "_APP_ID":         appID,
		prefix + "_CLIENT_ID":      clientID,
		prefix + "_WEBHOOK_SECRET": webhookSecret,
	})
}

// writeBindings writes the `<ROLE>_APP=<app-name>` bindings (brief 01) for every role bound
// to spec, or the `READ_APP=<app-name>` line for the team-tier read App (brief 02 facts:
// "team → all six roles → <prefix>-act except reads ... via a READ_APP=<prefix>-read line").
func writeBindings(spec AppSpec) error {
	updates := map[string]string{}
	if spec.ReadOnly {
		updates["READ_APP"] = spec.Name
	} else {
		for _, role := range spec.Roles {
			key := strings.ToUpper(strings.ReplaceAll(role, "-", "_")) + "_APP"
			updates[key] = spec.Name
		}
	}
	return mergeAppsEnv(updates)
}

// mergeAppsEnv rewrites apps.env: existing lines are kept verbatim except for keys present
// in updates (rewritten in place, preserving position), and any update key not already
// present is appended at the end in sorted order for a deterministic diff. The file is
// written 0600 — it carries App IDs and secrets, so it follows the credential-plane mode
// discipline the PEM does.
func mergeAppsEnv(updates map[string]string) error {
	p := appsEnvPath()
	existing, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var lines []string
	seen := map[string]bool{}
	if len(existing) > 0 {
		for _, line := range strings.Split(strings.TrimRight(string(existing), "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			key := ""
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				noExport := strings.TrimPrefix(trimmed, "export ")
				if eq := strings.IndexByte(noExport, '='); eq >= 0 {
					key = strings.TrimSpace(noExport[:eq])
				}
			}
			if key != "" {
				if v, ok := updates[key]; ok {
					lines = append(lines, key+"="+v)
					seen[key] = true
					continue
				}
			}
			lines = append(lines, line)
		}
	}

	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !seen[k] {
			lines = append(lines, k+"="+updates[k])
		}
	}

	if err := mkdirSecure(filepath.Dir(p)); err != nil {
		return err
	}
	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	return writeFileSecure(p, []byte(out))
}

// readAppsEnv returns the current apps.env contents, or "" if it does not exist yet —
// diagnostics and tests read the file back through this rather than re-deriving the path.
func readAppsEnv() string {
	b, err := os.ReadFile(appsEnvPath())
	if err != nil {
		return ""
	}
	return string(b)
}
