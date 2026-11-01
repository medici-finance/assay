#!/bin/sh
# desk-run.sh — fire up one assay desk interactively from a desktop.
#
# Runs the chosen desk image the way a desk is run from a terminal today:
#
#   docker run -it --name <desk> -v <desk>-work:/work \
#     --mount type=bind,src=<pem>,dst=/run/secrets/assay/app.pem,ro \
#     --env-file <env-file> ghcr.io/medici-finance/assay/<desk>:<tag>
#
# That is the `docker run` recipe of containers/secrets.md section 4.1, composed
# from three inputs this script validates: a desk name, an App PEM file and a
# model env-file. This script implements the runtime credential contract and
# adds no credential handling of its own:
#
#   - It passes credential PATHS to docker and nothing else. It never reads a
#     PEM or an env-file, never copies one anywhere, and never prints a value
#     from either (--dry-run included). A credential is mounted or passed by
#     reference at runtime; nothing here writes one into an image.
#   - Fail-closed preflight: an unknown desk, a missing or unreadable PEM, or a
#     missing or unreadable env-file refuses with a message naming the expected
#     location. There is no fallback to ambient host credentials, no search of
#     other directories and no anonymous mode. An override that is given but
#     empty (--pem "", --env-file "", or an empty ASSAY_DESK_PEM_FILE /
#     ASSAY_DESK_ENV_FILE) names no file, so it refuses too: it is never read
#     as "not given" and never falls through to another credential.
#   - Misuse is not echoed: an unknown option, desk name or tag is reported by
#     its position and length, never its text, because stderr often lands in
#     shell logs and a credential pasted in the wrong place must not come back.
#   - The container re-runs its own credential preflight on every start
#     (containers/entrypoint.sh), so a missing model credential INSIDE the
#     env-file is caught there, not guessed at here.
#
# Exit codes: 0 success (or a printed --dry-run command); 2 usage error or
# unknown desk; 3 credential preflight refused (including an empty override);
# 4 docker unavailable or the container is in the wrong state for the request. docker's own exit status is
# passed through once it has been exec'd.
#
# POSIX sh; needs docker (not for --dry-run) and coreutils only.
set -eu

# --- fixed facts -------------------------------------------------------------
registry="ghcr.io/medici-finance/assay"
desks="intake-desk worker-desk pr-review-desk verify-desk the-desk"
pem_dst="/run/secrets/assay/app.pem"   # containers/secrets.md section 2
prog="desk-run.sh"

usage() {
  cat <<EOF
Usage: $prog <desk-name> [--version <tag>] [--dry-run] [--attach]
                         [--pem <file>] [--env-file <file>]

Run an assay desk interactively in a container.

  <desk-name>          one of: $desks
  --version <tag>      image tag to run (default: latest)
  --dry-run            print the docker command instead of running it
  --attach             start the existing container of that name again and
                       attach (docker start -ai): its entrypoint runs into a
                       NEW shell; /work and the container filesystem are kept,
                       earlier processes are not. Takes no --version, --pem or
                       --env-file: the container's image and mounts are fixed.
  --pem <file>         bot App PEM on this host
                       (env ASSAY_DESK_PEM_FILE; default <config>/assay/app.pem);
                       one default serves every desk, so pass --pem per desk
                       when the desks use different Apps
  --env-file <file>    model credentials as an env-file on this host
                       (env ASSAY_DESK_ENV_FILE; default <config>/assay/desk.env)
  -h, --help           this text

<config> is \$XDG_CONFIG_HOME, or \$HOME/.config when that is unset or empty.
An override that is given but empty is refused, not skipped.
Credentials are passed to docker by path and are never printed.
See containers/secrets.md and docs/docker.md.
EOF
}

# die <exit-code> <line>... : print the lines to stderr and exit.
die() {
  code=$1
  shift
  for line in "$@"; do
    printf '%s\n' "$line" >&2
  done
  exit "$code"
}

# quote <word> : print a word shell-quoted only when it needs it, so a dry-run
# line can be pasted into a shell and means the same thing.
quote() {
  case $1 in
    '') printf "''" ;;
    *[!A-Za-z0-9_@%+=:,./-]*)
      printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
      ;;
    *) printf '%s' "$1" ;;
  esac
}

