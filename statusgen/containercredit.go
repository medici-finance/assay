package main

// The container bracket's credit check (desktools-v2/71 Task 8): `verifyrun
// --bracket-digest --witness <brief> --credit --source <40 hex>`.
//
// verdict=match says only that a bracket agrees with its own span. The digest
// is keyless, so anyone who can edit the brief can write a matching bracket.
// Credit needs three more things, each read here:
//
//	a. provenance: every line from the span's header through the bracket was
//	   committed by the roster's bound verifier, matched on the author email's
//	   pinned numeric id, line by line (flipJudgeBlame's test, without its
//	   roster-human allowance);
//	b. record: the span's attestation line is the line rendered from the
//	   dispatcher-authored attestation record it names, admitted under the rules
//	   deskkit's ReadVerifierAttestation applies;
//	c. binding: that attestation names this brief and the source under
//	   verification.
//
// statusgen does not link deskkit, so (b) reads the record through `deskread`
// (issue, trust and label-events) and applies the rules itself. Row 11's parity
// test holds the two readers to the same verdict on shared fixtures
// (testdata/credit-records/).
//
// Per-line provenance reads the author git recorded, which whoever can commit
// to the brief can set: it is author-asserted, not authorization. The forge-
// attributed record in (b) is the anchor; server-side push rules close the rest.

import (
	"github.com/medici-finance/assay/deskcore/attestation"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	creditCredited      = "credited"
	creditNotCredited   = "not-credited"
	creditCouldNotCheck = "could-not-check"

	// verifierRecordTitle and verifierRecordPreamble are deskkit's
	// VerifierAttestationTitle and the fixed first paragraph of verifierBody.
	verifierRecordTitle    = attestation.Title
	verifierRecordPreamble = attestation.Preamble

	// The dispatcher's model stamp labels (deskkit DispatchedModelPrefix,
	// DispatchedTierPrefix).
	dispatchedModelPrefix = "dispatched-model:"
	dispatchedTierPrefix  = "dispatched-tier:"
)

// creditDeskreadBin names the read verb in messages. It is launched by that
// literal name, as deskgit and deskdispatch are, so the guard in
// TestRowEnvSingleSource reads it as a fixed tool; tests put a fake on PATH.
const creditDeskreadBin = "deskread"

// creditVerdict is one condition's or one bracket's answer.
type creditVerdict struct {
	verdict string
	why     string
}

func creditOK() creditVerdict { return creditVerdict{verdict: creditCredited} }

func creditNot(format string, a ...any) creditVerdict {
	return creditVerdict{verdict: creditNotCredited, why: fmt.Sprintf(format, a...)}
}

func creditCouldNot(format string, a ...any) creditVerdict {
	return creditVerdict{verdict: creditCouldNotCheck, why: fmt.Sprintf(format, a...)}
}

// combineCredit folds condition answers: any not-credited wins, then any
// could-not-check, else credited.
func combineCredit(vs ...creditVerdict) creditVerdict {
	var could []string
	var not []string
	for _, v := range vs {
		switch v.verdict {
		case creditNotCredited:
			not = append(not, v.why)
		case creditCouldNotCheck:
			could = append(could, v.why)
		}
	}
	switch {
	case len(not) > 0:
		return creditVerdict{verdict: creditNotCredited, why: strings.Join(not, "; ")}
	case len(could) > 0:
		return creditVerdict{verdict: creditCouldNotCheck, why: strings.Join(could, "; ")}
	}
	return creditOK()
}

// creditContext is what every bracket of one witness file is judged against.
type creditContext struct {
	source string // --source
	dir    string // the witness's directory, where git runs
	base   string // the witness's file name
	rel    string // the witness's path from the repository toplevel
	repo   string // the brief's own repository, owner/name from origin
	policy evidenceActorPolicy
	desk   string // the dispatcher's slug, lowercased
	lines  []string
	// place, prov and rec are the answers that apply to every bracket: where
	// the witness sits, what provenance can be read against, and what a
	// record can be read against. Each is credited when it does not decide.
	place, prov, rec creditVerdict
	records          map[string]creditRecord
}

// newCreditContext reads what does not depend on the bracket: the witness's
// place in its repository, the roster, the clone's depth and the origin.
func newCreditContext(witness, source, text string) *creditContext {
	cc := &creditContext{source: source, lines: strings.Split(text, "\n"), records: map[string]creditRecord{}}
	cc.place, cc.prov, cc.rec = creditOK(), creditOK(), creditOK()
	if cc.place = cc.placeWitness(witness); cc.place.verdict != creditCredited {
		return cc
	}
	cc.prov = cc.provenanceSetup()
	cc.rec = cc.recordSetup()
	return cc
}

