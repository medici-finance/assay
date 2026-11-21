package main

// GitLab ruling links (#7252).
//
// A gate: human brief or a design-decision record on a GitLab-hosted repository
// records its ruling as a GitLab issue note:
//
//	https://<gitlab-host>/<group>[/<subgroup>...]/<project>/-/issues/<N>#note_<ID>
//
// The link is bound exactly as the GitHub form is: the host must be the
// configured GitLab host (the host deskread reads — GITLAB_API_BASE, else
// gitlab.com), the project must be the record's own repository, and online the
// note is read through deskread (the seam the GitLab auto-flip reads through,
// forgeread.go) and held to the same author, edit, record-naming and
// decision-issue rules. Every read that fails is a refusal, never a pass.

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// gitlabRulingURLGrammar is the GitLab ruling-link form, quoted in messages.
const gitlabRulingURLGrammar = "https://<gitlab-host>/<group>[/<subgroup>...]/<project>/-/issues/<N>#note_<ID>"

// gitlabRulingURLRe is the GitLab form as a regexp, anchored at both ends. A
// merge-request note (/-/merge_requests/), a bare issue URL, a port or user info
// on the host, or trailing text are all malformed.
var gitlabRulingURLRe = regexp.MustCompile(`^https://([A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?)/((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+)/-/issues/([1-9][0-9]*)#note_([1-9][0-9]*)$`)

