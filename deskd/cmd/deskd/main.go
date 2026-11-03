// Command deskd polls a cell's repos, derives an index of escalations, and
// serves it over HTTP: a read-only surface plus the LOCAL-ONLY
// read/ack write endpoints (ruling D7) that write deskd's own SQLite and
// nothing on the platform.
//
// It binds localhost by DEFAULT and REFUSES to bind anything else unless
// caller-token validation is switched on (--auth=bearer, the service contract P3):
// "deployment is config" (design §4) never meant "an unauthenticated surface on
// a network address is one flag away".
//
// It holds NO human credential. Under the ruled custody model (b) (#1819) the
// person supplies their credential WITH a named action and the service drops it
// when the request returns; the only human-authored writes it can perform are
// the enumerated named actions {answer, ack, send, session} declared in
// internal/contract (send/session carry the viewer identity, not a credential).
//
// Exit codes:
//
//	0  clean shutdown
//	1  refused to start — bad config, unopenable index, no read token
//	2  could not run — the binary itself is broken
//
// deskd is deliberately NOT on the work path. It being down means the inbox
// is stale and notifications are late; it never means work is lost or a loop
// is blocked, because GitHub is the record and deskd only reads it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/custody"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/fleet"
	"github.com/medici-finance/assay/deskd/internal/httpsecurity"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/mcp"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
	"github.com/medici-finance/assay/deskd/web"
)

// The PER-USER OAuth client, DISTINCT from the operator device-flow
// client. A viewer authenticates through this client; the answer path still
// authors with the operator credential. When these are unset, deskd serves but no
// viewer can sign in — a private-repo cell then shows only the login prompt.
const (
	oauthClientEnv   = "DESKD_OAUTH_CLIENT_ID"
	oauthSecretEnv   = "DESKD_OAUTH_SECRET"
	oauthRedirectEnv = "DESKD_OAUTH_REDIRECT_URL" // e.g. http://127.0.0.1:8787/auth/callback
)

// tokenEnv names the environment variable holding a READ-SCOPED GitHub token.
// It is now the DEFAULT token — used for any org that has no org-specific
// DESKD_GITHUB_TOKEN_<ORG> override (see adapter.ResolveTokens). It is read
// from the environment and never written anywhere: not to the index, not to a
// log line, not to a config file.
const tokenEnv = "DESKD_GITHUB_TOKEN"

// writeTokenEnv names the environment variable holding the WRITE-SCOPED App
// token desk__escalate / desk__report_file file as (assay-desk-app[bot],
// mcp.AppIdentity). It is SEPARATE from the read token by design: the read
// surface's transport structurally cannot write, and the MCP write path stands
// beside it with its own client (the service contract). When it is unset, the MCP
// surface still serves — its read tools work and its mutating tools return an
// honest could-not-check rather than a silent no-op.
const writeTokenEnv = "DESKD_DESK_APP_TOKEN"

// recordTokenEnv / recordAppEnv name the WRITE-ISSUES App identity the
// cell-override record PR is filed under (the service contract). It is SEPARATE
// from the desk App write token: the record PR is a config-repo write by a
// dedicated cross-repo issue-writing identity (the per-cell write App), and
// if it is unset the override table is still written but the record PR is
// could-not-check — the filer never borrows the read App or a person's token.
const (
	recordTokenEnv = "DESKD_WRITE_ISSUES_TOKEN"
	recordAppEnv   = "DESKD_WRITE_ISSUES_APP"
	// overrideConfigRepoEnv names the owner/name repo the cell's cells.yaml
	// lives in — the record PR's target. Empty leaves the record PR unable to
	// resolve a target (could-not-check). A deployment sets it to the repository
	// that holds its cells.yaml.
	overrideConfigRepoEnv = "DESKD_OVERRIDES_CONFIG_REPO"
)

// apiUnreachableEnv, when set, makes deskd treat EVERY repo as could-not-check
// without opening a socket — the offline envelope for a dev/CI/verify run that
// must not contact GitHub (the offline-envelope rule, and the service contract
// Verify row 11 runs a bounded `--once` under it). It is a test/dev knob: the
// poll returns a degraded (could-not-check) cycle exactly as a real network
// failure would, so the startup path — including the terminal-registry banner —
// runs without any live infrastructure.
const apiUnreachableEnv = "DESKD_API_UNREACHABLE"

// webMCPDevEnv gates the read-only WebMCP diagnostics experiment
// (the service contract): three page-callable, read-only tools on the inbox view
// only, driven from a live Chrome DevTools MCP bridge in a developer session.
// It is OFF by default and MUST stay off in any deployed config — set it only
// on a local dev instance. It is never a CI rung (WebMCP has no headless
// support) and never a production dependency (single-vendor origin trial).
const webMCPDevEnv = "DESKD_WEBMCP_DEV"

// terminalAttachUsage is the --terminal-attach help. It states the input path
// an attach arms (DR-deskd-attach-start leaves `writable` to the service contract),
// because an operator decides whether to leave `configured` on from this text.
const terminalAttachUsage = "deskd's own terminal attach lifecycle (the service contract): " +
	"configured = attach every desk cells.yaml names at start and re-attach with bounded backoff. " +
	"The lifecycle itself writes nothing, but an attached desk accepts send from its cell's answerer " +
	"(the service contract) with no session open; a local desk's shell runs as deskd's own OS user. " +
	"Cluster desks read unauthorized — deskd does not link the pod driver. " +
	"off = attach nothing and open no session, so no desk accepts send (off disables both)"

