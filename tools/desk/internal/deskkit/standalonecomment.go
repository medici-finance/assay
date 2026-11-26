package deskkit

import (
	"errors"
	"fmt"
	"strings"
)

// OutboundBodyMaxBytes is the size cap for outward comment and review bodies.
const OutboundBodyMaxBytes = 16 * 1024

const standaloneWithheldNotice = "NOTICE: withheld identifiers could not be checked because publication policy supplied none"

type StandalonePublication struct {
	Number        int
	Head          string
	Body          string
	MaxVisibility string // operator-authorized private, internal or public
	// AlreadyPosted records a prior success for this exact change, head and body.
	// The caller serializes the lookup and records successful writes.
	AlreadyPosted       bool
	HumanLogins         []string
	PrivateRepositories []string
	WithheldIdentifiers []string
}

func (c *StandaloneChanges) PublishComment(in StandalonePublication) error {
	if !standaloneSHAPattern.MatchString(in.Head) {
		return errors.New("a full reviewed head is required to publish")
	}
	if strings.TrimSpace(in.Body) == "" || len(in.Body) > OutboundBodyMaxBytes {
		return fmt.Errorf("comment body must contain between 1 and %d bytes of text", OutboundBodyMaxBytes)
	}
	visibility, err := c.forge.RepoVisibility(c.repo)
	if err != nil {
		return standaloneOperationError("read repository visibility", err)
	}
	if err := c.checkComment(in, visibility); err != nil {
		return err
	}
	change, err := c.Change(in.Number)
	if err != nil {
		return err
	}
	if change.State != "open" && change.State != "opened" {
		return errors.New("change is no longer open")
	}
	if change.HeadSHA != in.Head {
		return fmt.Errorf("reviewed head %s differs from current head %s", in.Head, change.HeadSHA)
	}
	if in.AlreadyPosted {
		return nil
	}
	ref, err := c.forge.PostCommentTyped(c.repo, in.Number, TargetChange, in.Body)
	if err != nil {
		return standaloneOperationError("post comment", err)
	}
	if ref == nil || ref.DatabaseID < 1 {
		return errors.New("forge did not confirm the posted comment")
	}
	return nil
}

func (c *StandaloneChanges) checkComment(in StandalonePublication, live string) error {
	configured := ParseVisibility(in.MaxVisibility)
	configuredRank := visibilityExposure(configured)
	liveRank := visibilityExposure(ParseVisibility(live))
	if liveRank < 0 || configuredRank < liveRank {
		return errors.New("repository visibility exceeds the configured publication scope or could not be verified")
	}
	humans := splitWithheldIdentifiers(strings.Join(in.HumanLogins, ","))
	scanners := outboundScanners{
		rulingClaim: func(surface string, text []byte) error {
			if name, found := impersonatedRulingBy(markerSurface(string(text)), humans); found {
				return impersonationRefusal(surface, name)
			}
			return nil
		},
		personalData: func(text string) []piiFinding { return scanPersonalData(text, defaultEmailAllowed) },
	}
	repo := c.repo.Slug()
	missingWithheld := false
	if configuredRank > 0 {
		private := make(map[string]bool, len(in.PrivateRepositories))
		shorts := make(map[string]bool, len(in.PrivateRepositories))
		_, selfName := splitRepo(repo)
		for _, name := range splitWithheldIdentifiers(strings.Join(in.PrivateRepositories, ",")) {
			addPrivateRepoNames(private, shorts, repo, selfName, name)
		}
		withheld := splitWithheldIdentifiers(strings.Join(in.WithheldIdentifiers, ","))
		missingWithheld = len(withheld) == 0
		if missingWithheld {
			c.notice(standaloneWithheldNotice)
		}
		scanners.selfContain = func(surface, text string, opts SelfContainOpts) ([]scFinding, []string) {
			return scanSelfContainment(surface, text, opts, private, shorts, withheld)
		}
	}
	write := OutboundWrite{Repo: repo, Kind: OutboundKindComment, NumberHint: in.Number, Visibility: configured}
	findings, notices := scanOutboundField(write, OutboundField{Name: "body", Text: in.Body}, scanners)
	if len(findings) != 0 {
		first := findings[0]
		// Logs may be shared with a wider audience than the comment being checked.
		return fmt.Errorf("comment body refused by %s at line %d", first.rule, first.line)
	}
	// Desk notices can quote matched text. Report additional notices without
	// excerpts, excluding the missing-withheld diagnostic already sent above.
	if len(notices) > 1 || (len(notices) == 1 && !missingWithheld) {
		c.notice("NOTICE: publication scan flagged a possible private reference or personal-data shape")
	}
	return nil
}
