#
# This value is updated each time a new feature is added
# to the go.mk targets and build rules file.
#
_GO_MK_CURRENT_VERSION := 202510051100
ifeq ($(_GO_MK_MINIMUM_VERSION),)
	_GO_MK_MINIMUM_VERSION := 0
endif

#
# Test if minimum go.mk version requirement is met
#
ifneq ($(shell test $(_GO_MK_CURRENT_VERSION) -ge $(_GO_MK_MINIMUM_VERSION); echo $$?),0)
	@echo "minimum go.mk version requirement not met (expected at least $(_GO_MK_MINIMUM_VERSION), got $(_GO_MK_CURRENT_VERSION))" && exit 1
endif

#
# Extract application variable values from Makefile global context
# into go.mk specific variables if available.
#
ifdef _APPLICATION_NAME
	_GO_MK_VARS_NAME ?= $(_APPLICATION_NAME)
endif
ifdef _APPLICATION_VERSION
	_GO_MK_VARS_VERSION ?= $(_APPLICATION_VERSION)
else
	_GO_MK_VARS_VERSION := $$(git describe --tags $$(git rev-list --tags --max-count=1) 2>/dev/null || echo "0.0.0")
endif
ifdef _APPLICATION_DESCRIPTION
	_GO_MK_VARS_DESCRIPTION ?= $(_APPLICATION_DESCRIPTION)
endif
ifdef _APPLICATION_COPYRIGHT
	_GO_MK_VARS_COPYRIGHT ?= $(_APPLICATION_COPYRIGHT)
endif
ifdef _APPLICATION_LICENSE
	_GO_MK_VARS_LICENSE ?= $(_APPLICATION_LICENSE)
endif
ifdef _APPLICATION_LICENSE_URL
	_GO_MK_VARS_LICENSE_URL ?= $(_APPLICATION_LICENSE_URL)
endif
ifdef _APPLICATION_MAINTAINER
	_GO_MK_VARS_MAINTAINER ?= $(_APPLICATION_MAINTAINER)
endif
ifdef _APPLICATION_VENDOR
	_GO_MK_VARS_VENDOR ?= $(_APPLICATION_VENDOR)
endif
ifdef _APPLICATION_PRODUCER_URL
	_GO_MK_VARS_PRODUCER_URL ?= $(_APPLICATION_PRODUCER_URL)
endif
ifdef _APPLICATION_DOWNLOAD_URL
	_GO_MK_VARS_DOWNLOAD_URL ?= $(_APPLICATION_DOWNLOAD_URL)
endif
ifdef _APPLICATION_METADATA_PACKAGE
	_GO_MK_VARS_METADATA_PACKAGE ?= $(_APPLICATION_METADATA_PACKAGE)
endif
ifdef _APPLICATION_DOTENV_VAR_NAME
	_GO_MK_VARS_DOTENV_VAR_NAME ?= $(_APPLICATION_DOTENV_VAR_NAME)
endif

#
# fill undefined variables with default values
#
_GO_MK_VARS_NAME ?= my-app
_GO_MK_VARS_DESCRIPTION ?= <Provide your description here>
_GO_MK_VARS_COPYRIGHT ?= <20XX> © <your name>
_GO_MK_VARS_LICENSE ?= MIT
_GO_MK_VARS_LICENSE_URL ?= https://opensource.org/license/mit/
_GO_MK_VARS_MAINTAINER ?= <your-email>@gmail.com
_GO_MK_VARS_VENDOR ?= <your-email>@gmail.com
_GO_MK_VARS_PRODUCER_URL ?= https://github.com/<your-github-username>/
_GO_MK_VARS_DOWNLOAD_URL ?= $(_GO_MK_VARS_PRODUCER_URL)$(_GO_MK_VARS_NAME)
_GO_MK_VARS_METADATA_PACKAGE ?= $$(grep "module .*" go.mod | sed 's/module //gi')/metadata
_GO_MK_VARS_DOTENV_VAR_NAME ?= $$(echo $(_GO_MK_VARS_NAME) | tr '[:lower:]' '[:upper:]' | tr '-' '_')_DOTENV

#
# GoReleaser version
#
_GORELEASER_VERSION := $(shell goreleaser --version | grep 'GitVersion:' | awk '{print $$2}')

#
# NOTE: use Bash as the shell, otherwise some targets will fail
#
SHELL := /bin/bash

