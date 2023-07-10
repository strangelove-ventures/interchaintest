#!/usr/bin/make -f

CWD := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

ldflags = -X main.MakeFileInstallDirectory=$(CWD)
ldflags := $(strip $(ldflags))
BUILD_FLAGS := -ldflags '$(ldflags)'

.PHONY: build
build:
	go build $(BUILD_FLAGS) -o ../bin/local-ic ./cmd/local-ic 

.PHONY: run
run:
	../bin/local-ic $(filter-out $@,$(MAKECMDGOALS))

.PHONY: install
install:
	go install $(BUILD_FLAGS) ./cmd/local-ic ./interchain

