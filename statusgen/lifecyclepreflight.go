package main

// Prospective delivery admission uses reconcile's classifiers and lint hold. It
// never updates a board: CI checks the state a delivery would establish before
// the merge becomes a reconciliation witness.
import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/workref/trailers"
)

type preflightBrief struct {
	ID             string   `json:"id"`
	Classification string   `json:"classification"`
	From           string   `json:"from,omitempty"`
	To             string   `json:"to,omitempty"`
	Problems       []string `json:"problems,omitempty"`
}
type lifecyclePreflightResult struct {
	Schema   string           `json:"schema"`
	Repo     string           `json:"repo"`
	PR       int              `json:"pr"`
	HeadSHA  string           `json:"headSha"`
	LookedAt bool             `json:"lookedAt"`
	Passed   bool             `json:"passed"`
	Briefs   []preflightBrief `json:"briefs"`
	Reason   string           `json:"reason,omitempty"`
}

var preflightSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var preflightRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func (c *ghClient) preflightPull(repo string, n int) (ghPull, error) {
	var p ghPull
	b, status, err := c.get(fmt.Sprintf("%s/repos/%s/pulls/%d", c.base, repo, n))
	if err != nil {
		return p, err
	}
	if status != http.StatusOK {
		return p, fmt.Errorf("%s", httpReason(status, b))
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("unreadable pull response: %w", err)
	}
	if p.Number != n || !preflightSHA.MatchString(p.Head.SHA) {
		return p, fmt.Errorf("pull response lacks matching number or head SHA")
	}
	return p, nil
}