// gitlabRulingHost is the one host a GitLab ruling link may name: the host of
// GITLAB_API_BASE, else gitlab.com — the host deskread reads the note from, so
// the note checked is always the note the link names. "" refuses every GitLab
// link (an API base this build cannot read a host from).
func gitlabRulingHost() string {
	base := strings.TrimSpace(os.Getenv("GITLAB_API_BASE"))
	if base == "" {
		return "gitlab.com"
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Port() != "" || u.User != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// parseGitLabRulingURL parses the GitLab form, on the configured host only. A
// link on any other host is malformed, as a GitHub-form link off github.com is.
func parseGitLabRulingURL(raw string) (rulingLink, bool) {
	m := gitlabRulingURLRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return rulingLink{}, false
	}
	host := gitlabRulingHost()
	if host == "" || !strings.EqualFold(m[1], host) {
		return rulingLink{}, false
	}
	for _, seg := range strings.Split(m[3], "/") {
		if seg == "-" || seg == "." || seg == ".." {
			return rulingLink{}, false
		}
	}
	n, err1 := strconv.Atoi(m[4])
	id, err2 := strconv.ParseInt(m[5], 10, 64)
	if err1 != nil || err2 != nil {
		return rulingLink{}, false
	}
	return rulingLink{Repo: m[3], Issue: n, CommentID: id, GitLab: true, Host: host}, true
}

// configuredForgeFn is the forge the roster binds the review identity to — the
// same forge configuration the GitLab auto-flip reads (modelReviewer, #7246).
// Ruling links and reconcile both key on it. A seam for tests.
var configuredForgeFn = func() forgeKind { return modelReviewer().Forge }

// deskreadReaderFn is the deskread seam GitLab rulings and reconcile read
// through. A seam for tests.
var deskreadReaderFn = newDeskreadReader

// gitlabBotLoginRe is GitLab's bot-user username shape: project and group access
// tokens act as project_<id>_bot[_<hex>] / group_<id>_bot[_<hex>].
var gitlabBotLoginRe = regexp.MustCompile(`^(project|group)_[0-9]+_bot(_[0-9a-f]+)?$`)

// gitlabRulingBot reports whether a GitLab note author is a bot: GitHub's
// suffix, GitLab's bot-user shape, a stated account type other than User, or a
// login the roster binds to a bot identity. It is condition 3 only: an EMPTY
// type is not a bot here, and is refused by condition 3b, never read as User.
func gitlabRulingBot(login, typ string) bool {
	l := strings.ToLower(login)
	if strings.HasSuffix(l, "[bot]") || gitlabBotLoginRe.MatchString(l) {
		return true
	}
	if t := strings.TrimSpace(typ); t != "" && !strings.EqualFold(t, "User") {
		return true
	}
	for _, b := range scanEffectiveConfig().BotIdents {
		if loginInSet(b.acceptedLogins(), login) {
			return true
		}
	}
	return false
}

// resolveGitLabRuling is resolveRulingOn's GitLab arm, entered after the link's
// grammar, host and repository have been checked. The conditions and their
// order are the GitHub arm's (1, 2b, 3, 3b, 4, 5, 6; condition 2's parent check
// is the read itself, the note is looked up in the linked issue's own thread);
// only the reads differ.
//
// Condition 3b binds here too: a note whose author account type was not read is
// unresolvable-link, never read as User (deskread's CommentJSON contract). The
// deskread GitLab backend does not report a note author's account type yet, so
// until it does every GitLab ruling note stops at 3b and the record stays
// unruled: the arm is closed, never open on a guess about who wrote the note.
func resolveGitLabRuling(out rulingOutcome, link rulingLink, rec decisionRecordStamp, humanLogins map[string]string, binding rulingIssueBinding) rulingOutcome {
	refuse := func(r rulingReason, format string, a ...any) rulingOutcome {
		out.Reason = r
		out.Detail = fmt.Sprintf(format, a...)
		return out
	}
	if f := configuredForgeFn(); f != forgeGitLab {
		return refuse(rulingUnrelatedIssue, "ruling links a GitLab note on %s, but the roster binds this repository's review identity to %s, not gitlab — a ruling is read on the configured forge only", link.Host, f)
	}
	d := deskreadReaderFn()
	where := fmt.Sprintf("%s#%d", link.Repo, link.Issue)

	// 1. The note, read from the issue's complete thread: a note on another
	// issue is absent here, so the link cannot pair issue #A with a note of #B.
	env, i, err := d.readItem("comments", link.Repo, link.Issue)
	if err != nil {
		return refuse(rulingUnresolvable, "reading note %d on %s: %v", link.CommentID, where, err)
	}
	cs := env.Items[i].Comments
	if cs == nil {
		return refuse(rulingUnresolvable, "reading note %d on %s: the item carries no comment list", link.CommentID, where)
	}
	found := -1
	for j, c := range *cs {
		if c.DatabaseID == link.CommentID {
			found = j
			break
		}
	}
	if found < 0 {
		return refuse(rulingDeleted, "note %d is not on %s — the ruling note is deleted, never existed, or sits on another issue", link.CommentID, where)
	}
	cm := (*cs)[found]
	if cm.AuthorLogin == "" {
		return refuse(rulingUnresolvable, "note %d on %s names no author", link.CommentID, where)
	}
	out.Author = cm.AuthorLogin
	bot := gitlabRulingBot(cm.AuthorLogin, cm.AuthorType)
	if !bot {
		out.ObservedHuman = humanNameForLogin(humanLogins, cm.AuthorLogin)
		if out.ObservedHuman != "" {
			out.ObservedBody = cm.Body
		}
	}

	// 2b. The note is unedited; unreadable times are could-not-check.
	created, cerr := time.Parse(time.RFC3339, cm.CreatedAt)
	updated, uerr := time.Parse(time.RFC3339, cm.UpdatedAt)
	if cerr != nil || uerr != nil {
		return refuse(rulingUnresolvable, "note %d on %s: createdAt %q / updatedAt %q did not parse — whether the note was edited after it was posted could not be checked", link.CommentID, where, cm.CreatedAt, cm.UpdatedAt)
	}
	if !updated.Equal(created) {
		return refuse(rulingEditedComment, "note %d on %s was edited after it was posted (createdAt %s, updatedAt %s) — link an unedited ruling note; to correct a ruling, the human posts a new note", link.CommentID, where, cm.CreatedAt, cm.UpdatedAt)
	}

	// 3. A bot never rules: a stated type other than User, GitLab's bot-user
	// login shape, GitHub's [bot] suffix, or a roster bot identity. Checked
	// before 3b so a known bot is named as one even when its type is unread.
	if bot {
		return refuse(rulingBotAuthor, "note %d is authored by %s (type %q) — a bot account is not a human allowed to decide", link.CommentID, cm.AuthorLogin, cm.AuthorType)
	}
	// 3b. Only a personal account rules: the account type must read User. A
	// type the read leaves out could not be checked — never read as User.
	if strings.TrimSpace(cm.AuthorType) == "" {
		return refuse(rulingUnresolvable, "note %d on %s: the read names no account type for %s — whether a personal account wrote it could not be checked (the GitLab note read does not report an author account type yet)", link.CommentID, where, cm.AuthorLogin)
	}

	// 4. The author maps to a human allowed to decide.
	name, why := rulingAuthorName(rec, humanLogins, cm.AuthorLogin)
	if why != "" {
		return refuse(rulingWrongAuthor, "note %d is authored by %s, %s", link.CommentID, cm.AuthorLogin, why)
	}
	out.Name = name

	// 5. The note names THIS record, in text the human wrote.
	if !commentNamesRecord(cm.Body, rec.ID) {
		return refuse(rulingRecordNotNamed, "note %d on %s does not name the record %s — the ruling note must name the record it rules on", link.CommentID, where, rec.ID)
	}
	if binding.comment != nil {
		if why := binding.comment(cm.Body); why != "" {
			return refuse(rulingRecordNotNamed, "note %d on %s %s", link.CommentID, where, why)
		}
	}

	// 6. The issue is a decision issue for THIS record. deskread refuses a
	// merge request read under the issue kind.
	ienv, ii, ierr := d.readItem("issue", link.Repo, link.Issue)
	if ierr != nil {
		return refuse(rulingUnresolvable, "reading issue %s: %v", where, ierr)
	}
	iss := ienv.Items[ii].Issue
	if iss == nil {
		return refuse(rulingUnresolvable, "reading issue %s: the item carries no issue", where)
	}
	if !binding.rules(rulingIssue{Body: iss.Body}, link) {
		return refuse(rulingUnrelatedIssue, "%s carries no decision-gate marker for %s — the note is not on this record's decision issue", where, binding.what)
	}

	out.Reason = rulingOK
	out.Body = cm.Body
	out.Detail = rec.Ruling
	if cm.URL != "" {
		out.Detail = cm.URL
	}
	return out
}
