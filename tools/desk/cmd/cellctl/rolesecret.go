package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Role secrets (issue #6065): a project secret a worker or verifier
// window needs for a `service:<name>` Verify row, reaching that window as a PATH, never a value.
//
// This cut builds the CONTAINER surface only. The cell names its bindings in CELL_ROLE_SECRETS
// and declares CELL_ROLE_SECRET_SURFACE=container; the native container plan mounts each file
// read-only at /run/secrets/role-<service> into the worker and verifier containers ONLY, and
// exports one ASSAY_ROLE_SECRET_<SERVICE>_FILE variable naming that path. Every other surface —
// any host kind, the one-OS-user desktop cell, a container cell on an external launcher, and a
// declared `host` surface (whose per-role identity launch is not built yet) — binds no role
// secret: `check` reports a MISS and `desk` refuses to launch.
//
// Nothing here ever opens a secret file for reading: the checks are Lstat, Stat and the ACL
// probe, and every line printed, receipt written or error returned names a path, never a value.

const (
	envRoleSecrets       = "CELL_ROLE_SECRETS"
	envRoleSecretSurface = "CELL_ROLE_SECRET_SURFACE"
	roleSecretReceipts   = "role-secrets.receipts"
)

// roleSecretHolderRoles are the desk roles that may hold a role secret. The reviewer, intake and
// the coordinator never do.
var roleSecretHolderRoles = map[string]bool{"worker-desk": true, "verify-desk": true}

// roleSecretBinding is one CELL_ROLE_SECRETS entry, `<owner>/<repo>:<role>:<service>=<path>`.
type roleSecretBinding struct {
	Project, Role, Service, Path, EnvName string
}

// Seams for tests: the identity a secret file must be owned by (the identity cellctl runs as on
// a container surface), the access-control-list probe, and the clock receipts are stamped with.
var (
	roleSecretOwnerUID = os.Getuid
	roleSecretACLProbe = probeACL
	roleSecretNow      = time.Now
)

// roleSecretsConfigured reports whether the cell names any role-secret key at all.
func (c *Cell) roleSecretsConfigured() bool {
	return c.Env.IsSet(envRoleSecrets) && strings.TrimSpace(c.Env.Get(envRoleSecrets)) != "" ||
		c.Env.IsSet(envRoleSecretSurface) && strings.TrimSpace(c.Env.Get(envRoleSecretSurface)) != ""
}

// roleSecretBindings parses CELL_ROLE_SECRETS. Unset or empty is no binding. Both keys are read
// only from the cell's own cell.env: a value the launching shell exported, or any other layer,
// is refused, so no ambient variable can bind a secret to a cell.
func (c *Cell) roleSecretBindings() ([]roleSecretBinding, error) {
	for _, k := range []string{envRoleSecrets, envRoleSecretSurface} {
		if c.Env.IsSet(k) && strings.TrimSpace(c.Env.Get(k)) != "" && c.Env.Source(k) != layerCellEnv {
			return nil, fmt.Errorf("%s is set by the %s; a role-secret binding is read only from the cell's own cell.env", k, c.Env.Source(k))
		}
	}
	raw := strings.TrimSpace(c.Env.Get(envRoleSecrets))
	if raw == "" {
		return nil, nil
	}
	var out []roleSecretBinding
	seen := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		lhs, path, ok := strings.Cut(entry, "=")
		parts := strings.Split(lhs, ":")
		if !ok || len(parts) != 3 {
			return nil, fmt.Errorf("%s entry %q is not <owner>/<repo>:<role>:<service>=<absolute path>", envRoleSecrets, redactEntry(entry))
		}
		b := roleSecretBinding{Project: strings.TrimSpace(parts[0]), Role: strings.TrimSpace(parts[1]), Service: strings.TrimSpace(parts[2]), Path: strings.TrimSpace(path)}
		if !deskkit.ValidRoleSecretService(b.Service) {
			return nil, fmt.Errorf("%s: service %q is not a service name ([a-z0-9][a-z0-9._-]*)", envRoleSecrets, b.Service)
		}
		if !repoSlugShape(b.Project) {
			return nil, fmt.Errorf("%s: project %q is not <owner>/<repo>", envRoleSecrets, b.Project)
		}
		if !roleSecretHolderRoles[b.Role] {
			return nil, fmt.Errorf("%s binds service %s to role %s; only worker-desk and verify-desk may hold a role secret", envRoleSecrets, b.Service, b.Role)
		}
		if !filepath.IsAbs(b.Path) || filepath.Clean(b.Path) != b.Path || strings.ContainsAny(b.Path, ",\n\r\x00") {
			return nil, fmt.Errorf("%s: the path for %s/%s must be a clean absolute path", envRoleSecrets, b.Role, b.Service)
		}
		b.EnvName = deskkit.RoleSecretEnvName(b.Service)
		key := b.Role + "\x00" + b.EnvName
		if prev, dup := seen[key]; dup {
			return nil, fmt.Errorf("%s binds %s and %s to role %s, which spell the same variable %s", envRoleSecrets, prev, b.Service, b.Role, b.EnvName)
		}
		seen[key] = b.Service
		out = append(out, b)
	}
	return out, nil
}

