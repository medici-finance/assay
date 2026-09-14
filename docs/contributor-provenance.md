# Contributor provenance probe

`deskprovenance` (`tools/desk/cmd/deskprovenance`) gathers a fixed set of mechanical signals
about a pull request from an identity the roster does not know, and posts them as a comment on
that pull request. This document is the published description of what the card measures, and
what it deliberately does not — see
[docs/streams/contributor-trust/brief-01-provenance-probe.md](streams/contributor-trust/brief-01-provenance-probe.md)
for the brief this shipped against and
[docs/streams/decisions/DR-provenance-card.md](streams/decisions/DR-provenance-card.md) for the
design record.

## What the card is

A description of the **submission's shape and timing** — never of the person who sent it. It
renders no score, no rating and no conclusion: there is no field anywhere in the tool that a
caller could sum, average or otherwise fold into a single figure. Every signal is a cheap proxy
with an ordinary, innocent explanation, and the card states that explanation next to the
measurement rather than leaving the reader to supply it. The card always ends with the same
statement: it is not a judgement of the change, and the change is reviewed on its merits.

## What it measures

| Signal | What it is |
|---|---|
| Account age at first activity | How much time passed between the account being created and its first observed activity. |
| Fork-to-pull-request elapsed | How much time passed between the fork being created and this pull request being opened. |
| Cross-repository burst | How many pull requests this author opened across repositories in the preceding 24 hours. |
| Prior merged/closed ratio | What proportion of this author's earlier submissions were merged rather than closed. |
| Body-shape similarity | Whether this submission's description is near-identical in shape to one of the author's other recent submissions. |
| Commit signature | Whether the commits carried by this submission are signed. |
| Build/dependency paths touched | Whether the diff touches continuous-integration configuration, a dependency lockfile, an install script or a container definition. |

Every signal is a **three-state** measurement: it is either gathered (and, if it falls in a
notable band, shown with its ordinary explanation), or it could not be gathered, which is shown
as **could-not-check** — never rendered as "nothing to report", and never rounded up to a clean
result. A card carrying even one could-not-check signal is itself reported could-not-check, not
as a plain clean or flagged card with the gap left unstated.

## What it deliberately excludes

The card never measures anything that describes the **person** rather than the submission:
profile text, avatar, follower or following counts, named employer, geography, or the shape of
the account's name. None of these predicts whether a diff is correct; they correlate with
attributes this project must not sort contributors by, and a card that carried them would read
as a dossier on a person rather than a description of a submission. This exclusion is enforced
by a test (`TestProvenanceExcludedSignals` in `tools/desk/internal/deskkit/provenance_test.go`)
that scans every registered signal against a denylist of these categories, so a signal that
reads as identity-adjacent fails the build the moment it is added, whether or not its author
read this document first.

## What it never does

The tool never fetches, checks out, builds or executes the pull request's head. Every signal
comes from metadata — either a JSON fixture (for testing, or for a `--dry-run` read against a
real PR without posting) or a forge read of the pull request's own body and changed-file paths.
This is the same posture the repository's existing `inbound-triage.yml` advisory keeps for
exactly the same reason: reading metadata about an unknown author's submission never requires
running anything that submission contains.

## The card is replaced, not accumulated

Re-running the tool against the same pull request finds its own previous card (matched by an
exact marker line, `<!-- assay:provenance-card -->`) and edits it in place, so one pull request
never grows a column of stale cards.

## Pending: the human decision

Whether this card is ever posted publicly on a real pull request is a decision recorded in
[DR-provenance-card.md](streams/decisions/DR-provenance-card.md) — as of this writing that
record is **PROPOSED**, with no ruling recorded. `deskprovenance --dry-run` never posts
anything regardless, so building and testing this tool did not require that ruling; using it
against a real pull request does.
