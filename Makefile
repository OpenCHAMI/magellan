# SPDX-FileCopyrightText: © 2023-2025 Triad National Security, LLC. All rights reserved.
# SPDX-FileCopyrightText: © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Set path to commands
GO             ?= $(shell command -v go 2>/dev/null)
GOLANGCI_LINT  ?= $(shell command -v golangci-lint 2>/dev/null)
GORELEASER     ?= $(shell command -v goreleaser 2>/dev/null)
GIT            ?= $(shell command -v git 2>/dev/null)
AWK            ?= $(shell command -v awk 2>/dev/null)
MISSPELL       ?= $(shell command -v misspell 2>/dev/null)
REUSE          ?= $(shell command -v reuse 2>/dev/null)
# Use HOSTCMD to not conflict with Make's $(HOSTNAME)
HOSTCMD        ?= $(shell command -v hostname 2>/dev/null)
INSTALL        ?= $(shell command -v install 2>/dev/null)
SCDOC          ?= $(shell command -v scdoc 2>/dev/null)
CONTAINER_PROG ?= $(shell command -v docker 2>/dev/null)
SHELL          ?= /bin/sh

INSTALL_PROGRAM ?= $(INSTALL) -Dm755
INSTALL_DATA    ?= $(INSTALL) -Dm644

# Recursive wildcard function, obtained from https://stackoverflow.com/a/18258352
#
# Arg 1: Space-separated list of directories to recurse into
# Arg 2: Space-separated list of patterns to match
rwildcard = $(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))

# Function to check if a command is available and error if not found
#
# Arg 1: Command path (can be a variable like $(GO) or direct path)
# Arg 2: Command name for error message
# Usage: $(call require-command-shell,$(GO),go)
define require-command-shell
@if [ -z "$(1)" ]; then \
	echo "make: *** $(2) command not found" >&2; \
	exit 1; \
fi
endef

# require-command-shell, but for Makefile-level checks
#
# Arg 1: Command path (can be a variable like $(GO) or direct path)
# Arg 2: Command name for error message
# Usage: $(call require-command-make,$(GO),go)
#
# Note: This function is intended to be used with $(eval $(call ...)).
define require-command-make
ifeq ($$(strip $(1)),)
$$(error '$(2)' command not found)
endif
endef

# Check that required commands are present
$(eval $(call require-command-make,$(GIT),git))
$(eval $(call require-command-make,$(HOSTCMD),hostname))

# Ensure shell is defined
ifeq ($(SHELL),)
$(error '$(SHELL)' undefined.)
endif

NAME       ?= magellan
VERSION    ?= $(shell $(GIT) describe --tags --always --dirty --broken --abbrev=0)
BUILD      ?= $(shell $(GIT) rev-parse --short HEAD)
GIT_BRANCH ?= $(shell $(GIT) rev-parse --abbrev-ref HEAD)
GIT_TAG    ?= $(shell $(GIT) describe --tags --abbrev=0 2>/dev/null || echo "unknown")
GIT_STATE  ?= $(shell if $(GIT) diff-index --quiet HEAD --; then echo 'clean'; else echo 'dirty'; fi)
BUILD_HOST ?= $(shell $(HOSTCMD))
GO_VERSION ?= $(shell $(GO) env GOVERSION)
BUILD_USER ?= $(shell whoami)
IMPORT     := github.com/OpenCHAMI/magellan/
CONTAINER_TAG ?= latest
FQCN          ?= ghcr.io/openchami/$(NAME):$(CONTAINER_TAG)
LDFLAGS := -s \
	   -X $(IMPORT)internal/version.GitCommit=$(BUILD) \
	   -X $(IMPORT)internal/version.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
	   -X $(IMPORT)internal/version.Version=$(VERSION) \
	   -X $(IMPORT)internal/version.GitBranch=$(GIT_BRANCH) \
	   -X $(IMPORT)internal/version.GitTag=$(GIT_TAG) \
	   -X $(IMPORT)internal/version.GitState=$(GIT_STATE) \
	   -X $(IMPORT)internal/version.BuildHost=$(BUILD_HOST) \
	   -X $(IMPORT)internal/version.GoVersion=$(GO_VERSION) \
	   -X $(IMPORT)internal/version.BuildUser=$(BUILD_USER)
