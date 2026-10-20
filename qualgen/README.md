# qualgen

`qualgen` mines Git history into inspectable quality artifacts. It runs locally,
reads the target repository, and writes only below an explicit tracking root.
Use the release binary for your platform, or build this independent Go module:

```sh
go build -o qualgen .
./qualgen mine --repo /path/to/repository --out /path/to/tracking-root
./qualgen report --out /path/to/tracking-root
```

`report` renders Markdown to stdout. `pr <number>` emits JSON file features;
`check <paths>` emits advisory notices. Help documents the options. No tracker,
telemetry collection, credential lookup or network query runs implicitly.

## Defect tracing from recorded issue and PR metadata

```sh
qualgen mine --repo /path/to/repository --out /path/to/tracking-root \
  --fix-source /path/to/fix-source.json --defect-labels bug,defect,incident
qualgen report --out /path/to/tracking-root
```

`--fix-source` enables fix identification, the existing B-SZZ trace and derived
metrics. `--defect-labels` replaces the configured label set (case-insensitive),
defaulting to `bug,defect,incident`. The source must be one JSON object:

```json
{
  "schema_version": "qualgen-fix-source-v1",
  "repository": "example/project",
  "head_sha": "FULL_LOWERCASE_HEAD_SHA",
  "issues": [
    {"number": 7, "labels": ["bug"], "reported_at": "2026-01-03T00:00:00Z"}
  ],
  "pull_requests": [
    {"number": 2, "merge_sha": "FULL_LOWERCASE_MERGE_SHA",
     "branch": "fix/divisor", "title": "Correct divisor", "body": "Closes #7"}
  ],
  "merged_pr_count": 2
}
```

Replace the two SHA placeholders with full commit IDs. `head_sha` must match the
local HEAD. Supplied PR merge commits must be on its first-parent history. The
exporter is responsible for selecting the correct repository's tracker data;
the HEAD binding catches stale or different-history exports, not forged data or
forks sharing the same commit. `repository` records the declared provenance.

Issue records require positive, unique numbers and explicit label arrays. An
empty label array means the issue was read and has no labels; an absent issue
means its classification is unavailable. `reported_at` is optional; without it,
the report-date refinement is inapplicable. Unknown fields, duplicate PR numbers
or merge SHAs, malformed dates and incompatible history are errors.

PR records are optional. They make closing references in PR bodies and PR
taxonomy available even when the Git commit message omits them. Ordinary
commits use their own messages. Only first-parent landed changes are classified,
so a branch fix and its merge are not counted twice. Closing references follow
the existing adapter's unqualified `Fixes #N` / `Closes #N` forms. The three
evidence tiers remain separate: defect-labelled issue, PR taxonomy, keyword
fallback. Keyword matches are candidates, not confirmed causal findings.

`merged_pr_count` is an optional **full-history** denominator supplied by the
exporter. Omit it or set it to zero when unavailable. It must not be the number
of fixes or a partial-page count. The existing inducing-rate definition is
distinct inducing commits / merged PRs; it can exceed 1 and is not a probability.
Unresolved identification and wholly untraceable fix sets cannot produce a
measured inducing rate. Trace rate, evidence tiers and missing counts accompany
the results. This is historical attribution advice, not a causal proof or gate.

### Producing a GitHub snapshot

Collection is a separate, explicit operation using your own authorised account.
Choose the repository and local target, and keep exports in a private directory
when the repository is private. Fetch complete pages successfully before building
the snapshot; a failed collection must not be treated as an empty export.

```sh
GH_TOKEN="$(gh auth token --hostname github.com --user YOUR_ACCOUNT)" \
  gh api --paginate --slurp 'repos/OWNER/REPOSITORY/issues?state=all&per_page=100' > issues.json
GH_TOKEN="$(gh auth token --hostname github.com --user YOUR_ACCOUNT)" \
  gh api --paginate --slurp 'repos/OWNER/REPOSITORY/pulls?state=closed&per_page=100' > pulls.json
```

After both commands succeed, transform those recorded pages (no network access):

```sh
python3 - /path/to/repository OWNER/REPOSITORY issues.json pulls.json fix-source.json <<'PY'
import json, pathlib, subprocess, sys
target, repository, issue_file, pr_file, output = sys.argv[1:]
def git(*args):
    return subprocess.check_output(['git', '-C', target, *args], text=True).strip()
def records(path):
    pages = json.loads(pathlib.Path(path).read_text())
    if not isinstance(pages, list) or any(not isinstance(p, list) for p in pages):
        raise ValueError('expected gh api --paginate --slurp pages')
    return [record for page in pages for record in page]
landed = set(git('rev-list', '--first-parent', 'HEAD').splitlines())
prs = [p for p in records(pr_file)
       if p.get('merged_at') and p.get('merge_commit_sha') in landed]
source = {
    'schema_version': 'qualgen-fix-source-v1',
    'repository': repository, 'head_sha': git('rev-parse', 'HEAD'),
    'issues': [{'number': i['number'],
                'labels': [label['name'] for label in i['labels']],
                'reported_at': i['created_at']}
               for i in records(issue_file) if 'pull_request' not in i],
    'pull_requests': [{'number': p['number'], 'merge_sha': p['merge_commit_sha'],
                       'branch': p['head']['ref'], 'title': p['title'],
                       'body': p.get('body') or ''} for p in prs],
    'merged_pr_count': len(prs)
}
pathlib.Path(output).write_text(json.dumps(source, indent=2) + '\n')
PY
```

The denominator describes supplied GitHub PRs merged on this HEAD's first-parent
history, not direct commits or PRs from another forge. If complete coverage cannot
be established, remove `merged_pr_count` instead of asserting completeness.

## Artifacts and consumers

- `docs/quality/defects.jsonl` retains fix classifications and trace records.
  Identical facts are not appended again on repeat runs; corrections append new
  facts without deleting history.
- `docs/quality/metrics.jsonl` holds derived metrics and a completed M2 snapshot
  with source digest, HEAD, labels, membership, trace hashes and coverage counts.
  The completion record is written last. Readers use that snapshot, so partial
  retries and corrected classifications do not resurrect old density values.
- `report` reads actual trace artifacts, including existing library-written
  corpora. A source-free run leaves M2 unavailable until a corpus is supplied.
  Advancing the mined HEAD without refreshing M2 marks its snapshot stale.
- `pr` and `check` consume the same current per-file defect density and trace-rate
  records. Missing, stale or no-longer-current evidence stays unavailable.

The target repository is not modified. `--in-repo` explicitly opts into storing
artifacts there. Telemetry/session forensics, era markers and process-stage
report integration are separate follow-ups to the defect-tracing surface.