#
# show all the externally set build variables
#
.PHONY: go-show-vars
go-show-vars: ## show build metadata variables used by goreleaser
	@echo "_GORELEASER_VERSION=${_GORELEASER_VERSION}"
	@echo "_GO_MK_VARS_NAME=${_GO_MK_VARS_NAME}"
	@echo "_GO_MK_VARS_VERSION=${_GO_MK_VARS_VERSION}"
	@echo "_GO_MK_VARS_DESCRIPTION=${_GO_MK_VARS_DESCRIPTION}"
	@echo "_GO_MK_VARS_COPYRIGHT=${_GO_MK_VARS_COPYRIGHT}"
	@echo "_GO_MK_VARS_LICENSE=${_GO_MK_VARS_LICENSE}"
	@echo "_GO_MK_VARS_LICENSE_URL=${_GO_MK_VARS_LICENSE_URL}"
	@echo "_GO_MK_VARS_MAINTAINER=${_GO_MK_VARS_MAINTAINER}"
	@echo "_GO_MK_VARS_VENDOR=${_GO_MK_VARS_VENDOR}"
	@echo "_GO_MK_VARS_PRODUCER_URL=${_GO_MK_VARS_PRODUCER_URL}"
	@echo "_GO_MK_VARS_DOWNLOAD_URL=${_GO_MK_VARS_DOWNLOAD_URL}"
	@echo "_GO_MK_VARS_METADATA_PACKAGE=${_GO_MK_VARS_METADATA_PACKAGE}"
	@echo "_GO_MK_VARS_DOTENV_VAR_NAME=${_GO_MK_VARS_DOTENV_VAR_NAME}"

#
# create a goreleaser snapshot build
#
.PHONY: go-snapshot
go-snapshot: ## perform a snapshot build using goreleaser
	@echo "Building snapshot release with goreleaser..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser release --snapshot --clean

#
# create a goreleaser development build (single platform)
#
.PHONY: go-dev
go-dev: ## perform a development build (targeting the current GOOS/GOARCH) using goreleaser
	@echo "Building single target development build with goreleaser..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser build --single-target --snapshot --clean

#
# build and release the application to github
#
.PHONY: go-release
go-release: ## perform a release build and push binaries to GitHub using goreleaser
	@echo "Building with goreleaser and pushing to GitHub..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser release --clean

#
# build for all platforms using goreleaser
#
.PHONY: go-build
go-build: ## perform a development build for all platforms using goreleaser
	@echo "Building for all platforms with goreleaser..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser build --snapshot --clean --single-target

#
# dry-run the goreleaser build; requires a clean git repo
#
.PHONY: go-dry-run
go-dry-run: ## perform a dry-run of the goreleaser release build
	@echo "Running goreleaser dry run..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser release --clean --skip=publish

#
# clean up all built binaries
#
.PHONY: go-clean
go-clean: ## clean the goreleaser dist directory
	@echo "Cleaning goreleaser dist directory..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser --clean

#
# remove all cached pre-built libraries from compiler cache
#
.PHONY: go-purge
go-purge: ## remove all cached Golang pre-built libraries
	@go clean -x -cache

.PHONY: go-quality
go-quality: ## run golang quality checks
	@echo -e "Performing quality checks"
	@echo -e " - govulncheck..."
	@govulncheck -show verbose ./...
	@echo -e " - shadow..."
	@-shadow ./...
	@echo -e " - staticcheck..."
	@staticcheck ./...
	@echo -e " - gosec..."
	@-gosec ./...
	@echo -e "Quality checks done!"

#
# check the current goreleaser configuration
#
.PHONY: go-check-goreleaser-configuration
go-check-goreleaser-configuration: ## check the goreleaser configuration
	@echo "Checking goreleaser configuration..."
	@_GO_MK_VARS_NAME="${_GO_MK_VARS_NAME}" \
	_GO_MK_VARS_VERSION="${_GO_MK_VARS_VERSION}" \
	_GO_MK_VARS_DESCRIPTION="${_GO_MK_VARS_DESCRIPTION}" \
	_GO_MK_VARS_COPYRIGHT="${_GO_MK_VARS_COPYRIGHT}" \
	_GO_MK_VARS_LICENSE="${_GO_MK_VARS_LICENSE}" \
	_GO_MK_VARS_LICENSE_URL="${_GO_MK_VARS_LICENSE_URL}" \
	_GO_MK_VARS_MAINTAINER="${_GO_MK_VARS_MAINTAINER}" \
	_GO_MK_VARS_VENDOR="${_GO_MK_VARS_VENDOR}" \
	_GO_MK_VARS_PRODUCER_URL="${_GO_MK_VARS_PRODUCER_URL}" \
	_GO_MK_VARS_DOWNLOAD_URL="${_GO_MK_VARS_DOWNLOAD_URL}" \
	_GO_MK_VARS_METADATA_PACKAGE="${_GO_MK_VARS_METADATA_PACKAGE}" \
	_GO_MK_VARS_DOTENV_VAR_NAME="${_GO_MK_VARS_DOTENV_VAR_NAME}" \
	_GORELEASER_VERSION=${_GORELEASER_VERSION} \
	goreleaser check