func main() {
	var (
		humanLogin = flag.String("human-login", "", "accountable human GitHub login; absent disables human operations (no credential is stored)")
		configPath = flag.String("config", "config/cells.yaml", "cells.yaml path")
		idxPath    = flag.String("index", "deskd-index.db", "SQLite index path")
		addr       = flag.String("addr", "127.0.0.1:8787", "listen address (localhost by default)")
		interval   = flag.Duration("interval", index.DefaultInterval, "poll interval (floor 60s applies to derived deadlines)")
		apiBase    = flag.String("api", "https://api.github.com", "GitHub API base")
		once       = flag.Bool("once", false, "run a single poll cycle, print its result as JSON, and exit")
		rebuild    = flag.Bool("rebuild-derived", false, "drop the derived cache before polling; the local read/ack table is untouched")
		authMode   = flag.String("auth", string(identity.AuthLoopback), "how callers are authenticated: loopback (the machine boundary; the only mode a loopback --addr needs) or bearer (validate the /mcp caller's Authorization: Bearer token against the viewer OAuth App; /v1 and the data routes stay gated by the viewer session, not the bearer — contract doc §10.2). A non-loopback --addr REQUIRES bearer")
		attachFlag = flag.String("terminal-attach", string(terminal.AttachModeConfigured), terminalAttachUsage)
		tokensDir  = flag.String("tokens-dir", "", "directory holding one per-org read-token file (the service contract), re-read at the top of EVERY poll cycle and on every permission-querier and fleet-map read, so a rotated token needs no restart. One file per owner, named <ORG> (uppercase, '-'->'_' — the same suffix as DESKD_GITHUB_TOKEN_<ORG>). Absent (the default) keeps the env source (DESKD_GITHUB_TOKEN[_<ORG>]), unchanged")
		intentKeys = flag.String("intent-keys", intentkeys.DefaultPath, "the reviewed intent-keys file (the authorization contract): the ONLY source of trusted ed25519 keys for signed intents on /v1. Missing, empty or invalid ⇒ no key trusted ⇒ every signed intent refused")
	)
	flag.Parse()
	if err := humanid.Configure(*humanLogin); err != nil {
		fatal(1, "%v", err)
	}

	// ── P3: the bind gate, before anything listens ────────────────────────
	//
	// The refusal happens here — ahead of the config load, the index open and
	// every goroutine — because the failure it prevents is "an unauthenticated
	// surface reachable from the network", and a check that runs after the
	// server is built is a check that can be skipped by an early return.
	mode, err := identity.ParseAuthMode(*authMode)
	if err != nil {
		fatal(1, "%v", err)
	}
	validator := identity.NewValidator(os.Getenv(identity.ViewerClientIDEnv), os.Getenv(identity.ViewerClientSecretEnv), *apiBase)
	if err := identity.CheckBind(*addr, mode, validator != nil); err != nil {
		fatal(1, "%v", err)
	}

	cfg, err := schema.Load(*configPath)
	if err != nil {
		fatal(1, "config %s did not load: %v", *configPath, err)
	}

	db, err := index.Open(*idxPath)
	if err != nil {
		fatal(1, "index %s did not open: %v", *idxPath, err)
	}
	defer db.Close()

	if *rebuild {
		if err := db.DropDerived(); err != nil {
			fatal(1, "could not drop the derived cache: %v", err)
		}
		fmt.Fprintln(os.Stderr, "deskd: derived cache dropped; local read/ack table retained (ruling D7)")
	}

	// deskd routes each repo to the read token for its OWNER, not one token for
	// every repo: a GitHub App mints ONE installation token per org, so a cell
	// spanning N orgs needs N tokens even with a dedicated read App
	// (the service contract, /18). Per-org tokens arrive as DESKD_GITHUB_TOKEN_<ORG>;
	// the legacy DESKD_GITHUB_TOKEN (tokenEnv) is the default for any org with no
	// org-specific override, so a single-org cell is configured exactly as before.
	owners := configOwners(cfg)
	// tokenSource is the reload contract this brief adds (the service contract):
	// absent --tokens-dir it is the env source, byte-for-byte today's
	// behaviour (Verify row 6); set, it is the per-org token-FILE directory a
	// sidecar shares with deskd (the service contract), re-read at the top of
	// EVERY poll cycle — see [buildTokenSource] and [adapter.ReloadingRouter].
	// The startup check reads only WHICH orgs are missing a token; no token
	// map is kept here, so no consumer can be built from a start-time
	// snapshot (the authorization contract).
	tokenSource := buildTokenSource(*tokensDir, os.Getenv)
	missing := adapter.MissingTokens(tokenSource, owners)
	if len(missing) > 0 {
		// Refusing to start is the honest move — the same reasoning as the old
		// single-token refusal, now applied per org and per source. Starting
		// with an org unmapped would poll its repos blind and serve an inbox
		// that is empty because it cannot see, which is indistinguishable at a
		// glance from an inbox that is empty because there is nothing to do.
		// We name each org and WHERE to fix it (an env var or a file path); we
		// NEVER log a token value, and there is no fallback to another org's
		// credential.
		var names []string
		for _, m := range missing {
			names = append(names, fmt.Sprintf("%s (%s)", m.Owner, m.Detail))
		}
		fatal(1, "no read token for %d org(s) this cell spans: %s. deskd refuses to start rather than poll an org blind "+
			"(a GitHub App mints one installation token per org, so each org needs its own token; an unmapped org is never "+
			"read with another org's credential).", len(missing), strings.Join(names, "; "))
	}

	// The cockpit terminal registry (the service contract): desk id → target,
	// resolved from cells.yaml. It is built here — before the --once early
	// return and before the HTTP surface — so the startup banner names it in
	// EVERY mode, and one instance is shared by the read surface and the MCP
	// desk__desks tool below (the single source of truth once drivers attach).
	termRegistry := terminal.NewRegistry(cfg)
	attachMode, err := terminal.ParseAttachMode(*attachFlag)
	if err != nil {
		fatal(1, "%v", err)
	}
	fmt.Fprintf(os.Stderr, "deskd: terminal registry: %d desk(s) resolved from cells.yaml (%d with an attachable target:); %s\n",
		termRegistry.Len(), termRegistry.WithTarget(), attachBanner(attachMode, *once))

	// The offline envelope: DESKD_API_UNREACHABLE swaps the GitHub router for a
	// fetcher that reports every repo could-not-check without a socket, so a
	// dev/CI/verify --once run touches no live infrastructure.
	//
	// The escalation poller's fetcher is the RELOADING router (the service contract):
	// index.Poller calls its RefreshCycle at the top of every cycle (via
	// index.CycleRefresher), so it re-resolves tokenSource — the env source or
	// the token-file directory — every time rather than once here at start.
	// The permission querier and the fleet cache below read through the same
	// tokenSource, resolved on every read (the authorization contract), so all three
	// read-token consumers pick up a rotated token with no restart.
	var fetcher index.Fetcher = adapter.NewReloadingRouter(*apiBase, tokenSource, owners, nil)
	if os.Getenv(apiUnreachableEnv) != "" {
		fetcher = unreachableFetcher{}
		fmt.Fprintf(os.Stderr, "deskd: %s set — every repo is could-not-check without a network call (offline envelope)\n", apiUnreachableEnv)
	}

	dir := index.NewDirectory(cfg)

	// The event bus (the service contract increment 3, task 2): the /v1/events
	// ring buffer AND the poller's transition/badge/health producer share this
	// one instance, so an event the poller publishes is exactly what a
	// connected SSE client sees — there is no second queue to drift from it.
	eventBus := serve.NewEventBus()

	// The poll-side answer-record verifier (the authorization contract gap 3, item
	// (5)): every bot-authored answer marker is re-verified against the
	// reviewed intent-keys file and the cell's current answer_as before it is
	// derived as a human's answer. Loaded fail-closed, like the write path's
	// own load below: no usable file ⇒ no key ⇒ every bot-carried record
	// derives answered-unverified.
	answerKeys, _ := intentkeys.LoadOrEmpty(*intentKeys)
	answerVerifiers := pollAnswerVerifiers(cfg, answerKeys)

	poller := &index.Poller{
		DB:       db,
		Fetcher:  fetcher,
		Config:   cfg,
		Dir:      dir,
		Interval: *interval,
		Events:   eventBus,
		Answers:  answerVerifiers,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *once {
		res := poller.PollOnce(ctx)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		if res.Outcome != "ok" {
			// A degraded cycle exits non-zero so a cron wrapper notices.
			// "Some repos could not be checked" is a result, not a success.
			os.Exit(1)
		}
		return
	}

	go poller.Run(ctx)

	// The HTTP surface is composed of two handlers on one mux:
	//   - serve.New — the read-only surface, unchanged and still
	//     structurally incapable of a write.
	//   - local.New — the LOCAL-ONLY read/ack write endpoints (ruling D7):
	//     POST /escalations/{id}/read|ack, which write deskd's own SQLite and
	//     NOTHING on the platform. They are the endpoints the answer flow will call.
	// The /escalations/ prefix routes to the local writer; everything else to
	// the read-only surface. No platform write path exists on either.
	// The MCP surface (the service contract) — the AGENTIC-FIRST write path the
	// loops use. It is served alongside HTTP from this one binary (design
	// §7.1, §9.3: the in-cluster endpoint lets pod loops reach desk__escalate
	// without leaving the cluster). Its write path authenticates as the desk
	// App; without a write token the mutating tools return could-not-check.
	var filer mcp.Filer
	if wt := os.Getenv(writeTokenEnv); wt != "" {
		filer = mcp.NewGitHubFiler(*apiBase, wt, db)
		fmt.Fprintf(os.Stderr, "deskd: MCP write path enabled — desk__escalate files as %s\n", mcp.AppIdentity)
	} else {
		fmt.Fprintf(os.Stderr, "deskd: %s unset — MCP read tools serve, mutating tools return could-not-check (no silent no-op)\n", writeTokenEnv)
	}
	// P3: /mcp authenticates its caller. Until this increment the surface was
	// mounted ahead of the capability loop and performed no authentication of
	// its own — it read the caller login straight from the X-Desk-Caller header,
	// so an off-loopback bind made desk__escalate / desk__report_file /
	// desk__claim reachable unauthenticated with a caller login the requester
	// supplied. The gate below refuses first and resolves the caller second, and
	// in bearer mode the header is not read at all.
	mcpSrv := mcp.NewServer(mcp.Options{
		DB:       db,
		Config:   cfg,
		Dir:      index.NewDirectory(cfg),
		Filer:    filer,
		Terminal: termRegistry,
		Caller:   mcpCallerAuth(mode, validator),
	})

	// The human-gate ANSWER write path — the ONE path that produces a
	// operator-authored mutation, behind a live session.
	//
	// P1: the writer holds NO credential. Under custody model (b) it resolves
	// the person's credential from the request that carries it and drops it when
	// that request returns, so a service compromised at rest yields no human
	// write authority — there is nothing at rest to compromise. The session
	// store's service-wide presence question is answered "never": presence is a
	// per-request fact now, decided by handlers through CredentialFor.
	operatorWriter := answer.NewPerRequestHumanWriter(*apiBase)
	sessions := answer.NewSessionStore(nil, func() bool { return false })
	answerSvc := answer.NewService(operatorWriter, draftAdapter{db}, db, nil)
	answerSvc.SetAnswerAs(resolveAnswerAs(cfg)) // the cell's authorized answerer

	// The signed-intent path (the authorization contract gap 3): on ONLY when the
	// reviewed intent-keys file enrols a key AND the bot gateway credential is
	// configured; otherwise every signed intent is refused, as before. The
	// gateway credential is a BOT's, enforced on the token (an App
	// installation token only; user-bound shapes refused) and re-read from its
	// file on every write, never held for the process lifetime — the human
	// credential stays per request, so "no human credential at rest" holds.
	intents := buildIntentWiring(*intentKeys, os.Getenv, nil)
	if intents.Gateway != nil {
		operatorWriter.SetIntentGateway(intents.Gateway)
	}

	// ── Per-user identity + capability ladder ──────────────────
	//
	// The identity stack replaces the single-persona model: an OAuth session
	// per viewer, a tier derived from that viewer's GitHub repo permission
	// (queried with deskd's OWN read token, per-org), and a capability
	// middleware wrapping every data and write route. The operator answer path
	// above is UNCHANGED — it becomes a consumer of the answerer tier.
	userSessions := identity.NewSessionStore(nil)
	oauthCfg := oauthConfigFromEnv(*addr, *apiBase)

	// The ROLLOUT GATE (the service contract Task 9, the authorization contract,
	// the authorization contract). The per-user path is OFF on every cell that has not
	// set `per_user_identity: enabled`, and the env vars above are a
	// PRECONDITION for serving what the config already permitted — never the
	// enable. Before this, `if oauthCfg.Configured()` was the whole switch,
	// which made the property the ruling turns on a matter of deployment
	// convention rather than of code.
	rollout := buildCellRollout(cfg)

	auth := identity.NewAuth(oauthCfg, userSessions, nil)
	auth.Rollout = rollout
	switch {
	case perUserSignInEnabled(oauthCfg, rollout):
		fmt.Fprintf(os.Stderr, "deskd: per-user GitHub sign-in enabled on %s (client %s…, callback %s)\n",
			rollout.Describe(), clip(oauthCfg.ClientID), oauthCfg.RedirectURL)
	case !rollout.SignInServed():
		fmt.Fprintf(os.Stderr, "deskd: per-user identity path OFF on every cell (no cell sets per_user_identity: %s) — "+
			"the sign-in route is not served and no viewer rises above anonymous; a private cell stays login-gated and renders nothing pre-auth\n",
			schema.PerUserIdentityEnabled)
	default:
		fmt.Fprintf(os.Stderr, "deskd: %s opts in to the per-user path but %s/%s/%s are unset — sign-in cannot be served; the surface stays login-gated\n",
			rollout.Describe(), oauthClientEnv, oauthSecretEnv, oauthRedirectEnv)
	}

	// The permission querier: deskd's own per-org read token answers "what
	// permission does this viewer hold on this repo?" — never the viewer's
	// token (Verify 6). Same token source the escalation poll routes through,
	// re-resolved per query (the authorization contract). The fleet map's cache is built
	// here too, over the same source; it is wired into the web console below.
	permRouter, fleetCache := buildReadConsumers(*apiBase, tokenSource, owners, *interval)
	deriver := identity.NewDeriver(permRouter, nil)

	// Private-repo gate (task 6): resolve which cells may serve anonymously and
	// whether the surface is login-gated. An anonymous opt-in is verified
	// against the GitHub API here, fail-closed — a private repo in an opted-in
	// cell refuses startup.
	policy := buildCellPolicy(ctx, cfg, permRouter)

	guard := &identity.Guard{
		Sessions: userSessions,
		Deriver:  deriver,
		Repos:    repoResolver{db},
		Policy:   policy,
		Rollout:  rollout,
	}

	// P1: the per-request human credential. It comes from the viewer's LIVE
	// OAuth session — which internal/identity keeps in memory only
	// (TestTokenNeverSerialised) and never writes anywhere — so the credential
	// exists for the length of the request that uses it and the service has
	// nothing to store. This is the deskd-side realization of custody model (b).
	// There is no cmd/deskclient (re-homed to the Electron cockpit,
	// the authorization contract); accepting the credential from a non-browser client's
	// Authorization header instead is the deferred the service contract item recorded
	// in the local override contract, and this resolver is where it
	// lands.
	humanCredential := humanCredentialFor(userSessions)

	// Wire the OAuth login into the answer path: a live-answer session may be
	// begun ONLY as the authenticated viewer, and /session reports that login.
	//
	// AuthLogin is wired ONLY when the per-user path is actually live. With
	// the path OFF no session can exist (the sign-in route is not served), so
	// this is belt-and-braces rather than the gate — but it means /session
	// reports no live login on an OFF deployment by construction, not merely
	// because nobody managed to sign in (Verify 12).
	var authLogin func(*http.Request) (string, bool)
	if perUserSignInEnabled(oauthCfg, rollout) {
		authLogin = authLoginFn(userSessions)
	}
	answerH := (&answer.Handlers{
		Cookies:  httpsecurity.CookiesForURL(oauthCfg.RedirectURL),
		Service:  answerSvc,
		Sessions: sessions,
		Lookup:   lookupAdapter{db},
		// No ambient Credential: the service owns none. CredentialFor answers
		// the per-request question instead (custody model (b)). The rollout
		// gate's authLogin (#55, merged from main) is kept as-is: with the
		// per-user path OFF there is no session, so there is no credential
		// either — the two gates agree without either being told about the
		// other.
		CredentialFor: humanCredential,
		AuthLogin:     authLogin,
	}).Handler()

	// P2: the free-text human write POST /comment/{id} is GONE from the
	// service. The enumerated allow-list is {answer, ack, send, session} (internal/contract);
	// a free-text comment on an arbitrary indexed escalation is not in it, and
	// the reasoning that refuses a dormant-but-built write path refuses a
	// dormant-but-mounted one too. The handler package is not merely unmounted:
	// it is out of this binary's import graph (TestServiceExcludesFreeTextCommentPath),
	// so it cannot be re-enabled by a flag. The comment path re-homes to the
	// client host in the service contract, where it is the person's own write on
	// their own machine.

	fmt.Fprintf(os.Stderr, "deskd: holds no human credential at rest; performs only the named actions %v — answer/ack with a credential supplied per request (custody model (b), #1819), send/session on the per-request viewer identity with no credential\n", contract.NamedActions)
	fmt.Fprintln(os.Stderr, intents.Banner)

	// The HTML console (the answer flow web/) is the human surface; the JSON read
	// surface (the read surface serve) is the machine surface. Both answer /inbox, so a
	// browser (Accept: text/html) gets the console and a machine gets JSON.
	webMCPDev := os.Getenv(webMCPDevEnv) == "1"
	if webMCPDev {
		fmt.Fprintf(os.Stderr, "deskd: WebMCP diagnostics dev flag ON (%s=1) — three read-only tools registered on /inbox for a live Chrome DevTools MCP session; never set this in a deployed config (the service contract)\n", webMCPDevEnv)
	}
	// The spatial fleet map's read client (the service contract, one-week
	// experiment). fleetCache (built above by buildReadConsumers) reuses the
	// SAME per-org token source the escalation poller reads through — no new
	// token plumbing, no new daemon: [fleet.Cache] does a live bounded read
	// per request, falling back to its own in-memory last-good snapshot
	// (marked Stale) only when that live read fails.
	webSrv, err := web.New(web.Deps{DB: db, Config: cfg, WebMCPDev: webMCPDev, Viewer: guard, Fleet: fleetCache})
	if err != nil {
		fatal(1, "web console did not initialise: %v", err)
	}
	htmlConsole := webSrv.Handler()
	jsonAPI := serve.New(db, cfg, guard)
	// The VERSIONED read surface (the service contract, increment 2): the typed
	// `/v1` route table a client host reads over the ruled client↔service seam
	// (#1819). It shares the read Server shape and the same per-repo private-repo
	// gate as the un-versioned surface; the un-versioned paths above are now
	// DEPRECATED aliases that keep serving for one release. Increment 2 adds the
	// ENUMERATED named-action write handlers to the seam — the two routes
	// internal/contract declares as KindNamedAction and no others; the SSE
	// stream (/v1/events) and the refresh nudge are the remaining increments.
	// The cell-override wiring (the service contract): the local override table,
	// the answerer gate on the POST, and the record-PR filer. The filer is the
	// SAME desk-App filer as the escalation write path, extended with the
	// write-issues App identity the record PR files under — SEPARATE from the
	// desk App token, and could-not-check when unset (the filer never falls back
	// to the read App or a person's token).
	var recordFiler mcp.RecordFiler
	if gf, ok := filer.(*mcp.GitHubFiler); ok {
		if rt := os.Getenv(recordTokenEnv); rt != "" {
			gf.SetRecordApp(rt, os.Getenv(recordAppEnv))
			fmt.Fprintf(os.Stderr, "deskd: cell-override record PRs enabled — filed as the write-issues App %s\n", clip(os.Getenv(recordAppEnv)))
		} else {
			fmt.Fprintf(os.Stderr, "deskd: %s unset — cell overrides are written locally but the record PR is could-not-check (no fallback identity, the service contract)\n", recordTokenEnv)
		}
		recordFiler = gf
	}
	overridesWiring := &serve.Overrides{
		Store:      local.NewOverrideStore(db),
		Filer:      recordFiler,
		Answerer:   overrideAnswerer(userSessions, cfg),
		Cell:       cellByName(cfg),
		ConfigRepo: os.Getenv(overrideConfigRepoEnv),
		ConfigPath: *configPath,
	}
	// The refresh nudge's wiring (increment 3, task 2): only mounted when the
	// configured fetcher actually supports a single-issue re-derive — the
	// offline envelope's unreachableFetcher does not, and the route stays
	// absent (a 404) on that build rather than present-but-erroring.
	var refresher serve.Refresher
	if one, ok := fetcher.(adapter.OneFetcher); ok {
		refresher = refreshAdapter{db: db, fetcher: one, dir: dir, answers: answerVerifiers}
	} else {
		fmt.Fprintf(os.Stderr, "deskd: the configured fetcher does not support a single-issue refresh — POST /v1/escalations/{id}/refresh is not mounted\n")
	}

	// The guard is also the per-item gate POST /v1/escalations/{id}/read runs
	// (the service contract increment 4); the serve.LocalStateAuthorizer assertion at
	// the end of this file makes that a build property, so the route cannot
	// silently fall back to unmounted.
	namedActions := &serve.NamedActions{
		Service:    answerSvc,
		Sessions:   sessions,
		Lookup:     lookupAdapter{db},
		Credential: humanCredential,
		Intents:    intents.Verifier,
		Terminal:   terminalActions(userSessions, validator, rollout, cfg, termRegistry),
	}
	v1API := serve.NewV1(db, cfg, guard, namedActions, overridesWiring, termRegistry, eventBus, refresher)

	// The protected route table: EVERY data and write route
	// registers through the capability middleware. It is validated before
	// mounting, so a route without the gate cannot ship (Verify 9 asserts the
	// same rule structurally). The table itself lives in routes.go so the P2
	// walk can read it without starting a server
	// (TestServiceWriteRoutesInAllowList).
	//
	// The MCP caller gate is shared by the mux entry and the RPC handler: two
	// independent refusals on the same signal, one at the route and one inside
	// the surface, so neither is the single control.
	mcpCaller := mcpCallerAuth(mode, validator)
	// The served mux is built by buildServedMux (served.go), the SAME function
	// the route-walk test calls (the authorization contract): it validates the table,
	// mounts the few unauthenticated routes, and wraps every table route in the
	// guard. Mount nothing here — a pattern added outside that function is one
	// the walk cannot see, and TestServedMuxIsTheOnlyMux refuses it.
	served, err := buildServedMux(servedDeps{
		Routes: routeHandlers{
			HTMLConsole: htmlConsole,
			LocalState:  local.New(db).Handler(),
			Answer:      answerH,
			Inbox:       negotiate(htmlConsole, jsonAPI, "/inbox"),
			V1:          v1API,
			Root:        negotiate(htmlConsole, jsonAPI, "/"),
			MCP:         mcpSrv.Handler(),
			MCPAuthenticated: func(r *http.Request) bool {
				_, ok := mcpCaller.AuthenticateCaller(r)
				return ok
			},
			V1Authenticated: namedActions.IntentAuthenticated,
		},
		Auth:    auth.Handler(),
		Static:  htmlConsole,
		Session: answerH,
		Guard:   guard,
		Origin:  serve.OriginGuard{Hosts: serve.ServedHosts(*addr, oauthCfg.RedirectURL)},
	})
	if err != nil {
		fatal(1, "capability route table is invalid — a data/write route is missing its gate: %v", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           served,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// The attach supervisor (the service contract) starts only once the HTTP
	// surface is listening, and never in --once mode (that branch returned
	// above). It stops inside the same 5s shutdown window.
	configureDrivers(cfg, *addr, mode, validator != nil)
	attach := terminal.NewSupervisor(termRegistry, nil, terminal.SupervisorOptions{
		Mode: attachMode,
		Logf: func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) },
	})
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		attach.Stop()
		_ = srv.Shutdown(shutdown)
	}()

	fmt.Fprintf(os.Stderr, "deskd: on http://%s (read: ", *addr)
	for i, r := range serve.Routes {
		if i > 0 {
			fmt.Fprint(os.Stderr, ", ")
		}
		fmt.Fprintf(os.Stderr, "GET %s", r.Path)
	}
	fmt.Fprintf(os.Stderr, "; local-only write (ruling D7, no platform write): "+
		"POST /escalations/{id}/read, POST /escalations/{id}/ack; MCP (the service contract): "+
		"POST /mcp (JSON-RPC: initialize, tools/list, tools/call)); polling every %s\n", *interval)
	fmt.Fprintf(os.Stderr, "deskd: versioned surface (the service contract): GET %s/{cells,inbox,escalations/{id},roadmap,portfolio,metrics} "+
		"(the un-versioned paths above are deprecated aliases for one release); named-action writes POST %s/escalations/{id}/{answer,ack} "+
		"(each executed with the credential the caller supplies for that request) and POST %s/desks/{id}/terminal/send, POST|DELETE %s/sessions "+
		"(the terminal actions, on the viewer identity, answerer only) — the CLOSED allow-list\n", contract.Prefix, contract.Prefix, contract.Prefix, contract.Prefix)
	readSourceDesc := fmt.Sprintf("env %s (per-org)", tokenEnv)
	if *tokensDir != "" {
		readSourceDesc = fmt.Sprintf("--tokens-dir %s (per-org, re-read every poll cycle and every permission/fleet read)", *tokensDir)
	}
	fmt.Fprintf(os.Stderr, "deskd: identities held — read %s, write %s (%s); auth=%s\n",
		readSourceDesc, writeTokenEnv, mcp.AppIdentity, mode)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fatal(1, "listen: %v", err)
	}
	attach.Start(ctx)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fatal(1, "listen: %v", err)
	}
}

