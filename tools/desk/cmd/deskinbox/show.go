package main

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// inboxAction is also the observation consumed by changes. An unknown read is
// never evidence that an outstanding action has been completed.
type inboxAction struct {
	State, Owner, Title, URL, Act, Evidence, Verify string
	command                                         string // single verified runsheet line, displayed only by show
	awaitsPacket                                    bool   // no ruling or fork to carry; a reviewed runsheet can supply the act
}

func (a inboxAction) next(state, owner, act string) inboxAction {
	a.State, a.Owner, a.Act = state, owner, act
	return a
}

func showCommand(stdout, stderr io.Writer, pre func(io.Writer) int, observer func() deskkit.RequestObserver) *cobra.Command {
	return &cobra.Command{
		Use: "show owner/repo#issue", Short: "show who owns the next action on one issue", Args: cobra.ExactArgs(1),
		Long: `Look up the named issue in the current desk role. Reports a ready human action, desk or verifier preparation, a prior ruling, stale evidence, an unreadable observation, or closure.

Verification uses statusgen and a clean checkout at the observed default branch. DESK_ROOTS maps owner/repo to the checkout; a matching working directory can also supply it. For a prepared human act, link the reviewed Markdown as a standalone [Runsheet](https://github.com/owner/repo/blob/COMMIT/path.md) line. The file must name this issue and contain one complete human-runsheet entry.

Exit 6 means a required observation could not be checked.`,
		RunE: func(_ *cobra.Command, args []string) error {
			repo, raw, ok := strings.Cut(args[0], "#")
			n, err := strconv.Atoi(raw)
			if !ok || validateRepos([]string{repo}) != nil || slices.ContainsFunc(strings.Split(repo, "/"), func(p string) bool { return p == "." || p == ".." }) || n <= 0 || err != nil || raw != strconv.Itoa(n) {
				return deskkit.Refused("expected owner/repo#issue with a positive decimal issue number")
			}
			deskkit.PerfCommand(perfVerbs, "show")
			if code := begin(stderr, pre); code != deskkit.ExitOK {
				return codeErr(code)
			}
			a, err := readInboxAction(repo, n, observer())
			if err != nil {
				fmt.Fprintf(stderr, "deskinbox: %s#%d could-not-check: %s\n", repo, n, cleanText(err.Error()))
				return codeErr(deskkit.ExitCodeOf(err))
			}
			fmt.Fprintf(stdout, "%s#%d — %s\nState: %s\nNext owner: %s\nNext action: %s\n", repo, n, a.Title, a.State, a.Owner, a.Act)
			if a.command != "" {
				fmt.Fprintln(stdout, "Command:")
				fmt.Fprintln(stdout, a.command)
			}
			for _, line := range [][2]string{{"Issue", a.URL}, {"Evidence", a.Evidence}, {"Desk verification", a.Verify}} {
				if line[1] != "" {
					fmt.Fprintf(stdout, "%s: %s\n", line[0], line[1])
				}
			}
			if a.State == "unknown" {
				return codeErr(deskkit.ExitUnverifiable)
			}
			return nil
		},
	}
}