// placeWitness finds the witness's repository-relative path.
func (cc *creditContext) placeWitness(witness string) creditVerdict {
	resolved, err := filepath.EvalSymlinks(witness)
	if err != nil {
		return creditCouldNot("cannot resolve %s: %v", witness, err)
	}
	cc.dir, cc.base = filepath.Dir(resolved), filepath.Base(resolved)
	top, err := historyGit(cc.dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return creditCouldNot("--credit needs a --witness file inside a git work tree; %s is not in one", witness)
	}
	toplevel, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil {
		return creditCouldNot("cannot resolve the repository toplevel: %v", err)
	}
	rel, err := filepath.Rel(toplevel, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return creditCouldNot("cannot place %s inside its repository", witness)
	}
	cc.rel = filepath.ToSlash(rel)
	return creditOK()
}

// provenanceSetup is what condition (a) needs before any blame: a roster
// whose verifier entry is id-pinned, a full-depth clone, a committed file.
func (cc *creditContext) provenanceSetup() creditVerdict {
	cc.policy = evidenceActorPolicyFromRoster()
	if cc.policy.Unavailable != "" {
		return creditCouldNot("provenance: %s", cc.policy.Unavailable)
	}
	if !cc.policy.idPinned() {
		return creditCouldNot("provenance: the verifier match is %s, not id-pinned, so no line can be credited to the verifier", cc.policy.matchStrengthNote())
	}
	if isShallowRepository(cc.dir) {
		return creditCouldNot("provenance: %s is in a shallow clone; blame cannot reach the commits behind its lines", cc.base)
	}
	if err := historyGit(cc.dir, "cat-file", "-e", "HEAD:"+cc.rel).Run(); err != nil {
		return creditNot("provenance: %s is not in HEAD, so no line of it has a committed author", cc.rel)
	}
	return creditOK()
}

// recordSetup is what condition (b) needs before any read: distinct
// dispatcher and verifier identities, a GitHub dispatcher, and the brief's
// own repository from a github.com origin.
func (cc *creditContext) recordSetup() creditVerdict {
	cfg := scanEffectiveConfig()
	desk := strings.ToLower(strings.TrimSpace(cfg.RoleBots["desk"]))
	verifier := strings.ToLower(strings.TrimSpace(cfg.RoleBots["verifier"]))
	switch {
	case desk == "" || verifier == "":
		return creditCouldNot("record: the roster does not bind both a dispatcher (desk) and a verifier identity")
	case desk == verifier:
		return creditNot("record: the roster binds the dispatcher and the verifier to one actor (%s), so no record has independent authority", desk)
	}
	if ident, ok := cfg.BotIdents[desk]; ok && ident.Forge != forgeGitHub {
		return creditCouldNot("record: the dispatcher %s is bound on forge %q; this check reads GitHub records only", desk, ident.ForgeRaw)
	}
	cc.desk = desk
	origin, err := historyGit(cc.dir, "remote", "get-url", "origin").Output()
	if err != nil || remoteHost(string(origin)) != "github.com" {
		return creditCouldNot("record: the brief's repository needs a github.com origin to name the repository its record must be in")
	}
	if cc.repo = originOwnerRepo(strings.TrimSpace(string(origin))); cc.repo == "" {
		return creditCouldNot("record: cannot read owner/name from the origin remote")
	}
	return creditOK()
}

// credit judges one bracket. Conditions run cheapest first and stop at the
// first not-credited, so a bracket that already fails reaches no forge.
func (cc *creditContext) credit(c bracketCheck) creditVerdict {
	if c.verdict != "match" {
		return creditNot("verdict=%s; only a matching bracket can be credited", c.verdict)
	}
	if cc.place.verdict != creditCredited {
		return cc.place
	}
	_, attestation, problem := spanOwnLines(cc.lines[c.header-1 : c.bracket-1])
	if problem != "" {
		return creditNot("%s", problem)
	}
	a := attestationV1Re.FindStringSubmatch(attestation)
	if a == nil {
		return creditNot("the span's attestation line is not the version-1 form; a version-2 (cross-repository) span is not credited by this rule")
	}
	var verdicts []creditVerdict
	for _, step := range []func() creditVerdict{
		func() creditVerdict { return cc.binding(a) },
		func() creditVerdict { return cc.provenance(c) },
		func() creditVerdict { return cc.record(a, attestation) },
	} {
		v := step()
		if v.verdict == creditNotCredited {
			return v
		}
		verdicts = append(verdicts, v)
	}
	return combineCredit(verdicts...)
}

