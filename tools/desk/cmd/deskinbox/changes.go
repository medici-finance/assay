package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type changeOptions struct {
	consumer    string
	ack         string
	remindAfter time.Duration
}

func newChangesCommand(out, stderr io.Writer, now time.Time, pre func(io.Writer) int, observer *deskkit.RequestObserver) *cobra.Command {
	var opts changeOptions
	cmd := &cobra.Command{
		Use:   "changes [owner/repo ...]",
		Short: "report changes to human actions since this consumer's last delivery",
		Long:  "Print JSON with events and a receipt. After successful delivery, pass that receipt to --ack with the same consumer and repository set. An empty events array needs no acknowledgement. Withdrawn events cancel a previous request.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			deskkit.PerfCommand(perfVerbs, "changes")
			if cmd.Flags().Changed("ack") && opts.ack == "" {
				_, _ = fmt.Fprintln(stderr, "deskinbox: --ack needs a delivery receipt")
				return codeErr(deskkit.ExitRefused)
			}
			if strings.TrimSpace(opts.consumer) == "" || opts.remindAfter < 0 {
				_, _ = fmt.Fprintln(stderr, "deskinbox: provide a nonempty --consumer and a nonnegative --remind-after")
				return codeErr(deskkit.ExitRefused)
			}
			if code := begin(stderr, pre); code != deskkit.ExitOK {
				return codeErr(code)
			}
			if err := runChanges(out, args, now, opts, *observer); err != nil {
				_, _ = fmt.Fprintln(stderr, "deskinbox changes:", err)
				return codeErr(deskkit.ExitCodeOf(err))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.consumer, "consumer", "default", "stable delivery destination or reader name")
	cmd.Flags().StringVar(&opts.ack, "ack", "", "accept this receipt after successful delivery")
	cmd.Flags().DurationVar(&opts.remindAfter, "remind-after", 0, "repeat an outstanding action after this much time since its last delivery (0 disables)")
	return cmd
}

func runChanges(out io.Writer, args []string, now time.Time, opts changeOptions, observer deskkit.RequestObserver) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repos, err := resolveRepos(args, cwd)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return deskkit.Refused("no repositories resolved")
	}
	if err := validateRepos(repos); err != nil {
		return err
	}
	// Use the same canonical scope for the state file and for each read. An
	// invocation over a subset must never withdraw rows in the larger scope.
	repos = slices.Clone(repos)
	slices.Sort(repos)
	repos = slices.Compact(repos)
	root, err := deskkit.StateDir()
	if err != nil {
		return err
	}
	scope := digest(opts.consumer + "\x00" + strings.Join(repos, "\x00"))
	return withChangesState(filepath.Join(root, "inbox-changes", scope), func(path string, state *changeState) error {
		if opts.ack != "" {
			return acknowledgeChanges(path, state, opts.ack, now)
		}
		live, err := actionableSnapshot(repos, observer)
		if err != nil {
			return err
		}
		return deliverChanges(out, path, state, live, now, opts.remindAfter)
	})
}

func digest(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

// The payload includes the current issue title for context. Event identity uses
// the required act and its proof; cosmetic title edits cannot re-page a consumer.
type changeAction struct {
	rank      int
	createdAt string
	Title     string `json:"title,omitempty"`
	Owner     string `json:"owner"`
	Act       string `json:"act"`
	Verify    string `json:"verify,omitempty"`
	Evidence  string `json:"evidence,omitempty"`
	URL       string `json:"url,omitempty"`
}

var readChangeAction = readInboxAction

func actionableSnapshot(repos []string, observer deskkit.RequestObserver) (map[string]changeAction, error) {
	rows, failures := fetchQueueMatching(repos, func(labels []string) bool {
		return hasEscalationLabel(labels) || hasVerifyGateLabel(labels)
	}, observer)
	if len(failures) > 0 {
		f := failures[0]
		return nil, deskkit.Unverifiable("incomplete discovery for "+f.Repo, f.Err)
	}
	live := make(map[string]changeAction)
	for _, row := range rows {
		id := row.Repo + "#" + strconv.Itoa(row.Number)
		a, err := readChangeAction(row.Repo, row.Number, observer)
		if err != nil {
			return nil, deskkit.Unverifiable("incomplete observation for "+id, err)
		}
		switch a.State {
		case "ready":
			if a.Owner == "human" {
				live[id] = changeAction{
					rank: row.Rank, createdAt: row.CreatedAt,
					Title: a.Title, Owner: a.Owner, Act: a.Act, Verify: a.Verify, Evidence: a.Evidence, URL: a.URL,
				}
			}
		case "prepare", "stale", "closed", "screened":
			// Withdraw a previously announced act once it leaves the human.
		default:
			return nil, deskkit.Unverifiable(fmt.Sprintf("unverified action state for %s (%s): %s", id, a.Owner, a.Act), nil)
		}
	}
	return live, nil
}
