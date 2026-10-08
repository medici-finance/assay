package deskkit

// outboundcallout.go — the HOUSE CALLOUT of the outbound-write check (desktools-v2/11).
//
// WHY. The compiled outbound check (outbound.go) is generic on purpose: it cannot know one
// deployment's withheld names, and compiling them in would make the shipped binary the
// disclosure. A deployment whose rules are a token map or a sweep over its own register
// supplies an EXECUTABLE and the tool asks it — the same seam as the write guard's callout
// (cmd/writeguard/callout.go), on the same plumbing (callout.go). None of the deployment's
// vocabulary ships with, or is written by, these tools.
//
// THE RULES, each stated at the code that holds it:
//
//	ONLY-WIDENS    OutboundCheck consults the callout AFTER the compiled layer returned nil
//	               (outboundHouseCheck is the last statement of OutboundCheck, reached only on
//	               a nil compiled verdict). There is no value the callout can print that
//	               reaches a compiled verdict, so `allow` needs no defending.
//	UNSET          key unset => nothing here runs, byte for byte the compiled layer's
//	               behaviour — publishing this must not block a deployment that has adopted none.
//	FAIL-CLOSED    key SET and anything wrong — a malformed value, a missing file, a bad mode,
//	               a non-zero exit, a timeout, no output, oversize output, an unknown first
//	               token — REFUSES the write, naming WHICH failure, so "my callout said no" and
//	               "my callout is broken" read differently. A configured callout is never skipped.
//	REQUIRED       ASSAY_OUTBOUND_CALLOUT_REQUIRED=public refuses a write to a public or
//	               unknown-visibility target when NO callout is configured. Also honoured from the
//	               process environment, because a roster that fails validation collapses to
//	               unconfigured and would take the key with it.
//	NO LEAK        the callout's reason may name a withheld token. It is printed to stderr and
//	               nowhere else: the returned error is generic (verbs log err.Error() into the
//	               audit trail), and the audit row carries the rule id, the field NAME and the
//	               content digest — never the reason, never the text. Nothing composes a forge
//	               write from a refusal.
//	NO CREDENTIAL  the callout runs with an explicit environment — PATH, HOME, TMPDIR, LANG —
//	               never the caller's: the caller may hold a minted forge token and the
//	               executable is third-party to the tools.
//
// A house.callout refusal is NEVER overridable: a block follows the withheld-identifier class
// (a written reason does not publish a withheld word), and a broken callout is fixed or unset
// by a human's configuration, not waived per write.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RuleHouseCallout is the rule id of every refusal that comes from the house callout — a
// block, or a callout that is configured and could not answer. Never overridable.
const RuleHouseCallout = "house.callout"

// Bounds on ASSAY_OUTBOUND_CALLOUT_TIMEOUT. The upper bound is the inbound scan's own ceiling
// (untrustscan.go's semgrepTimeout), well under an agent watchdog.
const (
	MinOutboundCalloutTimeout = 1 * time.Second
	MaxOutboundCalloutTimeout = 60 * time.Second
)

// outboundCalloutRequestVersion bumps only when a request field's MEANING changes.
const outboundCalloutRequestVersion = 1

// outboundCalloutRequiredPublic is the one value of ASSAY_OUTBOUND_CALLOUT_REQUIRED.
const outboundCalloutRequiredPublic = "public"

// maxCalloutReasonRunes caps the reason printed to the terminal.
const maxCalloutReasonRunes = 300

// OutboundCalloutSettings is the resolved configuration of the house callout.
type OutboundCalloutSettings struct {
	// Path is the absolute executable path, "" when unset.
	Path string
	// Required is true under REQUIRED=public from the roster OR the process environment.
	Required bool
	// Timeout is the configured bound, zero for the default.
	Timeout time.Duration
	// Problem is non-empty when a SET key was malformed: the write refuses on it.
	Problem string
}

// parseOutboundCalloutKeys lands the three keys on cfg. A malformed SET value never refuses
// the roster and never degrades to "unconfigured": the first problem is carried on
// cfg.OutboundCalloutProblem and the outbound check refuses on it.
func parseOutboundCalloutKeys(cfg *Config, vals map[string]string) {
	var problem string
	bad := func(format string, a ...any) {
		if problem == "" {
			problem = fmt.Sprintf(format, a...)
		}
	}
	if raw := strings.TrimSpace(vals[EnvOutboundCallout]); raw != "" {
		switch {
		case strings.ContainsAny(raw, ",;\t\n\r"):
			bad("%s=%q contains a separator. It names ONE executable, never a list", EnvOutboundCallout, raw)
		case !filepath.IsAbs(raw):
			bad("%s=%q is not an absolute path. A relative callout path resolves against the "+
				"directory the tool happened to be spawned in — caller-influenced input choosing "+
				"the gate's own policy source", EnvOutboundCallout, raw)
		default:
			cfg.OutboundCallout = raw
		}
	}
	if raw := strings.TrimSpace(vals[EnvOutboundCalloutRequired]); raw != "" {
		if strings.EqualFold(raw, outboundCalloutRequiredPublic) {
			cfg.OutboundCalloutRequired = true
		} else {
			// The strictest reading: an unrecognised requirement still requires.
			cfg.OutboundCalloutRequired = true
			bad("%s=%q is not %q. Set it to %q or unset it", EnvOutboundCalloutRequired, raw,
				outboundCalloutRequiredPublic, outboundCalloutRequiredPublic)
		}
	}
	if raw := strings.TrimSpace(vals[EnvOutboundCalloutTimeout]); raw != "" {
		d, err := time.ParseDuration(raw)
		switch {
		case err != nil:
			bad("%s=%q is not a Go duration (for example 5s or 30s)", EnvOutboundCalloutTimeout, raw)
		case d < MinOutboundCalloutTimeout || d > MaxOutboundCalloutTimeout:
			bad("%s=%q is outside %s-%s", EnvOutboundCalloutTimeout, raw,
				MinOutboundCalloutTimeout, MaxOutboundCalloutTimeout)
		default:
			cfg.OutboundCalloutTimeout = d
		}
	}
	cfg.OutboundCalloutProblem = problem
}

