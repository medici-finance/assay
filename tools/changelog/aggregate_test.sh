#!/usr/bin/env bash
# Offline, network-free unit test for the release-time aggregation (aggregate.py).
# Builds throwaway fragment directories + CHANGELOG files under a temp dir. No
# cluster, no GitHub.
#
# Default impl is ../aggregate.py. Point AGG_IMPL at testdata/old-aggregate.py to
# see the fragment cases fail against the retired (Unreleased-only) engine — the
# committed fail-first evidence:
#   AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh   # RED on A1..A4, C1..C5
#   ./aggregate_test.sh                                       # all green
#
# The C cases are external-contributor credit (contributor-trust/09). They are
# red against the retired engine because it has no `credits` subcommand and no
# `--credits` option at all. Note the diagnostics say `rc: N`, never `rc=N` —
# a caller greps this output for its own `echo rc=$?`, and a failure message
# carrying that token would answer the wrong question.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGG="${AGG_IMPL:-$here/aggregate.py}"
case "$AGG" in /*) ;; *) AGG="$here/$AGG" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

# Empty Unreleased so only fragments feed the aggregate (except where a case
# writes a residual bullet in).
CL_EMPTY=$'# Changelog\n\n## Unreleased\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n'

newcase() {
  D="$(mktemp -d "${TMPDIR:-/tmp}/clagg-XXXXXX")"
  mkdir -p "$D/changelog"
  printf '%s' "$CL_EMPTY" > "$D/CHANGELOG.md"
}

# A1: fragments aggregate into highlights (new). Retired engine refuses (exit 2)
#     because it never reads changelog/ and Unreleased is empty.
newcase
echo '- `alpha` added.' > "$D/changelog/a.md"
echo '- `bravo` added.' > "$D/changelog/b.md"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
if [ "$rc" = 0 ] && grep -q 'alpha' <<<"$out" && grep -q 'bravo' <<<"$out"; then
  ok "A1 fragments aggregate into highlights"
else
  bad "A1 fragments aggregate into highlights (rc: $rc)"
fi

# A2: sorted + de-duped within a bucket.
newcase
printf '### Added\n- zeta.\n- alpha.\n' > "$D/changelog/one.md"
printf '### Added\n- alpha.\n- mid.\n' > "$D/changelog/two.md"   # 'alpha.' duplicated
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
# expect exactly one 'alpha.' and order alpha < mid < zeta
n_alpha="$(grep -c -- '- alpha\.' <<<"$out")"
order="$(grep -E '^- ' <<<"$out" | tr -d ' ')"
if [ "$rc" = 0 ] && [ "$n_alpha" = 1 ] && [ "$order" = $'-alpha.\n-mid.\n-zeta.' ]; then
  ok "A2 sorted + de-duped"
else
  bad "A2 sorted + de-duped (rc: $rc n_alpha=$n_alpha)"; printf '%s\n' "$out"
fi

# A3: buckets separated + emitted in Added/Fixed/Changed order; unbucketed → Changed.
newcase
printf '### Fixed\n- a bug.\n' > "$D/changelog/fix.md"
printf '%s\n' '- no bucket here.' > "$D/changelog/plain.md"   # defaults to Changed
printf '### Added\n- a feature.\n' > "$D/changelog/add.md"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
seq="$(grep -E '^### ' <<<"$out" | tr '\n' ',')"
if [ "$rc" = 0 ] && [ "$seq" = "### Added,### Fixed,### Changed," ] && grep -q 'no bucket here' <<<"$out"; then
  ok "A3 bucket separation + order + default-Changed"
else
  bad "A3 bucket separation (rc: $rc seq=$seq)"; printf '%s\n' "$out"
fi

# A4: the cutover fold — a residual Unreleased bullet is aggregated alongside
#     fragments (new). Retired engine would emit ONLY the residual, dropping the
#     fragment.
newcase
printf '# Changelog\n\n## Unreleased\n\n### Changed\n- residual cutover bullet.\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n' > "$D/CHANGELOG.md"
echo '- `charlie` from a fragment.' > "$D/changelog/c.md"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
if [ "$rc" = 0 ] && grep -q 'residual cutover bullet' <<<"$out" && grep -q 'charlie' <<<"$out"; then
  ok "A4 cutover fold (residual Unreleased + fragment)"
else
  bad "A4 cutover fold (rc: $rc)"; printf '%s\n' "$out"
fi

# A5: empty — no fragments and empty Unreleased → REFUSE (exit 2). Same for new
#     and retired; asserts the empty-fragments refusal is preserved.
newcase
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
if [ "$rc" = 2 ]; then ok "A5 empty refuses (exit 2)"; else bad "A5 empty refuses (got rc: $rc)"; fi

# A6: roll rewrites CHANGELOG.md — Unreleased emptied to the pointer, dated
#     section inserted with the aggregate. (roll is new-engine only.)
if [ "$(basename "$AGG")" = "aggregate.py" ]; then
  newcase
  echo '- `delta` shipped.' > "$D/changelog/d.md"
  python3 "$AGG" roll "$D/changelog" "$D/CHANGELOG.md" v0.2.0 2026-02-02 >/dev/null 2>&1; rc=$?
  body="$(cat "$D/CHANGELOG.md")"
  if [ "$rc" = 0 ] \
     && grep -q '## v0.2.0 — 2026-02-02' <<<"$body" \
     && grep -q 'delta' <<<"$body" \
     && grep -q 'one-file-per-PR fragments' <<<"$body" \
     && ! awk '/^## Unreleased/{f=1;next} f&&/^## /{exit} f&&/^[[:space:]]*- /{print}' <<<"$body" | grep -q .; then
    ok "A6 roll: Unreleased emptied, dated section inserted"
  else
    bad "A6 roll (rc: $rc)"; printf '%s\n' "$body"
  fi
else
  echo "ok   - A6 roll (skipped for retired impl)"; pass=$((pass+1))
fi

# ---------------------------------------------------------------------------
# CREDIT CASES (contributor-trust/09). The release notes name the external
# author a fork change came from. The git half (fragment -> adding commit ->
# merge commit -> PR number) is `aggregate.py credits`; the forge half (PR ->
# author login -> external? opted out?) is the workflow step, whose identity
# and opt-out decisions are `credit-identity.sh`. `highlights`/`roll` only ever
# READ the resulting map, so these cases stay offline and network-free.
#
# All four aggregate cases assert rc=0 from `highlights` over a FRAGMENT set,
# which the retired engine refuses (exit 2) — so they are red under
# AGG_IMPL=testdata/old-aggregate.py, the committed fail-first evidence.
# ---------------------------------------------------------------------------
IDENT="$here/credit-identity.sh"

# C1: an external-authored fragment carries the credit suffix on its bullet.
newcase
printf '### Fixed\n- `widget` no longer drops the last frame.\n' > "$D/changelog/pr-1234-widget.md"
printf 'pr-1234-widget.md\t@octocat-example\n' > "$D/credits.map"
ident="$(ASSAY_TRUSTED_LOGINS='maintainer-example:11' \
         ASSAY_HUMAN_LOGIN_MAP='driver:driver-example' \
         ASSAY_TRUSTED_BOT_SLUGS='worker=worker-app-example:22' \
         bash "$IDENT" classify octocat-example /dev/null 2>/dev/null)"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" --credits "$D/credits.map" 2>/dev/null)"; rc=$?
if [ "$rc" = 0 ] \
   && [ "$ident" = "credit" ] \
   && grep -q 'no longer drops the last frame' <<<"$out" \
   && grep -q 'thanks @octocat-example' <<<"$out"; then
  ok "C1 external-authored fragment is credited in the highlights"
else
  bad "C1 external-authored fragment is credited (rc: $rc ident=$ident)"; printf '%s\n' "$out"
fi

# C2: NEGATIVE CONTROL — a roster-authored fragment is credited to nobody. The
#     forge half classifies the author as roster-known and therefore writes no
#     map line; the aggregate appends nothing.
newcase
printf '### Added\n- roster-authored change.\n' > "$D/changelog/roster.md"
printf '### Added\n- outside change.\n' > "$D/changelog/outside.md"
printf 'outside.md\t@octocat-example\n' > "$D/credits.map"
ident="$(ASSAY_TRUSTED_LOGINS='maintainer-example:11' \
         ASSAY_HUMAN_LOGIN_MAP='driver:driver-example' \
         ASSAY_TRUSTED_BOT_SLUGS='worker=worker-app-example:22' \
         bash "$IDENT" classify maintainer-example /dev/null 2>/dev/null)"
ident_bot="$(ASSAY_TRUSTED_LOGINS='maintainer-example:11' \
             ASSAY_HUMAN_LOGIN_MAP='driver:driver-example' \
             ASSAY_TRUSTED_BOT_SLUGS='worker=worker-app-example:22' \
             bash "$IDENT" classify 'worker-app-example[bot]' /dev/null 2>/dev/null)"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" --credits "$D/credits.map" 2>/dev/null)"; rc=$?
roster_line="$(grep -- '- roster-authored change\.' <<<"$out" || true)"
if [ "$rc" = 0 ] \
   && [ "$ident" = "skip:roster" ] && [ "$ident_bot" = "skip:roster" ] \
   && [ -n "$roster_line" ] \
   && ! grep -q 'thanks' <<<"$roster_line" \
   && grep -q 'outside change\..*thanks @octocat-example' <<<"$out"; then
  ok "C2 roster-authored fragment is NOT credited (negative control)"
else
  bad "C2 roster-authored not credited (rc: $rc ident=$ident ident_bot=$ident_bot)"; printf '%s\n' "$out"
fi

# C3: the documented opt-out marker in the PR body suppresses the credit. The
#     forge half records the opt-out explicitly in the map (never an omission),
#     and the aggregate appends nothing for it.
newcase
printf '### Fixed\n- opted-out change.\n' > "$D/changelog/optout.md"
printf 'A change from a fork.\n\n<!-- changelog-credit: no -->\n' > "$D/body.md"
ident="$(ASSAY_TRUSTED_LOGINS='maintainer-example:11' \
         ASSAY_HUMAN_LOGIN_MAP='driver:driver-example' \
         ASSAY_TRUSTED_BOT_SLUGS='worker=worker-app-example:22' \
         bash "$IDENT" classify octocat-example "$D/body.md" 2>/dev/null)"
printf 'optout.md\topt-out\n' > "$D/credits.map"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" --credits "$D/credits.map" 2>/dev/null)"; rc=$?
if [ "$rc" = 0 ] \
   && [ "$ident" = "skip:opt-out" ] \
   && grep -q 'opted-out change' <<<"$out" \
   && ! grep -q 'thanks' <<<"$out"; then
  ok "C3 the opt-out marker suppresses the credit"
else
  bad "C3 opt-out suppresses the credit (rc: $rc ident=$ident)"; printf '%s\n' "$out"
fi

# C4: an UNRESOLVABLE fragment aggregates uncredited and the run still exits 0 —
#     a credit never fails a cut. Covers all three unresolved map values and a
#     map naming a fragment that no longer exists.
newcase
printf '### Changed\n- unresolvable change.\n' > "$D/changelog/orphan.md"
printf '# credits map\norphan.md\tunresolved\ngone.md\t@octocat-example\n' > "$D/credits.map"
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" --credits "$D/credits.map" 2>/dev/null)"; rc=$?
out2="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" --credits "$D/nonexistent.map" 2>/dev/null)"; rc2=$?
# Byte-identical to the uncredited run: a map that credits nothing changes nothing.
plain="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc3=$?
if [ "$rc" = 0 ] && [ "$rc2" = 0 ] && [ "$rc3" = 0 ] \
   && grep -q 'unresolvable change' <<<"$out" \
   && ! grep -q 'thanks' <<<"$out" \
   && [ "$out" = "$plain" ] && [ "$out2" = "$plain" ]; then
  ok "C4 unresolvable fragment aggregates uncredited, exit 0"
else
  bad "C4 unresolvable aggregates uncredited (rc: $rc rc2: $rc2 rc3: $rc3)"; printf '%s\n' "$out"
fi

# C5: `credits` resolves fragment -> PR number over a real fixture repository
#     with a real merge commit, and reports an EXPLICIT unresolved marker (never
#     an omission) for a fragment it cannot map. The fixture is built here, and
#     lives at the path Verify row 13 names.
bash "$here/testdata/make-credits-fixture.sh" >/dev/null 2>&1 || true
FIX="$here/testdata/credits-fixture-repo"
out="$(python3 "$AGG" credits "$FIX/changelog" 2>/dev/null)"; rc=$?
if [ "$rc" = 0 ] \
   && grep -q '^merged-via-merge-commit\.md	4242$' <<<"$out" \
   && grep -q '^merged-via-squash\.md	4343$' <<<"$out" \
   && grep -q '^never-merged\.md	unresolved$' <<<"$out"; then
  ok "C5 credits resolves the PR number from git alone, unresolved marked explicitly"
else
  bad "C5 credits resolution (rc: $rc)"; printf '%s\n' "$out"
fi

echo "---"
echo "aggregate_test: $pass passed, $fail failed (impl: $AGG)"
[ "$fail" = 0 ]
