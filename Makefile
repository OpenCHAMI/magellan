# import config.
# You can change the default config with `make cnf="config_special.env" build`
cnf ?= config.env
include $(cnf)
export $(shell sed 's/=.*//' $(cnf))

ifndef NAME
$(error NAME is not set.  Please review and copy config.env.default to config.env and try again)
endif

ifndef VERSION
$(error VERSION is not set.  Please review and copy config.env.default to config.env and try again)
endif

ifndef BUILD
$(error BUILD is not set.  Please review and copy config.env.default to config.env and try again)
endif

LDFLAGS="-s -X=$(GIT)main.commit=$(BUILD) -X=$(GIT)main.version=$(VERSION) -X=$(GIT)main.date=$(shell date +%Y-%m-%d:%H:%M:%S)"

SHELL := /bin/bash
GOPATH ?= $(shell echo $${GOPATH:-~/go})

.DEFAULT_GOAL := all
.PHONY: all
all: ## build pipeline
all: mod inst build lint test

.PHONY: ci
ci: ## CI build pipeline
ci: all diff

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: clean
clean: ## remove files created during build pipeline
	$(call print-target)
	rm -rf dist
	rm -f coverage.*
	rm -f '"$(shell go env GOCACHE)/../golangci-lint"'
	go clean -i -cache -testcache -modcache -fuzzcache -x

.PHONY: mod
mod: ## go mod tidy
	$(call print-target)
	go mod tidy

.PHONY: inst
inst: ## go install tools
	$(call print-target)
	go install github.com/client9/misspell/cmd/misspell@v0.3.4
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2
	go install github.com/goreleaser/goreleaser/v2@v2.3.2
	go install github.com/cpuguy83/go-md2man/v2@latest

.PHONY: release
release: ## goreleaser build
	$(call print-target)
	$(GOPATH)/bin/goreleaser build --clean --single-target --snapshot

.PHONY: binaries
binaries: build

.PHONY: build
build: ## go build
	go build -v --tags=all -ldflags=$(LDFLAGS) -o $(NAME) main.go

.PHONY: docker
container: ## docker build
container:
	$(call print-target)
	docker build . --build-arg REGISTRY_HOST=${REGISTRY_HOST} --no-cache --pull --tag '${NAME}:${VERSION}' 

.PHONY: spell
spell: ## misspell
	$(call print-target)
	$(GOPATH)/bin/misspell -error -locale=US -w **.md

.PHONY: lint
lint: ## golangci-lint
	$(call print-target)
	$(GOPATH)/bin/golangci-lint run --fix

.PHONY: test
test: ## go test
	$(call print-target)
	./emulator/setup.sh &
	sleep 10
	go test -race -covermode=atomic -coverprofile=coverage.out -coverpkg=./... tests/api_test.go tests/compatibility_test.go
	go tool cover -html=coverage.out -o coverage.html

.PHONY: diff
diff: ## git diff
	$(call print-target)
	git diff --exit-code
	RES=$$(git status --porcelain) ; if [ -n "$$RES" ]; then echo $$RES && exit 1 ; fi

.PHONY: docs
docs: ## go docs
	$(call print-target)
	go doc github.com/OpenCHAMI/magellan/cmd
	go doc github.com/OpenCHAMI/magellan/internal
	go doc github.com/OpenCHAMI/magellan/pkg/crawler

.PHONY: emulator
emulator:
	$(call print-target)
	./emulator/setup.sh

magellan.1: README.md inst
	$(GOPATH)/bin/go-md2man -in $< -out $@

.PHONY: man
man:
	$(call print-target)
	$(MAKE) -f $(firstword $(MAKEFILE_LIST)) magellan.1

define print-target
    @printf "Executing target: \033[36m$@\033[0m\n"
endef
