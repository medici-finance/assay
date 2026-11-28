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
	return c.publishToChange(standaloneChangeWrite{
		number: in.Number, head: in.Head, maxVisibility: in.MaxVisibility,
		validate: func() error {
			if strings.TrimSpace(in.Body) == "" || len(in.Body) > OutboundBodyMaxBytes {
				return fmt.Errorf("comment body must contain between 1 and %d bytes of text", OutboundBodyMaxBytes)
			}
			return nil
		},
		scan: func(configured Visibility) error { return c.checkComment(in, configured) },
		write: func() error {
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
		},
	})
}

// standaloneChangeWrite is one outward write to a change. Every standalone
// write verb goes through publishToChange so that each one gets the same
// visibility scope and head pin.
type standaloneChangeWrite struct {
	number        int
	head          string
	maxVisibility string
	// validate checks the verb's own input before any network access.
	validate func() error
	// scan checks verb content against the configured publication scope after
	// the live visibility is known to be inside it. Nil means no free text.
	scan func(configured Visibility) error
	// write runs only after every check passed for the current head.
	write func() error
}

// publishToChange runs the shared publication guard and then the write: a full
// reviewed head, the verb's input check, the live repository visibility within
// the operator-authorized scope, the verb's content scan, and an open change
// whose current head is the reviewed head.
func (c *StandaloneChanges) publishToChange(w standaloneChangeWrite) error {
	if !standaloneSHAPattern.MatchString(w.head) {
		return errors.New("a full reviewed head is required to publish")
	}
	if w.validate != nil {
		if err := w.validate(); err != nil {
			return err
		}
	}
	visibility, err := c.forge.RepoVisibility(c.repo)
	if err != nil {
		return standaloneOperationError("read repository visibility", err)
	}
	configured := ParseVisibility(w.maxVisibility)
	configuredRank := visibilityExposure(configured)
	liveRank := visibilityExposure(ParseVisibility(visibility))
	if liveRank < 0 || configuredRank < liveRank {
		return errors.New("repository visibility exceeds the configured publication scope or could not be verified")
	}
	if w.scan != nil {
		if err := w.scan(configured); err != nil {
			return err
		}
	}
	change, err := c.Change(w.number)
	if err != nil {
		return err
	}
	if change.State != "open" && change.State != "opened" {
		return errors.New("change is no longer open")
	}
	if change.HeadSHA != w.head {
		return fmt.Errorf("reviewed head %s differs from current head %s", w.head, change.HeadSHA)
	}
	return w.write()
}

// checkComment scans a comment body. publishToChange has already checked that
// the live visibility is inside the configured scope.
func (c *StandaloneChanges) checkComment(in StandalonePublication, configured Visibility) error {
	configuredRank := visibilityExposure(configured)
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
