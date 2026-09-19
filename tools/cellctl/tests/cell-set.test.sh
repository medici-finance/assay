#!/usr/bin/env bash
# cell-set.test.sh — `cellctl set <cell> KEY=VALUE [...]`, persisting a cell.env change in place
# (issue-940 part 2), plus the `cellctl desk ... --model <m> --set` sugar that overrides-and-persists
# in one call.
#
# What it proves (each an `assert` below):
#   set     rewrites an EXISTING key's line in place (value changes, line position, surrounding
#           comments and every other line's ordering untouched); APPENDS a key that has no active
#           line yet; REFUSES a key that is not a known cell.env key unless --force (and touches
#           nothing when it refuses); writes exactly ONE backup file (cell.env.bak-<ts>) before the
#           first edit of a multi-key call, byte-identical to the pre-edit file; prints each key's
#           before/after value; applies the same the-desk/Opus refusal to DESK_MODEL_the_desk as a
#           live boot and `check` do, and touches nothing when it refuses (no backup either); a
#           call with zero KEY=VALUE arguments is a usage error
#   desk    `--model <m> --set` applies the override for this run AND persists it into cell.env
#           (the next plain boot, no --model, resolves to the persisted value); `--set` without
#           `--model`/DESK_MODEL_OVERRIDE is refused (nothing to persist); DRY_RUN=1 with --set
#           does not write cell.env (prints what it would do)
#   #1303   `set --kind/--cockpit/--harness/--provider` sugar (validated, refused before writing);
#           a kind change refuses naming the missing precondition key unless it is in cell.env or
#           the same call; `desk`/`up --set` persist EVERY override given (one backup); `show`
#           prints each effective value with its source (flag / cell.env / default)
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private PATH,
# and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-set.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }
sha(){ if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/DESK_ROOTS/DESK_LOOP/
# DESK_SESSION vars — unset them so the fixture below is the only source cellctl reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_MODEL_OVERRIDE \
  DESK_ROOTS DESK_LOOP DESK_SESSION 2>/dev/null || true

# ---------------------------------------------------------------- fixtures
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
git -C "$T/seed" add -A && git -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "seed"
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"
ROOTS="example-org/example-repo=$REPO"
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
printf '#!/usr/bin/env bash\necho "$HOME" > "%s/deskroster.home"\nexit 0\n' "$T" > "$DESK_TOOLS_BIN/deskroster"
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
echo "ARGS=$*" > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/claude"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"

"$CELLCTL" new house-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/house-cell"

# ---------------------------------------------------------------- set: existing key updated
echo "[set: existing key updated]"
cp "$CELL/cell.env" "$T/before-update.env"
before_lines="$(grep -vE '^DESK_MODEL_the_desk=' "$CELL/cell.env")"
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=sonnet 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
assert "cell.env now carries the new value" 'grep -qx "DESK_MODEL_the_desk=sonnet" "$CELL/cell.env"'
assert "every OTHER line is untouched" '[[ "$(grep -vE "^DESK_MODEL_the_desk=" "$CELL/cell.env")" == "$before_lines" ]]'
before_ln="$(grep -n "^DESK_MODEL_the_desk=" "$T/before-update.env" | cut -d: -f1)"
after_ln="$(grep -n "^DESK_MODEL_the_desk=" "$CELL/cell.env" | cut -d: -f1)"
assert "the updated line keeps its original position (line $before_ln)" '[[ -n "$before_ln" && "$before_ln" == "$after_ln" ]]'
assert "prints before -> after" 'grep -q "DESK_MODEL_the_desk: fable -> sonnet" <<<"$out"'

# ---------------------------------------------------------------- set: missing key appended
echo "[set: missing key appended]"
assert "TMUX_SESSION is not yet in cell.env" '! grep -q "^TMUX_SESSION=" "$CELL/cell.env"'
out="$("$CELLCTL" set house-cell TMUX_SESSION=my-session 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
assert "cell.env now carries the appended key" 'grep -qx "TMUX_SESSION=my-session" "$CELL/cell.env"'
assert "prints <unset> -> value for a fresh key" 'grep -q "TMUX_SESSION: <unset> -> my-session" <<<"$out"'

# ---------------------------------------------------------------- set: unknown key refused
echo "[set: unknown key refused without --force]"
cp "$CELL/cell.env" "$T/before-unknown.env"
out="$("$CELLCTL" set house-cell NOT_A_REAL_KEY=x 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names the key and --force" 'grep -q "NOT_A_REAL_KEY" <<<"$out" && grep -q -- "--force" <<<"$out"'
assert "cell.env is untouched by the refusal" '[[ "$(sha "$CELL/cell.env")" == "$(sha "$T/before-unknown.env")" ]]'
out="$("$CELLCTL" set house-cell NOT_A_REAL_KEY=x --force 2>&1)" && rc=0 || rc=$?
assert "--force allows the unknown key" '[[ $rc -eq 0 ]] && grep -qx "NOT_A_REAL_KEY=x" "$CELL/cell.env"'

# ---------------------------------------------------------------- set: comments/ordering preserved
echo "[set: comments and ordering preserved]"
printf '# a leading comment\nCELL_COCKPIT=auto\n# a trailing comment about the roles\nROLES="the-desk worker-desk"\n' > "$CELL/cell.env"
cp "$CELL/cell.env" "$T/before-comments.env"
"$CELLCTL" set house-cell CELL_COCKPIT=tmux >/dev/null
assert "comment lines are byte-identical and stay in place" \
  '[[ "$(sed -n 1p "$CELL/cell.env")" == "# a leading comment" && "$(sed -n 3p "$CELL/cell.env")" == "# a trailing comment about the roles" ]]'
assert "the edited line keeps its position (line 2)" '[[ "$(sed -n 2p "$CELL/cell.env")" == "CELL_COCKPIT=tmux" ]]'
assert "the line after it (ROLES) is untouched and still line 4" '[[ "$(sed -n 4p "$CELL/cell.env")" == "ROLES=\"the-desk worker-desk\"" ]]'
assert "file has the same line count as before" '[[ "$(wc -l < "$CELL/cell.env")" == "$(wc -l < "$T/before-comments.env")" ]]'

# ---------------------------------------------------------------- set: backup file written
echo "[set: backup file written]"
"$CELLCTL" new backup-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
BCELL="$CELLS_ROOT/backup-cell"
cp "$BCELL/cell.env" "$T/before-backup.env"
before_count="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | wc -l | tr -d ' ')"
out="$("$CELLCTL" set backup-cell DESK_MODEL_DEFAULT=haiku ROLES="worker-desk" 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
after_count="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | wc -l | tr -d ' ')"
assert "exactly one backup file was written for a two-key call" '[[ $((after_count - before_count)) -eq 1 ]]'
backup_file="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | head -n1)"
assert "the backup matches the pre-edit file" '[[ "$(sha "$backup_file")" == "$(sha "$T/before-backup.env")" ]]'
assert "the backup announcement names the file" 'grep -q "backup written: $backup_file" <<<"$out"'

# ---------------------------------------------------------------- set: opus-for-the-desk refused
echo "[set: opus-for-the-desk refused]"
cp "$CELL/cell.env" "$T/before-opus-set.env"
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=opus 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "same refusal message as a live boot / check" 'grep -q "the-desk runs on the top tier" <<<"$out" && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'
assert "cell.env is untouched (no backup, no edit)" '[[ "$(sha "$CELL/cell.env")" == "$(sha "$T/before-opus-set.env")" ]]'
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=claude-opus-9 2>&1)" && rc=0 || rc=$?
assert "a claude-opus-* id is refused too" '[[ $rc -ne 0 ]] && grep -q "claude-opus-9" <<<"$out"'
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=opus --force 2>&1)" && rc=0 || rc=$?
assert "--force does not bypass the opus refusal (it is a value check, not a key-allowlist check)" \
  '[[ $rc -ne 0 ]] && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'

