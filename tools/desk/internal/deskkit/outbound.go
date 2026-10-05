package deskkit

// outbound.go — ONE check every outward write passes before it leaves the machine.
//
// WHY ONE FUNCTION. What the desk tools refused to write used to depend on which scan each
// verb's author remembered to call: one verb filed issues on a public repository with no
// self-containment check at all, nothing looked at commit messages or branch names, and
// tool-composed comments and labels were never scanned. OutboundCheck composes every layer
// — the credential scan, the impersonation guard, the personal-data pass, the
// self-containment categories and the withheld register — and is reached from the places
// every write already goes through:
//
//   - the checking Forge decorator (outboundforge.go), which ResolveForge — the ONLY
//     construction site of a backend — returns, so a verb cannot hold an unchecked Forge;
//   - the push path (outboundpush.go), called by deskpr before its push and by the
//     deskpushguard pre-push hook, for text that leaves by `git push` and never crosses a
//     Forge.
//
// IT REFUSES AND NEVER REWRITES. The refusal is `refused: <rule id> at <field>:<line> — …`
// (exit 5). There is no redaction mode. Nothing about a match reaches the forge: stderr may
// name the rule, the line and the span; the override's audit row carries the rule id and a
// DIGEST of the content only; no refusal, notice or override composes a forge write.
//
// VISIBILITY IS THE KEY, and it is the CONFIGURED one (RepoVisibility), never a live read —
// a gate that must reach the forge fails open when the forge is down. The public layers
// (self-containment and the withheld register) run exactly when SelfContainApplies does:
// on a configured roster, for every target that is not known-private (public AND unknown).
//
// THE OVERRIDE POLICY (ruled 2026-09-21, #1319, option 1 — layered overrides). Credential,
// personal-data and self-containment refusals accept the audited --force-scan-override. A
// withheld-identifier refusal (it only runs on a public or unknown target) and the
// impersonation guard do NOT: the way through is to reword, or for a human to change the
// configured withheld set.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Kinds of outward write. Every OutboundWrite names exactly one.
const (
	OutboundKindIssue   = "issue"
	OutboundKindChange  = "change"
	OutboundKindComment = "comment"
	OutboundKindReview  = "review"
	OutboundKindLabel   = "label"
	OutboundKindFile    = "file"
	OutboundKindCommit  = "commit"
	OutboundKindRef     = "ref"
)

// OutboundFieldCommitMessage names the one field of a kind-`file` write that is NOT a file's
// content or path: the commit message the Forge file-write seam carries beside the file
// (outboundForge.WriteFile). The synthetic-fixture exemption (#2217, SelfContainOpts.InFile)
// is for file content only, so this field never gets it.
const OutboundFieldCommitMessage = "commit"

// Rule ids. Compiled and generic: none names a deployment value.
const (
	// RuleSecretPrefix prefixes the credential scan's own rule id ("secret.github-token",
	// "secret.high-entropy-run", …) — see ScanFinding.Rule.
	RuleSecretPrefix = "secret."
	// RuleVoiceRulingClaim is the impersonated-human-ruling guard. Never overridable.
	RuleVoiceRulingClaim = "voice.ruling-claim"
	// RulePIIEmail is an e-mail address outside the compiled allow-list (personaldata.go).
	RulePIIEmail = "pii.email"
	// RulePIIPhone is a telephone number in international (E.164) shape.
	RulePIIPhone = "pii.phone"
	// RulePIIPhoneAmbiguous is a separator-grouped 10-11 digit run with no `+`. NOTICE only.
	RulePIIPhoneAmbiguous = "pii.phone-ambiguous"
	// RuleSelfContainPrefix prefixes a self-containment category, spelled with dashes
	// ("selfcontain.private-repository-name", "selfcontain.absolute-machine-path", …).
	RuleSelfContainPrefix = "selfcontain."
	// RuleWithheldIdentifier is the configured withheld register. Not overridable.
	RuleWithheldIdentifier = "withheld.identifier"
)

// OutboundField is one named piece of text an outward write carries ("title", "body",
// "name", a file path, …).
type OutboundField struct {
	Name string
	Text string
}

