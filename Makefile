# Root Makefile — desk-tools build/install targets (the public release home).
#
# desk-build   : unprivileged local build of every tools/desk/cmd/* into
#                tools/desk/dist/. Safe for agents/CI; must succeed with ZERO cmds.
# desk-install : HUMAN-ONLY (`sudo make desk-install`). Builds via desk-build as the
#                invoking user ($SUDO_USER, never root) into tools/desk/dist/, THEN
#                installs those already-built binaries root-owned 0755 to
#                /opt/desk-tools/bin and writes tools/desk/MANIFEST.sha256.
#                The sudo password IS the manual permission gate — no agent runs
#                this target. `cellctl` is one of those cmd/* builds since
#                desk-containers/10 (it was a hand-maintained shell script COPIED
#                into the bindir until then), so a local `make desk-install`
#                matches what a downloaded desk-tools release tarball gives an
#                adopter (#850) with no special case either side.
#
# Version stamp: SourceSHA + BuiltAt are embedded via -ldflags so every audit
# record and --version shows exactly which source a binary was built from.
#
# `tools/desk/` is canonical here (medici-finance/assay) post-dehouse; this
# Makefile is its build home. The README documents these targets — keep the two in
# step.
#
# Windows counterpart: scripts/build-windows.ps1 mirrors the .PHONY target set
# below (PowerShell; no nmake/VS/make needed). Adding or removing a target here
# means editing that script too — tools/winparity asserts the two target sets are
# equal and reddens on drift (see tools/winparity/README.md).

SHELL       := /bin/sh
DESK_MODULE  := github.com/medici-finance/assay/tools/desk
DESK_PKG     := $(DESK_MODULE)/internal/deskkit
DESK_DIR     := tools/desk
DIST_DIR     := $(DESK_DIR)/dist
INSTALL_DIR  := /opt/desk-tools/bin
MANIFEST     := $(DESK_DIR)/MANIFEST.sha256

SHA      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT_AT := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X $(DESK_PKG).SourceSHA=$(SHA) -X $(DESK_PKG).BuiltAt=$(BUILT_AT)