// binding is condition (c): the attestation names this brief and --source.
func (cc *creditContext) binding(a []string) creditVerdict {
	if a[5] != cc.rel {
		return creditNot("binding: the attestation names brief=%s, not this brief %s", a[5], cc.rel)
	}
	if a[4] != cc.source {
		return creditNot("binding: the attestation names source=%s, not --source %s", a[4], cc.source)
	}
	return creditOK()
}

// provenance is condition (a): every line from the header through the bracket
// was committed by the bound verifier, matched on the pinned id.
func (cc *creditContext) provenance(c bracketCheck) creditVerdict {
	if cc.prov.verdict != creditCredited {
		return cc.prov
	}
	want := c.bracket - c.header + 1
	out, err := historyGit(cc.dir, "blame", "--line-porcelain", "-L", fmt.Sprintf("%d,%d", c.header, c.bracket), "--", cc.base).Output()
	if err != nil {
		return creditCouldNot("provenance: git blame %s: %v", cc.base, err)
	}
	lines, err := blameRawLines(cc.dir, string(out), false)
	if err != nil {
		return creditCouldNot("provenance: %s: %v", cc.base, err)
	}
	if len(lines) != want {
		return creditCouldNot("provenance: blame of %s returned %d record(s) for the %d line(s) from L%d to L%d", cc.base, len(lines), want, c.header, c.bracket)
	}
	for i, bl := range lines {
		at := c.header + i
		if bl.Author.Name == "" && bl.Author.Email == "" {
			return creditCouldNot("provenance: blame of %s named no author for L%d", cc.base, at)
		}
		if strings.Trim(bl.Commit, "0") == "" {
			return creditNot("provenance: L%d differs from HEAD; an uncommitted line has no author", at)
		}
		if v, why := cc.policy.classify(bl.Author.Name, bl.Author.Email); v != actorVerifier {
			return creditNot("provenance: L%d was not committed by the bound verifier (%s)", at, why)
		}
	}
	return creditOK()
}

// creditBinding mirrors deskkit's VerifierBinding field for field, in the same
// order with the same tags, so json.MarshalIndent renders the same body.
type creditBinding = attestation.Binding

// creditRecord is one <repo>#<issue> read: whether the record is admitted, and
// the attestation line rendered from its binding. It names no bracket.
type creditRecord struct {
	verdict          creditVerdict
	target, rendered string
}

// record is condition (b). Only the record read is cached per <repo>#<issue>;
// each bracket's attestation line is compared with the rendered line on its
// own, so a bracket never inherits another bracket's comparison.
func (cc *creditContext) record(a []string, attestation string) creditVerdict {
	if cc.rec.verdict != creditCredited {
		return cc.rec
	}
	key := a[1] + "#" + a[2]
	r, ok := cc.records[key]
	if !ok {
		r = cc.readRecord(a)
		cc.records[key] = r
	}
	if r.verdict.verdict != creditCredited {
		return r.verdict
	}
	if r.rendered != attestation {
		m := attestationV1Re.FindStringSubmatch(r.rendered)
		var differ []string
		for i, name := range []string{"repo", "issue", "run", "source", "brief", "model", "tier"} {
			if m == nil || m[i+1] != a[i+1] {
				differ = append(differ, name)
			}
		}
		return creditNot("record: the span's attestation differs from the line rendered from %s in %s", r.target, strings.Join(differ, ", "))
	}
	return creditOK()
}

// readRecord reads and admits a[1]#a[2] as ReadVerifierAttestation would.
func (cc *creditContext) readRecord(a []string) creditRecord {
	v, target, rendered := cc.admitRecord(a)
	return creditRecord{verdict: v, target: target, rendered: rendered}
}

