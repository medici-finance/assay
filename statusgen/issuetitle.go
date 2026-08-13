package main

import "unicode/utf8"

// maxIssueTitleLen is GitHub's issue-title limit: a longer title makes
// `gh issue create` fail with a 422, losing that issue from the batch.
// Shared by the verify-gate and needs-decision emitters.
const maxIssueTitleLen = 256

// issueTitle builds "<prefix><brief> — <title>" and truncates the result to
// maxIssueTitleLen runes (with a trailing ellipsis) so an over-long brief
// title can never 422 the issue-creation batch.
func issueTitle(prefix, brief, title string) string {
	t := prefix + brief + " — " + title
	if utf8.RuneCountInString(t) <= maxIssueTitleLen {
		return t
	}
	r := []rune(t)
	return string(r[:maxIssueTitleLen-1]) + "…"
}
