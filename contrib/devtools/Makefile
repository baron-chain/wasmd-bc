###############################################################################
###                             Environment Setup                           ###
###############################################################################

# OS detection and file system separator
ifeq ($(OS),Windows_NT)
    GO := $(shell where go.exe 2> NUL)
    FS := \\
    MKDIR := IF NOT EXIST $(1) ( mkdir $(1) ) else (cd .)
    CLONE := IF NOT EXIST $(1) ( cd $(2) && git clone $(3) ) else (cd .)
else
    GO := $(shell command -v go 2> /dev/null)
    FS := /
    MKDIR := mkdir -p $(1)
    CLONE := (test ! -d $(1) && cd $(2) && git clone $(3)) || true
endif

# Verify Go installation
ifeq ($(GO),)
    $(error Could not find Go installation. Please ensure Go is installed and in PATH)
endif

###############################################################################
###                               Variables                                  ###
###############################################################################

# Directory structure
PREFIX        ?= /usr/local
BIN           ?= $(PREFIX)/bin
GOPATH        ?= $(shell $(GO) env GOPATH)
GITHUBDIR     := $(GOPATH)$(FS)src$(FS)github.com
TOOLS_DESTDIR ?= $(GOPATH)/bin

# Tool versions
BUF_VERSION      ?= 0.11.0
STATIK_VERSION   := v0.1.6
RUNSIM_VERSION   := v1.0.0
GOLANGCI_VERSION := v1.47.0

# Tool binaries
STATIK          = $(TOOLS_DESTDIR)/statik
RUNSIM          = $(TOOLS_DESTDIR)/runsim
GOLANGCI_LINT   = $(TOOLS_DESTDIR)/golangci-lint

# System information
UNAME_S ?= $(shell uname -s)
UNAME_M ?= $(shell uname -m)

# Makefile information
mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir  := $(shell cd $(shell dirname $(mkfile_path)); pwd)

###############################################################################
###                              Functions                                   ###
###############################################################################

# Function to handle repository cloning
define go_get
	$(call MKDIR,$(GITHUBDIR)$(FS)$(1))
	$(call CLONE,$(GITHUBDIR)$(FS)$(1)$(FS)$(2),$(GITHUBDIR)$(FS)$(1),https://github.com/$(1)/$(2))
	cd $(GITHUBDIR)$(FS)$(1)$(FS)$(2) && git fetch origin && git checkout -q $(3)
endef

###############################################################################
###                           Tool Installation                              ###
###############################################################################

.PHONY: tools tools-clean statik runsim golangci-lint

tools: tools-stamp

tools-stamp: statik runsim golangci-lint
	touch $@

# Statik installation
statik: $(STATIK)
$(STATIK):
	@echo "Installing statik $(STATIK_VERSION)..."
	@(cd /tmp && go install github.com/rakyll/statik@$(STATIK_VERSION))

# Runsim installation
# Note: Using temporary directory to avoid go.mod pollution
runsim: $(RUNSIM)
$(RUNSIM):
	@echo "Installing runsim $(RUNSIM_VERSION)..."
	@(cd /tmp && go install github.com/cosmos/tools/cmd/runsim@$(RUNSIM_VERSION))

# Golangci-lint installation
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT):
	@echo "Installing golangci-lint $(GOLANGCI_VERSION)..."
	@(cd /tmp && go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION))

# Cleanup tools
tools-clean:
	@echo "Cleaning tools..."
	rm -f $(STATIK) $(GOLANGCI_LINT) $(RUNSIM)
	rm -f tools-stamp