// attachBanner is the startup banner's attach clause: the driver set this
// binary links (cmd/deskd/drivers.go) and what the attach supervisor will do.
func attachBanner(mode terminal.AttachMode, once bool) string {
	const linked = "drivers linked: container, local (not pod: no deskd path builds the pod driver)"
	switch {
	case once:
		return linked + "; attach: never in --once (a cron/CI/verify run never execs)"
	case mode == terminal.AttachModeOff:
		return linked + "; attach: off (--terminal-attach=off) — every desk reads off/disabled, no desk accepts send and every session open is refused"
	}
	return linked + "; attach: configured — each desk is attached at start and re-attached with bounded backoff; " +
		"the lifecycle itself writes nothing, but an attached desk accepts send from its cell's answerer (the service contract), " +
		"and a local desk's shell runs as deskd's own OS user; --terminal-attach=off disables both; " +
		"cluster (pod) desks read unauthorized; shutdown ends deskd's own shell in each container desk; a local desk is deskd's own child and ends with it"
}

// configOwners returns the sorted, de-duplicated set of repo OWNERS across
// every cell — the orgs a token must be resolved for. A slug that is not
// owner/name is skipped here: the poller reports it as could-not-check per repo
// at poll time (poller.go: ParseRepo failure), so it needs no token and must
// not make deskd refuse to start.
func configOwners(cfg *schema.File) []string {
	if cfg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range cfg.Cells {
		for _, slug := range c.Repos {
			repo, err := adapter.ParseRepo(slug)
			if err != nil {
				continue
			}
			if !seen[repo.Owner] {
				seen[repo.Owner] = true
				out = append(out, repo.Owner)
			}
		}
	}
	sort.Strings(out)
	return out
}

