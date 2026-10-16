# shellcheck shell=bash
# tools/fleet-gitlab-roles.sh — the ONE Assay GitLab fleet role table.
#
# Sourced (never executed) by tools/renew-fleet-gitlab-tokens.sh, which renews
# every role PAT in one run. The provisioner that mints the accounts and first
# PATs — tools/create-fleet-gitlab.sh, an adapter onto tools/desk/cmd/deskgitlab
# (desktools-v2/30) — carries the same table in Go; deskgitlab's
# tables_parity_test.go reads THIS file and fails if the two disagree.
#
# It lives in its own file so the role -> access-level -> scope mapping and the
# naming convention have a single source. A second copy in either script would
# let the renewal and the provisioner drift apart on scopes or PAT names, and a
# renewal that looks for the wrong PAT name silently creates a duplicate
# credential instead of rotating the live one. tools/renew-fleet-gitlab-tokens_test.sh
# fails if a role row is found anywhere but here.
#
# Design reference: docs/streams/forge-gitlab/spec.md §2 (identity model).

# ---------------------------------------------------------------------------
# Role table (spec.md §2) — one line per Assay role the fleet provisions.
# `promote` is deliberately absent: per spec §2 it usually has no GitLab
# identity at all, so there is nothing here to create for it.
#
# role:access_level_name:access_level_num:csv_scopes
# ---------------------------------------------------------------------------
# shellcheck disable=SC2034  # consumed by the scripts that source this file
ROLE_TABLE='
reviewer:developer:30:api
worker:developer:30:api,write_repository
verifier:developer:30:api,write_repository
desk:developer:30:api
issue-loop:reporter:20:api
intake-loop:reporter:20:api
board-writer:developer:30:api,write_repository
'

# FLEET_PAT_DAYS — the shared default PAT lifetime (spec.md §5's "7 days
# RECOMMENDED" expiry backstop). A second literal in either script would let
# a widened default silently relax that backstop. The renewal
# (--duration's default) reads it from here; deskgitlab's --pat-expiry-days
# default is parity-tested against it.
# shellcheck disable=SC2034  # consumed by the scripts that source this file
FLEET_PAT_DAYS=7

# fleet_username PREFIX ROLE — the service-account username the provisioner
# creates for ROLE under PREFIX.
fleet_username() { printf '%s-%s-bot' "$1" "$2"; }

# fleet_pat_name ROLE — the stable PAT name the provisioner mints for ROLE. A
# rotation keeps the name, so this is also the name a renewal matches on.
fleet_pat_name() { printf 'assay-%s-fleet' "$1"; }

# fleet_token_file ROLE — the custody file name the desk verbs read for ROLE
# (desktoken --forge gitlab <role>; docs/adopting-assay-gitlab.md §2, §5).
fleet_token_file() { printf 'gitlab-%s.token' "$1"; }