func runLifecyclePreflight(args []string, stdout, stderr *os.File) int {
	scanSetToolClass(scanClassForMode(false))
	fs := flag.NewFlagSet("lifecycle-preflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "materialized PR tree to evaluate (read only)")
	repo := fs.String("repo", "", "GitHub owner/name")
	pr := fs.Int("pr", 0, "pull request number")
	head := fs.String("expected-head", "", "required immutable PR head SHA")
	baseRev := fs.String("base-rev", "", "judged base commit (default: HEAD^1 of a --root merge checkout whose HEAD^2 is --expected-head)")
	tokenFile := fs.String("token-file", "", "GitHub read token file (else GITHUB_TOKEN)")
	jsonOut := fs.Bool("json", false, "emit lifecycle-preflight-v1 JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || !preflightRepo.MatchString(*repo) || *pr <= 0 || !preflightSHA.MatchString(*head) {
		fmt.Fprintln(stderr, "lifecycle-preflight requires --repo owner/name --pr N --expected-head SHA")
		return 2
	}
	res := lifecyclePreflightResult{Schema: "lifecycle-preflight-v1", Repo: *repo, PR: *pr, HeadSHA: *head, Briefs: []preflightBrief{}}
	code := 3
	token, err := resolveGitHubToken(*tokenFile)
	base, berr := preflightBaseRev(*root, *head, *baseRev)
	switch {
	case err != nil:
		res.Reason = err.Error()
	case berr != nil:
		res.Reason = "could-not-check: " + berr.Error()
	default:
		res, code = checkLifecyclePreflight(*root, *repo, *pr, *head, base, reconcileGHClient(token), stderr)
	}
	if *jsonOut {
		if err := json.NewEncoder(stdout).Encode(res); err != nil {
			fmt.Fprintln(stderr, err)
			return 3
		}
	} else {
		fmt.Fprintf(stdout, "lifecycle-preflight: passed=%t lookedAt=%t PR #%d head %s\n", res.Passed, res.LookedAt, res.PR, res.HeadSHA)
		for _, b := range res.Briefs {
			fmt.Fprintf(stdout, "  %s: %s %s -> %s\n", b.ID, b.Classification, b.From, b.To)
			for _, p := range b.Problems {
				fmt.Fprintf(stdout, "    %s\n", p)
			}
		}
		if res.Reason != "" {
			fmt.Fprintln(stdout, res.Reason)
		}
	}
	return code
}

// preflightBaseRev names the judged base the decision gate and alias registry
// are read at. The PR tree under judgment never supplies them: a change could
// otherwise downgrade its own brief's gate or widen the alias registry and be
// judged against that edit. Without an explicit --base-rev the root must be the
// PR merge checkout (HEAD^2 is the expected head); anything else is
// could-not-check, never a fall-back to the working tree.
func preflightBaseRev(root, head, explicit string) (string, error) {
	rev := func(spec string) (string, error) {
		out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", spec+"^{commit}").Output()
		if err != nil {
			return "", fmt.Errorf("git rev-parse %s: %w", spec, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if explicit != "" {
		return rev(explicit)
	}
	prHead, err := rev("HEAD^2")
	if err != nil {
		return "", fmt.Errorf("judged base unknown: --root is not a PR merge checkout and no --base-rev was given (%v)", err)
	}
	if !strings.EqualFold(prHead, head) {
		return "", fmt.Errorf("judged base unknown: merge checkout HEAD^2 is %s, not the expected head %s", prHead, head)
	}
	return rev("HEAD^1")
}

func checkLifecyclePreflight(root, repo string, n int, head, baseRev string, c *ghClient, stderr io.Writer) (res lifecyclePreflightResult, code int) {
	res = lifecyclePreflightResult{Schema: "lifecycle-preflight-v1", Repo: repo, PR: n, HeadSHA: head, Briefs: []preflightBrief{}}
	fail := func(err error) (lifecyclePreflightResult, int) {
		res.Reason = "could-not-check: " + err.Error()
		return res, 3
	}
	pull, err := c.preflightPull(repo, n)
	if err != nil {
		return fail(err)
	}
	if pull.Head.SHA != head {
		return fail(fmt.Errorf("PR head changed: expected %s, observed %s", head, pull.Head.SHA))
	}
	refs, err := preflightDeliveryRefs(pull.Body)
	if err != nil {
		return fail(err)
	}
	// deskflip classifies a Cross-repo-brief: carrier as a delivery, but this
	// checker evaluates only this repository's board. Passing it with an empty
	// brief list would be a success that judged nothing.
	if len(refs) == 0 && preflightHasCrossRepoDelivery(pull.Body) {
		return fail(fmt.Errorf("Cross-repo-brief: delivery is not evaluated by this repository's lifecycle check"))
	}
	// Authors:/Issue: carriers have no delivery edge, exactly as reconciliation.
	if len(refs) == 0 {
		latest, err := c.preflightPull(repo, n)
		if err != nil {
			return fail(err)
		}
		if latest.Head.SHA != head || latest.Body != pull.Body {
			return fail(fmt.Errorf("PR head or delivery trailers changed during preflight"))
		}
		res.LookedAt = true
		res.Passed = true
		return res, 0
	}
	env, err := loadLintEnv(root)
	if err != nil {
		return fail(err)
	}
	// readDeclaredPartials deliberately defers load errors to the fold caller;
	// make that same prerequisite explicit here before using its answer.
	if _, _, err := loadReconcileFoldData(root); err != nil {
		return fail(err)
	}
	cl := newAuthoringClassifier(c, repo, stderr)
	cl.partials = readDeclaredPartials(root, repo)
	seen := map[string]bool{}
	var cells []BriefCell
	delivered := map[string]*BriefFile{}
	for _, ref := range refs {
		id := canonicalBriefKey(ref)
		if seen[id] {
			continue
		}
		seen[id] = true
		stream, num, ok := briefStreamNum(id)
		if !ok {
			return fail(fmt.Errorf("invalid Brief: reference %q", ref))
		}
		row := env.findRow(stream, num)
		if row == nil {
			return fail(fmt.Errorf("Brief: %s has no readable board row", id))
		}
		// A malformed/missing brief must not disappear through the lint gate's
		// legacy/malformed skip arm, nor through a baseline-difference comparison.
		si, _ := env.findRowAt(stream, num)
		found := false
		for _, path := range briefFilePaths(env.streams[si]) {
			if !isBriefFileFor(filepath.ToSlash(pathRelative(root, path)), stream, num) {
				continue
			}
			bf, _, err := parseBriefFile(path)
			if err != nil {
				return fail(err)
			}
			delivered[id] = bf
			found = true
		}
		if !found {
			return fail(fmt.Errorf("Brief: %s has no readable brief file", id))
		}
		b := preflightBrief{ID: id, From: row.Status, To: "implemented", Classification: "delivery"}
		for key, why := range cl.partials.undecided {
			if canonicalBriefKey(key) == id {
				return fail(fmt.Errorf("%s", why))
			}
		}
		files := cl.files(n)
		if files.err != nil {
			return fail(files.err)
		}
		if len(files.files) == 0 {
			return fail(fmt.Errorf("PR #%d file list is empty", n))
		}
		if b.From != "todo" && b.From != "in-progress" {
			b.To = b.From
		}
		if cl.partials.partial(n, id) {
			b.Classification = "partial"
			b.To = b.From
		} else {
			only, err := cl.notImplementation(n, id)
			if err != nil {
				return fail(err)
			}
			if only {
				b.Classification = "authoring-or-evidence"
				b.To = b.From
			} else {
				cells = append(cells, BriefCell{ID: id, Cell: "implemented", Source: "pr", Witness: fmt.Sprintf("prospective PR #%d head %s", n, head)})
			}
		}
		res.Briefs = append(res.Briefs, b)
	}
	_, held, err := planReconcileWrites(root, cells)
	if err != nil {
		return fail(err)
	}
	for _, h := range held {
		for i := range res.Briefs {
			if canonicalBriefKey(h.ID) == res.Briefs[i].ID {
				res.Briefs[i].Problems = append(res.Briefs[i].Problems, h.Problems...)
			}
		}
	}

	// The reconcile hold reports only *added* problems. A delivering brief
	// already in-progress may already violate its design gate; judge that brief's
	// projected state directly with the production gate, without dragging unrelated
	// rows into this PR's result or changing immutable advanced statuses.
	_, known, registerOK := designGateRegister(root)
	if !registerOK {
		return fail(fmt.Errorf("design register could not be read"))
	}
	for i := range res.Briefs {
		b := &res.Briefs[i]
		if b.Classification != "delivery" {
			continue
		}
		stream, num, _ := briefStreamNum(b.ID)
		si, row := env.findRowAt(stream, num)
		projected := *row
		projected.Status = b.To
		scoped := *env.streams[si]
		scoped.Briefs = []Brief{projected}
		for _, problem := range designGateStreamProblems(&scoped, known) {
			exists := false
			for _, old := range b.Problems {
				exists = exists || old == problem
			}
			if !exists {
				b.Problems = append(b.Problems, problem)
			}
		}
	}
	// Apply only this PR's proposed deliveries to the PR board, then run the same
	// two-layer human decision evaluator used by --decision-gate against the board
	// as committed at the judged base, so a same-PR gate/risk downgrade is seen.
	gateBase, err := gateSnapshotAtRev(root, baseRev)
	if err != nil {
		return fail(err)
	}
	aliases := decisionGateAliases(root, baseRev, repo)
	gateHead := gateSnapshotOnDisk(root)
	for _, cell := range cells {
		for _, b := range gateHead {
			if canonicalBriefKey(b.Board) == canonicalBriefKey(cell.ID) && (b.Status == "todo" || b.Status == "in-progress") {
				b.Status = "implemented"
			}
		}
	}
	faults := judgeDecisionGate(gateBase, gateHead)
	for _, verdict := range resolveDecisionGateFaults(c, repo, faults, scanEffectiveConfig().HumanLogins, aliases) {
		if !verdict.Refused {
			continue
		}
		for i := range res.Briefs {
			if canonicalBriefKey(verdict.Board) == res.Briefs[i].ID {
				res.Briefs[i].Problems = append(res.Briefs[i].Problems, verdict.Detail)
			}
		}
	}
	// A pre-existing invalid record is baseline lint, so it is not part of the
	// hold's *added* problems. Validate the delivering brief's own cited record
	// explicitly with the very same register validator and ruling resolver.
	entries, err := parseDecisionsDir(root)
	if err != nil {
		return fail(err)
	}
	registerProblems := decisionRegisterProblems(root)
	for i := range res.Briefs {
		b := &res.Briefs[i]
		bf := delivered[b.ID]
		if b.Classification != "delivery" || bf == nil || !riskGated(bf) {
			continue
		}
		if strings.TrimSpace(bf.Design) == "" {
			continue
		}
		// The design gate's notice-only reading of an unparseable authored: date
		// cannot admit a delivery that cites a record this check never validated.
		after, ok := authoredAfterCutover(bf.Authored)
		if !ok {
			return fail(fmt.Errorf("Brief: %s authored: %q does not parse; its design record could not be checked", b.ID, bf.Authored))
		}
		if !after {
			continue
		}
		for _, entry := range entries {
			if entry.ID != bf.Design {
				continue
			}
			rel := "docs/streams/decisions/" + entry.File
			for _, p := range registerProblems {
				if strings.HasPrefix(p, rel+":") {
					b.Problems = append(b.Problems, p)
				}
			}
			rec := loadDecisionRecordStamp(root, rel)
			// Existing PR-anchor approvals remain supported: a ruling URL is optional.
			// A present assertion must be corroborated, even when the record was not
			// modified in this delivery PR (the ordinary diff-scoped lane misses it).
			if strings.TrimSpace(rec.Ruling) != "" {
				out := resolveRuling(c, repo, rec, scanEffectiveConfig().HumanLogins, decisionCitingBriefs(root, rec.ID), aliases)
				if out.Reason == rulingUnresolvable || out.Reason == rulingRecordUnreadable {
					return fail(fmt.Errorf("%s: %s", rec.ID, out.Detail))
				}
				if !out.Present || out.Reason != rulingOK {
					b.Problems = append(b.Problems, fmt.Sprintf("%s: ruling %s: %s", rec.ID, out.Reason, out.Detail))
				}
			}
		}
	}
	// Check the immutable head once more after classification/approval reads.
	latest, err := c.preflightPull(repo, n)
	if err != nil {
		return fail(err)
	}
	if latest.Head.SHA != head || latest.Body != pull.Body {
		return fail(fmt.Errorf("PR head or delivery trailers changed during preflight"))
	}
	res.LookedAt = true
	res.Passed = true
	for _, b := range res.Briefs {
		if len(b.Problems) > 0 {
			res.Passed = false
		}
	}
	if !res.Passed {
		return res, 1
	}
	return res, 0
}
func pathRelative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(rel)
}

// preflightHasCrossRepoDelivery reports an unfenced, non-empty
// Cross-repo-brief: trailer line.
func preflightHasCrossRepoDelivery(body string) bool {
	for _, src := range trailers.OutsideFences(body) {
		line := strings.TrimSpace(src.Text)
		if strings.HasPrefix(line, "Cross-repo-brief:") && strings.TrimSpace(strings.TrimPrefix(line, "Cross-repo-brief:")) != "" {
			return true
		}
	}
	return false
}

// The shared parser supplies canonical references and fence handling. Its
// permissive unlinked classification intentionally ignores an empty trailer;
// admission must distinguish that malformed declaration from a genuine exemption.
func preflightDeliveryRefs(body string) ([]string, error) {
	declarations := 0
	for _, src := range trailers.OutsideFences(body) {
		line := strings.TrimSpace(src.Text)
		if strings.HasPrefix(line, "Brief:") {
			declarations++
			if strings.TrimSpace(strings.TrimPrefix(line, "Brief:")) == "" {
				return nil, fmt.Errorf("empty Brief: declaration")
			}
		}
	}
	if declarations > 1 {
		return nil, fmt.Errorf("ambiguous delivery: more than one unfenced Brief: declaration")
	}
	_, refs := parseBriefLikeTrailers(body)
	if declarations != len(refs) {
		return nil, fmt.Errorf("malformed Brief: declaration")
	}
	for _, ref := range refs {
		if _, _, ok := briefStreamNum(ref); !ok {
			return nil, fmt.Errorf("invalid Brief: reference %q", ref)
		}
	}
	return refs, nil
}
