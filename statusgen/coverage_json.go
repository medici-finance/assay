package main

// coverage_json.go — the versioned `witness` field on `statusgen --coverage
// --json` (statusgen/36, Seams). Per Verify-row claim with a witness it reports
// what statusgen's existing reads say about the citing Evidence row, so no
// cross-module consumer parses a Runner cell or runs `git blame` itself:
//
//	schema          1 or 2 (0: a bracketed marker no reader knows)
//	judgment        the claim's result
//	gap             the v2 gap name the reason starts with, "" otherwise
//	cr, record      a v2 cell's `cr:` and `rec:` values
//	runner          the cell's identity token (the text before `@`)
//	independent     the existing offline identity reads for that row: the
//	                independent-row read and the verifier floor
//	evidence_actor  the Evidence-actor classification of the commit that
//	                authored the citing Evidence line: verifier, other or unknown
//	landing_cr      v1 only: the change request the first-parent commit that
//	                landed that line names, per the landing-form table
//
// The verdicts are the library's and unchanged; this file only reports. The
// field is computed in JSON mode only, so the plain `--coverage` line output
// (and its wall-clock budget) does none of these reads. witness_schema_version
// sits on each brief object: the top level stays an array, so the change is
// additive for every existing reader.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// coverageWitnessFieldVersion is the `witness` field's version
// (DR-desk-tools-45: additive and versioned; a missing or unknown version is a
// gap to the reader).
const coverageWitnessFieldVersion = 1

// Evidence-actor values the witness field reports.
const (
	evidenceActorVerifier = "verifier"
	evidenceActorOther    = "other"
	evidenceActorUnknown  = "unknown"
)

type coverageWitnessJSON struct {
	Schema        int     `json:"schema"`
	Judgment      string  `json:"judgment"`
	Gap           string  `json:"gap,omitempty"`
	CR            string  `json:"cr,omitempty"`
	Record        string  `json:"record,omitempty"`
	Runner        string  `json:"runner"`
	Independent   bool    `json:"independent"`
	EvidenceActor string  `json:"evidence_actor"`
	LandingCR     *string `json:"landing_cr,omitempty"`
}

type coverageClaimJSON struct {
	evaluation.Claim
	Witness *coverageWitnessJSON `json:"witness,omitempty"`
}

type coverageBriefJSON struct {
	BriefID              string              `json:"brief"`
	Released             bool                `json:"released"`
	WitnessSchemaVersion int                 `json:"witness_schema_version"`
	Claims               []coverageClaimJSON `json:"claims"`
}

// verifyRowClaimText is the claim text the library gives a Verify row
// (evaluation/coverage_api.go evaluateOneCoverage).
func verifyRowClaimText(r evaluation.VerifyRow) string {
	return "Verify row #" + r.ID + ": " + r.Command
}

// coverageJSONRows renders every brief's verdict with the witness field.
func coverageJSONRows(root string, ids []string, cov map[string]Coverage, observed map[string]observedBrief) []coverageBriefJSON {
	rows := make([]coverageBriefJSON, len(ids))
	var wg sync.WaitGroup
	sem := make(chan struct{}, evidenceActorBlameLimit)
	reads := newWitnessFieldReads(root)
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rows[i] = coverageBriefWithWitness(reads, cov[id], observed[id])
		}(i, id)
	}
	wg.Wait()
	return rows
}

func coverageBriefWithWitness(reads *witnessFieldReads, c Coverage, ob observedBrief) coverageBriefJSON {
	out := coverageBriefJSON{BriefID: c.BriefID, Released: c.Released, WitnessSchemaVersion: coverageWitnessFieldVersion}
	if c.Claims != nil {
		out.Claims = make([]coverageClaimJSON, len(c.Claims)) // nil stays nil, as the verdict marshals it
	}
	byClaim := map[string]evaluation.VerifyRow{}
	for _, r := range ob.Obs.VerifyRows {
		byClaim[verifyRowClaimText(r)] = r
	}
	var lines map[string]int // citing row text -> 1-based line in the brief file
	var authors map[int]blameLine
	for i, cl := range c.Claims {
		out.Claims[i] = coverageClaimJSON{Claim: cl}
		r, ok := byClaim[cl.Claim]
		if !ok || len(ob.WitnessTexts[r.ID]) == 0 {
			continue
		}
		texts := ob.WitnessTexts[r.ID]
		text := texts[len(texts)-1] // the last witness for the row wins, as the library reads it
		cells := witnessCells(text)
		if cells == nil {
			continue
		}
		runnerCell := strings.TrimSpace(cells[witnessCellRunner])
		parsed := evaluation.ParseRunnerCell(runnerCell)
		w := &coverageWitnessJSON{
			Schema:        parsed.Schema,
			Judgment:      cl.Result,
			Runner:        strings.TrimSpace(parsed.Identity),
			Independent:   witnessRowIndependent(text, parsed.Identity),
			EvidenceActor: evidenceActorUnknown,
		}
		if parsed.Schema != evaluation.WitnessSchemaV1 {
			w.Gap = evaluation.WitnessGap(cl.Reason)
		}
		if parsed.Schema == evaluation.WitnessSchemaV2 {
			w.CR, w.Record = parsed.CR, parsed.RecordID
		}
		if lines == nil {
			lines, authors = reads.citingLines(ob.Path)
		}
		var commit string
		if n, ok := lines[text]; ok {
			if bl, ok := authors[n]; ok {
				w.EvidenceActor = reads.actorOf(bl)
				commit = bl.Commit
			}
		}
		if parsed.Schema == evaluation.WitnessSchemaV1 {
			cr := reads.landingCR(commit)
			w.LandingCR = &cr
		}
		out.Claims[i].Witness = w
	}
	return out
}

