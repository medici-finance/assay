package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Refresh the requested role before the first fetch. Git fetch uses the first
// origin URL; additional URLs do not change the selected fetch destination.
func fetchRoleBase(dir, role, repo, username string) error {
	origins, err := originURLs(dir, false)
	if err != nil {
		return err
	}
	if len(origins) == 0 {
		return deskkit.Unverifiable("origin has no fetch URL", nil)
	}
	origin := origins[0]
	host, networked, err := transportHost(origin)
	if err != nil {
		return deskkit.Refused("refused: cannot resolve role-init fetch transport: " + err.Error())
	}
	if !networked {
		_, err = runGit(dir, "fetch", "--no-tags", "origin", "main")
		return err
	}
	if deskkit.IsSSHTransport(origin) {
		// Same alias resolution and App endpoint as the worktree provisioning step.
		origin = "https://" + host + ":443/" + repo + ".git"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return deskkit.Refused("refused: role-init requires HTTPS or an SSH origin that resolves to an App HTTPS endpoint; set origin with git remote set-url origin https://<host>/<owner>/<repo>.git and re-run")
	}
	path, err := roleCredentialPath(role, repo, origin)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "'\r\n") {
		return deskkit.Refused("refused: role-init requires an absolute credential file path without quotes or newlines")
	}
	return fetchRoleHTTPS(dir, role, origin, username, path)
}

func fetchRoleHTTPS(dir, role, origin, username, path string) error {
	u, _ := url.Parse(origin) // validated by fetchRoleBase
	// Snapshot effective settings before moving HOME. Retain TLS/proxy settings,
	// including global and conditional includes, without consulting a user's netrc.
	config, err := runGit(dir, "config", "--null", "--list", "--includes")
	if err != nil {
		return err
	}
	// Ask Git to expand its path-valued TLS options in the ORIGINAL context.
	// --path handles ~/ and ~user consistently with Git on every platform;
	// relative paths retain their meaning because the command directory is unchanged.
	// These are http.c's git_config_pathname options (cookieFile is suppressed).
	tls := deskkit.Run(deskkit.ToolCall{Name: "git", Args: []string{"config", "--null", "--path", "--includes", "--get-regexp",
		`^http\.(.*\.)?(sslcert|sslkey|sslcapath|sslcainfo|pinnedpubkey)$`}, Dir: dir, Start: execCommand})
	if tls.Failed() && tls.ExitCode != 1 { // exit 1 means there are no matching settings
		return tls.Fail(deskkit.ExitUnverifiable, "resolve role fetch TLS paths")
	}
	tlsPaths := map[string][]string{}
	for _, record := range strings.Split(tls.Stdout, "\x00") {
		if key, value, ok := strings.Cut(record, "\n"); ok {
			tlsPaths[key] = append(tlsPaths[key], value)
		}
	}
	home, err := os.MkdirTemp("", "deskwt-fetch-*")
	if err != nil {
		return deskkit.Unverifiable("cannot isolate role fetch home", err)
	}
	defer os.RemoveAll(home)
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if key == "HOME" || key == "USERPROFILE" || key == "XDG_CONFIG_HOME" || key == "NETRC" || strings.HasPrefix(key, "GIT_CONFIG") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+home, "NETRC="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
	args := []string{"-c", "credential.helper=", "-c", "credential.https://" + u.Host + ".helper=" + deskkit.AppTokenHelper(username, path),
		"-c", "credential.username=" + username, "-c", "http.extraHeader=", "-c", "http.cookieFile=", "-c", "http.saveCookies=false", "-c", "http.followRedirects=false"}
	count := 0
	for _, record := range strings.Split(config, "\x00") {
		if record == "" {
			continue
		}
		key, value, hasValue := strings.Cut(record, "\n")
		lower := strings.ToLower(key)
		if values := tlsPaths[key]; len(values) > 0 {
			value = values[0]
			tlsPaths[key] = values[1:]
		}
		if lower == "include.path" || (strings.HasPrefix(lower, "includeif.") && strings.HasSuffix(lower, ".path")) {
			continue
		}
		// A valueless Git boolean is true, not the false represented by an empty value.
		if !hasValue {
			value = "true"
		}
		env = append(env, "GIT_CONFIG_KEY_"+strconv.Itoa(count)+"="+key, "GIT_CONFIG_VALUE_"+strconv.Itoa(count)+"="+value)
		count++
		if strings.HasPrefix(lower, "http.") {
			suffix := lower[strings.LastIndex(lower, ".")+1:]
			switch suffix {
			case "extraheader", "cookiefile":
				args = append(args, "-c", key+"=")
			case "savecookies", "followredirects":
				args = append(args, "-c", key+"=false")
			}
		}
	}
	env = append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(count))
	// Prove the explicit endpoint is not rewritten to a different transport/host.
	resolved := deskkit.Run(deskkit.ToolCall{Name: "git", Args: append(append([]string{}, args...), "ls-remote", "--get-url", origin), Dir: dir, Env: env, Start: execCommand})
	if resolved.Failed() {
		return resolved.Fail(deskkit.ExitUnverifiable, "resolve role fetch endpoint")
	}
	if strings.TrimSpace(resolved.Stdout) != origin {
		return deskkit.Refused("refused: role fetch HTTPS endpoint is rewritten; correct url.*.insteadOf configuration and re-run")
	}
	args = append(args, "fetch", "--no-tags", "--no-recurse-submodules", origin, "+refs/heads/main:refs/remotes/origin/main")
	r := deskkit.Run(deskkit.ToolCall{Name: "git", Args: args, Dir: dir, Env: env, Start: execCommand})
	if r.Failed() {
		return r.Fail(deskkit.ExitUnverifiable, "role-init fetch as %s", role)
	}
	return nil
}