#
# go-check-goreleaser-installation checks if goreleaser is installed and at which version.
#
.PHONY: go-check-goreleaser-installation
go-check-goreleaser-installation: ## check if goreleaser is installed
ifeq (, $(shell which goreleaser))
	@echo -e "Install goreleaser first"
else
	@echo -e "goreleaser ver. $(_GORELEASER_VERSION) available"
endif

#
# go-check-tools checks if code and quality tools are installed
#
.PHONY: go-check-tools
go-check-tools: ## check if code and quality tools are installed
	@echo "Checking code and quality tools installation..."
	@declare -a tools; \
	tools[0]="gopls"; \
	tools[1]="gotests"; \
	tools[2]="gomodifytags"; \
	tools[3]="impl"; \
	tools[4]="goplay"; \
	tools[5]="dlv"; \
	tools[6]="staticcheck"; \
	tools[7]="govulncheck"; \
	tools[8]="gosec"; \
	tools[9]="shadow"; \
	tools[10]="golangci-lint"; \
	tools[11]="syft"; \
	for tool in "$${tools[@]}"; do \
  		if command -v $$tool &>/dev/null; then \
  			echo "$$tool is available!"; \
		else \
  			echo "$$tool is not available."; \
		fi; \
	done

.PHONY: go-setup-tools
go-setup-tools: ## install or update all necessary tools at the latest version
	@echo "Installing or updating all necessary tools at the latest version..."
	@declare -A tools; \
	tools["gopls"]="golang.org/x/tools/gopls@latest"; \
	tools["gotests"]="github.com/cweill/gotests/gotests@latest"; \
	tools["gomodifytags"]="github.com/fatih/gomodifytags@latest"; \
	tools["impl"]="github.com/josharian/impl@latest"; \
	tools["goplay"]="github.com/haya14busa/goplay/cmd/goplay@latest"; \
	tools["dlv"]="github.com/go-delve/delve/cmd/dlv@latest"; \
	tools["staticcheck"]="honnef.co/go/tools/cmd/staticcheck@latest"; \
	tools["govulncheck"]="golang.org/x/vuln/cmd/govulncheck@latest"; \
	tools["gosec"]="github.com/securego/gosec/v2/cmd/gosec@latest"; \
	tools["shadow"]="golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest"; \
	tools["golangci-lint"]="github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
	for tool in "$${!tools[@]}"; do \
		echo "Installing/updating $$tool to latest version..."; \
		go install $${tools[$$tool]}; \
	done; \
	curl -sSfL https://get.anchore.io/syft | sudo sh -s -- -b /usr/local/bin












#
# go-supported-platforms shows all platforms supported as targets by the golang compiler.
#
.PHONY: go-supported-platforms
go-supported-platforms: ## show supported build platforms
	@echo -e "Supported build platforms:"
	@OS=$$(uname -s); \
	OS=$${OS,,}; \
	ARCH=$$(uname -p); \
	if [ "$$ARCH" = "x86_64" ]; then \
		ARCH=amd64; \
	fi; \
	mapfile -t PLATFORMS < <(go tool dist list); \
	for platform in "$${PLATFORMS[@]}"; do \
		if [ "$$OS/$$ARCH" = "$$platform" ]; then \
			echo -e " [*] $$platform (current)"; \
		else \
			echo -e " [ ] $$platform"; \
		fi; \
	done

#
# go-how-to-tag shows a reminder on how to tag properly before release.
#
.PHONY: go-how-to-tag
go-how-to-tag: ## show how to set a tag before releaser
	@echo "git tag -a v1.2.3 -m \"Your message here\""
	@echo "make [go-]release"