// buildTokenSource resolves the --tokens-dir flag to the [adapter.TokenSource]
// deskd's escalation poller reads through every cycle (the service contract).
//
// Absent (the empty string, the flag's default), it is [adapter.EnvTokenSource]
// wrapping getenv — TODAY'S behaviour, byte-for-byte unchanged (Verify row 6,
// TestEnvTokenSourceUnchanged): the laptop `cellctl deskd` path, which mints
// tokens attended and passes them as env, keeps working exactly as before.
// Set, it is [adapter.FileTokenSource] over the directory — the sidecar-shared
// token-file contract the service contract writes into.
func buildTokenSource(tokensDir string, getenv func(string) string) adapter.TokenSource {
	if tokensDir == "" {
		return adapter.EnvTokenSource{Getenv: getenv}
	}
	return adapter.FileTokenSource{Dir: tokensDir}
}

// buildReadConsumers builds the two per-request read-token consumers: the
// permission querier and the fleet map cache. Both take src itself — the same
// [adapter.TokenSource] the escalation poller's [adapter.ReloadingRouter]
// re-resolves every cycle — and resolve the owner's token on every read, so
// under --tokens-dir a rotated token is used with no restart and a removed
// one fails closed (the authorization contract, TestReadConsumersReload).
func buildReadConsumers(apiBase string, src adapter.TokenSource, owners []string, interval time.Duration) (*adapter.PermissionRouter, *fleet.Cache) {
	return adapter.NewPermissionRouter(apiBase, src, owners, nil),
		fleet.NewCache(apiBase, src, owners, nil, interval, fleet.DefaultClaimStaleAfter, nil)
}