// OutboundWrite describes one outward write for OutboundCheck.
type OutboundWrite struct {
	// Tool and Verb name the invoking binary and its verb. Empty means the process's
	// OutboundContext (SetOutboundContext).
	Tool string
	Verb string
	// Role is the App role whose custody the write goes out under ("worker", "reviewer").
	Role string
	// Repo is the target, owner/name.
	Repo string
	// Visibility is the target's CONFIGURED visibility. OutboundCheck sets it from
	// RepoVisibility(Repo) itself; a caller-supplied value is never trusted, so no caller
	// can declare a target private to skip the public layers.
	Visibility Visibility
	// Kind is one of the OutboundKind* constants.
	Kind string
	// Fields is the text the write carries.
	Fields []OutboundField
	// NumberHint is the number of the item the write targets on Repo (the PR or issue being
	// commented on, reviewed or edited), or zero for a write that targets no numbered item
	// (a new issue, a file, a commit, a ref). It is the self-containment scan's only offline
	// evidence for the bare-`#N` notice (SelfContainOpts.NumberHint): with it, a bare
	// reference ABOVE the number is named as a probable cross-repo reference; without it the
	// category reports itself NOT CHECKED. Every write to a numbered item sets it
	// (TestOutboundWritesCarryNumber, TestOutboundNumberHintOnDecorator).
	NumberHint int
	// FileSources is for a kind-`file` write only: the FULL new-side content of each file
	// field, keyed by that field's Name (its path). It is never scanned. It is the evidence
	// for ONE decision — whether a line of the field's added-lines Text is the brief-v2
	// frontmatter `id:` line the session-id arm exempts (#2022, briefIDExemptLine). A file
	// with no entry gets no exemption, so a site that omits it is stricter, never looser;
	// TestFileKindSitesCarrySources pins that every kind-`file` site supplies it, so the
	// false refusal cannot come back through a new site.
	FileSources map[string]string
}

// OutboundContext is the per-process state the check needs but a Forge method's arguments
// do not carry: which tool and verb is writing, and the operator's override reason, if any.
type OutboundContext struct {
	Tool           string
	Verb           string
	OverrideReason string
}

var (
	outboundMu      sync.Mutex
	outboundCtx     OutboundContext
	outboundLogged  = map[string]bool{}
	outboundNoticed = map[string]bool{}
	// outboundNotices is where NOTICE and scan-override RECORDED lines go (stderr: desks run
	// silent on stdout).
	outboundNotices io.Writer = os.Stderr
)

// SetOutboundContext records which tool and verb this process is running and the operator's
// --force-scan-override reason ("" for none). Every verb calls it once its flags are parsed
// (RegisterOutboundOverride does so); a process that never calls it checks with no override
// and names itself after its own binary.
//
// It also resets the per-process dedupe of override rows and notices, so a test that runs
// several verb invocations in one process sees each one's own rows and notices.
func SetOutboundContext(c OutboundContext) {
	outboundMu.Lock()
	defer outboundMu.Unlock()
	c.OverrideReason = strings.TrimSpace(c.OverrideReason)
	outboundCtx = c
	outboundLogged = map[string]bool{}
	outboundNoticed = map[string]bool{}
}

func currentOutboundContext() OutboundContext {
	outboundMu.Lock()
	defer outboundMu.Unlock()
	c := outboundCtx
	if c.Tool == "" && len(os.Args) > 0 {
		c.Tool = filepath.Base(os.Args[0])
	}
	return c
}

// SetOutboundNoticeWriter redirects NOTICE output (tests capture it) and returns a restore
// function.
func SetOutboundNoticeWriter(w io.Writer) (restore func()) {
	outboundMu.Lock()
	defer outboundMu.Unlock()
	old := outboundNotices
	outboundNotices = w
	return func() {
		outboundMu.Lock()
		defer outboundMu.Unlock()
		outboundNotices = old
	}
}

// OutboundOverrideHelp is the usage string every verb gives the override flag.
const OutboundOverrideHelp = "override an outbound-check refusal (credential, personal-data or " +
	"self-containment), stating why in at least 12 characters; writes an audit row holding the " +
	"rule id and a digest of the content. A withheld-identifier or ruling-claim refusal is NOT overridable"

// flagStringRegistrar is the one flag-set method RegisterOutboundOverride needs;
// *flag.FlagSet satisfies it.
type flagStringRegistrar interface {
	String(name, value, usage string) *string
}

// RegisterOutboundOverride registers --force-scan-override on fs and returns the function
// the verb calls after fs.Parse: it validates a supplied reason (exit 5 on a malformed one)
// and records the process's OutboundContext. The apply function ALWAYS sets the context,
// with an empty reason when the flag was not given, so no earlier invocation's reason can
// carry over.
func RegisterOutboundOverride(fs flagStringRegistrar, tool, verb string) (apply func() error, reason *string) {
	r := fs.String(ScanOverrideFlag, "", OutboundOverrideHelp)
	return func() error {
		if strings.TrimSpace(*r) != "" {
			if err := ValidateScanOverride(*r); err != nil {
				return err
			}
		}
		SetOutboundContext(OutboundContext{Tool: tool, Verb: verb, OverrideReason: *r})
		return nil
	}, r
}

// outboundFinding is one refusing or noticing match.
type outboundFinding struct {
	rule   string
	field  string
	line   int
	detail string
	// digest is the SHA-256 of the field's text — what the override row records.
	digest string
	// finding carries the credential scan's redacted finding for --explain callers.
	finding *ScanFinding
}