DESK_CMDS := $(wildcard $(DESK_DIR)/cmd/*)

.PHONY: build-all desk-build desk-install desk-manifest desk-hook-install desk-test skillslint guardrail-sync paired-versions

SKILLSLINT_DIR       := tools/skillslint
PAIREDVERSIONS_DIR   := tools/pairedversions

## build-all: compile every tracked Go module, then emit every desk executable.
## Discovery matches CI; no hand-maintained module list can omit a new module.
build-all:
	@set -eu; modules="$$(git ls-files -- '*go.mod')"; \
	[ -n "$$modules" ] || { echo "build-all: no tracked Go modules" >&2; exit 1; }; \
	printf '%s\n' "$$modules" | while IFS= read -r mod; do \
		echo "build-all: $${mod%/go.mod}"; \
		( cd "$$(dirname "$$mod")" && go build ./... ) || exit 1; \
	done
	@$(MAKE) --no-print-directory desk-build

## desk-build: local unprivileged build into tools/desk/dist/
desk-build:
	@mkdir -p $(DIST_DIR)
	@cmds="$(DESK_CMDS)"; \
	if [ -z "$$cmds" ]; then \
		echo "desk-build: no $(DESK_DIR)/cmd/* yet — nothing to build (ok)"; \
	else \
		for d in $$cmds; do \
			name=`basename $$d`; \
			echo "desk-build: building $$name"; \
			( cd $(DESK_DIR) && go build -ldflags "$(LDFLAGS)" -o "dist/$$name" "./cmd/$$name" ) || exit 1; \
		done; \
	fi

## desk-install: HUMAN-ONLY via sudo — builds unprivileged (desk-build), then installs
## the already-built tools/desk/dist/ binaries root-owned 0755.
desk-install:
	@build_user="$${SUDO_USER:-`id -un`}"; \
	if [ "$$build_user" = "root" ]; then \
		echo "desk-install: building as root (no SUDO_USER — running desk-build directly)"; \
		make --no-print-directory desk-build || exit 1; \
	else \
		echo "desk-install: building as $$build_user (desk-build, unprivileged)"; \
		su "$$build_user" -c "cd '$(CURDIR)' && make --no-print-directory desk-build" || exit 1; \
	fi
	@echo ">>> desk-install: installing to $(INSTALL_DIR) (root-owned 0755); this is a HUMAN act"
	@install -d -m 0755 $(INSTALL_DIR)
	@cmds="$(DESK_CMDS)"; \
	if [ -z "$$cmds" ]; then \
		echo "desk-install: no $(DESK_DIR)/cmd/* yet — nothing to install (ok)"; \
	else \
		for d in $$cmds; do \
			name=`basename $$d`; \
			if [ ! -f "$(DIST_DIR)/$$name" ]; then \
				echo "desk-install: $(DIST_DIR)/$$name missing after desk-build — aborting"; \
				exit 1; \
			fi; \
			install -o root -g 0 -m 0755 "$(DIST_DIR)/$$name" "$(INSTALL_DIR)/$$name"; \
			echo "desk-install: installed $(INSTALL_DIR)/$$name"; \
		done; \
	fi
	@make --no-print-directory desk-hook-install
	@make --no-print-directory desk-manifest

## desk-manifest: (re)write tools/desk/MANIFEST.sha256 from installed binaries
desk-manifest:
	@if ls $(INSTALL_DIR)/* >/dev/null 2>&1; then \
		( cd $(INSTALL_DIR) && shasum -a 256 * ) > $(MANIFEST); \
		echo "desk-manifest: wrote $(MANIFEST)"; \
	else \
		: > $(MANIFEST); \
		echo "desk-manifest: no installed binaries; wrote empty $(MANIFEST)"; \
	fi

## desk-hook-install: delegate all hook ownership/migration to the built guard.
## desk-install builds first. Standalone callers must run desk-build first too;
## there is no second static-copy hook writer.
desk-hook-install:
	@guard="$(DIST_DIR)/deskpushguard"; \
	if [ ! -x "$$guard" ]; then \
		echo "desk-hook-install: $$guard missing -- run make desk-build first" >&2; \
		exit 1; \
	fi; \
	force_flag=""; \
	if [ "$(FORCE)" = "1" ]; then force_flag="--force"; fi; \
	"$$guard" hook-install --dir .githooks $$force_flag

## desk-test: run the deskkit test suite (incl. negative-path refusal tests)
desk-test:
	@cd $(DESK_DIR) && go test ./... -count=1

## skillslint: validate the desk-role skill homes under plugins/assay/skills/,
## check EVERY *.md under plugins/ for an unresolved house value in a driver
## position (the neutral `human:<name>` token is the only permitted form), AND
## byte-diff every shared-guardrail copy against its one declared source,
## .claude/guardrails/GUARDRAILS.md. Three-state: could-not-check is a failure,
## never a quiet pass. Offline; safe for agents/CI. See tools/skillslint/README.md.
skillslint:
	@cd $(SKILLSLINT_DIR) && go run . --root ../..

## paired-versions: assert the adopter FRONT DOOR is consistent — plugin.json's
## version equals paired-versions.yaml's `plugin`, each paired tag is a PUBLISHED
## release of its release home, and every per-platform sha256 equals that
## release's own checksums.txt entry, and no newer published release has gone
## un-pinned for more than seven days. `assay:install` resolves the binary an
## adopter gets from that manifest, so a plugin bump that skipped the re-pin
## installs the tool the PREVIOUS plugin was paired with.
## Three-state and fail-closed: a disagreement and a could-not-check both exit
## non-zero. Reads api.github.com; honours $GH_TOKEN/$GITHUB_TOKEN.
## See tools/pairedversions/README.md.
paired-versions:
	@cd $(PAIREDVERSIONS_DIR) && go run . --root ../..

## guardrail-sync: REGENERATE every shared-guardrail copy from its one declared
## source, .claude/guardrails/GUARDRAILS.md. Edit a shared rule THERE, run this,
## commit the regenerated skills — never hand-edit a copy in a SKILL.md.
guardrail-sync:
	@cd $(SKILLSLINT_DIR) && go run . --root ../.. --sync