func (cc *creditContext) admitRecord(a []string) (creditVerdict, string, string) {
	repo, issueNo := a[1], a[2]
	if !strings.EqualFold(repo, cc.repo) {
		return creditNot("record: %s#%s is not in the brief's own repository %s", repo, issueNo, cc.repo), "", ""
	}
	n, err := strconv.Atoi(issueNo)
	if err != nil || n <= 0 {
		return creditNot("record: %s is not a positive issue number", issueNo), "", ""
	}
	target := fmt.Sprintf("%s#%d", repo, n)

	first, err := creditReadIssue(target)
	if err != nil {
		return creditCouldNot("record: %v", err), target, ""
	}
	b, err := attestation.ParseBody(first.Body)
	if err != nil {
		return creditNot("record: %s's body is not a binding: %v", target, err), target, ""
	}

	if why := creditBindingProblem(b, repo); why != "" {
		return creditNot("record: %s: %s", target, why), target, ""
	}
	title, body := verifierRecordTitle+b.Run, attestation.Body(b)
	if why := cc.recordTarget(first, title, body); why != "" {
		return creditNot("record: %s %s", target, why), target, ""
	}

	trust, err := creditReadTrust(target)
	if err != nil {
		return creditCouldNot("record: %v", err), target, ""
	}
	if why := attestation.HistoryProblem(trust.BodyHistoryKnown != nil && *trust.BodyHistoryKnown, trust.Complete, trust.BodyEditedAt != ""); why != "" {
		return creditNot("record: %s's %s", target, why), target, ""
	}

	if first.State != "closed" {
		return creditNot("record: %s is %s, not closed; the pre-work attestation has not completed", target, first.State), target, ""
	}

	// A second read after the history read: a record edited or reopened in
	// between is not the one whose history was read.
	second, err := creditReadIssue(target)
	if err != nil {
		return creditCouldNot("record: %v", err), target, ""
	}
	if why := cc.recordTarget(second, title, body); why != "" {
		return creditNot("record: %s on the second read %s", target, why), target, ""
	}
	if second.State != "closed" {
		return creditNot("record: %s is %s on the second read, not closed", target, second.State), target, ""
	}

	events, err := creditReadLabelEvents(target)
	if err != nil {
		return creditCouldNot("record: %v", err), target, ""
	}
	if events.Complete == nil || !*events.Complete {
		return creditCouldNot("record: %s's label history is incomplete", target), target, ""
	}
	model, tier, why := cc.attestedStamp(second.Labels, events.Events)
	if why != "" {
		return creditNot("record: %s: %s", target, why), target, ""
	}
	if model != b.Model || tier != b.Tier {
		return creditNot("record: %s's dispatcher stamp names model=%s tier=%s, the binding model=%s tier=%s", target, model, tier, b.Model, b.Tier), target, ""
	}

	rendered := fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", b.Repo, n, b.Run, b.Source, b.Brief, b.Model, b.Tier)
	return creditOK(), target, rendered
}

// creditBindingProblem applies ReadVerifierAttestation's field checks. Only a
// version 1 (same-repository) binding is accepted. The bracket admits version
// 2 (cross-repository) receipts, but crediting one also needs the source
// repository checked against the configured repository set, which this rule
// does not read yet, so a version-2 span stays not-credited.
func creditBindingProblem(b creditBinding, repo string) string {
	return attestation.BindingProblem(b, repo, attestation.ContainerV1, false)
}

// creditModelSlugOK is deskkit's model-label shape: lowercased and trimmed,
// non-empty, [a-z0-9.-] only, with at least one letter or digit.
func creditModelSlugOK(slug string) bool { return attestation.ModelSlugOK(slug) }

// isDispatcher is deskkit's SameActor against the dispatcher's GitHub App
// login: a case-folded name match that is an App on both sides.
func (cc *creditContext) isDispatcher(login string) bool {
	s := strings.ToLower(strings.TrimSpace(login))
	if rest, ok := strings.CutPrefix(s, "app/"); ok {
		return rest != "" && rest == cc.desk
	}
	if rest, ok := strings.CutSuffix(s, "[bot]"); ok {
		return rest != "" && rest == cc.desk
	}
	return false
}

// recordTarget is deskkit's attestationTarget: the dispatcher authored it, and
// its title and body are exactly the binding's rendering. "" when it holds.
func (cc *creditContext) recordTarget(iss *creditIssue, title, body string) string {
	if iss == nil {
		return attestation.TargetProblem(false, false, false, "", "", title, body)
	}
	login := ""
	if iss.Author != nil {
		login = iss.Author.Login
	}
	return attestation.TargetProblem(true, false, cc.isDispatcher(login), iss.Title, iss.Body, title, body)
}

