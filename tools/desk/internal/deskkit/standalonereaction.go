package deskkit

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// StandaloneReaction adds one reaction to a change at its reviewed head.
type StandaloneReaction struct {
	Number int
	Head   string
	// Content is a GitHub reaction value: laugh, confused, heart, hooray,
	// rocket or eyes. GitLab receives the matching award emoji. +1 and -1 are
	// refused: an App's thumbs-up on a change reads as an approval.
	Content       string
	MaxVisibility string // operator-authorized private, internal or public
}

// standaloneReactionAwards maps each admitted reaction to its GitLab award
// name. It is the inverse of gitlabAwardToReaction, which pairs GitHub's eight
// reactions with GitLab's emoji by glyph, so the two directions cannot drift.
// The approval-shaped +1 and -1 are left out, so they have no GitLab name and
// are refused before any request.
var standaloneReactionAwards = func() map[string]string {
	out := make(map[string]string, len(gitlabAwardToReaction))
	for award, content := range gitlabAwardToReaction {
		if content == "+1" || content == "-1" {
			continue
		}
		out[content] = award
	}
	return out
}()

// AddReaction reacts to a change after the same publication guard as
// PublishComment. A reaction this identity already gave counts as success.
func (c *StandaloneChanges) AddReaction(in StandaloneReaction) error {
	award, admitted := standaloneReactionAwards[in.Content]
	return c.publishToChange(standaloneChangeWrite{
		number: in.Number, head: in.Head, maxVisibility: in.MaxVisibility,
		validate: func() error {
			if !admitted {
				return errors.New("reaction must be one of laugh, confused, heart, hooray, rocket or eyes")
			}
			return nil
		},
		write: func() error {
			var err error
			switch f := c.forge.(type) {
			case *GitHubForge:
				err = f.addChangeReaction(c.repo, in.Number, in.Content)
			case *GitLabForge:
				err = f.addChangeAward(c.repo, in.Number, award)
			default:
				return errors.New("forge cannot add a change reaction")
			}
			if err != nil {
				return standaloneOperationError("add reaction", err)
			}
			return nil
		},
	})
}

// addChangeReaction posts one reaction to a pull request through the issues
// reactions endpoint. GitHub answers 201 for a new reaction and 200 with the
// existing one when this identity already gave it, so both are success.
func (g *GitHubForge) addChangeReaction(repo ForgeRepo, number int, content string) error {
	if g.onlyRepo != "" && !strings.EqualFold(g.onlyRepo, repo.Slug()) {
		return Refused("change reaction is outside the admitted repository")
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", repo.Owner, repo.Name, number)
	var out struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	}
	if err := g.doJSON(http.MethodPost, path, map[string]string{"content": content}, &out); err != nil {
		return err
	}
	if out.ID < 1 || out.Content != content {
		return Unverifiable("forge did not confirm the reaction", nil)
	}
	return nil
}

// addChangeAward awards one emoji on a merge request. GitLab refuses a repeat
// award from the same user with a 404 ("Name has already been taken"), the same
// status it uses for an unreadable merge request. A refused award therefore
// counts as success only when a read shows this user's award of that name.
func (g *GitLabForge) addChangeAward(repo ForgeRepo, number int, name string) error {
	if g.onlyRepo != "" && !strings.EqualFold(g.onlyRepo, repo.Slug()) {
		return Refused("change reaction is outside the admitted repository")
	}
	cl, err := g.client()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/award_emoji", g.projectPath(repo), number)
	award, _, perr := cl.AwardEmoji.CreateMergeRequestAwardEmoji(repo.Slug(), int64(number),
		&gitlab.CreateAwardEmojiOptions{Name: name})
	if perr == nil {
		if award == nil || award.ID < 1 || award.Name != name {
			return Unverifiable("forge did not confirm the award", nil)
		}
		return nil
	}
	mapped := g.mapErr(http.MethodPost, path, perr)
	var api *ForgeAPIError
	if !errors.As(mapped, &api) || (api.Status != http.StatusNotFound && api.Status != http.StatusConflict &&
		api.Status != http.StatusUnprocessableEntity) {
		return mapped
	}
	user, _, uerr := cl.Users.CurrentUser()
	if uerr != nil || user == nil || user.ID < 1 {
		return mapped
	}
	awards, _, lerr := cl.AwardEmoji.ListMergeRequestAwardEmoji(repo.Slug(), int64(number),
		&gitlab.ListAwardEmojiOptions{ListOptions: gitlab.ListOptions{PerPage: 100}})
	if lerr != nil {
		return mapped
	}
	for _, a := range awards {
		if a != nil && a.Name == name && a.User.ID == user.ID {
			return nil
		}
	}
	return mapped
}