// redactEntry keeps a malformed entry's text out of a message past its first `=`: a mistyped
// line could hold a value pasted where a path belongs.
func redactEntry(entry string) string {
	if i := strings.IndexByte(entry, '='); i >= 0 {
		return entry[:i] + "=…"
	}
	return entry
}

func repoSlugShape(s string) bool {
	owner, name, ok := strings.Cut(s, "/")
	return ok && owner != "" && name != "" && !strings.ContainsAny(s, " \t:=,") && !strings.Contains(name, "/")
}

// roleSecretSurfaceProblem is why this cell's declared read-isolation surface binds no role
// secret, or "" when it is the one surface this cut builds: a native container cell.
func (c *Cell) roleSecretSurfaceProblem() string {
	surface := strings.TrimSpace(c.Env.Get(envRoleSecretSurface))
	switch {
	case surface == "":
		return envRoleSecretSurface + " is unset: a cell that declares no per-role read-isolation surface binds no role secret"
	case surface == "host":
		return envRoleSecretSurface + "=host is not built yet (per-role OS identity launch is a follow-up); only the container surface binds a role secret"
	case surface != "container":
		return fmt.Sprintf("%s=%s is not a known surface; only container binds a role secret", envRoleSecretSurface, surface)
	case c.Kind != "container":
		return fmt.Sprintf("%s=container is declared, but CELL_KIND=%s is not a native container cell (a host kind runs every window as one OS user)", envRoleSecretSurface, c.Kind)
	case c.Env.Get("CELL_CONTAINER_CONFIG") == "":
		return "an external CELL_CONTAINER_LAUNCHER cannot be verified to keep the secret out of non-holding windows; use a native CELL_CONTAINER_CONFIG"
	}
	return ""
}

// roleSecretProblems verifies every binding against the container configuration and the host,
// and returns each refusal (empty: the bindings may be applied). It reads no file's contents.
func (c *Cell) roleSecretProblems(cfg *cellcontainer.Config, binds []roleSecretBinding) []string {
	var probs []string
	if p := c.roleSecretSurfaceProblem(); p != "" {
		return []string{p}
	}
	cell, ok := cfg.Cells[c.Name]
	if !ok {
		return []string{fmt.Sprintf("container config defines no cell %q", c.Name)}
	}
	for _, b := range binds {
		if b.Project != cell.Repo {
			probs = append(probs, fmt.Sprintf("%s/%s is bound for project %s, but this container cell serves %s", b.Role, b.Service, b.Project, cell.Repo))
		}
		if !roleConfigured(cell, b.Role) {
			probs = append(probs, fmt.Sprintf("%s/%s is bound to a role the container config does not define", b.Role, b.Service))
		}
		if p := roleSecretFileProblem(b.Path); p != "" {
			probs = append(probs, fmt.Sprintf("%s/%s file %s: %s", b.Role, b.Service, b.Path, p))
		}
	}
	probs = append(probs, c.roleSecretSiblingProblems(cfg, binds)...)
	return probs
}

func roleConfigured(cell cellcontainer.Cell, deskRole string) bool {
	for key := range cell.Roles {
		if cellcontainer.Roles[key] == deskRole {
			return true
		}
	}
	return false
}

