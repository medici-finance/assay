package main

import (
	"strings"
	"testing"
)

const contextNote = " (isolated test host)"

func contextEdit(stage, body string) string {
	if stage == "original" {
		return strings.Replace(body, "`sh check.sh`", "`sh check.sh`"+contextNote, 1)
	}
	return body
}

// regression: #3492
func TestTransitionContext(t *testing.T) {
	cases := []string{"suffix", "prefix", "second-row", "both-extra", "both-hidden", "both-unclosed", "both-math", "changed", "removed", "added", "spacing", "moved", "delivery-context", "extra-span", "unclosed", "hidden", "escaped", "prose-led", "expect", "class", "obligation", "shell", "supplement"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := contextFixture(t, nil, func(stage, body string) string {
				body = contextEdit(stage, body)
				if stage == "original" {
					body = strings.Replace(body, "| Expect |", "| Expect | Class | Shell |", 1)
					body = strings.Replace(body, "| exit 1 |", "| exit 1 | check +flow | sh |", 1)
				}
				if stage == "original" {
					switch name {
					case "both-extra":
						body = strings.Replace(body, contextNote, contextNote+" `true`", 1)
					case "both-hidden":
						body = strings.Replace(body, "`sh check.sh`", "<!-- `sh check.sh` -->", 1)
					case "both-unclosed":
						body = strings.Replace(body, contextNote, contextNote+" `dangling", 1)
					case "both-math":
						body = strings.Replace(body, "`sh check.sh`", "$`sh check.sh`$", 1)
					}
				}
				if name == "prefix" {
					body = strings.Replace(body, "| `sh check.sh", "| On an isolated host: `sh check.sh", 1)
				}
				if name == "second-row" && stage == "original" {
					body = strings.Replace(body, "| 1 | `true` |", "| 1 | `test 1 = 1` (second host) |", 1)
				}
				if name == "delivery-context" && stage == "delivery" {
					body = strings.Replace(body, contextNote, " (different host)", 1)
				}
				if stage != "amendment" {
					return body
				}
				switch name {
				case "second-row":
					body = strings.Replace(body, "`test 1 = 1`", "`test 2 = 2`", 1)
				case "changed":
					body = strings.Replace(body, contextNote, " (shared host)", 1)
				case "removed":
					body = strings.Replace(body, contextNote, "", 1)
				case "added":
					body = strings.Replace(body, contextNote, contextNote+" extra", 1)
				case "spacing":
					body = strings.Replace(body, contextNote, "  (isolated test host)", 1)
				case "moved":
					body = strings.Replace(body, "`sh check.sh --no-pull`"+contextNote, contextNote+" `sh check.sh --no-pull`", 1)
				case "extra-span":
					body = strings.Replace(body, contextNote, contextNote+" `true`", 1)
				case "unclosed":
					body = strings.Replace(body, contextNote, contextNote+" `dangling", 1)
				case "hidden":
					body = strings.Replace(body, "`sh check.sh --no-pull`", "<!-- `sh check.sh --no-pull` -->", 1)
				case "escaped":
					body = strings.Replace(body, "`sh check.sh --no-pull`", "\\`sh check.sh --no-pull\\`", 1)
				case "prose-led":
					body = strings.Replace(body, "`sh check.sh --no-pull`", "Inspect `source.go`", 1)
				case "expect":
					body = strings.Replace(body, "| exit 1 |", "| exit 0 |", 1)
				case "class":
					body = strings.Replace(body, "check +flow", "check:ci +flow", 1)
				case "obligation":
					body = strings.Replace(body, "check +flow", "check", 1)
				case "shell":
					body = strings.Replace(body, "| sh |", "| pwsh |", 1)
				case "supplement":
					body = strings.Replace(body, "`printf supplemental`", "`printf replaced`", 1)
				}
				return body
			})
			// This test uses immutable Git history and a local synthetic approval only.
			if name == "second-row" {
				f.payload = strings.Replace(f.payload, "rows=2", "rows=1,2", 1)
				login, _ := HumanLogin("alex")
				f.forge.commentBody = rlCommentJSONWith(login, "User", rlRepo, rlIssue, f.payload, rlPosted, rlPosted)
			}
			read := func(rev string) (string, error) {
				raw, err := historyGit(f.root, "show", "--end-of-options", rev+":"+vfBriefRel).Output()
				return extractSectionByPrefix(string(raw), "Verify"), err
			}
			baseline, err := read(f.base)
			if err != nil {
				t.Fatal(err)
			}
			delivered, err := read(f.delivery)
			if err != nil {
				t.Fatal(err)
			}
			amended, err := read(f.amendment)
			if err != nil {
				t.Fatal(err)
			}
			want := name == "suffix" || name == "prefix" || name == "second-row"
			if !want {
				transitionClient = func() *ghClient { t.Fatal("invalid context reached approval lookup"); return nil }
			}
			effective, revisions, why := acceptanceTransition(f.path, amended, f.history, f.descriptor, f.delivery, f.base, f.base, baseline, delivered, false, read)
			if (why == "") != want {
				t.Fatalf("accepted=%v want=%v: %s", why == "", want, why)
			}
			if !want {
				return
			}
			if !strings.Contains(effective, "`sh check.sh --no-pull`"+contextNote) || revisions["2"] != f.amendment {
				t.Fatalf("lost command context or revision: %s %+v", effective, revisions)
			}
			if name == "second-row" && (!strings.Contains(effective, "`test 2 = 2` (second host)") || revisions["1"] != f.amendment) {
				t.Fatalf("lost second-row context: %s", effective)
			}
		})
	}
}

// regression: #3492
func TestContextExecution(t *testing.T) {
	runTransitionExecution(t, contextFixture(t, nil, contextEdit))
}