# ---------------------------------------------------------------- set: no KEY=VALUE is a usage error
echo "[set: no KEY=VALUE is a usage error]"
out="$("$CELLCTL" set house-cell 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "names KEY=VALUE" 'grep -q "KEY=VALUE" <<<"$out"'

# ---------------------------------------------------------------- desk --model --set sugar
echo "[desk --model --set: override now AND persist]"
"$CELLCTL" new sugar-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
SCELL="$CELLS_ROOT/sugar-cell"
assert "sugar-cell starts pinned to fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$SCELL/cell.env"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk --model sonnet --set 2>&1)" && rc=0 || rc=$?
assert "DRY_RUN + --set exits 0" '[[ $rc -eq 0 ]]'
assert "DRY_RUN + --set does not touch cell.env" 'grep -qx "DESK_MODEL_the_desk=fable" "$SCELL/cell.env"'
assert "DRY_RUN + --set says what it WOULD persist" 'grep -q "would persist DESK_MODEL_the_desk=sonnet" <<<"$out"'

out="$("$CELLCTL" desk sugar-cell the-desk --model sonnet --set 2>&1)" && rc=0 || rc=$?
assert "live --model --set exits 0" '[[ $rc -eq 0 ]]'
assert "this run's launch line shows the override" 'grep -q "model=sonnet (override)" <<<"$out"'
assert "the pin is now persisted in cell.env" 'grep -qx "DESK_MODEL_the_desk=sonnet" "$SCELL/cell.env"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "a later PLAIN boot (no --model) resolves to the persisted value" '[[ $rc -eq 0 ]] && grep -q "model=sonnet" <<<"$out" && ! grep -q "(override)" <<<"$out"'

