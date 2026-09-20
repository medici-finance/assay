#!/usr/bin/env bash
# parity.test.sh — the cross-implementation ORACLE DIFF for cellctl (desk-containers/10).
#
# Two implementations, one fixture, one diff. For every cell in the matrix below this runs
# `DRY_RUN=1 <impl> <verb> <args>` under BOTH implementations against byte-identical fixtures,
# normalises the two things that legitimately differ (the implementation's own path, and the
# fixture root each copy was given), and diffs stdout+stderr and the exit code. Any difference is
# a divergence naming `<kind>/<harness>/<cockpit>/<verb>`, and the harness exits 1.
#
#   CELLCTL_A=tools/cellctl/cellctl CELLCTL_B=tools/desk/cellctl bash tools/cellctl/tests/parity.test.sh
#
# Both sides default to the bash oracle, so a bare run is the TRIVIAL pass that proves the harness
# itself is wired up (the state Task step 2 of the brief asks for, before the port exists).
#
# Matrix: kinds {k8s, house, container, scrubbed} × harness {claude, codex} × cockpit
# {tmux, herdr, orca} × verbs {check, desk, up, down, set, ls, smoke, status}, plus `new` per kind
# × forge {github, gitlab}. `new` has no dry run — parity there is the byte-diff of the tree each
# implementation scaffolds into its own CELLS_ROOT, mode bits included.
#
# `deskd` is DELIBERATELY absent: it has no DRY_RUN plan path in the oracle, so there is nothing
# to diff, and giving it one would mean editing the oracle (which this brief forbids). Its port is
# proven at the source level instead — see the brief's rows 10 and 15.
#
#   PARITY_ONLY=<kind>/<harness>/<cockpit>/<verb>   run exactly one cell (substring match on the
#                                                   cell id, so PARITY_ONLY=desk narrows to the
#                                                   desk verb across the whole matrix)
#   CELLCTL_PARITY_MUTATE=<key>                     passed through to BOTH sides untouched; the Go
#                                                   binary honours it only in a `-tags parity`
#                                                   build (the brief's row 5 negative control)
#
# NO live anything: the harness runs `env -i`-clean-ish with a private stub PATH (claude, codex,
# tmux, herdr, orca, curl, pkill and the desk verbs are all stubs), KUBECONFIG=/dev/null, and a
# throwaway git checkout per fixture. Nothing here reaches a network, a cluster or a real harness.
# shellcheck disable=SC2016
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
abspath(){ case "$1" in /*) printf '%s\n' "$1";; *) printf '%s\n' "$PWD/$1";; esac; }
CELLCTL_A="$(abspath "${CELLCTL_A:-$HERE/../cellctl}")"
CELLCTL_B="$(abspath "${CELLCTL_B:-$HERE/../cellctl}")"
[[ -x "$CELLCTL_A" ]] || { echo "parity: CELLCTL_A is not executable: $CELLCTL_A" >&2; exit 2; }
[[ -x "$CELLCTL_B" ]] || { echo "parity: CELLCTL_B is not executable: $CELLCTL_B" >&2; exit 2; }

# /tmp, never $TMPDIR: a scrubbed fixture's cell dir carries `run/tmux.sock`, and `sun_path` has a
# hard ~104-byte limit macOS's default $TMPDIR blows on its own (scrubbed-cell.test.sh's note).
T="$(cd "$(mktemp -d /tmp/cellctl-parity.XXXXXX)" && pwd -P)"
trap 'rm -rf "$T"' EXIT

ONLY="${PARITY_ONLY:-}"
cells=0; divergent=0; divergent_names=()

# -------------------- stubs
# Written once into $T/stub and symlinked into every fixture's own bin, so the two implementations
# see the SAME stub behaviour and a stub's own path normalises away with the fixture root.
mk_stubs(){
  local b="$1"
  mkdir -p "$b"
  cat > "$b/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  --version) echo "1.0.0 (stub)"; exit 0;;
  plugin)
    case "${2:-}" in
      list) echo '[{"id":"assay@assay","enabled":true}]'; exit 0;;
      enable) echo 'Plugin "assay@assay" is already enabled'; exit 1;;
    esac; exit 0;;
esac
echo "[stub claude] $*"
exit 0
EOF
  cat > "$b/codex" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  --version) echo "codex 0.154.0 (stub)"; exit 0;;
  login) echo "Logged in (stub)"; exit 0;;
  config) case "${2:-}" in --help) echo "Usage: codex config <get|set>"; exit 0;; get) echo true; exit 0;; esac; exit 0;;
  plugin) case "${2:-}" in --help) echo "Usage: codex plugin <list>"; exit 0;; list) echo "assay@assay"; exit 0;; esac; exit 0;;
esac
echo "[stub codex] $*"
exit 0
EOF
  # tmux: never a real server. `has-session` always says no; everything else is a quiet no-op.
  cat > "$b/tmux" <<'EOF'
#!/usr/bin/env bash
for a in "$@"; do case "$a" in has-session) exit 1;; list-panes) echo 4242; exit 0;; esac; done
exit 0
EOF
  cat > "$b/herdr" <<'EOF'
#!/usr/bin/env bash
last="${*: -1}"
if [[ "$last" == "--help" ]]; then
  case "${1:-}" in
    tab)       echo "Usage: herdr tab <create|list|close>";;
    pane)      echo "Usage: herdr pane <run|read>";;
    workspace) echo "Usage: herdr workspace <create|list>";;
    *)         echo "Usage: herdr <tab|pane|workspace|agent>";;
  esac
  exit 0
fi
case "${1:-} ${2:-}" in
  "workspace list") echo '{"result":{"workspaces":[{"workspace_id":"ws-stub"}]}}'; exit 0;;
  "workspace create") echo '{"result":{"workspace":{"workspace_id":"ws-stub"},"tab":{"tab_id":"tab-default"}}}'; exit 0;;
  "tab list") echo '{"result":{"tabs":[]}}'; exit 0;;
  "tab create") echo '{"result":{"root_pane":{"pane_id":"pane-stub"}}}'; exit 0;;
esac
exit 0
EOF
  cat > "$b/orca" <<'EOF'
#!/usr/bin/env bash
last="${*: -1}"
if [[ "$last" == "--help" ]]; then
  case "${1:-} ${2:-}" in
    "terminal create") echo "Usage: orca terminal create --worktree <selector> --name <n> --command <cmd>";;
    "terminal close")  echo "Usage: orca terminal close --worktree <selector> --all";;
    "terminal --help") echo "Usage: orca terminal <create|list|close>";;
    "automations create") echo "Usage: orca automations create --name --repo --trigger --precheck --prompt --provider";;
    "automations --help") echo "Usage: orca automations <create|list|delete>";;
    "repo --help") echo "Usage: orca repo <add|list>";;
    *) echo "Usage: orca <repo|terminal|automations>";;
  esac
  exit 0
fi
exit 0
EOF
  # curl: deskd_up's health probe must FAIL deterministically (there is no deskd here).
  printf '#!/usr/bin/env bash\nexit 7\n' > "$b/curl"
  # pkill: never touches a real process from a test.
  printf '#!/usr/bin/env bash\nexit 1\n' > "$b/pkill"
  # deskwt: worktree_via_deskwt probes `deskwt role-init --help`; refuse it so the oracle and the
  # port both take cellctl's own worktree path (which is not reached under DRY_RUN anyway).
  printf '#!/usr/bin/env bash\nexit 1\n' > "$b/deskwt"
  chmod +x "$b"/*
}

mk_deskbin(){
  local b="$1" v
  mkdir -p "$b"
  for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
    printf '#!/usr/bin/env bash\nexit 0\n' > "$b/$v"
    chmod +x "$b/$v"
  done
}

# -------------------- fixture
# mk_fixture <root> <kind> <harness> <cockpit> <forge> — a complete, self-contained world: a HOME,
# an operator config home, a git checkout, a stub PATH, a desk-tools bindir and one cell named
# `cell`. Written by HAND, never by `cellctl new`: the fixture must not depend on the
# implementation under test.
mk_fixture(){
  local root="$1" kind="$2" harness="$3" cockpit="$4" forge="$5"
  local home="$root/home" real="$root/realconfig" repo="$root/repo"
  # Two `local` lines, never `local cells=… d="$cells/cell"`: in ONE `local` statement the
  # expansion of $cells is resolved before `cells` itself is assigned (the same bash gotcha
  # cmd_desk's mvar/model pair is split for) — under `set -u` that aborts the fixture build
  # silently, and every cell then compares two identical "no such cell" refusals.
  local cellsdir="$root/cells"
  local d="$cellsdir/cell"
  mkdir -p "$home/.config/gh" "$home/.claude" "$home/.codex" "$real" "$cellsdir"
  printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$home/.gitconfig"
  printf '[features]\nmulti_agent = true\n' > "$home/.codex/config.toml"
  printf 'ASSAY_ALLOWED_REPOS=example-org/example-repo\n' > "$real/roster.env"
  printf 'DESK_APP_ID=1\n' > "$real/apps.env"
  mk_stubs "$root/bin"
  mk_deskbin "$root/deskbin"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$root/bin/launcher"; chmod +x "$root/bin/launcher"
  # The cell's checkout. One commit, an origin pointing at itself, docs/streams so a stream-root
  # row resolves, AGENTS.md + .agents/skills so the codex harness rows resolve.
  mkdir -p "$repo/docs/streams" "$repo/.agents/skills/assay"
  printf 'Assay resident operating rules\n' > "$repo/AGENTS.md"
  git init -q -b main "$repo" >/dev/null 2>&1
  git -C "$repo" -c user.name=t -c user.email=t@example.invalid add -A >/dev/null 2>&1
  git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -qm init >/dev/null 2>&1
  git -C "$repo" remote add origin "$repo" >/dev/null 2>&1
  mkdir -p "$d/home/.config/assay" "$d/worktrees" "$d/index" "$d/bin"
  local roots="example-org/example-repo=$repo"
  local common="ROLES=\"the-desk pr-review-desk verify-desk intake-desk worker-desk\"
CELL_COCKPIT=$cockpit
DESK_MODEL_DEFAULT=sonnet
DESK_MODEL_the_desk=fable
CELL_HARNESS=$harness"
  case "$kind" in
    k8s)
      ln -s "$home/.config/gh" "$d/home/.config/gh"
      ln -s "$home/.gitconfig" "$d/home/.gitconfig"
      printf 'DESK_APP_ID=1\n' > "$d/home/.config/assay/apps.env"
      printf 'ASSAY_ALLOWED_REPOS=example-org/example-repo\n' > "$d/home/.config/assay/roster.env"
      # Deliberately NOT a PEM-shaped literal: `check`'s row only asserts the file is readable
      # ([[ -r ]]), nothing here ever parses a key, and a real BEGIN/END block in a committed
      # fixture is exactly what the outbound-write scan refuses.
      printf 'stub deskd app key placeholder (never parsed)\n' > "$d/home/.config/assay/deskd-app.pem"
      printf 'cells: []\n' > "$d/cells-cell.yaml"
      if [[ "$forge" == "gitlab" ]]; then
        printf 'stub\n' > "$d/home/.config/assay/gitlab-deskd.token"
        cat > "$d/cell.env" <<EOF
CELL=cell
CELL_KIND=k8s
CELL_FORGE=gitlab
CELL_REPO=$repo
CELL_ROOTS=$roots
CELLS_CONFIG=$d/cells-cell.yaml
FORGE_API_BASE=https://gitlab.example.invalid/api/v4
GITLAB_API_BASE=https://gitlab.example.invalid/api/v4
GITLAB_GROUP=example-group
GITLAB_TOKEN_STORE=$d/home/.config/assay
DESKD_GITLAB_TOKEN_FILE=$d/home/.config/assay/gitlab-deskd.token
DESKD_ADDR=127.0.0.1:8787
DESKD_INDEX=$d/index/index.db
$common
EOF
      else
        cat > "$d/cell.env" <<EOF
CELL=cell
CELL_KIND=k8s
CELL_FORGE=github
CELL_REPO=$repo
CELL_ROOTS=$roots
CELLS_CONFIG=$d/cells-cell.yaml
FORGE_API_BASE=https://api.github.example.invalid
DESKD_ADDR=127.0.0.1:8787
DESKD_INDEX=$d/index/index.db
DESKD_APP_PEM=$d/home/.config/assay/deskd-app.pem
DESKD_APP_ID_VAR=DESK_APP_ID
ORGS=example-org
$common
EOF
      fi
      ;;
    house)
      ln -s "$real" "$d/home/.config/assay"
      ln -s "$home/.config/gh" "$d/home/.config/gh"
      ln -s "$home/.gitconfig" "$d/home/.gitconfig"
      cat > "$d/cell.env" <<EOF
CELL=cell
CELL_KIND=house
CELL_FORGE=github
CELL_REPO=$repo
CELL_ROOTS=$roots
FORGE_API_BASE=https://api.github.example.invalid
$common
DESKD=0
DESKD_ADDR=127.0.0.1:8787
EOF
      ;;
    container)
      cat > "$d/cell.env" <<EOF
CELL=cell
CELL_KIND=container
CELL_REPO=$repo
CELL_CONTAINER_LAUNCHER=$root/bin/launcher
ROLES="the-desk"
CELL_ROOTS=$roots
CELL_COCKPIT=$cockpit
DESK_MODEL_DEFAULT=sonnet
DESK_MODEL_the_desk=fable
CELL_HARNESS=$harness
DESKD=0
EOF
      ;;
    scrubbed)
      mkdir -p "$d/tmp" "$d/run" "$d/home/.config/gh" "$d/home/.claude" "$d/home/.codex"
      : > "$d/home/.gitconfig"
      printf 'ASSAY_ALLOWED_REPOS=example-org/example-repo\n' > "$d/home/.config/assay/roster.env"
      chmod 700 "$d/home" "$d/home/.config" "$d/home/.config/assay" "$d/tmp"
      chmod 600 "$d/home/.config/assay/roster.env"
      cat > "$d/cell.env" <<EOF
CELL=cell
CELL_KIND=scrubbed
CELL_REPO=$repo
CELL_REPO_SLUG=example-org/example-repo
CELL_ROOTS=$roots
$common
DESKD=0
EOF
      ;;
  esac
}

# -------------------- run + normalise
# The ONLY things normalised: the implementation's own path (each copy is at a different path by
# construction), the fixture root (each copy gets its own), and the two clock-derived tokens the
# oracle itself stamps into a session name / backup filename (`<8>T<6>Z`, `<8>-<6>`) plus the
# scaffold date. Everything else is compared byte for byte.
normalise(){
  local root="$1" impl="$2"
  # ROOT FIRST, then the implementation path. The other order is a real bug: a build placed at
  # /tmp/cellctl-parity is a PREFIX of a fixture root at /private/tmp/cellctl-parity.XXXX, so
  # substituting the implementation first eats half of every root path and the two sides diff on
  # paths that are actually identical.
  sed \
    -e "s#$root#<root>#g" \
    -e "s#$impl#<cellctl>#g" \
    -e 's#[0-9]\{8\}T[0-9]\{6\}Z#<ts>#g' \
    -e 's#[0-9]\{8\}-[0-9]\{6\}#<ts>#g' \
    -e 's#scaffolded [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}#scaffolded <date>#g'
}

# run_impl <root> <impl> <outfile> <verb-and-args...>
run_impl(){
  local root="$1" impl="$2" out="$3"; shift 3
  local rc=0
  env -i \
    HOME="$root/home" \
    PATH="$root/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
    SHELL=/bin/bash \
    TERM=xterm-256color \
    LANG=C \
    TMPDIR="$root/tmp" \
    KUBECONFIG=/dev/null \
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_TERMINAL_PROMPT=0 \
    CELLS_ROOT="$root/cells" \
    DESK_TOOLS_BIN="$root/deskbin" \
    ASSAY_CONFIG_HOME="$root/realconfig" \
    CELLCTL_ORCA_TIMEOUT=1 \
    CELLCTL_DESKWT=0 \
    ${CELLCTL_PARITY_MUTATE:+CELLCTL_PARITY_MUTATE="$CELLCTL_PARITY_MUTATE"} \
    DRY_RUN=1 \
    "$impl" "$@" > "$out.raw" 2>&1 < /dev/null || rc=$?
  normalise "$root" "$impl" < "$out.raw" > "$out"
  printf 'exit=%d\n' "$rc" >> "$out"
}

# tree_manifest <root> <impl> <dir> — the byte-diff surface for `new`: every path under <dir>,
# its mode bits, and (for a regular file) its normalised content.
tree_manifest(){
  local root="$1" impl="$2" dir="$3" p mode
  [[ -d "$dir" ]] || { echo "<no tree>"; return 0; }
  while IFS= read -r p; do
    mode="$(stat -f '%Lp' "$p" 2>/dev/null || stat -c '%a' "$p" 2>/dev/null)"
    printf '%s %s %s\n' "$(printf '%s' "${p#$dir}")" "$mode" "$( [[ -L "$p" ]] && echo link || ( [[ -d "$p" ]] && echo dir || echo file ) )"
    if [[ -f "$p" && ! -L "$p" ]]; then
      normalise "$root" "$impl" < "$p" | sed 's/^/    | /'
    fi
  done < <(find "$dir" | LC_ALL=C sort) | normalise "$root" "$impl"
}

# fixture_ok <id> <rootA> <rootB> — a fixture that failed to build makes BOTH sides print the same
# "no such cell" refusal, which diffs clean. That is a green lamp wired to nothing, so the build is
# asserted here and a broken one is a DIVERGENT cell, never a silent pass.
fixture_ok(){
  local id="$1" a="$2" b="$3"
  if [[ -f "$a/cells/cell/cell.env" && -f "$b/cells/cell/cell.env" && -d "$a/repo/.git" && -d "$b/repo/.git" ]]; then
    return 0
  fi
  echo "  DIVERGENT  $id — fixture did not build (no cell.env or no checkout under $a / $b)"
  cells=$((cells+1)); divergent=$((divergent+1)); divergent_names+=("$id(fixture)")
  return 1
}

# verdict <id> — diff the two captured transcripts, count the cell, and (PARITY_DEBUG=1) show what
# was actually compared, so a cell that is "ok" because BOTH sides printed nothing is visible
# rather than silently green.
verdict(){
  local id="$1"
  if [[ "${PARITY_DEBUG:-0}" == "1" ]]; then
    echo "----- $id (A) -----"; sed 's/^/  A| /' "$T/outA"
  fi
  if diff -u "$T/outA" "$T/outB" > "$T/diff" 2>&1; then
    echo "  ok    $id"
  else
    echo "  DIVERGENT  $id"
    sed 's/^/      /' "$T/diff" | head -40
    divergent=$((divergent+1)); divergent_names+=("$id")
  fi
}

# -------------------- one matrix cell
# cell_case <id> <kind> <harness> <cockpit> <verb-and-args...>
cell_case(){
  local id="$1" kind="$2" harness="$3" cockpit="$4"; shift 4
  [[ -z "$ONLY" || "$id" == *"$ONLY"* ]] || return 0
  local rootA="$T/a/$cells" rootB="$T/b/$cells"
  mkdir -p "$rootA" "$rootB"
  mk_fixture "$rootA" "$kind" "$harness" "$cockpit" github
  mk_fixture "$rootB" "$kind" "$harness" "$cockpit" github
  fixture_ok "$id" "$rootA" "$rootB" || return 0
  run_impl "$rootA" "$CELLCTL_A" "$T/outA" "$@"
  run_impl "$rootB" "$CELLCTL_B" "$T/outB" "$@"
  cells=$((cells+1))
  verdict "$id"
  rm -rf "$rootA" "$rootB"
}

# new_case <kind> <forge> — `new` has no dry run, so parity is the tree each side scaffolds.
new_case(){
  local kind="$1" forge="$2" id="$kind/$forge/-/new"
  [[ -z "$ONLY" || "$id" == *"$ONLY"* ]] || return 0
  local rootA="$T/a/$cells" rootB="$T/b/$cells"
  mkdir -p "$rootA" "$rootB"
  mk_fixture "$rootA" "$kind" claude tmux "$forge"
  mk_fixture "$rootB" "$kind" claude tmux "$forge"
  fixture_ok "$id" "$rootA" "$rootB" || return 0
  local -a args=()
  case "$kind" in
    k8s)       args=(new fresh --kind k8s --forge "$forge" --repo "REPO" --cells-yaml "ROOT/cells/cell/cells-cell.yaml")
               if [[ "$forge" == github ]]; then args+=(--orgs example-org --deskd-app-pem "ROOT/cells/cell/home/.config/assay/deskd-app.pem")
               else args+=(--group example-group); fi ;;
    house)     args=(new fresh --kind house --repo "REPO" --roots "example-org/example-repo=REPO") ;;
    container) args=(new fresh --kind container --repo example-org/example-repo --launcher "ROOT/bin/launcher") ;;
    scrubbed)  args=(new fresh --kind scrubbed --repo "REPO" --repo-slug example-org/example-repo --roots "example-org/example-repo=REPO") ;;
  esac
  local -a argsA=() argsB=() a
  for a in "${args[@]}"; do
    argsA+=("${a//ROOT/$rootA}"); argsA[-1]="${argsA[-1]//REPO/$rootA/repo}"
    argsB+=("${a//ROOT/$rootB}"); argsB[-1]="${argsB[-1]//REPO/$rootB/repo}"
  done
  run_impl "$rootA" "$CELLCTL_A" "$T/outA" "${argsA[@]}"
  run_impl "$rootB" "$CELLCTL_B" "$T/outB" "${argsB[@]}"
  tree_manifest "$rootA" "$CELLCTL_A" "$rootA/cells/fresh" >> "$T/outA"
  tree_manifest "$rootB" "$CELLCTL_B" "$rootB/cells/fresh" >> "$T/outB"
  cells=$((cells+1))
  verdict "$id"
  rm -rf "$rootA" "$rootB"
}

# -------------------- the matrix
echo "parity: A=$CELLCTL_A"
echo "parity: B=$CELLCTL_B"
for kind in k8s house container scrubbed; do
  for harness in claude codex; do
    for cockpit in tmux herdr orca; do
      base="$kind/$harness/$cockpit"
      cell_case "$base/check"  "$kind" "$harness" "$cockpit" check cell
      cell_case "$base/desk"   "$kind" "$harness" "$cockpit" desk cell the-desk
      cell_case "$base/up"     "$kind" "$harness" "$cockpit" up cell --no-attach
      cell_case "$base/down"   "$kind" "$harness" "$cockpit" down cell
      cell_case "$base/set"    "$kind" "$harness" "$cockpit" set cell DESK_MODEL_DEFAULT=haiku
      cell_case "$base/ls"     "$kind" "$harness" "$cockpit" ls
      cell_case "$base/smoke"  "$kind" "$harness" "$cockpit" smoke cell
      cell_case "$base/status" "$kind" "$harness" "$cockpit" status cell
    done
  done
done
for kind in k8s house container scrubbed; do
  for forge in github gitlab; do
    # Only a k8s cell has a forge axis at `new`; the other kinds scaffold one shape, so their
    # gitlab leg is the SAME call and proves the flag is refused/ignored identically.
    new_case "$kind" "$forge"
  done
done

echo "parity: $cells cells, $divergent divergent"
if [[ "$divergent" -gt 0 ]]; then
  echo "parity: divergent cells: ${divergent_names[*]}" >&2
  exit 1
fi
exit 0