func fatal(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "deskd: "+format+"\n", args...)
	os.Exit(code)
}

// unreachableFetcher is the offline-envelope fetcher (DESKD_API_UNREACHABLE): it
// reports every repo as could-not-check WITHOUT opening a socket, so a dev/CI/
// verify run contacts no live infrastructure. It mirrors the degraded-poll shape
// a real 404/401/timeout produces — could-not-check, prior rows retained,
// BLIND-not-idle — rather than an empty clean cycle.
type unreachableFetcher struct{}

func (unreachableFetcher) FetchEscalations(_ context.Context, repo adapter.Repo) adapter.FetchResult {
	now := time.Now().UTC()
	p := domain.Probe{
		Source: apiUnreachableEnv + " set — the GitHub API is treated as unreachable and no network call is made",
		Method: domain.MethodLocalTable,
		Window: "offline envelope: every repo is could-not-check",
		Limit:  0,
	}
	reason := apiUnreachableEnv + " is set: this repo is could-not-check by construction (offline envelope), never empty"
	return adapter.FetchResult{
		Repo:       repo,
		Health:     adapter.RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(p, reason)},
		Candidates: domain.NoCount(p, reason),
		Derived:    domain.NoCount(p, reason),
	}
}

// negotiate routes a request to the HTML console or the JSON API by Accept. For
// the "/" path it only hands GET+HTML to the console (so `/cells` and friends
// stay JSON); for "/inbox" it hands any GET+HTML to the console.
func negotiate(html, json http.Handler, forPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptsHTML := r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
		if forPath == "/" {
			if r.URL.Path == "/" && acceptsHTML {
				html.ServeHTTP(w, r)
				return
			}
			json.ServeHTTP(w, r)
			return
		}
		if acceptsHTML {
			html.ServeHTTP(w, r)
			return
		}
		json.ServeHTTP(w, r)
	})
}