INTERNAL := $(call rwildcard,internal,*.go)
PKG      := $(call rwildcard,pkg,*.go)
MANSRC   := $(wildcard man/*.sc)
MANBIN   := $(subst .sc,,$(MANSRC))
MAN1BIN  := $(filter %.1,$(MANBIN))

prefix      ?= /usr/local
exec_prefix ?= $(prefix)
bindir      ?= $(exec_prefix)/bin
mandir      ?= $(exec_prefix)/man

.PHONY: all
all: binaries ## Build everything

.PHONY: binaries
binaries: $(NAME) ## Build binaries

.PHONY: help
help: ## Show this help
	$(call require-command-shell,$(AWK),awk)
	@$(AWK) 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m[VAR=val]... <target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z0-9_\/.-]+:.*##/ { \
	        printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)

.PHONY: ci
ci: all diff ## Run the CI build pipeline

.PHONY: clean
clean: ## Clean build artifacts
	$(call require-command-shell,$(GO),go)
	$(RM) -rf dist/
	$(RM) coverage.*
	$(RM) -rf "$$($(GO) env GOCACHE)/../golangci-lint"
	$(GO) clean -i -x

# Separate clean target for go modules, cache, etc.
#
# The user may not want their Go module cache cleaned by default, so a separate
# target is provided to do so.
.PHONY: clean-go
clean-go: ## Clean Go modules and caches
	$(call require-command-shell,$(GO),go)
	$(GO) clean -i -cache -testcache -modcache -fuzzcache -x

.PHONY: clean-man
clean-man: ## Clean generated manual pages
	$(RM) $(MANBIN)

.PHONY: container
container: ## Perform a multi-stage container build (accepts CONTAINER_PROG, CONTAINER_OPTS, CONTAINER_TAG, FQCN)
	$(call require-command-shell,$(CONTAINER_PROG),container program "$(CONTAINER_PROG)")
	$(CONTAINER_PROG) build -t $(FQCN) . $(CONTAINER_OPTS)

.PHONY: diff
diff: ## Check for uncommitted changes
	$(call require-command-shell,$(GIT),git)
	$(GIT) diff --exit-code
	RES=$$($(GIT) status --porcelain) ; if [ -n "$$RES" ]; then echo $$RES && exit 1 ; fi

.PHONY: distclean
distclean: clean clean-man ## Clean everything (prepare for distribution)

.PHONY: docs
docs: ## Show documentation generated from Go comments
	$(call require-command-shell,$(GO),go)
	$(GO) doc github.com/OpenCHAMI/magellan/cmd
	$(GO) doc github.com/OpenCHAMI/magellan/internal
	$(GO) doc github.com/OpenCHAMI/magellan/pkg/crawler

.PHONY: emulator
emulator: ## Run the Redfish emulator
	./emulator/setup.sh

.PHONY: goreleaser-build
goreleaser-build: ## Run `goreleaser build` (accepts GORELEASER_OPTS)
	$(call require-command-shell,$(GO),go)
	$(call require-command-shell,$(GORELEASER),goreleaser)
	env \
		GIT_STATE=$(GIT_STATE) \
		BUILD_HOST=$(BUILD_HOST) \
		GO_VERSION=$(GO_VERSION) \
		BUILD_USER=$(BUILD_USER) \
		$(GORELEASER) build $(GORELEASER_OPTS)

.PHONY: goreleaser-release
goreleaser-release: ## Run `goreleaser release` (accepts GORELEASER_OPTS)
	$(call require-command-shell,$(GO),go)
	$(call require-command-shell,$(GORELEASER),goreleaser)
	env \
		GIT_STATE=$(GIT_STATE) \
		BUILD_HOST=$(BUILD_HOST) \
		GO_VERSION=$(GO_VERSION) \
		BUILD_USER=$(BUILD_USER) \
		$(GORELEASER) release $(GORELEASER_OPTS)

.PHONY: goreleaser-clean
goreleaser-clean: ## Clean GoReleaser files (remove dist/)
	$(RM) -rf dist/

# Backwards-compatible alias for the previous snapshot build target.
.PHONY: goreleaser
goreleaser: GORELEASER_OPTS ?= --clean --single-target --snapshot
goreleaser: goreleaser-build ## Build a local GoReleaser snapshot (compatibility alias)

.PHONY: install
install: install-prog install-man ## Install everything

.PHONY: install-prog
install-prog: $(NAME) ## Install program
	$(call require-command-shell,$(INSTALL),install)
	$(INSTALL_PROGRAM) $(NAME) $(DESTDIR)$(bindir)/$(NAME)

.PHONY: install-man
install-man: $(MANBIN) ## Install manual pages
	$(call require-command-shell,$(INSTALL),install)
	mkdir -p $(DESTDIR)$(mandir)/man1
	$(INSTALL_DATA) $(MAN1BIN) $(DESTDIR)$(mandir)/man1/

.PHONY: lint
lint: ## Run golangci-lint and fix detected issues
	$(call require-command-shell,$(GOLANGCI_LINT),golangci-lint)
	$(GOLANGCI_LINT) run --fix

.PHONY: man
man: $(MANBIN) ## Generate manual pages

man/%: man/%.sc
	$(call require-command-shell,$(SCDOC),scdoc)
	$(SCDOC) < $< > $@

.PHONY: mod
mod: ## Download and prune Go modules
	$(call require-command-shell,$(GO),go)
	$(GO) mod tidy

.PHONY: prepare
prepare: ## Install development tools
	$(call require-command-shell,$(GO),go)
	$(GO) install github.com/client9/misspell/cmd/misspell@v0.3.4
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.1
	$(GO) install github.com/goreleaser/goreleaser/v2@v2.3.2

.PHONY: reuse
reuse: ## Check REUSE compliance
	$(call require-command-shell,$(REUSE),reuse)
	$(REUSE) lint --lines

.PHONY: spell
spell: ## Check Markdown spelling and fix detected issues
	$(call require-command-shell,$(MISSPELL),misspell)
	$(MISSPELL) -error -locale=US -w **.md


.PHONY: unit-test
unit-test: ## Run unit tests without external services
	$(call require-command-shell,$(GO),go)
	$(GO) test -race -covermode=atomic -coverprofile=unit-coverage.out -coverpkg=./... ./...
	$(GO) tool cover -html=unit-coverage.out -o unit-coverage.html

.PHONY: integration-test
integration-test: $(NAME) ## Run integration tests with the Redfish emulator
	$(call require-command-shell,$(GO),go)
	$(call require-command-shell,$(CONTAINER_PROG),container program "$(CONTAINER_PROG)")
	@set -eu; \
	cleanup() { \
		$(CONTAINER_PROG) compose -f emulator/rf-emulator.yml down --volumes --remove-orphans; \
	}; \
	trap cleanup EXIT INT TERM; \
	CONTAINER_PROG="$(CONTAINER_PROG)" ./emulator/setup.sh --detach --wait; \
	$(GO) test -tags=integration -race -count=1 -covermode=atomic -coverprofile=integration-coverage.out -coverpkg=./... ./tests; \
	$(GO) tool cover -html=integration-coverage.out -o integration-coverage.html

.PHONY: test
test: unit-test integration-test ## Run unit and integration tests

.PHONY: uninstall
uninstall: uninstall-prog uninstall-man ## Uninstall everything

.PHONY: uninstall-prog
uninstall-prog: ## Uninstall program
	$(RM) $(DESTDIR)$(bindir)/$(NAME)

.PHONY: uninstall-man
uninstall-man: ## Uninstall manual pages
	$(RM) $(foreach man1page,$(subst man/,,$(MAN1BIN)),$(DESTDIR)$(mandir)/man1/$(man1page))

$(NAME): *.go cmd/*.go $(INTERNAL) $(PKG)
	$(call require-command-shell,$(GO),go)
	$(GO) build -v -ldflags="$(LDFLAGS)"
