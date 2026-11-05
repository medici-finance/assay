package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Layer 3 of the local hotfix overlay: the boot marker. deskboot is never a hotfix
// (cellctl hotfix install refuses it as a target), so this step is always the pinned
// release's code. It classifies every cell shim by the path the shim EXECS, never by what
// the binary it names prints, and re-checks every hotfix itself — root ownership, sha256,
// pin, expiry, the ruled ceiling, the ruled source rule and the forge — so a hotfix that
// slipped past shim generation (layer 1) or its own self-check (layer 2) is still refused
// at boot. It is a read: it runs no hotfix binary.

// The seams. Production keeps the real values; only _test.go files reassign them. A binary
// built with the test-only hotfixseam tag reads deskkit's environment seams through
// ActiveHotfixOwner / ActiveHotfixForge; the release build never passes that tag.
var (
	binaryPinOwner = deskkit.ActiveHotfixOwner
	binaryPinForge = deskkit.ActiveHotfixForge
	binaryPinClock = time.Now
	binaryPinOS    = runtime.GOOS
)

// shimExecPath finds the absolute binary paths a generated cell shim execs: the ordinary
// shim's `"<bin>" "$@"` lines, and a hotfix shim's two assignments.
var (
	shimExecLine   = regexp.MustCompile(`(?m)"(/[^"\n]+)" "\$@"`)
	shimHotfixBin  = regexp.MustCompile(`(?m)^hotfix_bin="([^"\n]+)"`)
	shimPinnedBin  = regexp.MustCompile(`(?m)^pinned_bin="([^"\n]+)"`)
	shimSizeCapMax = int64(1 << 20)
)

// cellShimDirs are the cell shim directories this session runs behind: the ASSAY_CELL
// marker names the cell, and its shim directory is on PATH (and under $CELLS_ROOT/<cell>).
func cellShimDirs() []string {
	cell := strings.TrimSpace(os.Getenv(deskkit.CellMarkerEnv))
	if cell == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		d = filepath.Clean(d)
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	for _, e := range filepath.SplitList(os.Getenv("PATH")) {
		if e != "" && filepath.Base(e) == "shim" && filepath.Base(filepath.Dir(e)) == cell {
			add(e)
		}
	}
	if root := strings.TrimSpace(os.Getenv("CELLS_ROOT")); root != "" {
		add(filepath.Join(root, cell, "shim"))
	}
	return out
}