func readInboxAction(repo string, number int, observers ...deskkit.RequestObserver) (inboxAction, error) {
	f, fr, err := forgeFor(repo, observers...)
	if err != nil {
		return inboxAction{}, err
	}
	iss, err := f.GetIssueTyped(fr, number, deskkit.TargetIssue)
	if err != nil {
		return inboxAction{}, deskkit.Unverifiable("read named issue", err)
	}
	if iss == nil || iss.Number != number || iss.IsPullRequest {
		return inboxAction{}, deskkit.Unverifiable("forge did not return the named issue", nil)
	}
	a := inboxAction{Title: cleanText(iss.Title), URL: cleanText(iss.URL)}
	switch strings.ToLower(iss.State) {
	case "closed":
		return a.next("closed", "none", "Issue is closed."), nil
	case "open", "opened":
	default:
		return a.next("unknown", "desk", "Read the issue's current open or closed state."), nil
	}
	acceptance := hasVerifyGateLabel(iss.Labels)
	if !acceptance && !slices.ContainsFunc(labels, func(l string) bool { return hasInboxLabel(iss.Labels, l) }) {
		return a.next("prepare", "desk", "No escalation or verification gate is present."), nil
	}
	if !deskkit.EffectiveConfig().Configured() {
		return a.next("unknown", "desk", "Could not evaluate the issue's authority without a configured roster."), nil
	}
	if acceptance && hasInboxLabel(iss.Labels, "needs-decision") {
		return a.next("prepare", "desk", "Separate the unresolved decision from its verification sign-off card."), nil
	}
	// Never promote body text to an action when its current authorship is
	// unclear. A forge workflow's verify card must additionally match the
	// canonical generated packet and pass the live close preflight below.
	if !acceptance && (iss.Author.ID == 0 || !deskkit.TrustedAuthorID(iss.Author.Login, iss.Author.ID)) {
		return a.next("prepare", "desk", "Obtain the decision from a roster-trusted author."), nil
	}
	history, err := f.IssueTrustEvents(fr, number)
	if err != nil || history == nil || !history.BodyHistoryKnown {
		return a.next("unknown", "desk", "Could not verify the issue body's edit history."), nil
	}
	if !history.BodyEdited.IsZero() {
		return a.next("prepare", "desk", "Revalidate the edited issue packet and its authority."), nil
	}
	if acceptance {
		return readAcceptance(f, fr, iss, a)
	}
	cs, err := f.ListCommentsTyped(fr, number, deskkit.TargetIssue)
	if err != nil {
		return a.next("unknown", "desk", "Could not read the complete decision thread."), nil
	}
	action := screenDecision(iss, a, cs)
	if action.awaitsPacket && !hasInboxLabel(iss.Labels, "needs-decision") {
		links := sheetLink.FindAllStringSubmatch(visibleDecisionText(iss.Body), -1)
		if len(links) == 1 {
			return readRunsheet(f, fr, iss, a, links[0][1])
		}
	}
	return action, nil
}

func hasInboxLabel(names []string, want string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(strings.TrimSpace(name), want) })
}

func hasVerifyGateLabel(names []string) bool { return hasInboxLabel(names, deskkit.VerifyGateLabel) }

var (
	optionHeading = regexp.MustCompile(`(?i)^\s*#{1,6}\s*options?\b`)
	optionList    = regexp.MustCompile(`^(?:[-*+]\s|(?:\*\*)?[A-Za-z0-9][.)])`)
	rulingLetter  = regexp.MustCompile(`(?i)^(?:option\s+)?([A-D1-4])(?:$|[.)!:,]|\s*[—–-])`)
	ratifies      = regexp.MustCompile(`(?i)^(?:i\s+)?(?:ratif(?:y|ied)|approved?)\b`)
	rulingHold    = regexp.MustCompile(`(?i)\?|\b(no|not|hold|wait|later|undecided|unsure|pending)\b|n['’]t\b`)
	classField    = regexp.MustCompile(`(?i)\bclass[ \t]*:`)
	oneWay        = regexp.MustCompile(`(?i)\b(one-way|irreversible)\b`)
)