// ResolveOutboundCallout returns the house callout's effective configuration: the roster's
// values, with REQUIRED additionally tightened by the process environment. The environment
// can only ADD the requirement — it never supplies the path or the timeout, because a
// write-class tool does not take its policy sources from the environment.
func ResolveOutboundCallout() OutboundCalloutSettings {
	c := EffectiveConfig()
	s := OutboundCalloutSettings{
		Path:     c.OutboundCallout,
		Required: c.OutboundCalloutRequired,
		Timeout:  c.OutboundCalloutTimeout,
		Problem:  c.OutboundCalloutProblem,
	}
	if raw := strings.TrimSpace(os.Getenv(EnvOutboundCalloutRequired)); raw != "" {
		s.Required = true
		if !strings.EqualFold(raw, outboundCalloutRequiredPublic) && s.Problem == "" {
			s.Problem = fmt.Sprintf("%s=%q (environment) is not %q. Set it to %q or unset it",
				EnvOutboundCalloutRequired, raw, outboundCalloutRequiredPublic, outboundCalloutRequiredPublic)
		}
	}
	return s
}

// outboundCalloutEcho renders the callout path for the P3 echo: configured paths VERBATIM
// (a path that cannot be consulted must be loud about which one), the unset state named.
func outboundCalloutEcho(c Config) string {
	switch {
	case c.OutboundCalloutProblem != "" && c.OutboundCallout == "":
		return "(INVALID — outward writes REFUSE: " + c.OutboundCalloutProblem + ")"
	case c.OutboundCallout == "":
		return "(unset — compiled outbound checks only)"
	default:
		return c.OutboundCallout
	}
}

// outboundRequiredEcho renders the required mode for the P3 echo, naming where it came from.
func outboundRequiredEcho(c Config) string {
	env := strings.TrimSpace(os.Getenv(EnvOutboundCalloutRequired)) != ""
	switch {
	case c.OutboundCalloutRequired && env:
		return outboundCalloutRequiredPublic + " (roster and environment)"
	case c.OutboundCalloutRequired:
		return outboundCalloutRequiredPublic + " (roster)"
	case env:
		return outboundCalloutRequiredPublic + " (environment)"
	}
	return "(unset — a missing callout does not refuse)"
}