// roleSecretFileProblem is why one secret file may not be bound, or "". The file must be a
// regular file (a symlink is refused outright, wherever it points), owner-only, owned by the
// identity cellctl runs as, carry no access-control list, sit in a directory that identity owns
// and nobody else can write (and that carries no list either), and lie in no git checkout or
// worktree.
func roleSecretFileProblem(path string) string {
	st, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "does not exist (a removed file refuses the next launch)"
	case err != nil:
		return "cannot be inspected"
	case st.Mode()&os.ModeSymlink != 0:
		return "is a symlink; bind the file itself"
	case !st.Mode().IsRegular():
		return "is not a regular file"
	case st.Mode().Perm()&0o077 != 0:
		return fmt.Sprintf("mode %04o is readable or writable beyond its owner; chmod 600", st.Mode().Perm())
	}
	uid, err := fileOwnerUID(path)
	if err != nil {
		return "owner cannot be read on this platform"
	}
	if want := roleSecretOwnerUID(); uid != want {
		return fmt.Sprintf("is owned by uid %d, not uid %d (the identity cellctl runs as)", uid, want)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "its directory cannot be resolved"
	}
	dst, err := os.Stat(dir)
	if err != nil || !dst.IsDir() {
		return "its directory cannot be inspected"
	}
	if duid, err := fileOwnerUID(dir); err != nil || duid != roleSecretOwnerUID() {
		return "its directory is not owned by the identity cellctl runs as"
	}
	if dst.Mode().Perm()&0o022 != 0 {
		return fmt.Sprintf("its directory (mode %04o) is writable beyond its owner", dst.Mode().Perm())
	}
	for _, p := range []string{path, dir} {
		has, err := roleSecretACLProbe(p)
		if err != nil {
			return "the access-control-list probe failed, so the file is not known to be owner-only"
		}
		if has {
			if p == path {
				return "carries an access-control list; remove it (owner-only mode bits are the only grant)"
			}
			return "its directory carries an access-control list; remove it"
		}
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return "lies inside a git checkout or worktree (" + d + "); keep it outside every repository"
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return ""
}

// roleSecretSiblingProblems refuses a binding while any OTHER cell under the same cells root
// could reach the file or share a writable mount with a holder. A host-kind cell runs its
// windows on the host as the file's owner, so it could open the file by its path; a container
// cell on an external launcher cannot be shown not to. A native container cell with another
// configuration may not mount the file, and may not share a volume with this configuration. An
// unreadable sibling is counted as a refusal, never skipped.
func (c *Cell) roleSecretSiblingProblems(cfg *cellcontainer.Config, binds []roleSecretBinding) []string {
	if c.Root == "" {
		return []string{"the cells root is unknown, so other cells cannot be checked"}
	}
	entries, err := os.ReadDir(c.Root)
	if err != nil {
		return []string{"the cells root cannot be read, so other cells cannot be checked"}
	}
	ownVolumes := map[string]bool{}
	for _, cell := range cfg.Cells {
		for _, r := range cell.Roles {
			ownVolumes[r.Volume] = true
		}
	}
	ownConfig := filepath.Clean(c.Env.Get("CELL_CONTAINER_CONFIG"))
	var probs []string
	for _, en := range entries {
		dir := filepath.Join(c.Root, en.Name())
		st, serr := os.Stat(dir)
		if serr != nil || !st.IsDir() || filepath.Clean(dir) == filepath.Clean(c.Dir) {
			continue
		}
		envPath := filepath.Join(dir, "cell.env")
		if _, ferr := os.Stat(envPath); ferr != nil {
			if errors.Is(ferr, os.ErrNotExist) {
				continue
			}
			probs = append(probs, fmt.Sprintf("cell %s: its cell.env cannot be read", en.Name()))
			continue
		}
		e := &Env{vals: map[string]string{}, set: map[string]bool{}, src: map[string]string{}}
		if _, _, err := overlayCellDefaults(e, c.DefaultsPath); err != nil {
			probs = append(probs, fmt.Sprintf("cell %s: the defaults file cannot be read", en.Name()))
			continue
		}
		if err := parseCellEnv(e, envPath); err != nil {
			probs = append(probs, fmt.Sprintf("cell %s: its cell.env cannot be read", en.Name()))
			continue
		}
		// A sibling's own bindings may not name this cell's file either, whatever kind it is or configuration it shares.
		for _, entry := range strings.Split(e.Get(envRoleSecrets), ",") {
			if _, p, ok := strings.Cut(strings.TrimSpace(entry), "="); ok {
				for _, b := range binds {
					if filepath.Clean(strings.TrimSpace(p)) == b.Path {
						probs = append(probs, fmt.Sprintf("cell %s binds the same secret file as %s/%s", en.Name(), b.Role, b.Service))
					}
				}
			}
		}
		kind := e.GetOr("CELL_KIND", "k8s")
		if kind != "container" {
			probs = append(probs, fmt.Sprintf("cell %s (kind %s) runs windows on the host as the secret file's owner, so they could open it by its path", en.Name(), kind))
			continue
		}
		other := e.Get("CELL_CONTAINER_CONFIG")
		if other == "" {
			probs = append(probs, fmt.Sprintf("cell %s runs on an external container launcher, which cannot be shown to keep the secret file out of its windows", en.Name()))
			continue
		}
		if filepath.Clean(other) == ownConfig {
			continue // the same configuration: Validate already keeps every volume distinct
		}
		ocfg, err := cellcontainer.Load(other)
		if err != nil {
			probs = append(probs, fmt.Sprintf("cell %s: its container configuration cannot be read", en.Name()))
			continue
		}
		for oname, ocell := range ocfg.Cells {
			sources := []string{ocell.Incoming}
			for okey, r := range ocell.Roles {
				if ownVolumes[r.Volume] {
					probs = append(probs, fmt.Sprintf("cell %s: container %s shares writable volume %s with this cell's configuration", en.Name(), cellcontainer.ContainerName(oname, okey), r.Volume))
				}
				sources = append(sources, r.Config, r.AppKey, r.ClaudeToken)
			}
			for _, b := range binds {
				for _, s := range sources {
					if s != "" && pathWithin(b.Path, s) {
						probs = append(probs, fmt.Sprintf("cell %s mounts %s, which holds the %s/%s secret file", en.Name(), s, b.Role, b.Service))
					}
				}
			}
		}
	}
	return probs
}

