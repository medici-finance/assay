#!/usr/bin/env bash
# Offline, network-free unit test for the release-time aggregation (aggregate.py).
# Builds throwaway fragment directories + CHANGELOG files under a temp dir. No
# cluster, no GitHub.
#
# Default impl is ../aggregate.py. Point AGG_IMPL at testdata/old-aggregate.py to
# see the fragment cases fail against the retired (Unreleased-only) engine — the
# committed fail-first evidence:
#   AGG_IMPL=testdata/old-aggregate.py ./aggregate_test.sh   # RED on A1..A4
#   ./aggregate_test.sh                                       # all green
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
  bad "A1 fragments aggregate into highlights (rc=$rc)"
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
  bad "A2 sorted + de-duped (rc=$rc n_alpha=$n_alpha)"; printf '%s\n' "$out"
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
  bad "A3 bucket separation (rc=$rc seq=$seq)"; printf '%s\n' "$out"
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
  bad "A4 cutover fold (rc=$rc)"; printf '%s\n' "$out"
fi

# A5: empty — no fragments and empty Unreleased → REFUSE (exit 2). Same for new
#     and retired; asserts the empty-fragments refusal is preserved.
newcase
out="$(python3 "$AGG" highlights "$D/changelog" "$D/CHANGELOG.md" 2>/dev/null)"; rc=$?
if [ "$rc" = 2 ]; then ok "A5 empty refuses (exit 2)"; else bad "A5 empty refuses (got rc=$rc)"; fi

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
    bad "A6 roll (rc=$rc)"; printf '%s\n' "$body"
  fi
else
  echo "ok   - A6 roll (skipped for retired impl)"; pass=$((pass+1))
fi

echo "---"
echo "aggregate_test: $pass passed, $fail failed (impl: $AGG)"
[ "$fail" = 0 ]