// outboundCalloutEnv builds the callout's explicit environment: the four variables an
// executable needs to find its interpreter, its data and a scratch directory, and nothing
// else. It is never nil, which is what makes Callout.Env mean "exactly these".
func outboundCalloutEnv() []string {
	env := make([]string, 0, 4)
	for _, k := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// outboundCalloutRequest is the ONE JSON object the callout reads on stdin. No argv carries
// text.
type outboundCalloutRequest struct {
	Version    int                    `json:"version"`
	Verb       string                 `json:"verb"`
	Role       string                 `json:"role"`
	Repo       string                 `json:"repo"`
	Visibility string                 `json:"visibility"`
	Kind       string                 `json:"kind"`
	Fields     []outboundCalloutField `json:"fields"`
}

type outboundCalloutField struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

func buildOutboundCalloutRequest(w OutboundWrite) ([]byte, error) {
	verb := strings.TrimSpace(w.Tool + " " + w.Verb)
	req := outboundCalloutRequest{
		Version:    outboundCalloutRequestVersion,
		Verb:       verb,
		Role:       w.Role,
		Repo:       w.Repo,
		Visibility: w.Visibility.String(),
		Kind:       w.Kind,
		Fields:     []outboundCalloutField{},
	}
	for _, fd := range w.Fields {
		if fd.Text == "" {
			continue
		}
		req.Fields = append(req.Fields, outboundCalloutField{Name: fd.Name, Text: fd.Text})
	}
	return json.Marshal(req)
}

// outboundHouseCheck is the house callout's consult step. OutboundCheck calls it ONLY after
// the compiled layer has returned nil.
func outboundHouseCheck(w OutboundWrite) error {
	s := ResolveOutboundCallout()
	field := houseRefusalField(w)
	digest := houseWriteDigest(w)

	if s.Problem != "" {
		return houseRefuse(w, field, digest, "broken", "the house callout configuration is malformed: "+s.Problem+
			" Fix or unset it; this refusal is not overridable", "")
	}
	if s.Path == "" {
		if s.Required && RepoVisibility(w.Repo) != VisibilityPrivate {
			return houseRefuse(w, field, digest, "required-absent", EnvOutboundCalloutRequired+"="+
				outboundCalloutRequiredPublic+" but no house callout is configured ("+EnvOutboundCallout+
				" is unset, or the roster that carried it failed validation) — a public or unknown-visibility "+
				"write is refused rather than leaving with the compiled checks alone", "")
		}
		return nil
	}

	payload, err := buildOutboundCalloutRequest(w)
	if err != nil {
		return houseRefuse(w, field, digest, "broken", "the callout request could not be encoded: "+err.Error(), "")
	}
	res, err := Callout{Path: s.Path, Timeout: s.Timeout, Env: outboundCalloutEnv()}.Run(string(payload))
	if err != nil {
		return houseRefuse(w, field, digest, "broken", "the house callout did not answer: "+err.Error(), res.Stderr)
	}
	if res.Truncated {
		return houseRefuse(w, field, digest, "broken", fmt.Sprintf("the house callout printed more than %d bytes — "+
			"output that does not fit one answer line is not an answer", maxCalloutOutput), res.Stderr)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return houseRefuse(w, field, digest, "broken", "the house callout printed nothing — it must print "+
			"`allow` or `block <reason>`", res.Stderr)
	}
	verdict, rest, _ := strings.Cut(out, " ")
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "allow":
		return nil
	case "block":
		return houseRefuse(w, field, digest, "block", "", strings.TrimSpace(rest))
	default:
		return houseRefuse(w, field, digest, "broken", fmt.Sprintf("the house callout printed %q, which is "+
			"neither `allow` nor `block`", clipForTerminal(houseFirstLine(out), 80)), res.Stderr)
	}
}

// houseRefuse is the ONE exit of every house.callout refusal. kind is "block" (the callout
// said no), "broken" (it could not answer or is misconfigured) or "required-absent". detail
// is OUR message (generic text, a path, a failure class — never the callout's words);
// reason is the CALLOUT's words and goes to stderr only.
func houseRefuse(w OutboundWrite, field, digest, kind, detail, reason string) error {
	if kind == "block" {
		why := clipForTerminal(reason, maxCalloutReasonRunes)
		if why == "" {
			why = "(the callout gave no reason)"
		}
		fmt.Fprintf(outboundWriter(), "refused: %s at %s — %s\n", RuleHouseCallout, field, why)
		detail = "blocked by the configured house callout; its reason was printed to stderr and is " +
			"deliberately not logged or sent to the forge. Reword the write or change the callout's rules"
	} else if reason != "" {
		// The callout's own diagnostic helps its owner; it is stderr-only like a block reason.
		fmt.Fprintf(outboundWriter(), "house callout stderr: %s\n", clipForTerminal(reason, maxCalloutReasonRunes))
	}
	// The audit row: rule id, outcome class, kind, field NAME and the content digest.
	_ = Log(Entry{
		Tool:       w.Tool,
		Verb:       "outbound_callout",
		Result:     ResultRefused,
		Detail:     StripControl(fmt.Sprintf("outbound callout: rule=%s outcome=%s kind=%s field=%s visibility=%s", RuleHouseCallout, kind, w.Kind, field, w.Visibility)),
		Repo:       w.Repo,
		BodyDigest: digest,
		ArgsDigest: ArgsDigest(os.Args[1:]),
	})
	return Refused(fmt.Sprintf("refused: %s at %s — %s (%s write to %s, visibility %s). "+
		"%s is not overridable: --%s does not apply.",
		RuleHouseCallout, field, detail, w.Kind, w.Repo, w.Visibility, RuleHouseCallout, ScanOverrideFlag))
}

// houseRefusalField names the field a house refusal points at: the write's only non-empty
// field, else `(write)` — the callout decides on the whole write, not on one field.
func houseRefusalField(w OutboundWrite) string {
	name, n := "", 0
	for _, fd := range w.Fields {
		if fd.Text != "" {
			name = fd.Name
			n++
		}
	}
	if n == 1 {
		return name
	}
	return "(write)"
}

// houseWriteDigest is the SHA-256 of every field's text in order — the content digest the
// audit row records instead of the text.
func houseWriteDigest(w OutboundWrite) string {
	var b strings.Builder
	for _, fd := range w.Fields {
		if fd.Text == "" {
			continue
		}
		b.WriteString(fd.Name)
		b.WriteByte(0)
		b.WriteString(fd.Text)
		b.WriteByte(0)
	}
	return Sha256Hex([]byte(b.String()))
}

func houseFirstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// clipForTerminal strips control characters from text a third party produced and bounds its
// length: it is printed to an operator's terminal and may be replayed into agent context.
func clipForTerminal(s string, maxRunes int) string {
	s = strings.TrimSpace(StripControl(s))
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