// draftAdapter bridges the index's local draft store to the answer.DraftStore
// interface, so the write path's drafts survive a restart (Verify 9) on the
// same local, non-derived table read/ack live in (ruling D7).
type draftAdapter struct{ db *index.DB }

func (a draftAdapter) SaveDraft(d answer.Draft) error {
	return a.db.SaveDraft(index.DraftRecord{
		EscalationID: d.EscalationID, Option: d.Option, Text: d.Text, SavedAt: d.SavedAt,
	})
}

func (a draftAdapter) LoadDraft(id domain.EscalationID) (answer.Draft, bool, error) {
	r, ok, err := a.db.LoadDraft(id)
	if err != nil || !ok {
		return answer.Draft{}, ok, err
	}
	return answer.Draft{EscalationID: r.EscalationID, Option: r.Option, Text: r.Text, SavedAt: r.SavedAt}, true, nil
}

func (a draftAdapter) ClearDraft(id domain.EscalationID) error { return a.db.ClearDraft(id) }

// lookupAdapter resolves an escalation id to its platform ref against the
// TRUSTED index — the ref a write acts on never comes from the request body.
type lookupAdapter struct{ db *index.DB }

func (a lookupAdapter) Ref(id domain.EscalationID) (answer.Ref, bool, error) {
	rows, err := a.db.Inbox("", true)
	if err != nil {
		return answer.Ref{}, false, err
	}
	for _, r := range rows {
		if r.ID == id {
			ref, ok := answer.ParseRef(r.Links["platform"])
			return ref, ok, nil
		}
	}
	return answer.Ref{}, false, nil
}

// refreshAdapter is the [serve.Refresher] wiring for
// POST /v1/escalations/{id}/refresh (the service contract increment 3, task 2):
// resolve the id against the TRUSTED index exactly as [lookupAdapter] does for
// a write, re-derive the one issue with the SAME per-org App-identity routing
// the poller uses (never a human credential — this route authors nothing),
// and store the result with [index.DB.UpsertOne] so a targeted refresh cannot
// wipe the repo's other rows the way [index.DB.UpsertEscalations] would.
type refreshAdapter struct {
	db      *index.DB
	fetcher adapter.OneFetcher
	dir     adapter.AutomationDirectory
	// answers is the same per-cell answer-record verifier the poller uses, so
	// a targeted refresh derives an answer exactly as a full cycle would. Nil
	// trusts no bot-carried record.
	answers func(domain.CellName) adapter.AnswerVerifier
}

func (a refreshAdapter) Refresh(ctx context.Context, id domain.EscalationID) error {
	rows, err := a.db.Inbox("", true)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		ref, ok := answer.ParseRef(row.Links["platform"])
		if !ok {
			return fmt.Errorf("escalation %s carries no parseable platform link", id)
		}
		repo, err := adapter.ParseRepo(ref.Repo)
		if err != nil {
			return err
		}
		iss, found, err := a.fetcher.FetchOne(ctx, repo, ref.Number)
		if err != nil {
			return fmt.Errorf("re-derive %s#%d failed: %w", repo, ref.Number, err)
		}
		if !found {
			// GitHub named no such issue any more (deleted/transferred): the
			// escalation this id resolved to is gone, which is the same fact
			// "unknown escalation" reports for any other caller.
			return serve.ErrRefreshUnknownEscalation
		}
		var answers adapter.AnswerVerifier
		if a.answers != nil {
			answers = a.answers(row.Cell)
		}
		esc := adapter.MapEscalation(row.Cell, repo, iss, a.dir, a.db.MintID, answers)
		return a.db.UpsertOne(repo, esc, iss)
	}
	return serve.ErrRefreshUnknownEscalation
}