out="$("$CELLCTL" desk sugar-cell worker-desk --set 2>&1)" && rc=0 || rc=$?
assert "--set without --model/DESK_MODEL_OVERRIDE is refused" '[[ $rc -ne 0 ]] && grep -q -- "--set needs --model" <<<"$out"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk --model opus --set 2>&1)" && rc=0 || rc=$?
assert "--model opus --set is refused (opus rule outranks the sugar) and persists nothing" \
  '[[ $rc -ne 0 ]] && grep -qx "DESK_MODEL_the_desk=sonnet" "$SCELL/cell.env"'

# ================================================================ #1303 scope 2: every per-run choice
# `set` sugar flags, `desk`/`up --set` persisting EVERY override given, kind-change preconditions,
# and the `show` read. A codex stub and a no-op tmux stub so the live `up --set` path runs
# without a real cockpit.
for stub in codex; do
  cat > "$T/bin/$stub" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in --version|-V) echo "0.0.0-test"; exit 0;; doctor) exit 0;; esac
echo "ARGS=$*" > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
  chmod +x "$T/bin/$stub"
done
printf '#!/usr/bin/env bash\nexit 0\n' > "$T/bin/tmux"; chmod +x "$T/bin/tmux"
backups(){ find "$1" -maxdepth 1 -name 'cell.env.bak-*' | wc -l | tr -d ' '; }

"$CELLCTL" new wide-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
WCELL="$CELLS_ROOT/wide-cell"

# ---------------------------------------------------------------- set: sugar flags
echo "[set: --cockpit/--harness/--provider sugar writes the matching KEY]"
n0="$(backups "$WCELL")"
out="$("$CELLCTL" set wide-cell --cockpit tmux --harness codex --provider zai 2>&1)" && rc=0 || rc=$?
assert "sugar call exits 0" '[[ $rc -eq 0 ]]'
assert "CELL_COCKPIT=tmux written" 'grep -qx "CELL_COCKPIT=tmux" "$WCELL/cell.env"'
assert "CELL_HARNESS=codex written (no role given → CELL_HARNESS sugar, not a namespace selector)" 'grep -qx "CELL_HARNESS=codex" "$WCELL/cell.env"'
assert "CELL_PROVIDER=zai written" 'grep -qx "CELL_PROVIDER=zai" "$WCELL/cell.env"'
assert "exactly one backup for the three-key sugar call" '[[ "$(grep -c "backup written" <<<"$out")" -eq 1 ]]'
assert "prints before/after per key" 'grep -q "CELL_COCKPIT: auto -> tmux" <<<"$out" && grep -q "CELL_HARNESS: claude -> codex" <<<"$out"'

