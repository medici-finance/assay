package deskkit

// confighome.go — where the App-CREDENTIAL plane lives on this machine.
//
// #794: the minter resolved role pems, the token cache and apps.env from
// `~/.config/assay/`, but the provisioning walkthrough
// (docs/github-apps-setup.md) puts them somewhere else. A fresh shell therefore
// could not mint ANY App token once the ~1h cache lapsed; everything worked
// until it silently didn't, and the failure surfaced as an "Authentication
// failed" on a push, a long way from the cause.
//
// The fix ships the MECHANISM and lets the deployment supply the POLICY, exactly
// as the other generic knobs do (ASSAY_RELEASE_REPO, ASSAY_WRITEGUARD_CALLOUT):
// a search path whose head is configurable and whose tail is the shipped
// default. A product-specific directory is a deployment VALUE and does not
// belong compiled into a generic tool — the same rule that keeps product config
// out of the ASSAY_ namespace keeps a product path out of this resolver.
//
// Scope note — this knob governs the APP-CREDENTIAL plane ONLY: role pems, the
// token cache, and apps.env. It deliberately does NOT move `roster.env`. The
// roster is the trust surface and it fails CLOSED: pointing the whole config
// home at a directory with no roster.env would take the trust roster down
// fleet-wide, which is precisely the failure mode #819 already produced once.

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvConfigHome names the environment variable that puts a directory at the HEAD
// of the App-credential search path:
//
//	ASSAY_CONFIG_HOME=~/.config/<your-deployment>
//
// Unset is a COMPLETE configuration, not a degraded one: the shipped default
// (~/.config/assay) is searched either way. Setting it never LOSES a file that
// resolves today — the knob prepends, it does not replace.
const EnvConfigHome = "ASSAY_CONFIG_HOME"

// DefaultConfigHome is the shipped App-credential directory.
const DefaultConfigHome = "~/.config/assay"

// ConfigHomeDirs returns the App-credential search path, most-preferred first:
// the ASSAY_CONFIG_HOME directory when set, then the shipped default.
func ConfigHomeDirs() []string {
	var dirs []string
	if v := strings.TrimSpace(os.Getenv(EnvConfigHome)); v != "" {
		dirs = append(dirs, expandHome(v))
	}
	def := expandHome(DefaultConfigHome)
	for _, d := range dirs {
		if d == def {
			return dirs
		}
	}
	return append(dirs, def)
}

// FindConfigFile returns the first existing <dir>/<name> across the search path,
// along with every directory searched. found=false does NOT mean "use a default"
// — every caller here fails closed on it; the searched list exists so the
// refusal can name every place it looked instead of one.
func FindConfigFile(name string) (path string, searched []string, found bool) {
	dirs := ConfigHomeDirs()
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, dirs, true
		}
	}
	return filepath.Join(dirs[0], name), dirs, false
}

// ConfigHomeWritePath is where a NEW App-credential file (the token cache) is
// written: the head of the search path, so a deployment that sets the knob keeps
// its credentials in one directory rather than reading from one and writing to
// another — the split #794 is actually about.
func ConfigHomeWritePath(name string) string {
	return filepath.Join(ConfigHomeDirs()[0], name)
}
