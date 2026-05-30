# Makefile for the WASM-runtime memory benchmark suite.
# Usage:
#   make setup          — fetch + build everything (runs scripts 00-30)
#   make subduction     — fetch and build native Subduction binaries only
#   make wasm           — build WASM entrypoint modules only
#   make runtimes       — build all host runtimes only
#   make test SUITE=2   — run Suite 2 stdio tests
#   make test SUITE=1   — run Suite 1 server tests
#   make clean          — remove .cache/

SHELL := /bin/bash
CACHE_DIR := .cache
SCRIPTS_DIR := scripts
GO_TEST_FLAGS := -v -timeout 30m -tags wasmmem
TEST_PKG := ./...

.PHONY: setup subduction wasm runtimes test clean

setup: subduction wasm runtimes

subduction:
	@bash $(SCRIPTS_DIR)/00-fetch-subduction.sh
	@bash $(SCRIPTS_DIR)/10-build-native.sh

wasm:
	@bash $(SCRIPTS_DIR)/20-rust-wasm.sh

runtimes:
	@bash $(SCRIPTS_DIR)/30-build-runtimes.sh

test:
ifndef SUITE
	$(error SUITE is required: make test SUITE=1 or SUITE=2)
endif
	go test $(GO_TEST_FLAGS) -run TestSuite$(SUITE) $(TEST_PKG)

clean:
	rm -rf $(CACHE_DIR)

$(CACHE_DIR)/bin:
	@mkdir -p $@