echo "[set: sugar values are validated before anything is written]"
cp "$WCELL/cell.env" "$T/wide-before.env"; n0="$(backups "$WCELL")"
out="$("$CELLCTL" set wide-cell --cockpit bogus 2>&1)" && rc=0 || rc=$?
assert "--cockpit bogus refused" '[[ $rc -ne 0 ]] && grep -q "CELL_COCKPIT must be one of auto|tmux|herdr|orca" <<<"$out"'
out="$("$CELLCTL" set wide-cell --harness bogus 2>&1)" && rc=0 || rc=$?
assert "--harness bogus refused" '[[ $rc -ne 0 ]] && grep -q "CELL_HARNESS must be claude or codex" <<<"$out"'
out="$("$CELLCTL" set wide-cell --kind bogus 2>&1)" && rc=0 || rc=$?
assert "--kind bogus refused" '[[ $rc -ne 0 ]] && grep -q "CELL_KIND must be one of k8s|house|container|scrubbed" <<<"$out"'
out="$("$CELLCTL" set wide-cell CELL_COCKPIT=bogus --force 2>&1)" && rc=0 || rc=$?
assert "--force does not bypass the cockpit value check" '[[ $rc -ne 0 ]]'
out="$("$CELLCTL" set wide-cell --model sonnet 2>&1)" && rc=0 || rc=$?
assert "--model without a role is refused (the pin is per role)" '[[ $rc -ne 0 ]] && grep -q -- "--model needs a role" <<<"$out"'
assert "cell.env untouched by every refusal, no backup written" '[[ "$(sha "$WCELL/cell.env")" == "$(sha "$T/wide-before.env")" && $(backups "$WCELL") -eq $n0 ]]'

echo "[set: with a role, --harness still selects the namespace and is NOT persisted]"
out="$("$CELLCTL" set wide-cell worker-desk --harness claude --model cw 2>&1)" && rc=0 || rc=$?
assert "writes DESK_MODEL_worker_desk" '[[ $rc -eq 0 ]] && grep -qx "DESK_MODEL_worker_desk=cw" "$WCELL/cell.env"'
assert "CELL_HARNESS stays codex" 'grep -qx "CELL_HARNESS=codex" "$WCELL/cell.env"'