func (f outboundFinding) overridable() bool {
	return f.rule != RuleVoiceRulingClaim && f.rule != RuleWithheldIdentifier
}

// outboundPublicLayers reports whether the public layers (self-containment, withheld
// register) run for repo. It is SelfContainApplies, named here so the one decision the
// outbound check takes on visibility has one home.
func outboundPublicLayers(repo string) bool { return SelfContainApplies(repo) }

// OutboundCheck runs every layer over w and returns nil (the write may proceed), or a
// Refused error (exit 5) naming the rule, the field and the line, or an Unverifiable error
// when an override's audit row could not be written.
func OutboundCheck(w OutboundWrite) error {
	ctx := currentOutboundContext()
	if w.Tool == "" {
		w.Tool = ctx.Tool
	}
	if w.Verb == "" {
		w.Verb = ctx.Verb
	}
	w.Visibility = RepoVisibility(w.Repo)
	public := outboundPublicLayers(w.Repo)

	var refusals []outboundFinding
	var notices []string
	for _, fd := range w.Fields {
		if fd.Text == "" {
			continue
		}
		r, n := outboundScanField(w, fd, public)
		refusals = append(refusals, r...)
		notices = append(notices, n...)
	}
	emitOutboundNotices(notices)
	if len(refusals) == 0 {
		return nil
	}

	for _, r := range refusals {
		if !r.overridable() {
			msg := outboundRefusal(w, r)
			if ctx.OverrideReason != "" {
				msg += " --" + ScanOverrideFlag + " does not apply to " + r.rule + ": " +
					nonOverridableWhy(r.rule)
			}
			return RefusedFinding(msg, r.finding)
		}
	}
	if ctx.OverrideReason == "" {
		return RefusedFinding(outboundRefusal(w, refusals[0])+OverrideHint(), refusals[0].finding)
	}
	for _, r := range refusals {
		if err := logOutboundOverride(w, r, ctx.OverrideReason); err != nil {
			return err
		}
	}
	return nil
}

func nonOverridableWhy(rule string) string {
	if rule == RuleWithheldIdentifier {
		return "a withheld identifier on a public or unknown-visibility target is reworded, or a " +
			"human changes the configured " + EnvWithheldIdentifiers + " set; a written reason does not publish one."
	}
	return "that refusal is about WHOSE VOICE the text is in, and rewording it is always available."
}

// outboundRefusal renders the refusal message for one finding.
func outboundRefusal(w OutboundWrite, r outboundFinding) string {
	return fmt.Sprintf("refused: %s at %s:%d — %s (%s write to %s, visibility %s).",
		r.rule, r.field, r.line, r.detail, w.Kind, w.Repo, w.Visibility)
}

// outboundScanField runs every layer over one field.
func outboundScanField(w OutboundWrite, fd OutboundField, public bool) (refusals []outboundFinding, notices []string) {
	text := StripWorkpadMarkerLine(fd.Text)
	surface := strings.TrimSpace(w.Kind + " " + fd.Name)
	digest := Sha256Hex([]byte(fd.Text))
	add := func(rule string, line int, detail string, f *ScanFinding) {
		if line < 1 {
			line = 1
		}
		refusals = append(refusals, outboundFinding{
			rule: rule, field: fd.Name, line: line, detail: detail, digest: digest, finding: f,
		})
	}

	// secret.* — the credential scan, unchanged (every arm, same order, same messages).
	if err := ScanSurfaceSecrets(surface, []byte(text)); err != nil {
		var f *ScanFinding
		rule, line := RuleSecretPrefix+"unclassified", 1
		if errors.As(err, &f) && f != nil {
			rule, line = RuleSecretPrefix+f.Rule, f.Line
		}
		add(rule, line, trimRefusedPrefix(err.Error()), f)
	}
	// voice.ruling-claim — the impersonation guard, unchanged.
	if err := ScanSurfaceRulingClaim(surface, []byte(text)); err != nil {
		add(RuleVoiceRulingClaim, rulingClaimLine(surface, text), trimRefusedPrefix(err.Error()), nil)
	}
	// pii.* — the personal-data pass.
	for _, p := range personalDataScan(text) {
		switch p.rule {
		case RulePIIPhoneAmbiguous:
			notices = append(notices, fmt.Sprintf("%s at %s:%d — %q is shaped like a telephone "+
				"number with no country prefix; it is not refused (an id or a count can look the same), "+
				"but check it is not one", p.rule, fd.Name, p.line, p.span))
		case RulePIIEmail:
			add(p.rule, p.line, fmt.Sprintf("%s contains the e-mail address %q, which is outside the "+
				"allow-list (forge no-reply addresses, the reserved example domains and the roster's "+
				"own bot addresses)", surface, p.span), nil)
		default:
			add(p.rule, p.line, fmt.Sprintf("%s contains %q, a telephone number in international shape",
				surface, p.span), nil)
		}
	}
	// selfcontain.* and withheld.identifier — public and unknown-visibility targets only.
	if public {
		opts := SelfContainOpts{Repo: w.Repo, NumberHint: w.NumberHint}
		if w.Kind == OutboundKindFile {
			// A file write's content and path fields get the synthetic-fixture exemption
			// (#2217), with or without a FileSources entry; its commit message does not.
			opts.InFile = fd.Name != OutboundFieldCommitMessage
			if src, ok := w.FileSources[fd.Name]; ok {
				opts.FilePath, opts.FileSource = fd.Name, src
			}
		}
		findings, scNotices := selfContainFindings(surface, text, opts)
		for _, n := range scNotices {
			// A category that could not run is a property of the configuration, not of this
			// field: drop the surface prefix so the per-context dedupe prints it ONCE.
			if strings.Contains(n, "NOT CHECKED") {
				n = strings.TrimPrefix(n, surface+": ")
			}
			notices = append(notices, n)
		}
		for _, f := range findings {
			rule := RuleSelfContainPrefix + strings.ReplaceAll(f.category, " ", "-")
			if f.withheld {
				rule = RuleWithheldIdentifier
			}
			add(rule, f.line, fmt.Sprintf("%s names %s %q, which %s", surface, f.category, f.span, f.why), nil)
		}
	}
	return refusals, notices
}

