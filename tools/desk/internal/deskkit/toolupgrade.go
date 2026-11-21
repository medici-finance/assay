package deskkit

// toolupgrade.go — the one desk-tools change a landing recheck may accept (#7353).
//
// A run's prerequisite record binds the desk-tools release it was admitted under, and
// Recheck refuses any later difference. That left a record landed by a run admitted under
// one release impossible to retire with a fix that shipped only in the next: the old release
// lacks the mode, the new one is a "tool" change. The ruling recorded on #7353 (option A)
// narrows the tool comparison at LANDING only:
//
//   - the running binary is a stamped release (`vMAJOR.MINOR.PATCH`, an optional `+build`
//     suffix; never `dev`, never a pre-release),
//   - it is strictly newer by semver than the release the run was admitted under,
//   - it equals the desk-tools pin on the CURRENT main of the tracking repo, read through the
//     landing's own forge — main unreadable is could-not-check and refuses, and
//   - every accepted upgrade is recorded in the landing record (ToolUpgrade.Trailer).
//
// Nothing else is relaxed: every other input still refuses on any change, a row-execution
// admission still binds the admitted release exactly, and a downgrade or a sideways move
// refuses.

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolUpgrade is a desk-tools change a landing recheck accepted.
type ToolUpgrade struct {
	// From is the release the run was admitted under; To is the release now running.
	From, To string
	// Pin names where the matching pin was read: "<owner/repo>@main:<file> <artifact>".
	Pin string
}

// Trailer is the line the landing record carries for an accepted upgrade.
func (u ToolUpgrade) Trailer() string {
	return "Tool-Upgrade: " + u.From + " -> " + u.To + " (pin " + u.Pin + ")"
}

// MainToolPin reads the desk-tools pin on the current main of the tracking repo. It returns
// the pinned tag and where it was read. An error wrapping ExitUnverifiable means main could
// not be read; a refusal means main was read and pins no desk-tools release.
type MainToolPin func() (tag, source string, err error)

// stampedRelease is the only release-tag shape an upgrade may name.
var stampedRelease = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\+[0-9A-Za-z.-]+)?$`)

// releaseCore parses a stamped release tag (a `desk-tools/` prefix is dropped, as admitTool
// drops it) into its three numbers. ok is false for anything else.
func releaseCore(tag string) (core [3]int, ok bool) {
	m := stampedRelease.FindStringSubmatch(strings.TrimPrefix(tag, DeskToolsArtifact+"/"))
	if m == nil {
		return core, false
	}
	for i := range core {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return core, false
		}
		core[i] = n
	}
	return core, true
}

// releaseNewer reports whether a is strictly newer than b by semver precedence. Build
// metadata carries no precedence, so a `+hotfix` of the same core is not newer.
func releaseNewer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// acceptToolUpgrade decides one tool change. It returns the upgrade when every condition
// holds, a refusal naming the first that fails, or could-not-check when main is unreadable.
func acceptToolUpgrade(admitted, running string, mainPin MainToolPin) (ToolUpgrade, error) {
	from, okFrom := releaseCore(admitted)
	to, okTo := releaseCore(running)
	switch {
	case !okTo:
		return ToolUpgrade{}, Refused("desk-tools " + running + " is not a stamped release")
	case !okFrom:
		return ToolUpgrade{}, Refused("the admitted desk-tools " + admitted + " is not a stamped release")
	case !releaseNewer(to, from):
		return ToolUpgrade{}, Refused("desk-tools " + running + " is not newer than the admitted " + admitted)
	}
	tag, source, err := mainPin()
	if err != nil {
		return ToolUpgrade{}, err
	}
	norm := func(s string) string { return strings.TrimPrefix(s, DeskToolsArtifact+"/") }
	if norm(tag) != norm(running) {
		return ToolUpgrade{}, Refused("desk-tools " + running + " is not the " + tag + " pinned at " + source)
	}
	return ToolUpgrade{From: admitted, To: running, Pin: source}, nil
}

// PairedVersionsPath is the source tree's release manifest. A repo whose main carries no
// `.assay-versions` (the assay source tree itself) pins its desk tools there.
const PairedVersionsPath = "plugins/assay/paired-versions.yaml"

// mainBranch is the branch a tool upgrade's pin is read from.
const mainBranch = "main"

// ForgeMainToolPin reads the desk-tools pin from main of repo (slug names it in the source
// it reports) through f: the `.assay-versions` desk-tools line DeskToolsReleasePin selects
// for host, else, only when main carries no `.assay-versions`, the `desk-tools.tag` of
// PairedVersionsPath. A read that fails, or serves anything but a regular file, is
// could-not-check; a readable main with no desk-tools pin refuses.
func ForgeMainToolPin(f Forge, repo ForgeRepo, slug, host string) MainToolPin {
	return func() (string, string, error) {
		read := func(file string) ([]byte, bool, error) {
			if f == nil {
				return nil, false, Unverifiable("cannot read "+file+" on main of "+slug+": no forge", nil)
			}
			fc, err := f.ReadFile(repo, ReadFileInput{File: file, Ref: mainBranch})
			switch {
			case IsForgeNotFound(err):
				return nil, false, nil
			case err != nil:
				return nil, false, Unverifiable("cannot read "+file+" on main of "+slug, err)
			case fc == nil || !fc.Exists:
				return nil, false, nil
			case fc.Kind != FileKindRegular:
				return nil, false, Unverifiable(file+" on main of "+slug+" is not a regular file ("+fc.Kind+")", nil)
			}
			return fc.Content, true, nil
		}
		where := func(file string) string { return slug + "@" + mainBranch + ":" + file }
		raw, ok, err := read(AssayVersionsFile)
		if err != nil {
			return "", "", err
		}
		if ok {
			for _, name := range append([]string{deskToolsName}, DeskToolsPlatformArtifacts(host)...) {
				tag, _, found, lerr := lookupPin(raw, where(AssayVersionsFile), name)
				if lerr != nil {
					return "", "", lerr
				}
				if found && tag != "" {
					return tag, where(AssayVersionsFile) + " " + name, nil
				}
			}
			return "", "", Refused("no desk-tools pin on main of " + slug + " (" + AssayVersionsFile + " has no desk-tools line)")
		}
		raw, ok, err = read(PairedVersionsPath)
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", Refused("no desk-tools pin on main of " + slug)
		}
		var pv struct {
			DeskTools struct {
				Tag string `yaml:"tag"`
			} `yaml:"desk-tools"`
		}
		if yerr := yaml.Unmarshal(raw, &pv); yerr != nil {
			return "", "", Unverifiable("cannot parse "+where(PairedVersionsPath), yerr)
		}
		if strings.TrimSpace(pv.DeskTools.Tag) == "" {
			return "", "", Refused("no desk-tools pin on main of " + slug + " (" + PairedVersionsPath + " names no desk-tools tag)")
		}
		return strings.TrimSpace(pv.DeskTools.Tag), where(PairedVersionsPath) + " desk-tools.tag", nil
	}
}