func screenDecision(iss *deskkit.Issue, a inboxAction, cs []deskkit.Comment) inboxAction {
	raw := iss.Body
	body := visibleDecisionText(raw)
	options := deskkit.ParseDecisionOptions(body)
	// A status update or repetition of the same options cannot erase a
	// human's ruling. Only a newly proposed fork moves the anchor.
	anchor := -1
	for i, c := range cs {
		if c.Minimized || c.Author.ID == 0 || c.Author.Type != "Bot" || !deskkit.TrustedAuthorID(c.Author.Login, c.Author.ID) {
			continue
		}
		proposal := visibleDecisionText(c.Body)
		asked := deskkit.ParseDecisionOptions(proposal)
		if asked.Stated() && (!slices.Equal(asked.Letters, options.Letters) || !slices.Equal(asked.Texts, options.Texts)) {
			if c.UpdatedAt == "" || c.UpdatedAt != c.CreatedAt {
				return a.next("unknown", "desk", "Reconcile the changed decision's edit history.")
			}
			raw = iss.Body + "\n" + c.Body
			anchor, body, options = i, proposal, asked
		}
	}
	for i := len(cs) - 1; i > anchor; i-- {
		c := cs[i]
		if c.Minimized || c.Author.Type != "User" || !deskkit.IsBlessAuthorityIDStrict(c.Author.Login, c.Author.ID) {
			continue
		}
		spoke, err := time.Parse(time.RFC3339, c.CreatedAt)
		edit, editErr := time.Parse(time.RFC3339, c.UpdatedAt)
		if err != nil || editErr != nil || !spoke.Equal(edit) {
			return a.next("unknown", "desk", "Reconcile the ratifying identity's edited or undated response.")
		}
		lines := nonblankLines(bodyLines(visibleDecisionText(c.Body)))
		if len(lines) > 0 && (anchor >= 0 || options.Stated()) {
			first := strings.TrimSpace(demd(lines[0]))
			match := rulingLetter.FindStringSubmatch(first)
			if !rulingHold.MatchString(strings.Join(lines, " ")) && (ratifies.MatchString(first) || len(match) > 1 && slices.Contains(options.Letters, strings.ToUpper(match[1]))) {
				a.Evidence = cleanText(c.URL)
				return a.next("screened", "desk", "Already ruled. Record the ruling and update the issue.")
			}
		}
		// A hold, a question or an ambiguous reply needs desk reconciliation.
		return a.next("prepare", "desk", "Reconcile the ratifying identity's latest reply before asking again.")
	}
	// Only visible prose can describe an action. A packet with markup or
	// code examples needs desk preparation before interpreting its options.
	visible := bodyLines(body)
	var heading bool
	for _, l := range visible {
		heading = heading || optionHeading.MatchString(l)
	}
	listLines := 0
	for _, l := range section(visible, optionHeading) {
		if optionList.MatchString(strings.TrimSpace(l)) {
			listLines++
		}
	}
	fields := map[string][]string{}
	for _, l := range visible {
		key, value, ok := strings.Cut(strings.TrimSpace(demd(l)), ":")
		if ok {
			fields[strings.ToLower(key)] = append(fields[strings.ToLower(key)], strings.TrimSpace(value))
		}
	}
	// Positive evidence must be visible. Risk declarations are vetoes: Markdown
	// parsing must never hide them and thereby promote an item to a default.
	source := demd(raw)
	veto := classField.MatchString(source) || oneWay.MatchString(source)
	if !veto && (heading && len(options.Texts) == 1 && listLines == 1 || !heading && len(fields["option"]) == 1 && fields["option"][0] != "") {
		return a.next("screened", "desk", "No fork. Proceed with the single stated option or route it to its owner.")
	}
	if caught, def := fields["caught-by"], fields["default"]; len(caught) == 1 && caught[0] != "" && !strings.HasPrefix(strings.ToLower(caught[0]), "nothing") && len(def) == 1 && def[0] != "" && !veto {
		a.Evidence = "Default: " + cleanText(def[0]) + "; caught by: " + cleanText(caught[0])
		return a.next("screened", "desk", "Apply the documented reversible default.")
	}
	if !hasInboxLabel(iss.Labels, "needs-decision") || len(options.Texts) < 2 || len(options.Texts) != listLines || a.URL == "" {
		a.awaitsPacket = true
		return a.next("prepare", "desk", "Prepare the unresolved decision and its options or link the verified human runsheet.")
	}
	seen := map[string]bool{}
	var choices []string
	for i, text := range options.Texts {
		l := options.Letters[i]
		if seen[l] {
			return a.next("prepare", "desk", "Give each option a distinct reply letter.")
		}
		seen[l] = true
		if i == options.Recommended {
			text += " (recommended)"
		}
		choices = append(choices, l+". "+cleanText(text))
	}
	a.Evidence = strings.Join(choices, " | ")
	a.Verify = "The desk reads the authorized person's ruling on this issue, records it, and confirms the escalation label moved."
	return a.next("ready", "human", "Reply on "+a.URL+" with your chosen option ("+strings.Join(options.Letters, "/")+").")
}

// Ignore HTML comments, fenced samples, block quotes and indented code when
// screening for a ruling, an option or the fields of a reversible default.
func visibleDecisionText(body string) string {
	var lines []string
	var fence string
	var hidden bool
	for _, line := range toLines(body) {
		// A comment closes even on an indented or quoted line. Consume its
		// terminator before deciding how Markdown treats the remaining text.
		if hidden {
			_, tail, ok := strings.Cut(line, "-->")
			if !ok {
				continue
			}
			line, hidden = tail, false
		}
		trim := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trim, fence) && strings.Trim(trim, fence[:1]) == "" {
				fence = ""
			}
			continue
		}
		if !hidden && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
			fence = trim[:len(trim)-len(strings.TrimLeft(trim, trim[:1]))]
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(trim, ">") {
			continue
		}
		var visible strings.Builder
		for len(line) > 0 {
			if hidden {
				_, tail, ok := strings.Cut(line, "-->")
				if !ok {
					break
				}
				line = tail
				hidden = false
			} else {
				part, tail, ok := strings.Cut(line, "<!--")
				visible.WriteString(part)
				if !ok {
					break
				}
				line = tail
				hidden = true
			}
		}
		lines = append(lines, visible.String())
	}
	return strings.Join(lines, "\n")
}
