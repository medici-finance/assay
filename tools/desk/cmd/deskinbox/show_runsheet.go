package main

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Only an explicit, immutable, same-repo link selects a runsheet. An ordinary
// URL, a quoted command or a chat transcript never becomes an executable act.
var sheetLink = regexp.MustCompile(`(?im)^\s*\[runsheet\]\(([^)\s]+)\)\s*$`)

func readRunsheet(f deskkit.Forge, repo deskkit.ForgeRepo, iss *deskkit.Issue, a inboxAction, target string) (inboxAction, error) {
	u, err := url.Parse(target)
	base := "/" + repo.Slug() + "/blob/"
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || !strings.HasPrefix(u.Path, base) || u.RawPath != "" {
		return a.next("prepare", "desk", "Link the task's reviewed, commit-pinned runsheet in this repository."), nil
	}
	rev, file, ok := strings.Cut(strings.TrimPrefix(u.Path, base), "/")
	if !ok || !revision.MatchString(rev) || path.Clean(file) != file || strings.HasPrefix(file, "/") || strings.HasPrefix(file, "../") || !strings.HasSuffix(file, ".md") {
		return a.next("prepare", "desk", "Link the current revision of the task's Markdown runsheet."), nil
	}
	branch, err := f.RepoDefaultBranch(repo)
	if err != nil || branch == "" {
		return a.next("unknown", "desk", "Read the runsheet's default branch."), nil
	}
	pinned, err := f.ReadFile(repo, deskkit.ReadFileInput{File: file, Ref: rev})
	if err != nil || pinned == nil || !pinned.Exists || pinned.Kind != deskkit.FileKindRegular {
		return a.next("unknown", "desk", "Read the pinned runsheet as a committed regular file."), nil
	}
	current, err := f.ReadFile(repo, deskkit.ReadFileInput{File: file, Ref: branch})
	if err != nil || current == nil {
		return a.next("unknown", "desk", "Read the current runsheet from the default branch."), nil
	}
	if !current.Exists || current.Kind != deskkit.FileKindRegular || !bytes.Equal(current.Content, pinned.Content) {
		return a.next("stale", "desk", "Refresh the runsheet link to the current landed instructions."), nil
	}
	ref := fmt.Sprintf("%s#%d", repo.Slug(), iss.Number)
	text := string(current.Content)
	var commands []string
	for _, line := range strings.Split(text, "\n") {
		if cmd, ok := strings.CutPrefix(line, "! "); ok {
			commands = append(commands, strings.TrimSpace(cmd))
		}
	}
	_, resume, hasResume := strings.Cut(text, "Resume:")
	namesIssue := slices.Contains(strings.Fields(text), ref)
	oneCommand := len(commands) == 1 && commands[0] != "" && !strings.ContainsAny(commands[0], "<>")
	prose := strings.Join(strings.Fields(text), " ")
	hasPreparation := strings.Contains(prose, "What the session did:") && strings.Contains(prose, "Why:")
	if !namesIssue || !oneCommand || !hasPreparation || !hasResume || strings.TrimSpace(resume) == "" {
		return a.next("prepare", "desk", "Bind one complete human act, its authority and its post-act check to this issue in the runsheet."), nil
	}
	a.Evidence = target
	a.Verify = cleanText(strings.Join(strings.Fields(resume), " "))
	return a.next("ready", "human", "Perform the one reviewed act in "+target+" using the authorized account."), nil
}
