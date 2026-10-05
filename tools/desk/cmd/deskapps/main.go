// Command deskapps is the installer for Assay's GitHub App identities: one command, one
// browser sitting, driving GitHub's App Manifest flow so the person clicks only what GitHub
// reserves for a signed-in human (Create, Install, the avatar drop). See
// docs/streams/example-stream/design.md for the design of record.
//
// This brief (example-stream/02) ships `deskapps init`: the loopback page, the tier
// manifests, the manifest→code→conversion flow, and the key/record writes. `deskapps
// resume`, `status` and `avatar` are later briefs (03, 04, 06).
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage: deskapps init --tier team|family [--org <login>] [--owner org|me] [--prefix <name>] [--port 41873] [--no-browser] [--dry-run]
       deskapps init --manifest <file> [--org <login>] [--port 41873] [--no-browser] [--dry-run]

--manifest registers a single arbitrary GitHub App from a manifest JSON file (fields: name,
url, description, public, default_permissions, default_events, hook_attributes) instead of
a tier's fixed App set. --manifest and --tier are mutually exclusive.

deskapps resume, deskapps status, deskapps avatar are not implemented yet (example-stream/03, /04, /06).`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "deskapps: unknown verb %q\n%s\n", args[0], usage)
		return 2
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deskapps init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		tier         = fs.String("tier", "team", "team or family")
		manifestPath = fs.String("manifest", "", "path to a single-App manifest JSON file (mutually exclusive with --tier)")
		org          = fs.String("org", "", "org login (required with --owner org, the default; for --manifest, its presence selects org-owned)")
		owner        = fs.String("owner", "org", "org or me")
		prefix       = fs.String("prefix", "", `App name prefix (default: --org's value, else "assay")`)
		port         = fs.Int("port", 41873, "loopback port to bind")
		noBrowser    = fs.Bool("no-browser", false, "print the URL instead of opening a browser")
		dryRun       = fs.Bool("dry-run", false, "print the planned URL and App rows, then exit without serving")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage); fs.PrintDefaults() }
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// --tier has a non-empty default ("team"), so "was --tier given" has to be read off
	// which flags were actually set on the command line, not off *tier's value.
	tierExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "tier" {
			tierExplicit = true
		}
	})

	manifestMode := strings.TrimSpace(*manifestPath) != ""
	if manifestMode && tierExplicit {
		fmt.Fprintln(stderr, "deskapps init: --manifest and --tier are mutually exclusive")
		return 2
	}

	if manifestMode {
		return runInitManifest(*manifestPath, *org, *port, *noBrowser, *dryRun, stdout, stderr)
	}

	if *tier != "team" && *tier != "family" {
		fmt.Fprintf(stderr, "deskapps init: --tier must be team or family, got %q\n", *tier)
		return 2
	}
	if *owner != "org" && *owner != "me" {
		fmt.Fprintf(stderr, "deskapps init: --owner must be org or me, got %q\n", *owner)
		return 2
	}
	if *owner == "org" && *org == "" {
		fmt.Fprintln(stderr, "deskapps init: --org is required with --owner org (the default); pass --owner me for a personal-owned run")
		return 2
	}

	resolvedPrefix := strings.TrimSpace(*prefix)
	if resolvedPrefix == "" {
		if *org != "" {
			resolvedPrefix = *org
		} else {
			resolvedPrefix = "assay"
		}
	}

	specs, err := TierManifests(*tier, resolvedPrefix)
	if err != nil {
		fmt.Fprintln(stderr, "deskapps init:", err)
		return 2
	}

	return serveSpecs(specs, *tier, resolvedPrefix, *org, *owner, *port, *noBrowser, *dryRun, "/", stdout, stderr)
}

