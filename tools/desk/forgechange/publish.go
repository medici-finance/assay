package forgechange

import (
	"errors"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type Publication struct {
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

func (c *Client) PublishComment(in Publication) error {
	if !shaPattern.MatchString(in.Head) {
		return errors.New("a full reviewed head is required to publish")
	}
	if strings.TrimSpace(in.Body) == "" || len(in.Body) > deskkit.OutboundBodyMaxBytes {
		return fmt.Errorf("comment body must contain between 1 and %d bytes of text", deskkit.OutboundBodyMaxBytes)
	}
	visibility, err := c.forge.RepoVisibility(c.repo)
	if err != nil {
		return operationError("read repository visibility", err)
	}
	if err := deskkit.CheckStandaloneChangeComment(in.Body, c.repo.Slug(), in.Number,
		in.MaxVisibility, visibility, in.HumanLogins, in.PrivateRepositories, in.WithheldIdentifiers); err != nil {
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
	ref, err := c.forge.PostCommentTyped(c.repo, in.Number, deskkit.TargetChange, in.Body)
	if err != nil {
		return operationError("post comment", err)
	}
	if ref == nil || ref.DatabaseID < 1 {
		return errors.New("forge did not confirm the posted comment")
	}
	return nil
}