// rulingClaimLine finds the first line that trips the impersonation guard on its own, for
// the refusal's location. A claim the guard only sees across lines reports line 1.
func rulingClaimLine(surface, text string) int {
	for i, ln := range strings.Split(text, "\n") {
		if ScanSurfaceRulingClaim(surface, []byte(ln)) != nil {
			return i + 1
		}
	}
	return 1
}

func trimRefusedPrefix(msg string) string {
	return strings.TrimPrefix(msg, "refused: ")
}

// OutboundSecretRun reports the first credential-shaped run the check's secret.* layer would
// refuse in text, or nil. It decides nothing and records nothing: it is for a caller that
// NAMES text it is not publishing (deskevidence's pre-existing-content notice), so that
// notice reads the same layer the refusal does.
func OutboundSecretRun(text []byte) *ScanFinding {
	var f *ScanFinding
	if err := ScanSurfaceSecrets("target", text); err != nil && errors.As(err, &f) {
		return f
	}
	return nil
}

// emitOutboundNotices writes each notice once per OutboundContext.
func emitOutboundNotices(notices []string) {
	if len(notices) == 0 {
		return
	}
	outboundMu.Lock()
	defer outboundMu.Unlock()
	for _, n := range notices {
		if outboundNoticed[n] {
			continue
		}
		outboundNoticed[n] = true
		fmt.Fprintf(outboundNotices, "NOTICE: %s\n", n)
	}
}

// logOutboundOverride writes the audit row for one overridden finding: the rule id, the
// kind and field NAME, and a DIGEST of the field's text — never the text, never the span,
// never the refusal message (which quotes the span). It is fatal when the row cannot be
// written: an override whose row did not land is an unlogged bypass. One row per rule and
// digest per OutboundContext, so a write that is checked twice (a pre-flight and the seam)
// is recorded once.
func logOutboundOverride(w OutboundWrite, r outboundFinding, reason string) error {
	key := r.rule + "\x00" + w.Kind + "\x00" + r.field + "\x00" + r.digest
	outboundMu.Lock()
	done := outboundLogged[key]
	outboundMu.Unlock()
	if done {
		return nil
	}
	detail := fmt.Sprintf("outbound override: rule=%s kind=%s field=%s visibility=%s by %s: %s",
		r.rule, w.Kind, r.field, w.Visibility, OverrideIdentity(), reason)
	if err := Log(Entry{
		Tool:       w.Tool,
		Verb:       ScanOverrideVerb,
		Result:     ResultOK,
		Detail:     StripControl(detail),
		Repo:       w.Repo,
		BodyDigest: r.digest,
		ArgsDigest: ArgsDigest(os.Args[1:]),
	}); err != nil {
		return Unverifiable("cannot record the scan-override audit row — refusing rather "+
			"than taking an UNLOGGED bypass", err)
	}
	outboundMu.Lock()
	outboundLogged[key] = true
	out := outboundNotices
	outboundMu.Unlock()
	fmt.Fprintf(out,
		"scan-override RECORDED: tool=%s verb=%s rule=%s kind=%s field=%s digest=%s identity=%s\n",
		w.Tool, w.Verb, r.rule, w.Kind, r.field, r.digest[:12], OverrideIdentity())
	return nil
}
