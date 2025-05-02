#
# This value is updated each time a new feature is added
# to the golang.mk targets and build rules file.
#
_GOLANG_MK_CURRENT_VERSION := 202504112045
ifeq ($(_GOLANG_MK_MINIMUM_VERSION),)
	_GOLANG_MK_MINIMUM_VERSION := 0
endif

#
# Test if minimum golang.mk version requirement is met
#
ifneq ($(shell test $(_GOLANG_MK_CURRENT_VERSION) -ge $(_GOLANG_MK_MINIMUM_VERSION); echo $$?),0)
	@echo "minimum golang.mk version requirement not met (expected at least $(_GOLANG_MK_MINIMUM_VERSION), got $(_GOLANG_MK_CURRENT_VERSION))" && exit 1
endif

#
# Extract application variable values from Makefile global context 
# into golang.mk specific variables if available.
#
ifdef _APPLICATION_NAME
	_GOLANG_MK_VARS_NAME ?= $(_APPLICATION_NAME)
endif
ifdef _APPLICATION_DESCRIPTION
	_GOLANG_MK_VARS_DESCRIPTION ?= $(_APPLICATION_DESCRIPTION)
endif
ifdef _APPLICATION_COPYRIGHT
	_GOLANG_MK_VARS_COPYRIGHT ?= $(_APPLICATION_COPYRIGHT)
endif
ifdef _APPLICATION_LICENSE
	_GOLANG_MK_VARS_LICENSE ?= $(_APPLICATION_LICENSE)
endif
ifdef _APPLICATION_LICENSE_URL
	_GOLANG_MK_VARS_LICENSE_URL ?= $(_APPLICATION_LICENSE_URL)
endif
ifdef _APPLICATION_MAINTAINER
	_GOLANG_MK_VARS_MAINTAINER ?= $(_APPLICATION_MAINTAINER)
endif
ifdef _APPLICATION_VERSION_MAJOR
	_GOLANG_MK_VARS_VERSION_MAJOR ?= $(_APPLICATION_VERSION_MAJOR)
endif
ifdef _APPLICATION_VERSION_MINOR
	_GOLANG_MK_VARS_VERSION_MINOR ?= $(_APPLICATION_VERSION_MINOR)
endif
ifdef _APPLICATION_VERSION_PATCH
	_GOLANG_MK_VARS_VERSION_PATCH ?= $(_APPLICATION_VERSION_PATCH)
endif
ifdef _APPLICATION_VERSION
	_GOLANG_MK_VARS_VERSION ?= $(_APPLICATION_VERSION)
endif
ifdef _APPLICATION_VENDOR
	_GOLANG_MK_VARS_VENDOR ?= $(_APPLICATION_VENDOR)
endif
ifdef _APPLICATION_PRODUCER_URL
	_GOLANG_MK_VARS_PRODUCER_URL ?= $(_APPLICATION_PRODUCER_URL)
endif
ifdef _APPLICATION_DOWNLOAD_URL
	_GOLANG_MK_VARS_DOWNLOAD_URL ?= $(_APPLICATION_DOWNLOAD_URL)
endif
ifdef _APPLICATION_METADATA_PACKAGE
	_GOLANG_MK_VARS_METADATA_PACKAGE ?= $(_APPLICATION_METADATA_PACKAGE)
endif
ifdef _APPLICATION_DOTENV_VAR_NAME
	_GOLANG_MK_VARS_DOTENV_VAR_NAME ?= $(_APPLICATION_DOTENV_VAR_NAME)
endif

