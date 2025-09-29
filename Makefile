#
# COMMAND CONFIGURATION
#

# Paths to commands
GO             ?= $(shell command -v go 2>/dev/null)
GIT            ?= $(shell command -v git 2>/dev/null)
# Use HOSTCMD to not conflict with Make's $(HOSTNAME)
HOSTCMD        ?= $(shell command -v hostname 2>/dev/null)
INSTALL        ?= $(shell command -v install 2>/dev/null)
SCDOC          ?= $(shell command -v scdoc 2>/dev/null)
CONTAINER      ?= $(shell command -v docker 2>/dev/null)
CONTAINER_ARGS ?= ''
SHELL          ?= /bin/sh

# `install` command invocations
INSTALL_PROGRAM ?= $(INSTALL) -Dm755
INSTALL_DATA    ?= $(INSTALL) -Dm644

# Check that commands are present
ifeq ($(GIT),)
$(error git command not found.)
endif
ifeq ($(HOSTCMD),)
$(error hostname command not found.)
endif
ifeq ($(SHELL),)
$(error '$(SHELL)' undefined.)
endif

#
# FUNCTIONS
#

# Recursive wildcard function, obtained from https://stackoverflow.com/a/18258352
#
# Arg 1: Space-separated list of directories to recurse into
# Arg 2: Space-separated list of patterns to match
rwildcard = $(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))

# Print currently running target
define print-target
    @printf "Executing target: \033[36m$@\033[0m\n"
endef

#
# BUILD CONFIGURATION
#

NAME    ?= magellan
VERSION ?= $(shell git describe --tags --always --dirty --broken --abbrev=0)
BUILD   ?= $(shell git rev-parse --short HEAD)
GOPATH  ?= $(shell echo $${GOPATH:-~/go})
IMPORT  := github.com/OpenCHAMI/magellan/
LDFLAGS := -s \
	   -X='$(IMPORT)main.commit=$(BUILD)' \
	   -X='$(IMPORT)main.version=$(VERSION)' \
	   -X='$(IMPORT)main.date=$(shell date -Iseconds)'
INTERNAL := $(call rwildcard,internal,*.go)
PKG      := $(call rwildcard,pkg,*.go)
MANSRC   := $(wildcard man/*.sc)
MANBIN   := $(subst .sc,,$(MANSRC))
MAN1BIN  := $(filter %.1,$(MANBIN))

# Installation paths
prefix      ?= /usr/local
exec_prefix ?= $(prefix)
bindir      ?= $(exec_prefix)/bin
mandir      ?= $(exec_prefix)/man

#
# TARGETS
#

# Default target
.PHONY: all
all: binaries

# Build all program binaries
.PHONY: binaries
binaries: $(NAME)

# CI build pipeline
.PHONY: ci
ci: all diff

# Remove files created during build pipeline
.PHONY: clean
clean:
	$(call print-target)
ifeq ($(GO),)
	$(error go command not found.)
endif
	rm -rf dist
	rm -f coverage.*
	rm -f '"$(shell go env GOCACHE)/../golangci-lint"'
	$(GO) clean -i -x

# Separate clean target for go modules, cache, etc.
#
# The user may not want their Go module cache cleaned by default, so a separate
# target is provided to do so.
.PHONY: clean-go
clean-go:
	$(call print-target)
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) clean -i -cache -testcache -modcache -fuzzcache -x

.PHONY: clean-man
clean-man:
	$(call print-target)
	rm -f $(MANBIN)

# Build container
.PHONY: container
container:
	$(call print-target)
	$(CONTAINER) build . --build-arg REGISTRY_HOST=${REGISTRY_HOST} --no-cache --pull --tag '${NAME}:${VERSION}'

.PHONY: diff
diff:
	$(call print-target)
ifeq ($(GIT),)
	$(error git command not found.)
endif
	$(GIT) diff --exit-code
	RES=$$($(GIT) status --porcelain) ; if [ -n "$$RES" ]; then echo $$RES && exit 1 ; fi

.PHONY: distclean
distclean: clean clean-man

# Generate docs from Go comments
.PHONY: docs
docs:
	$(call print-target)
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) doc github.com/OpenCHAMI/magellan/cmd
	$(GO) doc github.com/OpenCHAMI/magellan/internal
	$(GO) doc github.com/OpenCHAMI/magellan/pkg/crawler

# Run Redfish emulator
.PHONY: emulator
emulator:
	$(call print-target)
	./emulator/setup.sh

# Build using Goreleaser
.PHONY: goreleaser
goreleaser:
	$(call print-target)
	$(GOPATH)/bin/goreleaser build --clean --single-target --snapshot

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: install
install: install-prog install-man

.PHONY: install-prog
install-prog: $(NAME)
	$(call print-target)
ifeq ($(INSTALL),)
	$(error install command not found.)
endif
	$(INSTALL_PROGRAM) $(NAME) $(DESTDIR)$(bindir)/$(NAME)

.PHONY: install-man
install-man: $(MANBIN)
	$(call print-target)
ifeq ($(INSTALL),)
	$(error install command not found.)
endif
	mkdir -p $(DESTDIR)$(mandir)/man1
	$(INSTALL_DATA) $(MAN1BIN) $(DESTDIR)$(mandir)/man1/

# Run golangci-lint to lint Go code
.PHONY: lint
lint:
	$(call print-target)
	$(GOPATH)/bin/golangci-lint run --fix

.PHONY: man
man: $(MANBIN)

man/%: man/%.sc
ifeq ($(SCDOC),)
	$(error scdoc command not found.)
endif
	$(SCDOC) < $< > $@

# Download/Prune Go modules
.PHONY: mod
mod:
	$(call print-target)
	go mod tidy

# Prepare by installing necessary Go tools
.PHONY: prepare
prepare:
	$(call print-target)
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) install github.com/client9/misspell/cmd/misspell@v0.3.4
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.1
	$(GO) install github.com/goreleaser/goreleaser/v2@v2.3.2

# Spellchecking
.PHONY: spell
spell:
	$(call print-target)
	$(GOPATH)/bin/misspell -error -locale=US -w **.md

# Run Go tests
.PHONY: test
test:
	$(call print-target)
ifeq ($(GO),)
	$(error go command not found.)
endif
	./emulator/setup.sh &
	sleep 10
	$(GO) test -race -covermode=atomic -coverprofile=coverage.out -coverpkg=./... tests/api_test.go tests/compatibility_test.go
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: uninstall
uninstall: uninstall-prog uninstall-man

.PHONY: uninstall-prog
uninstall-prog:
	$(call print-target)
	rm -f $(DESTDIR)$(bindir)/$(NAME)

.PHONY: uninstall-man
uninstall-man:
	$(call print-target)
	rm -f $(foreach man1page,$(subst man/,,$(MAN1BIN)),$(DESTDIR)$(mandir)/man1/$(man1page))

$(NAME): *.go cmd/*.go $(INTERNAL) $(PKG)
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) build -v -ldflags="$(LDFLAGS)"