// pathWithin reports whether p is dir itself or lies under it.
func pathWithin(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// roleSecretBindMap is the verified bindings in the form the container plan takes: container
// cell name → desk role → binds.
func (c *Cell) roleSecretBindMap(binds []roleSecretBinding) map[string]map[string][]cellcontainer.SecretBind {
	if len(binds) == 0 {
		return nil
	}
	m := map[string]map[string][]cellcontainer.SecretBind{c.Name: {}}
	for _, b := range binds {
		m[c.Name][b.Role] = append(m[c.Name][b.Role], cellcontainer.SecretBind{Service: b.Service, Source: b.Path, EnvName: b.EnvName})
	}
	return m
}

// applyRoleSecrets verifies (when verify is set) and installs the cell's bindings on cfg. desk,
// check and container-run verify on every call, so a file removed, re-moded or re-owned since
// the last launch refuses the next one. status and down only re-plan what is running: they
// parse and apply, and never refuse to stop a container because its file has since gone.
func (c *Cell) applyRoleSecrets(cfg *cellcontainer.Config, verify bool) error {
	if !c.roleSecretsConfigured() {
		return nil
	}
	binds, err := c.roleSecretBindings()
	if err != nil {
		return err
	}
	if verify {
		if probs := c.roleSecretProblems(cfg, binds); len(probs) > 0 {
			return fmt.Errorf("refusing every role-secret binding: %s", strings.Join(probs, "; "))
		}
	}
	return cfg.SetRoleSecrets(c.roleSecretBindMap(binds))
}

// checkRoleSecrets prints the role-secret rows of `check`. A cell that sets neither key prints
// nothing, so every existing check transcript is unchanged. Rows name paths and roles, never a
// value.
func (c *Cell) checkRoleSecrets(k *checker) {
	if !c.roleSecretsConfigured() {
		return
	}
	binds, err := c.roleSecretBindings()
	if err != nil {
		k.chk(false, "role secrets: %v", err)
		return
	}
	if p := c.roleSecretSurfaceProblem(); p != "" {
		k.chk(false, "role secrets: refused — %s", p)
		return
	}
	cfg, err := cellcontainer.Load(c.Env.Get("CELL_CONTAINER_CONFIG"))
	if err != nil {
		k.chk(false, "role secrets: the container configuration cannot be read: %v", err)
		return
	}
	if probs := c.roleSecretProblems(cfg, binds); len(probs) > 0 {
		for _, p := range probs {
			k.chk(false, "role secrets: refused — %s", p)
		}
		return
	}
	if err := cfg.SetRoleSecrets(c.roleSecretBindMap(binds)); err != nil {
		k.chk(false, "role secrets: refused — %v", err)
		return
	}
	k.chk(true, "role secrets: container surface verified (no other cell under the cells root runs a host window or shares a writable mount)")
	for _, b := range binds {
		k.chk(true, "role secret %s/%s: %s → %s (%s; owner-only, no ACL, outside every checkout)", b.Role, b.Service, b.Path, cellcontainer.RoleSecretTargetPrefix+b.Service, b.EnvName)
	}
}

// roleSecretLaunchGate refuses a launch on any surface this cut does not bind role secrets on.
// The native container path re-verifies everything at plan time; this catches every other kind
// (and an external launcher) before anything is composed.
func (c *Cell) roleSecretLaunchGate() {
	if !c.roleSecretsConfigured() {
		return
	}
	if _, err := c.roleSecretBindings(); err != nil {
		die("desk: role secrets: %v", err)
	}
	if p := c.roleSecretSurfaceProblem(); p != "" {
		die("desk: role secrets: refusing to launch — %s", p)
	}
}

// printRoleSecretPlan is the dry run's view of a plan's bindings: role, service, host path,
// mount point and the variable that names it. Never a value: the file is not read.
func printRoleSecretPlan(p *cellcontainer.Plan) {
	for _, m := range p.Mounts {
		if !strings.HasPrefix(m.Target, cellcontainer.RoleSecretTargetPrefix) {
			continue
		}
		service := strings.TrimPrefix(m.Target, cellcontainer.RoleSecretTargetPrefix)
		fmt.Printf("[role-secret] role=%s project=%s service=%s path=%s mount=%s readonly=%t env=%s (path only; the value is never read)\n",
			cellcontainer.Roles[p.RoleKey], p.Env["CELL_REPO"], service, m.Source, m.Target, m.ReadOnly, deskkit.RoleSecretEnvName(service))
	}
}

// roleSecretReceipt is one line of the cell's receipt log. It records which file a launch bound,
// never its value and never a hash of it (a hash of a short password is a guessing oracle).
type roleSecretReceipt struct {
	LaunchedAt string `json:"launched_at"`
	Cell       string `json:"cell"`
	Role       string `json:"role"`
	Project    string `json:"project"`
	Service    string `json:"service"`
	Path       string `json:"path"`
	FileMtime  string `json:"file_mtime"`
}

// writeRoleSecretReceipts appends one receipt per bound service to <cell>/state/role-secrets.receipts
// (directory 0700, file 0600) before a container launch. A receipt that cannot be written
// refuses the launch: an unrecorded binding is not made.
func (c *Cell) writeRoleSecretReceipts(cfg *cellcontainer.Config, p *cellcontainer.Plan) error {
	binds := cfg.RoleSecrets[p.CellName][cellcontainer.Roles[p.RoleKey]]
	if len(binds) == 0 {
		return nil
	}
	dir := filepath.Join(c.Dir, "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, roleSecretReceipts), os.O_WRONLY|os.O_APPEND|os.O_CREATE|oNoFollow, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	now := roleSecretNow().UTC().Format(time.RFC3339)
	sorted := append([]cellcontainer.SecretBind(nil), binds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Service < sorted[j].Service })
	for _, b := range sorted {
		st, err := os.Lstat(b.Source)
		if err != nil {
			return fmt.Errorf("role secret %s: the file is gone", b.Service)
		}
		line, _ := json.Marshal(roleSecretReceipt{LaunchedAt: now, Cell: c.Name, Role: cellcontainer.Roles[p.RoleKey], Project: cfg.Cells[p.CellName].Repo,
			Service: b.Service, Path: b.Source, FileMtime: st.ModTime().UTC().Format(time.RFC3339Nano)})
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return f.Sync()
}

// unsetRoleSecretEnv removes every role-secret path variable from a host window environment: a
// host surface binds none, so one the launching shell carried never reaches a window.
func unsetRoleSecretEnv(env []string) []string { return deskkit.StripRoleSecretEnv(env) }