# ---------------------------------------------------------------- set: kind change preconditions
echo "[set: a kind change refuses before writing when the target kind's precondition is missing]"
cp "$WCELL/cell.env" "$T/wide-before.env"; n0="$(backups "$WCELL")"
out="$("$CELLCTL" set wide-cell --kind container 2>&1)" && rc=0 || rc=$?
assert "house → container without CELL_CONTAINER_LAUNCHER is refused" '[[ $rc -ne 0 ]]'
assert "names the missing key" 'grep -q "CELL_KIND=container needs CELL_CONTAINER_LAUNCHER" <<<"$out"'
assert "nothing written, no backup" '[[ "$(sha "$WCELL/cell.env")" == "$(sha "$T/wide-before.env")" && $(backups "$WCELL") -eq $n0 ]]'
out="$("$CELLCTL" set wide-cell --kind container CELL_CONTAINER_LAUNCHER=relative/launcher 2>&1)" && rc=0 || rc=$?
assert "a non-absolute/non-executable launcher given in the same call is refused too" '[[ $rc -ne 0 ]] && grep -q "absolute executable CELL_CONTAINER_LAUNCHER" <<<"$out"'
printf '#!/usr/bin/env bash\nexit 0\n' > "$T/launcher"; chmod +x "$T/launcher"
out="$("$CELLCTL" set wide-cell --kind container CELL_CONTAINER_LAUNCHER="$T/launcher" 2>&1)" && rc=0 || rc=$?
assert "the same change with the launcher given in ONE call passes" '[[ $rc -eq 0 ]] && grep -qx "CELL_KIND=container" "$WCELL/cell.env" && grep -qx "CELL_CONTAINER_LAUNCHER=$T/launcher" "$WCELL/cell.env"'
out="$("$CELLCTL" set wide-cell --kind house 2>&1)" && rc=0 || rc=$?
assert "back to house passes (CELL_ROOTS is in the file)" '[[ $rc -eq 0 ]] && grep -qx "CELL_KIND=house" "$WCELL/cell.env"'
# drop the launcher again so the container-kind refusals below have something to refuse on
grep -v '^CELL_CONTAINER_LAUNCHER=' "$WCELL/cell.env" > "$WCELL/cell.env.tmp" && mv "$WCELL/cell.env.tmp" "$WCELL/cell.env"
printf 'cells: []\n' > "$T/cells.yaml"; printf 'placeholder, not a real key\n' > "$T/app.pem"
"$CELLCTL" new noroots-cell --kind k8s --forge github --repo "$REPO" --cells-yaml "$T/cells.yaml" --orgs example-org --deskd-app-pem "$T/app.pem" >/dev/null
out="$("$CELLCTL" set noroots-cell --kind house 2>&1)" && rc=0 || rc=$?
assert "k8s → house on a cell with no CELL_ROOTS is refused naming CELL_ROOTS" '[[ $rc -ne 0 ]] && grep -q "CELL_KIND=house needs CELL_ROOTS" <<<"$out"'
out="$("$CELLCTL" set noroots-cell --kind house CELL_ROOTS="$ROOTS" 2>&1)" && rc=0 || rc=$?
assert "…and passes when CELL_ROOTS is given in the same call" '[[ $rc -eq 0 ]] && grep -qx "CELL_KIND=house" "$CELLS_ROOT/noroots-cell/cell.env"'