// ── the capability model wiring helpers ───────────────────────────────────────────────

// resolveAnswerAs picks the Service-level authorized-answerer login. When every
// cell shares one answer_as it is that value; otherwise it falls back to the
// default (the per-repo answerer tier is still enforced authoritatively by the
// capability middleware, which reads answer_as per item's cell).
func resolveAnswerAs(cfg *schema.File) string {
	if cfg == nil || len(cfg.Cells) == 0 {
		return schema.DefaultAnswerAs()
	}
	set := map[string]bool{}
	for i := range cfg.Cells {
		set[cfg.Cells[i].AnswerAsLogin()] = true
	}
	if len(set) == 1 {
		for k := range set {
			return k
		}
	}
	return schema.DefaultAnswerAs()
}

// oauthConfigFromEnv reads the per-user OAuth client from the environment. It
// is factored out so the rollout-gate tests can assert the thing #55 reports:
// that a FULLY configured client still enables nothing on its own.
func oauthConfigFromEnv(addr, apiBase string) identity.Config {
	return identity.Config{
		ClientID:     os.Getenv(oauthClientEnv),
		ClientSecret: os.Getenv(oauthSecretEnv),
		RedirectURL:  oauthRedirectURL(addr),
		AuthBase:     "", // github.com by default
		APIBase:      apiBase,
	}
}

// perUserSignInEnabled is the AND that replaces the old global
// `if oauthCfg.Configured()`: the per-user path serves only where a CELL
// opted in (the rollout gate) AND the deployment holds a usable OAuth client
// (the credential precondition). Neither alone is an enable — that ordering
// is the whole content of the authorization contract.
func perUserSignInEnabled(cfg identity.Config, rollout identity.RolloutGate) bool {
	if rollout == nil || !rollout.SignInServed() {
		return false
	}
	return cfg.Configured()
}

// cellRollout resolves the per-cell rollout gate to the repos the middleware
// asks about. It is the deskd-side implementation of identity.RolloutGate,
// built once at startup from the validated config.
//
// Nothing is defaulted in: a repo absent from byRepo is OFF, which covers a
// repo no cell owns, a repo whose cell could not be resolved, and the empty
// repo. That is the "fails closed to OFF when a cell's environments/opt-in
// cannot be resolved" half of the ruling — the map only ever holds answers
// that were positively derived.
type cellRollout struct {
	byRepo  map[string]bool
	enabled []string // cell names that opted in, for the startup log
}

func (r *cellRollout) EnabledForRepo(owner, name string) bool {
	if r == nil || owner == "" || name == "" {
		return false
	}
	return r.byRepo[owner+"/"+name]
}

func (r *cellRollout) SignInServed() bool { return r != nil && len(r.enabled) > 0 }

// Describe names the opted-in cells for the startup line, so an operator can
// see at a glance WHICH cells the path went live on rather than only that it
// did.
func (r *cellRollout) Describe() string {
	if !r.SignInServed() {
		return "no cell"
	}
	return "cell(s) " + strings.Join(r.enabled, ", ")
}

// buildCellRollout derives the gate from the config. The per-cell decision is
// schema.Cell.PerUserIdentityEnabled — the SAME predicate cellsvalidate's
// refusal is written against, re-evaluated here at runtime so deskd fails
// closed on its own signal rather than on the assumption that the file was
// validated by someone else.
func buildCellRollout(cfg *schema.File) *cellRollout {
	r := &cellRollout{byRepo: map[string]bool{}}
	if cfg == nil {
		return r // unresolvable config: nothing is enabled
	}
	for i := range cfg.Cells {
		c := &cfg.Cells[i]
		if !c.PerUserIdentityEnabled() {
			continue
		}
		r.enabled = append(r.enabled, c.Name)
		for _, slug := range c.Repos {
			r.byRepo[slug] = true
		}
	}
	return r
}

// oauthRedirectURL is the OAuth callback: the DESKD_OAUTH_REDIRECT_URL override,
// or derived from the listen address for the localhost default.
func oauthRedirectURL(addr string) string {
	if v := os.Getenv(oauthRedirectEnv); v != "" {
		return v
	}
	return "http://" + addr + "/auth/callback"
}

