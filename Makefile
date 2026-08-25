# Root Makefile — desk-tools build/install targets (the public release home).
#
# desk-build   : unprivileged local build of every tools/desk/cmd/* into
#                tools/desk/dist/. Safe for agents/CI; must succeed with ZERO cmds.
# desk-install : HUMAN-ONLY (`sudo make desk-install`). Builds via desk-build as the
#                invoking user ($SUDO_USER, never root) into tools/desk/dist/, THEN
#                installs those already-built binaries root-owned 0755 to
#                /opt/desk-tools/bin and writes tools/desk/MANIFEST.sha256.
#                The sudo password IS the manual permission gate — no agent runs
#                this target.
#
# Version stamp: SourceSHA + BuiltAt are embedded via -ldflags so every audit
# record and --version shows exactly which source a binary was built from.
#
# `tools/desk/` is canonical here (medici-finance/assay) post-dehouse; this
# Makefile is its build home. The README documents these targets — keep the two in
# step.

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

.PHONY: desk-build desk-install desk-manifest desk-hook-install desk-test skillslint guardrail-sync

SKILLSLINT_DIR := tools/skillslint

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

## desk-hook-install: install the pre-push hook shim into .githooks/ (core.hooksPath).
desk-hook-install:
	@hook=".githooks/pre-push"; \
	src="$(DESK_DIR)/hooks/pre-push"; \
	if [ ! -f "$$src" ]; then \
		echo "desk-hook-install: $$src not found — nothing to install (ok)"; \
		exit 0; \
	fi; \
	if [ -f "$$hook" ]; then \
		if grep -q deskpushguard "$$hook" 2>/dev/null; then \
			echo "desk-hook-install: pre-push hook already installed (idempotent skip)"; \
			exit 0; \
		fi; \
		old_name="`head -1 \"$$hook\" 2>/dev/null`"; \
		if [ "$(FORCE)" = "1" ]; then \
			echo "desk-hook-install: FORCE=1: overwriting existing non-deskpushguard pre-push hook: $$old_name"; \
		else \
			echo "desk-hook-install: refusing to clobber existing non-deskpushguard pre-push hook: $$old_name"; \
			echo "  Use FORCE=1 to overwrite, e.g.: make desk-hook-install FORCE=1"; \
			exit 1; \
		fi; \
	fi; \
	cp "$$src" "$$hook"; \
	chmod +x "$$hook"; \
	echo "desk-hook-install: installed pre-push hook (deskpushguard)"

## desk-test: run the deskkit test suite (incl. negative-path refusal tests)
desk-test:
	@cd $(DESK_DIR) && go test ./... -count=1

## skillslint: validate the desk-role skill homes under plugins/assay/skills/ AND
## byte-diff every shared-guardrail copy against its one declared source,
## .claude/guardrails/GUARDRAILS.md. Three-state: could-not-check is a failure,
## never a quiet pass. Offline; safe for agents/CI.
skillslint:
	@cd $(SKILLSLINT_DIR) && go run . --root ../..

## guardrail-sync: REGENERATE every shared-guardrail copy from its one declared
## source, .claude/guardrails/GUARDRAILS.md. Edit a shared rule THERE, run this,
## commit the regenerated skills — never hand-edit a copy in a SKILL.md.
guardrail-sync:
	@cd $(SKILLSLINT_DIR) && go run . --root ../.. --sync
