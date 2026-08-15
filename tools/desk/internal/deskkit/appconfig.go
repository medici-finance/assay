package deskkit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appConfigFile is the config FILE NAME that carries the per-deployment GitHub App IDs.
// The App IDs are org-specific and MUST NOT be baked into source (a hardcoded ID couples
// the tool to one org and breaks when the toolkit ships to other adopters), so they live
// here and are resolved at runtime — exactly like the App PEM path already does.
//
// The DIRECTORY is no longer hardcoded either: it is resolved across the
// App-credential search path (confighome.go), because a fixed directory that the
// provisioning walkthrough does not use is exactly #794.
const appConfigFile = "apps.env"

// AppConfigPath is the apps.env this machine will actually read, for diagnostics.
func AppConfigPath() string {
	p, _, _ := FindConfigFile(appConfigFile)
	return p
}

// roleEnvName converts a desk role to its App-ID env-var name: "reviewer" → "REVIEWER_APP_ID",
// "issue-loop" → "ISSUE_LOOP_APP_ID".
func roleEnvName(role string) string {
	return strings.ToUpper(strings.ReplaceAll(role, "-", "_")) + "_APP_ID"
}

// expandHome expands a leading "~/" to the user's home directory. No-op if not ~/.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// AppID resolves the GitHub App ID for a desk role WITHOUT requiring the caller to pre-export
// anything: a fresh tool invocation works with no shell sourcing. Resolution order:
//
//	(a) env <ROLE>_APP_ID if non-empty (e.g. REVIEWER_APP_ID, ISSUE_LOOP_APP_ID);
//	(b) else the matching <ROLE>_APP_ID line in the first apps.env on the
//	    App-credential search path (leading "export " tolerated, first "=" splits
//	    key/value, quotes/space trimmed, "#" comment lines skipped);
//	(c) else an error naming both fixes AND every directory searched.
//
// The App ID VALUE never appears in source — only this resolution logic does.
func AppID(role string) (string, error) {
	envName := roleEnvName(role)
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return v, nil
	}

	path, searched, _ := FindConfigFile(appConfigFile)
	if v, err := appIDFromFile(path, envName); err == nil && v != "" {
		return v, nil
	}

	return "", fmt.Errorf("no App ID for role %q: set %s, or add it to %s in one of: %s",
		role, envName, appConfigFile, strings.Join(searched, ", "))
}

// roleInstallEnvName converts a desk role to its SINGLE-installation install-id env-var
// name: "reviewer" → "REVIEWER_INSTALL_ID".
func roleInstallEnvName(role string) string {
	return strings.ToUpper(strings.ReplaceAll(role, "-", "_")) + "_INSTALL_ID"
}

// ownerInstallEnvName is the PER-OWNER install-id key for a role and repo owner:
// role "reviewer", owner "medici-finance" → "REVIEWER_INSTALL_ID_MEDICI_FINANCE".
func ownerInstallEnvName(role, owner string) string {
	return roleInstallEnvName(role) + "_" + strings.ToUpper(strings.ReplaceAll(owner, "-", "_"))
}

// resolveAppConfigValue looks a key up env-first, then in the apps.env on the
// App-credential search path — the same resolution AppID uses. Returns "" when
// neither supplies a non-empty value.
func resolveAppConfigValue(key string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	path, _, _ := FindConfigFile(appConfigFile)
	if v, err := appIDFromFile(path, key); err == nil {
		return strings.TrimSpace(v)
	}
	return ""
}

// InstallID resolves the GitHub App INSTALLATION id for a desk role acting on owner's
// repos. Like the App ID, the id VALUE must never be baked into source: a hardcoded
// installation id is a machine-identity reference that couples the tool to one org's
// installations (a public-repo security review). It is resolved at runtime, env-first then apps.env:
//
//	(a) <ROLE>_INSTALL_ID — a SINGLE-installation override that wins for every owner
//	    (the shape a one-account deployment and the tests use);
//	(b) else <ROLE>_INSTALL_ID_<OWNER> — the per-owner installation, for an App installed
//	    on more than one account (owner upper-cased, '-'→'_': owner "medici-finance" →
//	    REVIEWER_INSTALL_ID_MEDICI_FINANCE);
//	(c) else an error naming both fixes — FAIL CLOSED: a token cannot target an
//	    installation this process cannot name, so mint refuses rather than guessing.
func InstallID(role, owner string) (string, error) {
	single := roleInstallEnvName(role)
	if v := resolveAppConfigValue(single); v != "" {
		return v, nil
	}
	perOwner := ownerInstallEnvName(role, owner)
	if v := resolveAppConfigValue(perOwner); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("no installation id for role %q owner %q: set %s (single installation) "+
		"or %s (per-owner), in the environment or %s", role, owner, single, perOwner, AppConfigPath())
}

// appIDFromFile parses a dotenv-style file and returns the value for key. A missing file,
// unreadable file, or absent key yields an empty value (the caller treats that as "not found").
func appIDFromFile(path, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		if strings.TrimSpace(line[:eq]) != key {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		return strings.TrimSpace(val), nil
	}
	return "", sc.Err()
}