#
# default application metadata
#
_GOLANG_MK_VARS_NAME ?= my-app
_GOLANG_MK_VARS_DESCRIPTION ?= <Provide your description here>
_GOLANG_MK_VARS_COPYRIGHT ?= <20XX> © <your name>
_GOLANG_MK_VARS_LICENSE ?= MIT
_GOLANG_MK_VARS_LICENSE_URL ?= https://opensource.org/license/mit/
_GOLANG_MK_VARS_VERSION_MAJOR ?= 0
_GOLANG_MK_VARS_VERSION_MINOR ?= 0
_GOLANG_MK_VARS_VERSION_PATCH ?= 1
_GOLANG_MK_VARS_VERSION ?= $(_GOLANG_MK_VARS_VERSION_MAJOR).$(_GOLANG_MK_VARS_VERSION_MINOR).$(_GOLANG_MK_VARS_VERSION_PATCH)
_GOLANG_MK_VARS_MAINTAINER ?= <your-email>@gmail.com
_GOLANG_MK_VARS_VENDOR ?= <your-email>@gmail.com
_GOLANG_MK_VARS_PRODUCER_URL ?= https://github.com/<your-github-username>/
_GOLANG_MK_VARS_DOWNLOAD_URL ?= $(_GOLANG_MK_VARS_PRODUCER_URL)$(_GOLANG_MK_VARS_NAME)
_GOLANG_MK_VARS_METADATA_PACKAGE ?= $$(grep "module .*" go.mod | sed 's/module //gi')/metadata
_GOLANG_MK_VARS_DOTENV_VAR_NAME ?= $$(echo $(_GOLANG_MK_VARS_NAME) | tr '[:lower:]' '[:upper:]' | tr '-' '_')_DOTENV

#
# default feature flag values
#
_GOLANG_MK_FLAG_TIDY_DEPS ?= 1
_GOLANG_MK_FLAG_ENABLE_CGO ?= 1
_GOLANG_MK_FLAG_ENABLE_GOGEN ?= 1
_GOLANG_MK_FLAG_ENABLE_RACE ?= 1
_GOLANG_MK_FLAG_STATIC_LINK ?= 0
_GOLANG_MK_FLAG_ENABLE_NETGO ?= 0
_GOLANG_MK_FLAG_STRIP_SYMBOLS ?= 0
_GOLANG_MK_FLAG_STRIP_DBG_INFO ?= 0
_GOLANG_MK_FLAG_FORCE_DEP_REBUILD ?= 0
_GOLANG_MK_FLAG_OMIT_VCS_INFO ?= 0

#
# Set this flag to 1 to enable automatic dependency tidying.
#
ifneq ($(_GOLANG_MK_FLAG_TIDY_DEPS),1)
	_GOLANG_MK_FLAG_TIDY_DEPS := 0
else # neet to enable CGO
	_GOLANG_MK_FLAG_TIDY_DEPS := 1
endif

#
# In order to enable race detector, the _GOLANG_MK_FLAG_ENABLE_RACE
# must be set to 1; any other value disables race detector;
# note that the race detector requires CGO to be enabled.
#
ifneq ($(_GOLANG_MK_FLAG_ENABLE_RACE),1)
	_GOLANG_MK_FLAG_ENABLE_RACE := 0
else # neet to enable CGO
	_GOLANG_MK_FLAG_ENABLE_CGO := 1
endif

#
# In order to enable CGO, the _GOLANG_MK_FLAG_ENABLE_CGO must be
# set to 1; any other value disables CGO.
#
ifneq ($(_GOLANG_MK_FLAG_ENABLE_CGO),1)
	_GOLANG_MK_FLAG_ENABLE_CGO := 0
endif

#
# In order to enable go generate, the _GOLANG_MK_FLAG_ENABLE_GOGEN
# must be set to 1; any other value disables go generate.
#
ifneq ($(_GOLANG_MK_FLAG_ENABLE_GOGEN),1)
	_GOLANG_MK_FLAG_ENABLE_GOGEN := 0
endif

#
# In order to statically link the generated binary against libc
# and other libraries (both with and without CGO, see this
# thread https://github.com/golang/go/issues/26492), set this
# value to 1; any other value will produce dynamically linked
# binaries.
#
ifneq ($(_GOLANG_MK_FLAG_STATIC_LINK),1)
	_GOLANG_MK_FLAG_STATIC_LINK := 0
endif

#
# In order to use the pure Go network stack implementation (which
# does not require linking against libc), set this to 1; any other
# value uses the native platform's network stack implementation (and
# requires linking against system C libraries).
#
ifneq ($(_GOLANG_MK_FLAG_ENABLE_NETGO),1)
	_GOLANG_MK_FLAG_ENABLE_NETGO := 0
