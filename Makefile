# Main binary configuration
CMD ?= jvmtool
MAIN_GO_FILE ?= jvmtool.go

GOOS ?= linux
GOARCH ?= amd64

.DEFAULT_GOAL := all

# go-install-tool will 'go install' any package $2 and install it locally to $1.
# This will prevent that they are installed in the $USER/go/bin folder and different
# projects ca have different versions of the tools
PROJECT_DIR := $(shell dirname $(abspath $(firstword $(MAKEFILE_LIST))))

# gomod-version returns the version number of the go.mod dependency
define gomod-version
$(shell sh -c "echo $$(grep $(1) go.mod | awk '{print $$2}')")
endef

# Check that given variables are set and all have non-empty values,
# die with an error otherwise.
#
# Params:
#   1. Variable name(s) to test.
#   2. (optional) Error message to print.
check_defined = \
	$(strip $(foreach 1,$1, \
		$(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
	$(if $(value $1),, \
	  $(error Undefined $1$(if $2, ($2))))

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: install-hooks
install-hooks:
	@if [ ! -f .git/hooks/pre-commit ]; then \
		echo "Installing pre-commit hook..."; \
		cp hooks/pre-commit .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit; \
		echo "Pre-commit hook installed."; \
	fi

.PHONY: build
build: compile

.PHONY: all
all: build

.PHONY: compile
compile:
	@echo "### Compiling JVMTool"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -a -o bin/$(CMD) $(MAIN_GO_FILE)