// shimTargets returns every binary path the shim at p execs (a hotfix shim's overlay and
// pinned paths both), resolved for a symlink shim.
func shimTargets(p string) (paths []string, err error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			return nil, err
		}
		return []string{r}, nil
	}
	if !fi.Mode().IsRegular() || fi.Size() > shimSizeCapMax {
		return nil, fmt.Errorf("%s is not a regular shim file", p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if m := shimHotfixBin.FindSubmatch(b); m != nil {
		paths = append(paths, string(m[1]))
		if pm := shimPinnedBin.FindSubmatch(b); pm != nil {
			paths = append(paths, string(pm[1]))
		}
		return paths, nil
	}
	for _, m := range shimExecLine.FindAllSubmatch(b, -1) {
		paths = append(paths, string(m[1]))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s names no binary it execs", p)
	}
	return paths, nil
}

// binaryPinCheck is boot step `binary-pin`. It returns the lines to print (one per hotfix, or
// the single `all pinned` line) and whether any hotfix is running.
func binaryPinCheck(o bootOpts) (lines []string, hotfix bool, err error) {
	if binaryPinOS == "windows" {
		return []string{"all pinned (the hotfix overlay is not supported on Windows)"}, false, nil
	}
	pinDir := deskkit.DeskToolsBinDir()
	overlay := deskkit.HotfixOverlayDir()
	shimDirs := cellShimDirs()
	if len(shimDirs) == 0 {
		return []string{"all pinned (no cell shim directory: " + deskkit.CellMarkerEnv + " is unset, nothing shims a desk tool)"}, false, nil
	}
	pinTag, _, pinFound := deskkit.DeskToolsReleasePin(deskkit.NearestPinRoot(o.root), hostPlatform)
	pin := pinTag
	if i := strings.LastIndex(pin, "/"); i >= 0 {
		pin = pin[i+1:]
	}

	type hot struct{ tool, shim string }
	var hots []hot
	for _, dir := range shimDirs {
		es, rerr := os.ReadDir(dir)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return nil, false, binaryPinShimRefusal("", pinDir, "shim directory %s unreadable: %v", dir, rerr)
		}
		for _, en := range es {
			if en.IsDir() || strings.HasPrefix(en.Name(), ".") {
				continue
			}
			tool, shim := en.Name(), filepath.Join(dir, en.Name())
			paths, terr := shimTargets(shim)
			if terr != nil {
				return nil, false, binaryPinShimRefusal(tool, pinDir, "shim target outside the pinned and hotfix dirs: %v", terr)
			}
			isHot := false
			for _, p := range paths {
				switch {
				case deskkit.WithinDir(p, overlay):
					isHot = true
					if filepath.Clean(p) != filepath.Clean(deskkit.HotfixBinaryPath(overlay, tool)) {
						return nil, false, binaryPinRefusal(tool, "%s: shim %s targets %s, not the overlay's own %s",
							deskkit.HotfixReasonTampered, shim, p, deskkit.HotfixBinaryPath(overlay, tool))
					}
				case deskkit.WithinDir(p, pinDir):
				default:
					return nil, false, binaryPinShimRefusal(tool, pinDir, "shim target outside the pinned and hotfix dirs: %s targets %s (pinned dir %s, hotfix dir %s)",
						shim, p, pinDir, overlay)
				}
			}
			if isHot {
				hots = append(hots, hot{tool: tool, shim: shim})
			}
		}
	}
	if own := deskkit.ReleaseTag; own != "" && ownTagVerdict(own, pin) == "refuse" {
		return nil, false, deskkit.Unverifiable(fmt.Sprintf("step %s: %s: this deskboot is stamped %s, a hotfix tag; deskboot is never a hotfix target (tool deskboot). "+
			"Install the pinned desk-tools release's deskboot and re-run it.", stepBinaryPin, deskkit.HotfixReasonTampered, own), nil)
	}
	binaryPinWarnLag(pin)
	if len(hots) == 0 {
		if !pinFound {
			return []string{"all pinned (no desk-tools pin line to compare against; no hotfix in use)"}, false, nil
		}
		return []string{"all pinned (" + pin + ")"}, false, nil
	}
	sort.Slice(hots, func(i, j int) bool { return hots[i].tool < hots[j].tool })
	if !pinFound {
		return nil, true, binaryPinRefusal(hots[0].tool, "%s: no desk-tools pin line at or above %s to compare the hotfix's base against", deskkit.HotfixReasonNoPin, o.root)
	}
	owner, now, forge := binaryPinOwner(), binaryPinClock(), binaryPinForge()
	for _, h := range hots {
		e, verr := deskkit.ValidateHotfix(overlay, h.tool, pin, now, owner)
		if verr == nil && deskkit.HotfixNeverTarget(h.tool) {
			verr = &deskkit.HotfixError{Reason: deskkit.HotfixReasonTampered, Detail: h.tool + " is never a hotfix target", Code: 1}
		}
		if verr != nil {
			if errorsIsNoHotfix(verr) {
				verr = &deskkit.HotfixError{Reason: deskkit.HotfixReasonTampered, Detail: "shim " + h.shim + " execs the overlay but the overlay has no entry for " + h.tool, Code: 1}
			}
			return nil, true, binaryPinRefusal(h.tool, "%v", verr)
		}
		if ferr := forge.CheckSourceOnMain(e.SourceSHA); ferr != nil {
			return nil, true, binaryPinRefusal(h.tool, "%v", ferr)
		}
		lines = append(lines, fmt.Sprintf("running hotfix %s %s over pin %s (source %s, expires %s)",
			h.tool, e.Tag(), e.BasePin, e.SourceSHA[:12], e.Expires))
	}
	return lines, true, nil
}

