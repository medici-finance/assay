package main

// ci.go — the CI workflow-token transport's gate, record and refusals (forge-neutral brief 34).
//
// The transport is an explicit, CI-only, read-only opt-in BESIDE the App custody default:
// --ci-workflow-token is the only switch, the token comes from DESKREAD_CI_WORKFLOW_TOKEN and
// nowhere else, the kinds are the closed ciTransportKinds set, and only the job's own repository
// is read. Every refusal below happens before any forge is resolved, so a refused run makes zero
// network calls, and each carries a fixed [ci-transport:<layer>] tag so a test (and an operator)
// can tell WHICH layer refused. CI is detected from environment variables, which a forger can set:
// the layers are guards against honest misuse and against a wider blast radius, not a proof of CI.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	// ciWorkflowTokenFlag is the opt-in. No environment variable, config key or roster entry
	// turns the transport on.
	ciWorkflowTokenFlag = "--ci-workflow-token"
	// ciTokenEnv is the one variable the token is read from. GH_TOKEN and GITHUB_TOKEN are never
	// read as a substitute.
	ciTokenEnv = "DESKREAD_CI_WORKFLOW_TOKEN"

	transportCustody = "app-custody"
	transportCI      = "ci-workflow-token"
)

// The refusal layer tags, fixed per refusal.
const (
	layerNoToken    = "no-token"
	layerOutsideCI  = "outside-ci"
	layerEvent      = "event"
	layerTokenShape = "token-shape"
	layerKind       = "kind"
	layerRepository = "repository"
)

// ciEnv is the environment the CI transport reads, captured once by readCIEnv.
type ciEnv struct {
	token      string
	repository string
	runID      string
	event      string
}

// ciTransport is a run's settled CI identity. token is held for the forge constructor only and is
// never rendered anywhere; repository and runID are copied from the environment and unverified.
type ciTransport struct {
	token      string
	repository string
	runID      string
}

// readCIEnv is the ONLY function that reads the dedicated token variable. It returns the values;
// every decision is made on the returned struct, so the gate is a pure function of it.
func readCIEnv() ciEnv {
	return ciEnv{
		token:      os.Getenv(ciTokenEnv),
		repository: strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")),
		runID:      strings.TrimSpace(os.Getenv("GITHUB_RUN_ID")),
		event:      strings.TrimSpace(os.Getenv("GITHUB_EVENT_NAME")),
	}
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ciGate applies refusals 1-5 (the ones that need no address): the transport it returns is nil
// with a layer and reason when any refuses. Order: no-token, outside-ci, event, token-shape, kind.
func ciGate(kind string, ce ciEnv) (tr *ciTransport, layer, reason string) {
	if ce.token == "" {
		return nil, layerNoToken, ciTokenEnv + " is empty or unset; the CI transport takes its token from that variable only (never GH_TOKEN or GITHUB_TOKEN)"
	}
	if !deskkit.InCI() || !allDigits(ce.runID) || !validRepoSlug(ce.repository) {
		return nil, layerOutsideCI, "not inside a CI job: CI is detected from GITHUB_ACTIONS=true, a numeric GITHUB_RUN_ID and an owner/name GITHUB_REPOSITORY, and those variables are not proof of a CI job"
	}
	if ce.event == "pull_request_target" {
		return nil, layerEvent, "the job runs on pull_request_target, on behalf of a fork; the token is not handed to this binary there"
	}
	if !strings.HasPrefix(ce.token, "ghs_") {
		return nil, layerTokenShape, "not an app installation token (a personal, OAuth or user-to-server token is never accepted)"
	}
	if !ciTransportKinds[kind] {
		return nil, layerKind, fmt.Sprintf("the %q kind is not served over the CI workflow-token transport (served: %s)", kind, strings.Join(sortedCIKinds(), ", "))
	}
	return &ciTransport{token: ce.token, repository: ce.repository, runID: ce.runID}, "", ""
}

func sortedCIKinds() []string {
	var out []string
	for _, k := range sortedKinds() {
		if ciTransportKinds[k] {
			out = append(out, k)
		}
	}
	return out
}

// ciGateTargets is refusal 6 for the per-item kinds: an --issue or --change that names another
// repository is refused outright. (A --repo that names another repository is not a refusal: it
// lands in "partial" without the token ever being sent, see ciForgeFor.)
func ciGateTargets(kind string, o readOpts) (layer, reason string) {
	if repoKinds[kind] {
		return "", ""
	}
	for _, it := range o.items {
		if !strings.EqualFold(it.Repo, o.ciT.repository) {
			return layerRepository, fmt.Sprintf("the CI workflow-token transport reads only the job's own repository (%s); %s#%d is another repository", o.ciT.repository, it.Repo, it.Number)
		}
	}
	return "", ""
}

func ciRefuse(stderr io.Writer, layer, reason string) int {
	fmt.Fprintf(stderr, "deskread: refused [ci-transport:%s]: %s\n", layer, reason)
	return deskkit.ExitRefused
}

// IdentityJSON is the additive identity record on the envelope. Custody carries the role; the CI
// transport carries the repository and run id copied from the environment (unverified). It has no
// field a credential could occupy.
type IdentityJSON struct {
	Transport  string `json:"transport"`
	Role       string `json:"role,omitempty"`
	Repository string `json:"repository,omitempty"`
	RunID      string `json:"runId,omitempty"`
}

func identityFor(o readOpts) *IdentityJSON {
	if o.ciT != nil {
		return &IdentityJSON{Transport: transportCI, Repository: o.ciT.repository, RunID: o.ciT.runID}
	}
	return &IdentityJSON{Transport: transportCustody, Role: custodyRoleSeen()}
}

// writeIdentityLine names the transport on stderr, on every run that reaches the reads. It prints
// the identity record and nothing else, so it cannot carry the token.
func writeIdentityLine(stderr io.Writer, id *IdentityJSON) {
	if id.Transport == transportCI {
		fmt.Fprintf(stderr, "deskread: identity: transport=%s repository=%s runId=%s (copied from the environment, not verified)\n", id.Transport, id.Repository, id.RunID)
		return
	}
	fmt.Fprintf(stderr, "deskread: identity: transport=%s role=%s\n", id.Transport, id.Role)
}