// attestedStamp is deskkit's AttestedModelStampOf for the dispatched pair:
// every present stamp label's standing application must be the dispatcher's,
// and the labels must name one model and one tier.
func (cc *creditContext) attestedStamp(present []string, events []creditLabelEvent) (string, string, string) {
	facts := make([]attestation.LabelEvent, 0, len(events))
	for _, e := range events {
		facts = append(facts, attestation.LabelEvent{Label: e.Label, Removed: e.Removed, Authority: cc.isDispatcher(e.ActorLogin)})
	}
	return attestation.Stamp(present, facts)
}

// --- deskread -----------------------------------------------------------------

type creditAccount struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type creditIssue struct {
	State  string         `json:"state"`
	Title  string         `json:"title"`
	Body   string         `json:"body"`
	Labels []string       `json:"labels"`
	Author *creditAccount `json:"author"`
}

type creditTrust struct {
	BodyEditedAt     string `json:"bodyEditedAt"`
	Complete         bool   `json:"complete"`
	BodyHistoryKnown *bool  `json:"bodyHistoryKnown"`
}

type creditLabelEvent struct {
	Label      string `json:"label"`
	Removed    bool   `json:"removed"`
	ActorLogin string `json:"actorLogin"`
}

type creditLabelEvents struct {
	Complete *bool              `json:"complete"`
	Events   []creditLabelEvent `json:"events"`
}

type creditEnvelope struct {
	Schema int    `json:"schema"`
	Kind   string `json:"kind"`
	Items  []struct {
		Repo        string             `json:"repo"`
		Number      int                `json:"number"`
		Issue       *creditIssue       `json:"issue"`
		Trust       *creditTrust       `json:"trust"`
		LabelEvents *creditLabelEvents `json:"labelEvents"`
	} `json:"items"`
	Partial []struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
		Reason string `json:"reason"`
	} `json:"partial"`
}

// creditDeskread runs `deskread <kind> --issue <repo>#<n>` and returns the
// item for that one issue. Every way it went unread is an error, which the
// caller reports as could-not-check.
func creditDeskread(kind, target string) (*creditEnvelope, int, error) {
	repo, num, _ := strings.Cut(target, "#")
	n, _ := strconv.Atoi(num)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "deskread", kind, "--issue", target)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var env creditEnvelope
	jerr := json.Unmarshal(out, &env)
	if jerr == nil && env.Schema != deskreadSchema {
		return nil, 0, fmt.Errorf("%s %s answered envelope schema %d, this build understands %d", creditDeskreadBin, kind, env.Schema, deskreadSchema)
	}
	if jerr == nil {
		for _, p := range env.Partial {
			if strings.EqualFold(p.Repo, repo) && p.Number == n {
				return nil, 0, fmt.Errorf("%s %s --issue %s: %s", creditDeskreadBin, kind, target, p.Reason)
			}
		}
		for i, it := range env.Items {
			if strings.EqualFold(it.Repo, repo) && it.Number == n {
				return &env, i, nil
			}
		}
	}
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s --issue %s: %v %s", creditDeskreadBin, kind, target, err, firstLine(stderr.String()))
	}
	if jerr != nil {
		return nil, 0, fmt.Errorf("parsing the %s %s envelope for %s: %v", creditDeskreadBin, kind, target, jerr)
	}
	return nil, 0, fmt.Errorf("%s %s --issue %s: the envelope names no result and no reason for this issue", creditDeskreadBin, kind, target)
}

func creditReadIssue(target string) (*creditIssue, error) {
	env, i, err := creditDeskread("issue", target)
	if err != nil {
		return nil, err
	}
	if env.Items[i].Issue == nil {
		return nil, fmt.Errorf("%s issue --issue %s: the item carries no issue", creditDeskreadBin, target)
	}
	return env.Items[i].Issue, nil
}

func creditReadTrust(target string) (*creditTrust, error) {
	env, i, err := creditDeskread("trust", target)
	if err != nil {
		return nil, err
	}
	if env.Items[i].Trust == nil {
		return nil, fmt.Errorf("%s trust --issue %s: the item carries no trust payload", creditDeskreadBin, target)
	}
	return env.Items[i].Trust, nil
}

func creditReadLabelEvents(target string) (*creditLabelEvents, error) {
	env, i, err := creditDeskread("label-events", target)
	if err != nil {
		return nil, err
	}
	if env.Items[i].LabelEvents == nil {
		return nil, fmt.Errorf("%s label-events --issue %s: the item carries no label history", creditDeskreadBin, target)
	}
	return env.Items[i].LabelEvents, nil
}

// creditSourceOK is --source's form: a 40-hex commit id.
func creditSourceOK(s string) bool { return hex40Re.MatchString(s) }
