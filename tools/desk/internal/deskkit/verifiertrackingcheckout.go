package deskkit

import (
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// Refuse external checkout programs before materializing the tracking tree.
// Rejecting their output after checkout would be too late: a configured filter
// or hook can already have executed. No global/local hook or filter is disabled.
func checkoutVerifierTracking(root, revision string) error {
	// The own-work-tree check compares git's resolved toplevel to root by
	// string, so a caller's linked or differently spelled path is canonicalised
	// here, at the one choke point, rather than at every caller.
	_, root, err := verifierHome(root)
	if err != nil {
		return err
	}
	for _, check := range []func(string) error{verifierOwnWorkTree, verifierNoReplacements, verifierNoLocalAttributes} {
		if err := check(root); err != nil {
			return err
		}
	}
	hook, err := verifierGit(root, "rev-parse", "--path-format=absolute", "--git-path", "hooks/post-checkout")
	if err != nil {
		return err
	}
	if info, err := os.Stat(hook); err == nil {
		if runtime.GOOS == "windows" || info.Mode().Perm()&0111 != 0 {
			return Refused("tracking checkout has an executable post-checkout hook")
		}
	} else if !os.IsNotExist(err) {
		return Unverifiable("cannot inspect tracking checkout hook", err)
	}
	tree, err := verifierTree(root, revision)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(tree))
	for path, entry := range tree {
		if strings.HasPrefix(entry, "160000 ") {
			return Refused("tracking source contains an unattested submodule")
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	conversion, err := readVerifierCheckout(root)
	if err != nil {
		return err
	}
	conversion.source = revision
	args := conversion.args("check-attr", "-z", "--stdin", "filter")
	cmd := exec.Command("git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
	cmd.Env = verifierEnv()
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	raw, err := cmd.Output()
	if err != nil {
		return Unverifiable("cannot inspect immutable tracking attributes before checkout (Git 2.41 or later required)", err)
	}
	fields := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if len(fields) != 3*len(paths) {
		return Unverifiable("incomplete immutable tracking attribute read", nil)
	}
	for i, path := range paths {
		if fields[3*i] != path || fields[3*i+1] != "filter" {
			return Unverifiable("unexpected immutable tracking attribute read", nil)
		}
		if value := fields[3*i+2]; value != "unspecified" && value != "unset" {
			return Refused("tracking source requires a Git filter before admission")
		}
	}
	_, err = verifierGit(root, conversion.args("checkout", "--detach", revision)...)
	return err
}