endif

#
# Set this flag to 1 if you want to reduce the executable size by
# stripping all the symbols. You will not be able to run go tool nm
# against the binary.
#
ifneq ($(_GOLANG_MK_FLAG_STRIP_SYMBOLS),1)
	_GOLANG_MK_FLAG_STRIP_SYMBOLS := 0
endif

#
# Set this flag to 1 if you want to reduce the executable size by
# stripping all the GDB debug information; you will not be able to
# debug the resulting application.
#
ifneq ($(_GOLANG_MK_FLAG_STRIP_DBG_INFO),1)
	_GOLANG_MK_FLAG_STRIP_DBG_INFO := 0
endif

#
# Set this flag to 1 if you want to force the rebuild of all dependencies
# even if they are up-to-date. This can be useful when changing the value
# of CGO, in order to make sure that all object files (.a) are compiled
# with the desired settings.
#
ifneq ($(_GOLANG_MK_FLAG_FORCE_DEP_REBUILD),1)
	_GOLANG_MK_FLAG_FORCE_DEP_REBUILD := 0
endif

#
# Set this flag to 1 to omit VCS information from the binary.
#
ifneq ($(_GOLANG_MK_FLAG_OMIT_VCS_INFO), 1)
	_GOLANG_MK_FLAG_OMIT_VCS_INFO := 0
endif

#
# TARGETS
#

SHELL := /bin/bash

platforms="$$(go tool dist list)"
module := $$(grep "module .*" go.mod | sed 's/module //gi')
ifeq ($(_GOLANG_MK_VARS_METADATA_PACKAGE),)
	package := $(module)/commands/version
else
	package := $(_GOLANG_MK_VARS_METADATA_PACKAGE)
endif

now := $$(date --rfc-3339=seconds)

-include .piped

