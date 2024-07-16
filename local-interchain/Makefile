#!/usr/bin/make -f

CWD := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
COMMIT := $(shell git log -1 --format='%H')

# don't override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --tags --always)
  ifeq (,$(VERSION))
    HEAD := $(shell git rev-parse --abbrev-ref HEAD)
    VERSION := $(HEAD)-$(COMMIT)
  endif
endif

# if env IGNORE_STATIC_LINK=true, then only set the main version in ldflags
# required so releasing the binary does not set the main directory incorrectly
ldflags = -X main.MakeFileInstallDirectory=$(CWD) -X main.Version=$(VERSION)
ifeq ($(IGNORE_STATIC_LINK),true)
  ldflags = -X main.Version=$(VERSION)
endif

ldflags := $(strip $(ldflags))
BUILD_FLAGS := -ldflags '$(ldflags)'

.PHONY: build
build:
	go build $(BUILD_FLAGS) -o ../bin/local-ic ./cmd/local-ic

.PHONY: run
run:
	go run ./cmd/local-ic $(filter-out $@,$(MAKECMDGOALS))

.PHONY: install
install:
	go install $(BUILD_FLAGS) ./cmd/local-ic ./interchain