// runInitManifest is the --manifest path (assay--issue-1116-manifest): load and validate a
// single-App manifest file, turn it into the one AppSpec the shared machinery drives, and
// hand off to serveSpecs — the SAME loopback bind, state-nonce issuance, callback →
// conversion → PEM-write path and §8 mismatch check the --tier path uses, never a second
// implementation. ownerKind is inferred from whether --org was given: present → org-owned,
// absent → personal-owned, mirroring design.md §2's "org-owned (default) or personal-owned"
// without adding a second --owner flag this command's minimal surface does not ask for.
func runInitManifest(path, org string, port int, noBrowser, dryRun bool, stdout, stderr io.Writer) int {
	mf, err := LoadManifestFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "deskapps init:", err)
		return 2
	}
	spec := ManifestAppSpec(mf)

	ownerKind := "me"
	if strings.TrimSpace(org) != "" {
		ownerKind = "org"
	}

	return serveSpecs([]AppSpec{spec}, "manifest", spec.Name, org, ownerKind, port, noBrowser, dryRun, "/run", stdout, stderr)
}

// serveSpecs is the shared tail of deskapps init for BOTH --tier and --manifest: bind the
// loopback listener, report and exit for --dry-run, else resolve identity, plant one
// pending apps.state.json row per spec (keyed by spec.Name — for a --manifest spec that is
// the App's manifest name, never a role), build the server and serve until the process
// exits. startPath is "/" for --tier (Screen 0, the tier-comparison page) and "/run" for
// --manifest (Screen 0/1 are a tier decision with nothing to decide for a single
// already-specified App, so a manifest run goes straight to the Create board).
func serveSpecs(specs []AppSpec, tierLabel, prefix, org, ownerKind string, port int, noBrowser, dryRun bool, startPath string, stdout, stderr io.Writer) int {
	ln, boundPort, err := listenLoopback(port)
	if err != nil {
		fmt.Fprintln(stderr, "deskapps init: cannot bind any loopback port:", err)
		return 1
	}

	if dryRun {
		_ = ln.Close()
		fmt.Fprintf(stdout, "deskapps init: tier=%s owner=%s prefix=%s port=%d (dry-run)\n",
			tierLabel, ownerDesc(ownerKind, org), prefix, boundPort)
		fmt.Fprintf(stdout, "would serve at http://127.0.0.1:%d%s\n", boundPort, startPath)
		fmt.Fprintln(stdout, "planned Apps:")
		for _, spec := range specs {
			fmt.Fprintf(stdout, "  %s\n", spec.Name)
		}
		return 0
	}

	id, err := ghIdentity()
	if err != nil {
		_ = ln.Close()
		fmt.Fprintln(stderr, "deskapps init: resolving identity (gh auth needed):", err)
		return 6
	}

	sf := &StateFile{Schema: stateSchema}
	now := time.Now().UTC()
	for _, spec := range specs {
		nonce, nerr := newStateNonce()
		if nerr != nil {
			_ = ln.Close()
			fmt.Fprintln(stderr, "deskapps init: generating state nonce:", nerr)
			return 1
		}
		sf.Apps = append(sf.Apps, AppRow{
			App:        spec.Name,
			Tier:       tierLabel,
			Owner:      org,
			OwnerKind:  ownerKind,
			Roles:      spec.Roles,
			ReadOnly:   spec.ReadOnly,
			State:      StatePending,
			StateNonce: nonce,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	if err := saveState(sf); err != nil {
		_ = ln.Close()
		fmt.Fprintln(stderr, "deskapps init: writing apps.state.json:", err)
		return 1
	}

	srv := newServer(boundPort, tierLabel, prefix, org, ownerKind, specs, sf)
	srv.identity = id

	url := fmt.Sprintf("http://127.0.0.1:%d%s", boundPort, startPath)
	fmt.Fprintln(stdout, url)
	if !noBrowser {
		_ = openBrowser(url)
	}

	stop := make(chan struct{})
	go srv.watchThrottle(stop)
	defer close(stop)

	httpSrv := &http.Server{Handler: srv.mux()}
	serveErr := httpSrv.Serve(ln)
	if serveErr != nil && serveErr != http.ErrServerClosed {
		fmt.Fprintln(stderr, "deskapps init:", serveErr)
		return 1
	}
	return 0
}

func ownerDesc(kind, org string) string {
	if kind == "me" {
		return "me"
	}
	return "org:" + org
}

// openBrowser best-effort opens url in the system browser. Failures are silent: --no-browser
// exists for exactly the case where no browser can be opened, and a failed open here just
// means the printed URL (already on stdout) is what the person uses instead.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