// witnessRowIndependent is the existing offline identity reads for one
// witness row: the independent-row read (evidenceHasIndependentRow over the
// row alone) and the verifier floor on the cell's identity token.
func witnessRowIndependent(text, identity string) bool {
	return evidenceHasIndependentRow(witnessHeader+"\n"+text) && runnerClearsFloor(strings.TrimSpace(identity))
}

// witnessFieldReads holds the run-wide reads the witness field shares: the
// Evidence-actor policy, and the landing commit and its change request per
// blamed commit (memoized).
type witnessFieldReads struct {
	root   string
	policy evidenceActorPolicy

	once      sync.Once
	firstPars map[string]bool
	tip       string

	mu      sync.Mutex
	landing map[string]string
}

func newWitnessFieldReads(root string) *witnessFieldReads {
	return &witnessFieldReads{root: root, policy: evidenceActorPolicyFromRoster(), landing: map[string]string{}}
}

// citingLines maps each witness row's trimmed text in the brief's Evidence
// section to the 1-based line it sits on (the last occurrence wins), and blames
// the section once.
func (w *witnessFieldReads) citingLines(path string) (map[string]int, map[int]blameLine) {
	lines := map[string]int{}
	authors := map[int]blameLine{}
	rawBytes, err := os.ReadFile(path)
	if err != nil || w.root == "" {
		return lines, authors
	}
	raw := string(rawBytes)
	start, end, ok := evidenceLineRange(raw)
	if !ok {
		return lines, authors
	}
	all := strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
	inComment := false
	for n := start; n <= end && n <= len(all); n++ {
		var bearing bool
		bearing, inComment = blameLineBearsContent(all[n-1], inComment)
		if bearing && isWitnessRow(strings.TrimSpace(all[n-1])) {
			lines[strings.TrimSpace(all[n-1])] = n
		}
	}
	rel, err := filepath.Rel(w.root, path)
	if err != nil || strings.HasPrefix(rel, "..") || len(lines) == 0 {
		return lines, authors
	}
	out, err := historyGit(w.root, "blame", "--line-porcelain", "-L", strconv.Itoa(start)+","+strconv.Itoa(end), "--", filepath.ToSlash(rel)).Output()
	if err != nil {
		return lines, authors
	}
	blamed, err := blameRawLines(w.root, string(out), true)
	if err != nil || len(blamed) != end-start+1 {
		return lines, authors
	}
	for i, bl := range blamed {
		authors[start+i] = bl
	}
	return lines, authors
}

// actorOf classifies a blamed line's commit author with the Evidence-actor
// policy. An unavailable policy, an uncommitted line or an unreadable author
// is unknown; only the bound verifier App is verifier.
func (w *witnessFieldReads) actorOf(bl blameLine) string {
	if w.policy.Unavailable != "" || bl.Commit == "" || strings.Trim(bl.Commit, "0") == "" || bl.Author.Email == "" {
		return evidenceActorUnknown
	}
	if v, _ := w.policy.classify(bl.Author.Name, bl.Author.Email); v == actorVerifier {
		return evidenceActorVerifier
	}
	return evidenceActorOther
}

// landingCR is the change request the first-parent commit of the default
// branch that landed commit names (evaluation.LandingCR), "" when the commit
// is unknown, not landed, or landed in no recognised form.
func (w *witnessFieldReads) landingCR(commit string) string {
	if commit == "" || strings.Trim(commit, "0") == "" || w.root == "" {
		return ""
	}
	w.mu.Lock()
	if cr, ok := w.landing[commit]; ok {
		w.mu.Unlock()
		return cr
	}
	w.mu.Unlock()
	w.once.Do(w.readFirstParents)
	cr := ""
	if w.tip != "" {
		landing := commit
		if !w.firstPars[commit] {
			out, err := historyGit(w.root, "rev-list", "--first-parent", "--ancestry-path", "--reverse", commit+".."+w.tip).Output()
			landing = ""
			if err == nil {
				if f := strings.Fields(string(out)); len(f) > 0 {
					landing = f[0]
				}
			}
		}
		if landing != "" {
			if out, err := historyGit(w.root, "show", "-s", "--format=%P%x1f%B", landing).Output(); err == nil {
				if parents, msg, ok := strings.Cut(string(out), "\x1f"); ok {
					cr = evaluation.LandingCR(msg, len(strings.Fields(parents)))
				}
			}
		}
	}
	w.mu.Lock()
	w.landing[commit] = cr
	w.mu.Unlock()
	return cr
}

// readFirstParents resolves the default branch's tip (origin/HEAD, then
// origin/main, then main) and its first-parent history.
func (w *witnessFieldReads) readFirstParents() {
	w.firstPars = map[string]bool{}
	for _, ref := range []string{"refs/remotes/origin/HEAD", "refs/remotes/origin/main", "refs/heads/main"} {
		out, err := historyGit(w.root, "rev-parse", "--verify", "-q", ref+"^{commit}").Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			w.tip = strings.TrimSpace(string(out))
			break
		}
	}
	if w.tip == "" {
		return
	}
	out, err := historyGit(w.root, "rev-list", "--first-parent", w.tip).Output()
	if err != nil {
		w.tip = ""
		return
	}
	for _, c := range strings.Fields(string(out)) {
		w.firstPars[c] = true
	}
}
