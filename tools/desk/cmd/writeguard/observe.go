package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/writepolicy"
)

// Config is the hook's observation context. It grants no authority itself.
// SharedOK is retained for the inherited adapter fixtures; run supplies the
// complete claim observation, whose expiry and scope the core evaluates.
type Config struct {
	SharedRoot, ProjectDir, Cwd, CwdTop, SentinelPath string
	SharedOK                                          bool
	Callout                                           string
	CalloutTimeout                                    time.Duration
	claim                                             *writepolicy.ClaimObservation
}
type Verdict struct {
	Block  bool
	Reason string
}
type observationStats struct{ Paths, Callouts int }

// classifiedCall is a tool call with the hook's one classification of it: run
// makes it once for a Bash call in a desk session and hands the same value to
// the ambient-token refusal and to evaluateWith.
type classifiedCall struct {
	tool, target, cwd string
	cl                writepolicy.Classification
}

func classifyCall(tool, target, cwd string) *classifiedCall {
	home, _ := os.UserHomeDir()
	env := map[string]string{}
	for _, entry := range os.Environ() {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			env[k] = v
		}
	}
	platform := writepolicy.POSIX
	if runtime.GOOS == "windows" {
		platform = writepolicy.Windows
	}
	return &classifiedCall{tool: tool, target: target, cwd: cwd,
		cl: writepolicy.Classify(writepolicy.Request{Tool: tool, Target: target}, writepolicy.LexicalContext{
			Platform: platform, Cwd: cwd, Home: home, Env: env})}
}

func (c Config) evaluate(tool, target string) (Verdict, writepolicy.Outcome, observationStats) {
	return c.evaluateWith(nil, tool, target)
}

// evaluateWith evaluates call's classification when it is of this tool call
// (same tool, target and cwd), and classifies the call itself otherwise.
func (c Config) evaluateWith(call *classifiedCall, tool, target string) (Verdict, writepolicy.Outcome, observationStats) {
	if call == nil || call.tool != tool || call.target != target || call.cwd != c.Cwd {
		call = classifyCall(tool, target, c.Cwd)
	}
	cl := call.cl
	claim := writepolicy.ClaimObservation{Source: writepolicy.NoClaim}
	if c.claim != nil {
		claim = *c.claim
	} else if c.SharedOK {
		claim.Source = writepolicy.EnvironmentClaim
		claim.Token = "fixture"
	}
	state := writepolicy.CalloutUnconfigured
	if c.Callout != "" {
		state = writepolicy.CalloutPending
	}
	in := writepolicy.Evaluation{Classified: cl, ClassificationID: cl.ID(),
		Policy: writepolicy.Policy{Profile: writepolicy.LegacyHook, SharedRoot: c.SharedRoot,
			ProjectDir: c.ProjectDir, CwdTop: c.CwdTop, SentinelPath: c.SentinelPath},
		Claim: claim, Callout: writepolicy.CalloutObservation{State: state}}
	var stats observationStats
	var calloutReason string
	for {
		out := writepolicy.Evaluate(in)
		if len(out.Need.Paths) > 0 {
			for _, req := range out.Need.Paths {
				in.Paths = append(in.Paths, observePath(req.Input))
				stats.Paths++
			}
			continue
		}
		if out.Need.Callout {
			stats.Callouts++
			reason, blocked := c.checkCallout(target)
			in.Callout.State = writepolicy.CalloutAllow
			if blocked {
				in.Callout.State = writepolicy.CalloutRefuse
				calloutReason = reason
			}
			continue
		}
		if out.Need.ClaimBody {
			in.Claim.BodyPending = false
			p, err := sentinelPath()
			if err == nil {
				b, err := os.ReadFile(p)
				in.Claim.Readable = err == nil
				in.Claim.Body = string(b)
			}
			continue
		}
		return c.renderOutcome(out, calloutReason), out, stats
	}
}

// renderOutcome maps only a terminal core result. Observation requests are
// satisfied above; an unresolved result cannot authorize the hook call. Legacy
// malformed-input and Git-discovery no-decision exits remain in run, before this
// evaluator is called, and are not core Unknown outcomes.
func (c Config) renderOutcome(out writepolicy.Outcome, calloutReason string) Verdict {
	switch out.Decision {
	case writepolicy.Allow:
		return Verdict{}
	case writepolicy.Deny:
		switch out.Reason {
		case writepolicy.SentinelDenied:
			return c.sentinelBlock(out.Detail.Tool, out.Detail.Target)
		case writepolicy.SharedDenied:
			return c.renderBlock(out.Detail)
		case writepolicy.CalloutDenied:
			return Verdict{Block: true, Reason: calloutReason}
		default:
			return Verdict{Block: true, Reason: "writeguard: BLOCKED — unknown refusal code"}
		}
	default:
		return Verdict{Block: true, Reason: "writeguard: BLOCKED — policy evaluation is unresolved (" + string(out.Reason) + ")"}
	}
}

// observePath performs the legacy deepest-existing-ancestor algorithm. A failed
// observation remains explicit; only the named legacy core profile may apply its
// lexical fallback. Paths depending on an observed cwd are requested next.
func observePath(p string) writepolicy.PathObservation {
	p = filepath.Clean(p)
	rest, cur := "", p
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return writepolicy.PathObservation{Input: p, State: writepolicy.PathResolved,
				Resolved: filepath.Join(r, rest), Ancestor: cur, ResolvedAncestor: r, Suffix: rest}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return writepolicy.PathObservation{Input: p, State: writepolicy.PathFailed}
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// observeClaim collects evidence; expiry, scope and exemption are core policy.
func observeClaim() writepolicy.ClaimObservation {
	if token := os.Getenv("WRITEGUARD_SHARED_OK"); token != "" {
		return writepolicy.ClaimObservation{Source: writepolicy.EnvironmentClaim, Token: token}
	}
	o := writepolicy.ClaimObservation{Source: writepolicy.SentinelClaim, TTL: int64(sentinelTTL())}
	p, err := sentinelPath()
	if err != nil {
		return o
	}
	fi, err := os.Stat(p)
	if err != nil {
		return o
	}
	o.Directory = fi.IsDir()
	o.ModifiedAt = fi.ModTime().UnixNano()
	o.ObservedAt = time.Now().UnixNano()
	o.BodyPending = true
	return o
}