ifeq ($(piped),1)
black:=
red:=
green:=
yellow:=
blue:=
magenta:=
cyan:=
white:=
bold:=
reset:=
else
black:=\033[30m
red:=\033[31m
green:=\033[32m
yellow:=\033[33m
blue:=\033[34m
magenta:=\033[35m
cyan:=\033[36m
white:=\033[37m
bold:=\033[1m
reset:=\033[0m
endif

#
# Linux x86-64 build settings
#
linux/amd64: GOAMD64 ?= v3

#
# Windows x86-64 build settings
#
windows/amd64: GOAMD64 ?= v3

#
# This targets builds the application for a specific platform.
#
%: ## replace % with one or more <goos>/<goarch> combinations, e.g. linux/amd64, to build it
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
	@echo -e "Build Flags:"
ifeq ($(_GOLANG_MK_FLAG_TIDY_DEPS),1)
	@echo -e " - tidy dependencies               : $(green)enabled$(reset)"
	@go mod tidy
else
	@echo -e " - tidy dependencies               : $(yellow)disabled$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_OMIT_VCS_INFO),1)
	@echo -e " - stamp binary with VCS info      : $(yellow)no$(reset)"
	$(eval cvsflags=-buildvcs=false)
else
	@echo -e " - stamp binary with VCS info      : $(green)yes$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_ENABLE_GOGEN),1)
	@echo -e " - go generate                     : $(green)enabled$(reset)"
	@go generate ./...
else
	@echo -e " - go generate                     : $(yellow)disabled$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_ENABLE_CGO),1)
	@echo -e " - CGO dependencies                : $(green)enabled$(reset)"
else
	@echo -e " - CGO dependencies                : $(yellow)disabled$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_ENABLE_NETGO),1)
	@echo -e " - network stack                   : $(green)pure go$(reset)"
else
	@echo -e " - network stack                   : $(yellow)native$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_STRIP_SYMBOLS),1)
	@echo -e " - strip symbols                   : $(yellow)yes$(reset)"
	$(eval strip_symbols=-s)
else
	@echo -e " - strip symbols                   : $(green)no$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_STRIP_DBG_INFO),1)
	@echo -e " - strip debug info                : $(yellow)yes$(reset)"
	$(eval strip_dbg_info=-w)
else
	@echo -e " - strip debug info                : $(green)no$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_ENABLE_CGO),1)
	$(eval linkmode=-linkmode 'external')
endif
ifeq ($(_GOLANG_MK_FLAG_STATIC_LINK),1)
	@echo -e " - linking                         : $(green)static$(reset)"
	$(eval static=-extldflags '-static')
ifeq ($(_GOLANG_MK_FLAG_ENABLE_CGO),1)
	$(eval linkmode=-linkmode 'external')
endif
else
	@echo -e " - linking                         : $(yellow)dynamic$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_FORCE_DEP_REBUILD),1)
	@echo -e " - build cache                     : $(yellow)disabled$(reset)"
	$(eval recompile=-a)
else
	@echo -e " - build cache                     : $(green)enabled$(reset)"
endif
ifeq ($(_GOLANG_MK_FLAG_ENABLE_RACE),1)
	@echo -e " - race detector                   : $(green)enabled$(reset)"
	$(eval race=-race)
else
	@echo -e " - race detector                   : $(yellow)disabled$(reset)"
endif
	@echo -e " - metadata package                : $(green)$(package)$(reset)"
	@$(MAKE) golang-show-vars
	@for platform in "$(platforms)"; do \
		if test "$(@)" = "$$platform"; then \
			echo -e "PLATFORM: $(green)$(@)$(reset)"; \
			echo -e "PACKAGES:"; \
			mkdir -p dist/$(@); \
			GOOS=$(shell echo $(@) | cut -d "/" -f 1) \
			GOARCH=$(shell echo $(@) | cut -d "/" -f 2) \
			GOAMD64=$(GOAMD64) \
			CGO_ENABLED=$(_RULES_MK_FLAG_ENABLE_CGO); \
			go build -v \
			$(cvsflags) \
			$(race) \
			$(recompile) \
			-ldflags="\
			$(strip_dbg_info) \
			$(strip_symbols) \
			$(linkmode) \
			$(static) \
			-X '$(package).Name=$(_GOLANG_MK_VARS_NAME)' \
			-X '$(package).Description=$(_GOLANG_MK_VARS_DESCRIPTION)' \
			-X '$(package).Copyright=$(_GOLANG_MK_VARS_COPYRIGHT)' \
			-X '$(package).License=$(_GOLANG_MK_VARS_LICENSE)' \
			-X '$(package).LicenseURL=$(_GOLANG_MK_VARS_LICENSE_URL)' \
			-X '$(package).BuildTime=$(now)' \
			-X '$(package).VersionMajor=$(_GOLANG_MK_VARS_VERSION_MAJOR)' \
			-X '$(package).VersionMinor=$(_GOLANG_MK_VARS_VERSION_MINOR)' \
			-X '$(package).VersionPatch=$(_GOLANG_MK_VARS_VERSION_PATCH)' \
			-X '$(package).Vendor=$(_GOLANG_MK_VARS_VENDOR)' \
			-X '$(package).Maintainer=$(_GOLANG_MK_VARS_MAINTAINER)' \
			-X '$(package).RulesMkVersion=$(_GOLANG_MK_CURRENT_VERSION)' \
			-X '$(package).DotEnvVarName=$(_GOLANG_MK_VARS_DOTENV_VAR_NAME)'" \
			-o dist/$(@)/ . && echo -e "RESULT: $(green)OK$(reset)" || echo -e "RESULT: $(red)KO$(reset)";\
		fi; \
	done
	@rm -f .piped

#
# golang-release performs a build for the default target, 
# a code quality check and packages the application in the
# RPM, DEB and APK formats for the default platform (linux/amd64)
#
.PHONY: golang-release 
golang-release: golang-quality golang-compile nfpm-deb nfpm-rpm nfpm-apk ## build, check and release in DEB, RPM and APK formats

#
# golang-clean removes all build artifacts.
#
.PHONY: golang-clean 
golang-clean: ## remove all build artifacts
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
	@echo -e "$(green)Cleaning up$(reset) directory..."
	@rm -rf dist
	@rm -rf fetch/server.key fetch/server.crt
	@rm -f .piped

#
# golang-clean-cache removes all cached build entries
# from the compiler's local cache.
#
.PHONY: golang-clean-cache 
golang-clean-cache: ## remove all cached build entries
	@go clean -x -cache

#
# golang-test runs the tests.
#
.PHONY: golang-test 
golang-test: ## run tests
	go test ./...

#
# golang-quality performs static analysis on the code.
#
.PHONY: golang-quality
golang-quality: ## perform static analysis on the code
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
# golang-show-vars shows the actual build variables values.
#
.PHONY: golang-show-vars
golang-show-vars: ## show actual build variables values
	@echo -e "Build Variables:"
	@echo -e " - _GOLANG_MK_VARS_NAME             : $(green)$(_GOLANG_MK_VARS_NAME)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_DESCRIPTION      : $(green)$(_GOLANG_MK_VARS_DESCRIPTION)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_COPYRIGHT        : $(green)$(_GOLANG_MK_VARS_COPYRIGHT)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_LICENSE          : $(green)$(_GOLANG_MK_VARS_LICENSE)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_LICENSE_URL      : $(green)$(_GOLANG_MK_VARS_LICENSE_URL)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_VERSION_MAJOR    : $(green)$(_GOLANG_MK_VARS_VERSION_MAJOR)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_VERSION_MINOR    : $(green)$(_GOLANG_MK_VARS_VERSION_MINOR)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_VERSION_PATCH    : $(green)$(_GOLANG_MK_VARS_VERSION_PATCH)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_VERSION          : $(green)$(_GOLANG_MK_VARS_VERSION)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_MAINTAINER       : $(green)$(_GOLANG_MK_VARS_MAINTAINER)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_VENDOR           : $(green)$(_GOLANG_MK_VARS_VENDOR)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_PRODUCER_URL     : $(green)$(_GOLANG_MK_VARS_PRODUCER_URL)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_DOWNLOAD_URL     : $(green)$(_GOLANG_MK_VARS_DOWNLOAD_URL)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_METADATA_PACKAGE : $(green)$(_GOLANG_MK_VARS_METADATA_PACKAGE)$(reset)"
	@echo -e " - _GOLANG_MK_VARS_DOTENV_VAR_NAME  : $(green)$(_GOLANG_MK_VARS_DOTENV_VAR_NAME)$(reset)"

#
# golang-compress compresses all the executables with UPX (good quality).
#
.PHONY: golang-compress
golang-compress: ## compress all the executables with UPX (good quality)
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
ifeq (, $(shell which upx))
	@echo -e "Need to $(green)install UPX$(reset) first..."
	@sudo apt install upx
endif
	@for binary in `find dist/ -type f -regex '.*$(_GOLANG_MK_VARS_NAME)[\.exe]*'`; do \
		upx -9 $$binary; \
	done;
	@rm -f .piped

#
# golang-extra-compress compresses all the executables with UPX (best quality, slooow!)
#
.PHONY: golang-extra-compress
golang-extra-compress: ## compress all the executables with UPX (best quality, slooow!)
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
ifeq (, $(shell which upx))
	@echo-e  "Need to $(green)install UPX$(reset) first..."
	@sudo apt install upx
endif
	@for binary in `find dist/ -type f -regex '.*$(_GOLANG_MK_VARS_NAME)[\.exe]*'`; do \
		upx --brute $$binary; \
	done;
	@rm -f .piped

#
# golang-supported shows all platforms supported as targets by the golang compiler.
#
.PHONY: golang-supported
golang-supported: ## show supported build platforms
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
	@echo -e "Supported build platforms:"
	@OS=$$(uname -s); \
	OS=$${OS,,}; \
	ARCH=$$(uname -p); \
	if [ "$$ARCH" = "x86_64" ]; then \
		ARCH=amd64; \
	fi; \
	for platform in "$(platforms)"; do \
		if [ "$$OS/$$ARCH" = "$$platform" ]; then \
	 		echo -e " - $(green)$$platform$(reset) (current)"; \
		else \
	 		echo -e " - $$platform"; \
		fi; \
	done
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
