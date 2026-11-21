package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/attestation"
)

func runAcceptanceRouteWorkflow(args []string, out, stderr io.Writer) int {
	verb := args[0]
	fs := flag.NewFlagSet("acceptance-route "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "canonical repository checkout")
	ref := fs.String("brief", "", "canonical brief reference")
	var preparation, card int
	var ruling, markers string
	if verb == "issue" {
		fs.IntVar(&preparation, "preparation", 0, "independent preparation issue number")
		fs.StringVar(&ruling, "ruling", "", "exact human adjudication comment URL")
		fs.StringVar(&markers, "existing-markers", "", "complete canonical issue-marker inventory")
	} else {
		fs.IntVar(&card, "card", 0, "canonical final acceptance card issue number")
	}
	reset := transitionFlags(fs)
	defer reset()
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || *ref == "" || !transitionForge ||
		(verb == "issue" && (preparation <= 0 || ruling == "" || markers == "")) ||
		(verb == "close" && card <= 0) {
		fmt.Fprintln(stderr, "acceptance-route workflow: missing brief, target, or --acceptance-forge")
		return 2
	}
	path, err := filepath.Abs(*root)
	if err == nil {
		err = runAcceptanceEffect(verb, path, *ref, preparation, card, ruling, markers, out)
	}
	if err != nil {
		fmt.Fprintln(stderr, "statusgen:", err)
		return 1
	}
	return 0
}

func runAcceptanceEffect(verb, root, ref string, preparation, card int, ruling, markers string, out io.Writer) error {
	repo, err := attestationTrackingRepo(root)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv("DESKREAD_CI_WORKFLOW_TOKEN"))
	if !scanInCI() || !strings.EqualFold(repo, os.Getenv("GITHUB_REPOSITORY")) ||
		os.Getenv("GITHUB_REF") != "refs/heads/main" || os.Getenv("GITHUB_EVENT_NAME") == "pull_request_target" ||
		!strings.HasPrefix(token, "ghs_") || os.Getenv("GH_TOKEN") != token {
		return fmt.Errorf("acceptance-route workflow: requires the main checkout and its explicit CI workflow token")
	}
	scanSetToolClass(scanClassCI)
	cfg := scanEffectiveConfig()
	if len(cfg.Problems) != 0 {
		return fmt.Errorf("acceptance-route workflow: could-not-check roster")
	}
	forge := newGHClient(token)
	if verb == "issue" {
		if _, err := os.Stat(markers); err != nil {
			return fmt.Errorf("acceptance card: read complete issue inventory: %w", err)
		}
		existing, err := loadExistingMarkers(markers)
		if err != nil {
			return err
		}
		issue, err := makeAdjudicatedVerifyIssue(root, ref, preparation, ruling, forge, cfg.HumanLogins, true)
		if err != nil {
			return err
		}
		issues := []verifyIssue{}
		if !existing[issue.Marker] {
			issues = append(issues, issue)
		}
		return json.NewEncoder(out).Encode(issues)
	}
	return closeAdjudicatedCard(root, ref, repo, card, forge, cfg)
}

func checkClosedAdjudicatedCard(issue *creditIssue, brief string, closer scanIdentity) (adjudicatedCardRef, error) {
	if issue == nil || issue.State != "closed" || issue.Author == nil ||
		issue.Author.Type != "Bot" || issue.Author.Login != "github-actions[bot]" || issue.Author.ID != 41898282 {
		return adjudicatedCardRef{}, fmt.Errorf("acceptance card: expected a closed card issued by GitHub Actions")
	}
	if closer.ID <= 0 || issue.ClosedBy == nil || issue.ClosedBy.Type != "User" ||
		!strings.EqualFold(issue.ClosedBy.Login, closer.Login) || issue.ClosedBy.ID != closer.ID {
		return adjudicatedCardRef{}, fmt.Errorf("acceptance card: final close is not from the pinned human")
	}
	label := false
	for _, l := range issue.Labels {
		label = label || strings.EqualFold(l, "verify-gate")
	}
	if !label {
		return adjudicatedCardRef{}, fmt.Errorf("acceptance card: missing verify-gate label")
	}
	markers := verifyMarkerRe.FindAllString(issue.Body, -1)
	if len(markers) != 1 || normalizeMarker(markers[0]) != verifyMarker(brief) {
		return adjudicatedCardRef{}, fmt.Errorf("acceptance card: wrong or ambiguous brief marker")
	}
	ref, present, err := parseAdjudicatedCardRef(issue.Body)
	if err != nil || !present {
		return adjudicatedCardRef{}, fmt.Errorf("acceptance card: no valid adjudication marker")
	}
	return ref, nil
}

func closeAdjudicatedCard(root, brief, repo string, issue int, forge *ghClient, cfg scanConfig) error {
	bf, err := locateAcceptanceBrief(root, brief)
	if err != nil {
		return err
	}
	cc := creditContext{ciWorkflowToken: true}
	target := fmt.Sprintf("%s#%d", repo, issue)
	first, err := cc.readIssue(target)
	if err != nil {
		return err
	}
	ref, err := checkClosedAdjudicatedCard(first, bf.Brief, cfg.Bless)
	if err != nil {
		return err
	}
	ruling, _ := parseRulingURL(ref.Ruling)
	if !strings.EqualFold(ruling.Repo, repo) {
		return fmt.Errorf("acceptance card: adjudication is outside the card's repository")
	}
	history, err := cc.readTrust(target)
	if err != nil {
		return err
	}
	if why := attestation.HistoryProblem(history.BodyHistoryKnown != nil && *history.BodyHistoryKnown, history.Complete, history.BodyEditedAt != ""); why != "" {
		return fmt.Errorf("acceptance card: %s", why)
	}
	m, err := loadAcceptancePreparation(repo, ref.Preparation, true)
	if err != nil {
		return err
	}
	digest, err := m.digest()
	if err != nil || digest != ref.Manifest {
		return fmt.Errorf("acceptance card: preparation does not match the card's manifest digest")
	}
	readme, updated, _, err := planAdjudicatedClose(root, brief, m, ref.Preparation, ref.Ruling, time.Now(), forge, cfg.HumanLogins, true)
	if err != nil {
		return err
	}
	fresh, err := cc.readIssue(target)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(first, fresh) {
		return fmt.Errorf("acceptance card: issue changed during close revalidation")
	}
	return os.WriteFile(readme, updated, 0o644)
}