# abspath <path> : expand a leading ~/ and anchor a relative path at $PWD.
# docker --mount type=bind needs an absolute source.
abspath() {
  case $1 in
    \~/*) printf '%s/%s' "${HOME:-}" "${1#\~/}" ;;
    /*) printf '%s' "$1" ;;
    *) printf '%s/%s' "$PWD" "$1" ;;
  esac
}

# --- argument parsing --------------------------------------------------------
desk=""
version="latest"
dry_run=0
attach=0
version_set=0
pem_arg=""
env_arg=""
pem_set=0
env_set=0
argn=0

# need_value <flag> <argc-left> : a flag that takes a value must have one.
need_value() {
  [ "$2" -ge 2 ] || die 2 "$prog: $1 needs a value" "run '$prog --help' for usage"
}

while [ $# -gt 0 ]; do
  argn=$((argn + 1))
  case $1 in
    -h | --help)
      usage
      exit 0
      ;;
    --dry-run) dry_run=1 ;;
    --attach) attach=1 ;;
    --version)
      need_value "$1" $#
      version=$2
      version_set=1
      shift
      argn=$((argn + 1))
      ;;
    --version=*)
      version=${1#--version=}
      version_set=1
      ;;
    --pem)
      need_value "$1" $#
      pem_arg=$2
      pem_set=1
      shift
      argn=$((argn + 1))
      ;;
    --pem=*)
      pem_arg=${1#--pem=}
      pem_set=1
      ;;
    --env-file)
      need_value "$1" $#
      env_arg=$2
      env_set=1
      shift
      argn=$((argn + 1))
      ;;
    --env-file=*)
      env_arg=${1#--env-file=}
      env_set=1
      ;;
    -*)
      die 2 "$prog: unknown option (argument $argn, ${#1} characters; not echoed)" \
        "run '$prog --help' for usage"
      ;;
    *)
      [ -z "$desk" ] || die 2 "$prog: more than one desk name given (second one is argument $argn, ${#1} characters; not echoed)" \
        "run one desk per invocation"
      desk=$1
      ;;
  esac
  shift
done

# --- validation --------------------------------------------------------------
[ -n "$desk" ] || die 2 "$prog: missing desk name" "valid desks: $desks" \
  "run '$prog --help' for usage"

valid=0
for d in $desks; do
  [ "$d" = "$desk" ] && valid=1
done
if [ "$valid" -ne 1 ]; then
  die 2 "$prog: unknown desk name (${#desk} characters; not echoed)" "valid desks: $desks"
fi

# A tag is interpolated into an image reference, so it follows docker's tag
# grammar exactly: [A-Za-z0-9_][A-Za-z0-9_.-]{0,127}.
case $version in
  '' | [!A-Za-z0-9_]* | *[!A-Za-z0-9_.-]*)
    die 2 "$prog: invalid --version (${#version} characters; not echoed)" \
      "a tag is letters, digits, '_', '.' and '-', and starts with a letter, digit or '_'"
    ;;
esac
[ "${#version}" -le 128 ] || die 2 "$prog: invalid --version: longer than 128 characters"

if [ "$attach" -eq 1 ] && [ "$version_set" -eq 1 ]; then
  die 2 "$prog: --attach resumes an existing container, whose image version is fixed" \
    "drop --version, or remove the container and start it again"
fi

# --attach starts a container that already carries its mounts and env, so a
# credential flag would be ignored while looking as if it took effect (an
# operator who rotated a PEM would keep the old mount). Refuse, like --version.
if [ "$attach" -eq 1 ] && { [ "$pem_set" -eq 1 ] || [ "$env_set" -eq 1 ]; }; then
  die 2 "$prog: --attach resumes an existing container, whose mounts and env-file are fixed" \
    "drop --pem and --env-file, or remove the container (docker rm $desk) and start it again"
fi

# --- credential preflight (fail-closed, new runs only) -----------------------
# --attach resumes a container that already carries its mounts and env; its
# entrypoint re-runs the container-side preflight on start.
pem=""
envf=""
if [ "$attach" -ne 1 ]; then
  # An override that is GIVEN and EMPTY is not the same as one not given: it
  # asked for a named credential and named none. Resolving it to the next
  # source would start the desk with a different App identity or model key and
  # say nothing, so it refuses (exit 3). `${VAR+set}` tells set-but-empty from
  # unset. The precedence is flag, then variable, then default; an empty
  # variable is only consulted, and so only refused, when no flag was given.
  empty_override() { # <what> <fix>
    printf '%s\n' "$prog: $desk cannot start: $1 was given empty." \
      "  an empty value names no file, so it is refused rather than read as 'not given'." \
      "  $2" >&2
  }
  refused=0
  pem_src=""
  env_src=""
  if [ "$pem_set" -eq 1 ]; then
    pem_src="--pem"
    pem=$pem_arg
    if [ -z "$pem" ]; then
      refused=1
      empty_override "--pem" "pass a PEM file path, or drop --pem to use ASSAY_DESK_PEM_FILE or the default; see containers/secrets.md section 2."
    fi
  elif [ "${ASSAY_DESK_PEM_FILE+set}" = set ]; then
    pem_src="env ASSAY_DESK_PEM_FILE"
    pem=$ASSAY_DESK_PEM_FILE
    if [ -z "$pem" ]; then
      refused=1
      empty_override "ASSAY_DESK_PEM_FILE" "set it to a PEM file path, or unset it to use the default; see containers/secrets.md section 2."
    fi
  else
    pem_src="default"
  fi
  if [ "$env_set" -eq 1 ]; then
    env_src="--env-file"
    envf=$env_arg
    if [ -z "$envf" ]; then
      refused=1
      empty_override "--env-file" "pass an env-file path, or drop --env-file to use ASSAY_DESK_ENV_FILE or the default; see containers/secrets.md section 3."
    fi
  elif [ "${ASSAY_DESK_ENV_FILE+set}" = set ]; then
    env_src="env ASSAY_DESK_ENV_FILE"
    envf=$ASSAY_DESK_ENV_FILE
    if [ -z "$envf" ]; then
      refused=1
      empty_override "ASSAY_DESK_ENV_FILE" "set it to an env-file path, or unset it to use the default; see containers/secrets.md section 3."
    fi
  else
    env_src="default"
  fi
  [ "$refused" -eq 0 ] || exit 3

  if [ -z "$pem" ] || [ -z "$envf" ]; then
    if [ -n "${XDG_CONFIG_HOME:-}" ]; then
      cfg=$XDG_CONFIG_HOME
    elif [ -n "${HOME:-}" ]; then
      cfg=$HOME/.config
    else
      die 3 "$prog: cannot work out the default credential locations: neither" \
        "XDG_CONFIG_HOME nor HOME is set" \
        "pass --pem and --env-file (or set ASSAY_DESK_PEM_FILE and ASSAY_DESK_ENV_FILE)"
    fi
    [ -n "$pem" ] || pem=$cfg/assay/app.pem
    [ -n "$envf" ] || envf=$cfg/assay/desk.env
  fi
  pem=$(abspath "$pem")
  envf=$(abspath "$envf")

  # Both are reported in one pass, so the operator fixes everything at once.
  if [ ! -e "$pem" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the bot App PEM is missing." \
      "  expected a readable file at: $pem  ($pem_src)" \
      "  override with --pem <file> or ASSAY_DESK_PEM_FILE; see containers/secrets.md section 2." >&2
  elif [ ! -f "$pem" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the bot App PEM path is not a regular file." \
      "  path: $pem  ($pem_src)" >&2
  elif [ ! -r "$pem" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the bot App PEM is not readable by this user." \
      "  path: $pem  ($pem_src)" >&2
  fi
  if [ ! -e "$envf" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the model env-file is missing." \
      "  expected a readable file at: $envf  ($env_src)" \
      "  override with --env-file <file> or ASSAY_DESK_ENV_FILE; see containers/secrets.md section 3." >&2
  elif [ ! -f "$envf" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the model env-file path is not a regular file." \
      "  path: $envf  ($env_src)" >&2
  elif [ ! -r "$envf" ]; then
    refused=1
    printf '%s\n' "$prog: $desk cannot start: the model env-file is not readable by this user." \
      "  path: $envf  ($env_src)" >&2
  fi
  [ "$refused" -eq 0 ] || exit 3

  # --mount takes comma-separated key=value pairs, so a comma or quote in the
  # source would add a mount option of the caller's choosing; a newline cannot
  # be passed on one line. Refuse them rather than guess at an escape.
  nl=$(printf '\n_')
  nl=${nl%_}
  case $pem in
    *,* | *\"* | *"$nl"*)
      die 3 "$prog: the PEM path contains a comma, a double quote or a newline," \
        "which cannot be passed safely to docker --mount: $pem" \
        "move or link the file to a plain path and pass that."
      ;;
  esac
fi

# --- compose -----------------------------------------------------------------
if [ "$attach" -eq 1 ]; then
  set -- start -ai "$desk"
else
  set -- run -it --name "$desk" \
    -v "$desk-work:/work" \
    --mount "type=bind,src=$pem,dst=$pem_dst,ro" \
    --env-file "$envf" \
    "$registry/$desk:$version"
fi

if [ "$dry_run" -eq 1 ]; then
  printf 'docker'
  for word in "$@"; do
    printf ' %s' "$(quote "$word")"
  done
  printf '\n'
  exit 0
fi

# --- run ---------------------------------------------------------------------
command -v docker >/dev/null 2>&1 || die 4 "$prog: docker is not on PATH" \
  "install docker and make sure the daemon is running, then retry"

# Does a container named for the desk already exist? `container ls` exits
# non-zero when the daemon cannot be reached, which is could-not-check here,
# never "no container".
if ! existing=$(docker container ls -a --filter "name=^/$desk\$" --format '{{.Names}}' 2>/dev/null); then
  die 4 "$prog: cannot ask docker about container '$desk' (is the daemon running?)"
fi
have=0
[ "$existing" = "$desk" ] && have=1

if [ "$attach" -eq 1 ]; then
  [ "$have" -eq 1 ] || die 4 "$prog: no container named '$desk' to re-attach to" \
    "start the desk first: $prog $desk"
else
  [ "$have" -eq 0 ] || die 4 "$prog: a container named '$desk' already exists" \
    "start it again and attach (its entrypoint runs into a NEW shell; /work and the container filesystem are kept): $prog $desk --attach" \
    "or remove it first: docker rm $desk"
fi

exec docker "$@"