# ---------------------------------------------------------------- desk --set persists every override
echo "[desk --set: every override given is persisted, each to its own key, one backup]"
"$CELLCTL" set wide-cell --harness claude --cockpit auto >/dev/null
"$CELLCTL" set wide-cell CELL_PROVIDER_ZAI_BASE_URL=https://api.example.invalid CELL_PROVIDER_ZAI_TOKEN_ENV=ZAI_TEST_KEY >/dev/null
export ZAI_TEST_KEY=fixture-not-a-secret
out="$(DRY_RUN=1 "$CELLCTL" desk wide-cell worker-desk --harness codex --model cm --provider zai --cockpit herdr --set 2>&1)" && rc=0 || rc=$?
assert "dry-run lists every key it would persist" '[[ $rc -eq 0 ]] && grep -q "would persist CODEX_MODEL_worker_desk=cm" <<<"$out" && grep -q "would persist CELL_HARNESS=codex" <<<"$out" && grep -q "would persist CELL_PROVIDER=zai" <<<"$out" && grep -q "would persist CELL_COCKPIT=herdr" <<<"$out"'
assert "dry-run writes nothing" 'grep -qx "CELL_HARNESS=claude" "$WCELL/cell.env" && ! grep -q "^CODEX_MODEL_worker_desk=" "$WCELL/cell.env"'
n0="$(backups "$WCELL")"
export CELLCTL_TEST_OUT="$T/launch-wide.env"
out="$("$CELLCTL" desk wide-cell worker-desk --harness codex --model cm --provider zai --cockpit herdr --set 2>&1)" && rc=0 || rc=$?
assert "live desk --set exits 0" '[[ $rc -eq 0 ]]'
assert "CODEX_MODEL_worker_desk=cm persisted (the ACTIVE harness namespace)" 'grep -qx "CODEX_MODEL_worker_desk=cm" "$WCELL/cell.env" && ! grep -q "^DESK_MODEL_worker_desk=cm" "$WCELL/cell.env"'
assert "CELL_HARNESS=codex persisted" 'grep -qx "CELL_HARNESS=codex" "$WCELL/cell.env"'
assert "CELL_PROVIDER=zai persisted" 'grep -qx "CELL_PROVIDER=zai" "$WCELL/cell.env"'
assert "CELL_COCKPIT=herdr persisted" 'grep -qx "CELL_COCKPIT=herdr" "$WCELL/cell.env"'
assert "exactly one backup written for the whole --set" '[[ "$(grep -c "backup written" <<<"$out")" -eq 1 ]]'
assert "the run itself used the overrides (codex stub launched with -m cm)" 'grep -q -- "-m cm" "$CELLCTL_TEST_OUT"'
out="$(DRY_RUN=1 "$CELLCTL" desk wide-cell worker-desk --harness codex --set 2>&1)" && rc=0 || rc=$?
assert "--set with only --harness (no --model) is accepted and persists just CELL_HARNESS" '[[ $rc -eq 0 ]] && grep -q "would persist CELL_HARNESS=codex" <<<"$out" && ! grep -q "would persist .*_MODEL_" <<<"$out"'
out="$("$CELLCTL" desk wide-cell worker-desk --set 2>&1)" && rc=0 || rc=$?
assert "--set with nothing to persist is still refused" '[[ $rc -ne 0 ]] && grep -q -- "--set needs --model" <<<"$out"'
out="$(DRY_RUN=1 "$CELLCTL" desk wide-cell the-desk --kind container --set 2>&1)" && rc=0 || rc=$?
assert "desk --kind container --set on a house cell with no launcher refuses at load, naming the launcher, before writing" '[[ $rc -ne 0 ]] && grep -q "CELL_CONTAINER_LAUNCHER" <<<"$out" && grep -qx "CELL_KIND=house" "$WCELL/cell.env"'
"$CELLCTL" set wide-cell --harness claude --cockpit auto >/dev/null
grep -v '^CELL_PROVIDER=' "$WCELL/cell.env" > "$WCELL/cell.env.tmp" && mv "$WCELL/cell.env.tmp" "$WCELL/cell.env"

# ---------------------------------------------------------------- up --set
echo "[up --set: persists the cockpit/harness/kind and --model per role window]"
out="$(DRY_RUN=1 "$CELLCTL" up wide-cell --cockpit tmux --harness codex --model km --set 2>&1)" && rc=0 || rc=$?
assert "up dry-run lists CELL_COCKPIT, CELL_HARNESS and one CODEX_MODEL_<role> per window" '[[ $rc -eq 0 ]] && grep -q "would persist CELL_COCKPIT=tmux" <<<"$out" && grep -q "would persist CELL_HARNESS=codex" <<<"$out" && grep -q "would persist CODEX_MODEL_the_desk=km" <<<"$out" && grep -q "would persist CODEX_MODEL_worker_desk=km" <<<"$out"'
assert "up dry-run writes nothing" '! grep -q "^CODEX_MODEL_the_desk=" "$WCELL/cell.env"'
n0="$(backups "$WCELL")"
out="$("$CELLCTL" up wide-cell --cockpit tmux --no-attach --harness codex --model km --set 2>&1)" && rc=0 || rc=$?
assert "live up --set exits 0 (tmux stub)" '[[ $rc -eq 0 ]]'
assert "CELL_COCKPIT=tmux and CELL_HARNESS=codex persisted by up" 'grep -qx "CELL_COCKPIT=tmux" "$WCELL/cell.env" && grep -qx "CELL_HARNESS=codex" "$WCELL/cell.env"'
assert "CODEX_MODEL_<role>=km persisted for every role window" 'grep -qx "CODEX_MODEL_the_desk=km" "$WCELL/cell.env" && grep -qx "CODEX_MODEL_worker_desk=km" "$WCELL/cell.env" && grep -qx "CODEX_MODEL_verify_desk=km" "$WCELL/cell.env"'
assert "exactly one backup for the whole up --set" '[[ "$(grep -c "backup written" <<<"$out")" -eq 1 ]]'
out="$(DRY_RUN=1 "$CELLCTL" up wide-cell --set 2>&1)" && rc=0 || rc=$?
assert "up --set with nothing to persist is refused" '[[ $rc -ne 0 ]] && grep -q -- "--set needs" <<<"$out"'
"$CELLCTL" set wide-cell --harness claude --cockpit auto >/dev/null

