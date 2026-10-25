package deskkit

// worldbrief.go — what a brief says its run needs, and the one place a
// verifier home's prerequisite record is admitted and bound on the shipped
// prepare path (example-stream/50).
//
// Both the dispatcher's prepare step and the loop's in-process plan read the
// same derivation here, so the declared siblings and the pinned instruments a
// record names cannot differ between the two.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

var worldSepRe = regexp.MustCompile(`^\s*\|?\s*-{2,}`)

// WorldSpecFromBrief reads what one run needs from its brief text: the
// declared sibling checkouts and the pinned instruments its runner-executed
// Verify rows (class check or check:ci, default check) call. Rows whose # cell
// is not an integer are outside the runner's contract and name nothing.
func WorldSpecFromBrief(root, brief string) WorldSpec {
	return WorldSpec{
		Root:        root,
		Siblings:    declaredSiblings(brief),
		Instruments: WorldInstruments(root, runnerCommands(brief)),
	}
}

// runnerCommands lists the Command cells the deterministic runner executes.
func runnerCommands(brief string) []string {
	lines := strings.Split(verifySection(brief), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := map[string]int{}
		for j, c := range verifycmd.SplitRow(line) {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		numCol, hasNum := idx["#"]
		cmdCol, hasCmd := idx["command"]
		if !hasNum || !hasCmd {
			continue
		}
		classCol, hasClass := idx["class"]
		var out []string
		for _, row := range lines[i+1:] {
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			if worldSepRe.MatchString(row) {
				continue
			}
			cells := verifycmd.SplitRow(row)
			cell := func(j int) string {
				if j >= 0 && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			if _, err := strconv.Atoi(cell(numCol)); err != nil {
				continue
			}
			class := "check"
			if hasClass && cell(classCol) != "" {
				class = cell(classCol)
			}
			if class != "check" && class != "check:ci" {
				continue
			}
			out = append(out, verifycmd.UnescapePipes(verifycmd.Lift(cell(cmdCol))))
		}
		return out
	}
	return nil
}

// verifySection is the body under the exact `## Verify` heading.
func verifySection(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Verify" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// verifierWorldCapability is the capability evidence a prepared run carries:
// the attestation round trip that just completed (the stamp was created,
// closed and read back as the verifier-attributed stamp) is itself the read of
// the dispatcher's write capability for this repo.
const verifierWorldCapability = "verifier-attestation"

// AdmitAndBindVerifierWorld is the shipped prepare path's admission of the
// run's prerequisite record. Called with the home of a run whose attestation
// has just completed, it reads the brief the run is bound to, admits its
// prerequisites against the verifier home (so the recorded source is the
// run's own attested source, not an ambient checkout), and persists the record
// on the run. A prerequisite that cannot be read or does not conform returns
// the admission's own error and binds nothing, so the dispatch emits no prompt
// for a world it could not record.
func AdmitAndBindVerifierWorld(home string) (VerifierReceipt, error) {
	r, err := verifierLoad(home)
	if err != nil {
		return r.VerifierReceipt, err
	}
	briefRoot := r.Home
	if r.Binding.Version == 2 {
		briefRoot = r.TrackingHome
	}
	body, err := os.ReadFile(filepath.Join(briefRoot, filepath.FromSlash(r.Binding.Brief)))
	raw := string(body)
	if err != nil {
		return r.VerifierReceipt, Unverifiable("environment could-not-check: cannot read the brief's prerequisites: "+oneLine(err.Error()), nil)
	}
	spec := WorldSpecFromBrief(r.Home, raw)
	spec.Repo, spec.Source = r.Binding.Repo, r.Binding.Source
	spec.Capability = clean(verifierWorldCapability, "stamp created, closed and read back", "")
	adm := AdmitWorld(spec, worldRecheckProbes)
	if !adm.Ready() {
		return r.VerifierReceipt, adm.Err()
	}
	return BindVerifierWorld(home, adm.World)
}