// binaryPinStep runs boot step 10: the check, its perf record and its output. A running hotfix
// is never quieted (the marker is the point); otherwise the `all pinned` line follows --quiet.
func binaryPinStep(o bootOpts) error {
	began := time.Now()
	lines, hotfix, err := binaryPinCheck(o)
	if err := stepPerf(stepBinaryPin, began, err); err != nil {
		return err
	}
	if hotfix {
		for _, l := range lines {
			fmt.Printf("deskboot: %s: %s\n", stepBinaryPin, l)
		}
	} else {
		o.say("%s OK: %s", stepBinaryPin, lines[0])
	}
	return nil
}

func errorsIsNoHotfix(err error) bool { return err == deskkit.ErrNoHotfix }

// binaryPinRefusal is the exit-6 refusal naming the step and the tool.
func binaryPinRefusal(tool, format string, a ...any) error {
	who := ""
	if tool != "" {
		who = " (tool " + tool + ")"
	}
	return deskkit.Unverifiable(fmt.Sprintf("step %s: %s%s. A hotfix that cannot be proven is not run: "+
		"retire it with `sudo cellctl hotfix clear` and relaunch the desk so the shims point at the pinned release, then re-run deskboot.",
		stepBinaryPin, fmt.Sprintf(format, a...), who), nil)
}

// binaryPinShimRefusal is the exit-6 refusal for a shim that is not one cellctl generated for
// this window: a directory that cannot be read, or a target in neither the pinned nor the hotfix
// directory. No hotfix is in use for that tool, so the remedy never points at `hotfix clear`: the
// usual cause is a window whose DESK_TOOLS_BIN differs from the one the shims were generated
// against, and relaunching the desk regenerates them against this window's value.
func binaryPinShimRefusal(tool, pinDir, format string, a ...any) error {
	who := ""
	if tool != "" {
		who = " (tool " + tool + ")"
	}
	return deskkit.Unverifiable(fmt.Sprintf("step %s: %s%s. A shim aimed outside the pinned directory is not run: "+
		"this window's %s resolves to %s; relaunch the desk with `cellctl desk` so the shims regenerate against it "+
		"(set %s in one place, the cell config or the launching shell, not both), then re-run deskboot.",
		stepBinaryPin, fmt.Sprintf(format, a...), who, deskkit.DeskToolsBinEnv, pinDir, deskkit.DeskToolsBinEnv), nil)
}

// ownTagVerdict classifies this deskboot's own release tag against the worktree's pin:
// "refuse" for a hotfix-stamped deskboot (deskboot is never a hotfix, so one is a build or
// install that must not run), "pinned" for the same release, "warn" for anything else (an
// install lagging a re-pin, or an unstamped build).
func ownTagVerdict(own, pin string) string {
	switch {
	case deskkit.IsHotfix(own):
		return "refuse"
	case deskkit.SameReleaseTag(own, pin):
		return "pinned"
	default:
		return "warn"
	}
}

// binaryPinWarnLag is the WARN-only line: this deskboot is the pinned install, and when its
// own release tag is not the worktree's pin the install lags a re-pin. It never refuses (the
// hotfix-stamped case is refused by binaryPinCheck before this runs).
func binaryPinWarnLag(pin string) {
	own := deskkit.ReleaseTag
	if pin == "" || own == "" || ownTagVerdict(own, pin) != "warn" {
		return
	}
	fmt.Fprintf(os.Stderr, "deskboot: WARN %s: the installed deskboot is %s but the pin is %s — the installed desk tools lag the pin; install the pinned release\n",
		stepBinaryPin, own, pin)
}