# ---------------------------------------------------------------- show
echo "[show: effective value + source per key]"
# drop the per-role pins the earlier --set cases persisted so the default/tier sources are visible
grep -vE '^(DESK_MODEL_worker_desk|CODEX_MODEL_[a-z_]+)=' "$WCELL/cell.env" > "$WCELL/cell.env.tmp" && mv "$WCELL/cell.env.tmp" "$WCELL/cell.env"
out="$("$CELLCTL" show wide-cell 2>&1)" && rc=0 || rc=$?
assert "show exits 0" '[[ $rc -eq 0 ]]'
assert "CELL_KIND from cell.env" 'grep -qx "\[show\] CELL_KIND=house (cell.env)" <<<"$out"'
assert "CELL_COCKPIT from cell.env" 'grep -qx "\[show\] CELL_COCKPIT=auto (cell.env)" <<<"$out"'
assert "CELL_HARNESS from cell.env" 'grep -qx "\[show\] CELL_HARNESS=claude (cell.env)" <<<"$out"'
assert "CELL_PROVIDER unset → default: anthropic" 'grep -qx "\[show\] CELL_PROVIDER=unset (default: anthropic)" <<<"$out"'
assert "the-desk model from its cell.env pin" 'grep -qx "\[show\] model the-desk=fable (cell.env DESK_MODEL_the_desk)" <<<"$out"'
assert "worker-desk model from the cell.env DESK_MODEL_DEFAULT line" 'grep -qx "\[show\] model worker-desk=sonnet (cell.env DESK_MODEL_DEFAULT)" <<<"$out"'
out="$("$CELLCTL" show wide-cell --harness codex --cockpit herdr --model xm 2>&1)" && rc=0 || rc=$?
assert "flags show as source=flag" '[[ $rc -eq 0 ]] && grep -qx "\[show\] CELL_HARNESS=codex (flag)" <<<"$out" && grep -qx "\[show\] CELL_COCKPIT=herdr (flag)" <<<"$out" && grep -qx "\[show\] model the-desk=xm (flag)" <<<"$out"'
out="$("$CELLCTL" show wide-cell --harness codex 2>&1)" && rc=0 || rc=$?
assert "a tier-map fallback shows as default with the tier entry named" 'grep -qx "\[show\] model the-desk=gpt-5.6-terra (default: tier:top (TIER_MODEL_TOP_CODEX))" <<<"$out"'
grep -v '^CELL_COCKPIT=' "$WCELL/cell.env" > "$WCELL/cell.env.tmp" && mv "$WCELL/cell.env.tmp" "$WCELL/cell.env"
out="$("$CELLCTL" show wide-cell 2>&1)" && rc=0 || rc=$?
assert "a key with no cell.env line shows as default" 'grep -qx "\[show\] CELL_COCKPIT=auto (default)" <<<"$out"'
out="$("$CELLCTL" show wide-cell --kind container 2>&1)" && rc=0 || rc=$?
assert "show --kind container on an unprovisioned cell refuses like desk would" '[[ $rc -ne 0 ]] && grep -q "CELL_CONTAINER_LAUNCHER" <<<"$out"'
out="$("$CELLCTL" show wide-cell --harness bogus 2>&1)" && rc=0 || rc=$?
assert "show validates its flags" '[[ $rc -ne 0 ]]'

echo
if [[ "$fails" -eq 0 ]]; then echo "cell-set.test.sh: OK"; else echo "cell-set.test.sh: $fails FAILED"; exit 1; fi
