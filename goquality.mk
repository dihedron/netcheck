#
# This value is updated each time a new feature is added
# to the goquality.mk targets and build rules file.
#
_GOQUALITY_MK_CURRENT_VERSION := 202509121745
ifeq ($(_GOQUALITY_MK_MINIMUM_VERSION),)
	_GOQUALITY_MK_MINIMUM_VERSION := 0
endif

#
# Test if minimum goquality.mk version requirement is met
#
ifneq ($(shell test $(_GOQUALITY_MK_CURRENT_VERSION) -ge $(_GOQUALITY_MK_MINIMUM_VERSION); echo $$?),0)
	@echo "minimum golang.mk version requirement not met (expected at least $(_GOQUALITY_MK_MINIMUM_VERSION), got $(_GOQUALITY_MK_CURRENT_VERSION))" && exit 1
endif

#
# TARGETS
#

SHELL := /bin/bash

#
# golang-quality performs static analysis on the code.
#
.PHONY: go-quality
go-quality: ## perform static analysis on the code
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
	@echo -e "Performing $(green)quality checks$(reset)"
ifeq (, $(shell which govulncheck))
	@go install golang.org/x/vuln/cmd/govulncheck@latest
endif
ifeq (, $(shell which gosec))
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
endif
ifeq (, $(shell which shadow))
	@go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest
endif
ifeq (, $(shell which staticcheck))
	@go install honnef.co/go/tools/cmd/staticcheck@latest
endif
	@echo -e "Running $(green)govulncheck$(reset)..."
	@govulncheck -show verbose ./...
	@echo -e "Running $(green)shadow$(reset)..."
	@-shadow ./...
	@echo -e "Running $(green)staticcheck$(reset)..."
	@staticcheck ./...
	@echo -e "Running $(green)gosec$(reset)..."
	@-gosec ./...
	@echo -e "$(green)Quality checks$(reset) done!"
	@rm -f .piped

#
# golang-setup-tools installs all necessary tools for golang development
# and quality checks.
#
.PHONY: golang-setup-tools
golang-setup-tools: ## install all necessary tools at the latest version
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
	@go install golang.org/x/tools/gopls@latest
	@go install github.com/cweill/gotests/gotests@v1.6.0
	@go install github.com/fatih/gomodifytags@v1.17.0
	@go install github.com/josharian/impl@v1.4.0
	@go install github.com/haya14busa/goplay/cmd/goplay@v1.0.0
	@go install github.com/go-delve/delve/cmd/dlv@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/mattn/goreman@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.2
	@rm -rf .piped

