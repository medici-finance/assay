#!/usr/bin/env bash
# make-credits-fixture.sh — build testdata/credits-fixture-repo/, the throwaway
# git repository `aggregate.py credits` is exercised against (case C5 of
# aggregate_test.sh, and Verify row 13 of contributor-trust/09).
#
# WHY IT IS BUILT AND NOT COMMITTED. The fixture has to contain a repository
# whose history the resolver walks — a real adding commit, a real
# `Merge pull request #N` merge commit, a real squash landing, and a fragment
# with neither. Git cannot track a nested repository's own `.git`, and a fixture
# living directly in THIS repository would resolve against THIS repository's
# history: its "unresolvable" fragment would silently acquire a pull-request
# number the day this change merged, and the row asserting the unresolved marker
# would then be asserting nothing. So the fixture is generated, and ignored.
#
# Re-runnable: the tree is rebuilt from scratch every time. Offline — git only,
# no network, no forge.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="$here/credits-fixture-repo"

rm -rf "$fixture"
mkdir -p "$fixture"

git init -q -b main "$fixture"
git -C "$fixture" config user.name  'fixture'
git -C "$fixture" config user.email 'fixture@example.invalid'
git -C "$fixture" config commit.gpgsign false
git -C "$fixture" config core.hooksPath /dev/null

mkdir -p "$fixture/changelog"
c() { git -C "$fixture" commit -q --no-verify "$@"; }

# A base commit, so the branches below have somewhere to fork from.
printf 'fixture\n' > "$fixture/README.md"
git -C "$fixture" add README.md
c -m 'base'

# 1) MERGE-COMMIT landing: a fragment added on a side branch, brought to main by
#    a merge commit whose subject carries the pull-request number.
git -C "$fixture" checkout -q -b pr-4242
printf '### Fixed\n- the merge-commit landing.\n' > "$fixture/changelog/merged-via-merge-commit.md"
git -C "$fixture" add changelog/merged-via-merge-commit.md
c -m 'fix: the merge-commit landing'
git -C "$fixture" checkout -q main
git -C "$fixture" merge -q --no-ff -m 'Merge pull request #4242 from example/pr-4242' pr-4242

# 2) SQUASH landing: the fragment's adding commit IS the landing commit, and the
#    number is the trailing `(#N)` of its own subject.
printf '### Added\n- the squash landing.\n' > "$fixture/changelog/merged-via-squash.md"
git -C "$fixture" add changelog/merged-via-squash.md
c -m 'feat: the squash landing (#4343)'

# 3) UNRESOLVABLE: an adding commit with no merge commit above it and no number
#    in its own subject. The resolver must print the explicit unresolved marker
#    for it rather than omitting the line.
printf '### Changed\n- no landing commit names a pull request.\n' > "$fixture/changelog/never-merged.md"
git -C "$fixture" add changelog/never-merged.md
c -m 'chore: landed with no pull request reference'

echo "$fixture"