func clip(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// authLoginFn resolves the authenticated login of a request's OAuth session, so
// the answer path binds a live-answer session to whoever signed in.
func authLoginFn(store *identity.SessionStore) func(*http.Request) (string, bool) {
	return func(r *http.Request) (string, bool) {
		c, err := r.Cookie(identity.SessionCookie)
		if err != nil {
			return "", false
		}
		sess, err := store.Validate(c.Value)
		if err != nil {
			return "", false
		}
		return sess.Login, true
	}
}

// repoResolver resolves an escalation id to its repo against the TRUSTED index —
// the middleware's per-item repo, never from the request.
type repoResolver struct{ db *index.DB }

func (rr repoResolver) RepoForItem(id string) (string, string, bool) {
	rows, err := rr.db.Inbox("", true)
	if err != nil {
		return "", "", false
	}
	for _, r := range rows {
		if string(r.ID) == id {
			ref, ok := answer.ParseRef(r.Links["platform"])
			if !ok {
				return "", "", false
			}
			owner, name, ok := strings.Cut(ref.Repo, "/")
			return owner, name, ok
		}
	}
	return "", "", false
}

// humanCredentialFor is the PER-REQUEST human credential resolver (custody
// model (b), #1819). It reads the viewer's live OAuth session — held in memory
// by internal/identity and never serialised (TestTokenNeverSerialised) — and
// builds a custody.Credential that lives for exactly this request.
//
// Three things it deliberately does NOT do: it never reads a login from
// anything the request supplied (the login is the session's, server-derived);
// it never caches what it builds; and it refuses outright when the signed-in
// viewer is not the accountable answerer, because custody.FromToken will not
// construct a credential that would author under the wrong identity.
func humanCredentialFor(store *identity.SessionStore) func(r *http.Request) (answer.Credential, bool) {
	return func(r *http.Request) (answer.Credential, bool) {
		c, err := r.Cookie(identity.SessionCookie)
		if err != nil {
			return nil, false
		}
		sess, err := store.Validate(c.Value)
		if err != nil {
			return nil, false
		}
		cred, err := custody.FromToken(sess.Token(), sess.Login)
		if err != nil {
			return nil, false
		}
		return cred, true
	}
}

// overrideAnswerer builds the cell-override POST's tier gate (the service contract).
// It resolves the request's authenticated viewer from the identity session and
// reports whether that login is the cell's answer_as — the answerer tier the
// POST requires. login=="" means no authenticated session (the handler answers
// 401); login!="" with isAnswerer=false is an authenticated non-answerer (403).
// The tier is enforced server-side here, never from anything the request body
// supplies.
func overrideAnswerer(store *identity.SessionStore, cfg *schema.File) func(r *http.Request, cell string) (string, bool) {
	return func(r *http.Request, cell string) (string, bool) {
		c, err := r.Cookie(identity.SessionCookie)
		if err != nil {
			return "", false
		}
		sess, err := store.Validate(c.Value)
		if err != nil {
			return "", false
		}
		answerAs := schema.DefaultAnswerAs()
		if cfg != nil {
			for i := range cfg.Cells {
				if cfg.Cells[i].Name == cell {
					answerAs = cfg.Cells[i].AnswerAsLogin()
					break
				}
			}
		}
		// GitHub logins are case-insensitive: compare exactly as the capability
		// ladder does (identity/capability.go), never with plain ==.
		return sess.Login, humanid.Enabled() && sess.Login != "" && answerAs != "" && strings.EqualFold(sess.Login, answerAs)
	}
}

// terminalActions wires the terminal named actions, send and session
// (the service contract). Their authority is identity.TerminalGate: the
// per-request VIEWER identity — the OAuth viewer session, or a bearer the
// viewer App validates (validator is nil unless one is configured, and then any
// Authorization header is refused) — and the Answerer tier on the desk's cell,
// behind the same per-cell rollout gate the capability middleware applies. No
// write credential is consulted: typing into a terminal is not a GitHub write.
func terminalActions(store *identity.SessionStore, validator *identity.Validator, rollout identity.RolloutGate, cfg *schema.File, reg *terminal.Registry) *serve.TerminalActions {
	cells := cellByName(cfg)
	return &serve.TerminalActions{
		Registry: reg,
		Authority: &identity.TerminalGate{
			Sessions:  store,
			Validator: validator,
			Rollout:   rollout,
			Cell: func(name string) (identity.TerminalCell, bool) {
				c, ok := cells(name)
				if !ok {
					return identity.TerminalCell{}, false
				}
				return identity.TerminalCell{Name: c.Name, Repos: c.Repos, AnswerAs: c.AnswerAsLogin()}, true
			},
		},
	}
}

// cellByName resolves a cell by name for the override POST's model-ladder
// validation (the cheapest place the cell's loops[].models are known).
func cellByName(cfg *schema.File) func(string) (*schema.Cell, bool) {
	return func(name string) (*schema.Cell, bool) {
		if cfg == nil {
			return nil, false
		}
		for i := range cfg.Cells {
			if cfg.Cells[i].Name == name {
				return &cfg.Cells[i], true
			}
		}
		return nil, false
	}
}

// mcpCallerAuth builds the /mcp caller gate for the running auth mode (P3).
//
// In loopback mode the machine boundary IS the authentication: a request off
// the loopback interface is refused, and the X-Desk-Caller header a loopback
// caller sets is the per-pod identity design §9.3 describes.
//
// In bearer mode nothing is taken on trust: the caller's token is validated
// against the viewer OAuth App and the caller login is the one the VALIDATED
// grant names, so the header is not consulted at all. That is the half of P3
// the audit's counter-example turned on — "a caller login the requester
// supplies" stops being possible the moment a validated identity exists.
func mcpCallerAuth(mode identity.AuthMode, v *identity.Validator) mcp.CallerAuth {
	return mcp.CallerAuthFunc(func(r *http.Request) (string, bool) {
		if mode == identity.AuthBearer {
			caller, err := v.Validate(r.Context(), identity.BearerFrom(r))
			if err != nil {
				return "", false
			}
			return caller.Login, true
		}
		if !identity.RemoteIsLoopback(r) {
			return "", false
		}
		return "", true
	})
}

// cellPolicy answers the middleware's cell-level questions from cells.yaml plus
// the startup public/private verification.
type cellPolicy struct {
	repoCell   map[string]*schema.Cell // "owner/name" -> owning cell
	anonOK     map[string]bool         // cell name -> verified anonymous-read
	loginGated bool
}

func (p *cellPolicy) AnswerAsForRepo(owner, name string) string {
	if c, ok := p.repoCell[owner+"/"+name]; ok {
		return c.AnswerAsLogin()
	}
	return ""
}

func (p *cellPolicy) AnonymousReadAllowed(owner, name string) bool {
	c, ok := p.repoCell[owner+"/"+name]
	if !ok {
		return false
	}
	return p.anonOK[c.Name]
}

func (p *cellPolicy) SurfaceLoginGated() bool { return p.loginGated }

// buildCellPolicy resolves the private-repo gate (task 6). An anonymous opt-in
// is VERIFIED against the GitHub API here — every repo in the cell must be
// confirmed public — and deskd REFUSES TO START if any repo is private or
// unverifiable, naming it. The surface is login-gated unless every cell is a
// verified anonymous opt-in (deny-by-default).
func buildCellPolicy(ctx context.Context, cfg *schema.File, pr *adapter.PermissionRouter) *cellPolicy {
	p := &cellPolicy{repoCell: map[string]*schema.Cell{}, anonOK: map[string]bool{}}
	if cfg == nil || len(cfg.Cells) == 0 {
		p.loginGated = true
		return p
	}
	allAnon := true
	for i := range cfg.Cells {
		c := &cfg.Cells[i]
		for _, slug := range c.Repos {
			p.repoCell[slug] = c
		}
		if !c.AllowAnonymousRead {
			allAnon = false
			continue
		}
		// Verify EVERY repo is public. Fail closed on the first that is not, or
		// that cannot be verified — an unverifiable repo is treated as private.
		for _, slug := range c.Repos {
			repo, err := adapter.ParseRepo(slug)
			if err != nil {
				fatal(1, "cell %q sets allow_anonymous_read but repo slug %q is not owner/name", c.Name, slug)
			}
			private, verified := pr.RepoPrivate(ctx, repo.Owner, repo.Name)
			if !verified {
				fatal(1, "cell %q sets allow_anonymous_read but repo %s could not be verified public against the GitHub API — refusing to serve it anonymously (an unverifiable repo is treated as private)", c.Name, slug)
			}
			if private {
				fatal(1, "cell %q sets allow_anonymous_read but repo %s is PRIVATE — anonymous read requires every repo in the cell be public; refusing to start", c.Name, slug)
			}
		}
		p.anonOK[c.Name] = true
	}
	p.loginGated = !allAnon
	return p
}

// A compile-time assertion: the capability guard
// deskd hands serve.NewV1 must answer the per-item local-state question, or
// POST /v1/escalations/{id}/read would be left unmounted (the service contract
// increment 4). A Guard that stops implementing it fails the build here.
var _ serve.LocalStateAuthorizer = (*identity.Guard)(nil)
