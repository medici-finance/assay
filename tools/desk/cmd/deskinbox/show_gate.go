package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The canonical emitter puts these two machine records at the head of its
// packet. Only use them to select the current verifier's output; later quoted
// markers, prose and commands in the thread have no routing authority.
var packetHead = regexp.MustCompile(`^<!-- verify-gate: ([^ <>\r\n]+) -->\n\n<!-- assay-card: (\{[^\r\n]*\}) -->\n`)
var revision = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type acceptancePacket struct {
	Repository string `json:"repository"`
	Purpose    string `json:"purpose"`
	Local      string `json:"local"`
	Source     string `json:"source_revision"`
	Run        string `json:"verification_revision"`
	SourceURL  string `json:"source_url"`
}

func readAcceptance(f deskkit.Forge, fr deskkit.ForgeRepo, iss *deskkit.Issue, a inboxAction) (inboxAction, error) {
	base := "https://github.com/" + fr.Slug()
	// The existing comment admission exception grants no permission to close
	// or trust arbitrary body text. A fixed workflow issuer, the canonical
	// packet and fresh landed evidence are independently required here.
	if !deskkit.EffectiveConfig().Configured() || iss.Author.Type != "Bot" || iss.Author.Login != "github-actions[bot]" || a.URL != fmt.Sprintf("%s/issues/%d", base, iss.Number) {
		return a.next("prepare", "verifier", "Verifier owes the workflow-issued sign-off card for this issue."), nil
	}
	body := strings.TrimSpace(strings.ReplaceAll(iss.Body, "\r\n", "\n"))
	m := packetHead.FindStringSubmatch(body)
	var packet acceptancePacket
	if len(m) != 3 || json.Unmarshal([]byte(m[2]), &packet) != nil || packet.Purpose != "verification-acceptance" || packet.Repository != fr.Slug() || packet.Local == "" || !revision.MatchString(packet.Source) || packet.Run == "" {
		return a.next("prepare", "verifier", "Verifier owes a sign-off card with the landed verification revision."), nil
	}
	commits, err := f.ListRecentCommits(fr, 1)
	if err != nil || len(commits) != 1 || !revision.MatchString(commits[0].SHA) {
		return a.next("unknown", "verifier", "Could not read the current default-branch revision."), nil
	}
	root := deskkit.RootForRepo(fr.Slug())
	if root == "" {
		if cwd, e := os.Getwd(); e == nil && deskkit.RepoSlugForDir(cwd) == fr.Slug() {
			root = cwd
		}
	}
	if root == "" || deskkit.RepoSlugForDir(root) != fr.Slug() {
		return a.next("unknown", "verifier", "Resolve the local checkout for the named repository before verifying this card."), nil
	}
	head := commits[0].SHA
	result := inspectGate(a, root, fr.Slug(), head, body, packet)
	if result.State == "ready" {
		after, err := f.ListRecentCommits(fr, 1)
		if err != nil || len(after) != 1 || after[0].SHA != head {
			return a.next("unknown", "verifier", "Refresh the default-branch observation; the verification snapshot changed or could not be confirmed."), nil
		}
	}
	return result, nil
}

func inspectGate(a inboxAction, root, repo, head, body string, packet acceptancePacket) inboxAction {
	reader, err := gitcore.Open(root)
	if err != nil {
		return a.next("unknown", "verifier", "Could not open the verification checkout.")
	}
	if why := gateCheckout(reader, head); why != "" {
		return a.next("unknown", "verifier", why)
	}
	// Compare against the same generator as the opening workflow rather than
	// declaring a PASS from an issue excerpt or duplicating its floor checks.
	bin, err := resolveFlowBin(statusgenBinEnv, "statusgen")
	if err != nil {
		return a.next("unknown", "verifier", "Could not resolve the statusgen verifier.")
	}
	out, _, err := runReaderFn(bin, []string{"--root", root, "--verify-issues"})
	var candidates []struct {
		Body string           `json:"body"`
		Card acceptancePacket `json:"card"`
	}
	if err != nil || json.Unmarshal(out, &candidates) != nil || candidates == nil {
		return a.next("unknown", "verifier", "Could not regenerate the current verification cards.")
	}
	for _, c := range candidates {
		if c.Card.Local == packet.Local && c.Card.Repository != repo {
			return a.next("unknown", "verifier", "The local verifier configuration identifies a different repository.")
		}
		if c.Card.Local != packet.Local || c.Card.Purpose != "verification-acceptance" {
			continue
		}
		// The emitter embeds the source commit in provenance links. Comparing
		// after substituting that full object ID tolerates unrelated commits
		// while detecting changed evidence, brief version, identity or options.
		if c.Card.Source != head || packet.Run != c.Card.Run || strings.ReplaceAll(body, packet.Source, head) != strings.TrimSpace(c.Body) {
			return a.next("stale", "verifier", "Refresh the card against the current landed brief and verification evidence.")
		}
		if ok, e := reader.IsAncestor(packet.Source, head); e != nil || !ok {
			return a.next("stale", "verifier", "The card's source revision is outside the current default-branch history.")
		}
		// Older emitters may omit the tracked close workflow or proof link.
		// Neither situation yields a runnable human action.
		if wf, e := reader.FileAt(head, ".github/workflows/verify-gate-close.yml"); e != nil || strings.TrimSpace(wf) == "" || c.Card.SourceURL == "" {
			return a.next("unknown", "verifier", "Could not verify the close workflow and its landed evidence link.")
		}
		_, diag, e := runReaderFn(bin, []string{"verify-gate-close", "--root", root, "--ref", c.Card.Local, "--dry-run"})
		if e != nil {
			return a.next("prepare", "verifier", "Resolve the close preflight: "+runeTrunc(cleanText(firstDiagnosticLine(diag)), 350))
		}
		if why := gateCheckout(reader, head); why != "" {
			return a.next("unknown", "verifier", why)
		}
		a.Evidence = packet.SourceURL + " (verification " + packet.Run + ")"
		a.Verify = "The desk reads the close workflow result and confirms the brief is done with the human review stamp on the default branch."
		return a.next("ready", "human", "Review the evidence and, if accepted, close "+a.URL+" using your authorized human account.")
	}
	return a.next("prepare", "verifier", "Verifier owes current eligible evidence and a matching sign-off card.")
}

func gateCheckout(repo *gitcore.Repo, head string) string {
	got, err := repo.HeadID()
	if err != nil || got != head {
		return "Sync the verification checkout to the observed default-branch revision."
	}
	dirty, err := repo.DirtyPorcelain()
	if err != nil || dirty != "" {
		return "Read the verifier inputs in a clean checkout of the observed default-branch revision."
	}
	return ""
}
