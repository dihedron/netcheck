.DEFAULT_GOAL := default

SHELL := /bin/bash

platforms="$$(go tool dist list)"
module := $$(grep "module .*" go.mod | sed 's/module //gi')
package := $(module)/version
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
linux/amd64: GOAMD64 = v3

#
# Windows x86-64 build settings
#
windows/amd64: GOAMD64 = v3

.PHONY: default
default: linux/amd64 ;

%: ## replace % with one or more <goos>/<goarch> combinations, e.g. linux/amd64, to build it
	@make .piped
	@go mod tidy
ifeq ($(DOCKER),true)
	$(eval cvsflags=-buildvcs=false)
endif
	@echo -e "Running $(green)go generate$(reset)..."
	@go generate ./...    
	@make quality
	@for platform in "$(platforms)"; do \
		if test "$(@)" = "$$platform"; then \
			echo -e "Building target $(green)$(@)$(reset)..."; \
			mkdir -p dist/$(@); \
			GOOS=$(shell echo $(@) | cut -d "/" -f 1) \
			GOARCH=$(shell echo $(@) | cut -d "/" -f 2) \
			GOAMD64=$(GOAMD64) \
			CGO_ENABLED=0 \
			go build -v \
			$(cvsflags) \
			-ldflags="\
			-w -s \
			-X '$(package).Name=$(NAME)' \
			-X '$(package).Description=$(DESCRIPTION)' \
			-X '$(package).Copyright=$(COPYRIGHT)' \
			-X '$(package).License=$(LICENSE)' \
			-X '$(package).LicenseURL=$(LICENSE_URL)' \
			-X '$(package).BuildTime=$(now)' \
			-X '$(package).VersionMajor=$(VERSION_MAJOR)' \
			-X '$(package).VersionMinor=$(VERSION_MINOR)' \
			-X '$(package).VersionPatch=$(VERSION_PATCH)'" \
			-o dist/$(@)/ .;\
			echo ...done!; \
		fi; \
	done
	@rm -f .piped

.PHONY: quality
quality: ## perform static analysis on the code
	@make .piped
	@echo -e "Performing $(green)quality checks$(reset)..."
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

.PHONY: compress
compress: ## compress all the executables with UPX (good quality)
ifeq (, $(shell which upx))
	@echo "Need to install UPX first..."
	@sudo apt install upx
endif	
	@for binary in `find dist/ -type f -regex '.*$(NAME)[\.exe]*'`; do \
		upx -9 $$binary; \
	done;	

.PHONY: extra-compress
extra-compress: ## compress all the executables with UPX (best quality, slooow!)
ifeq (, $(shell which upx))
	@echo "Need to install UPX first..."
	@sudo apt install upx
endif	
	@for binary in `find dist/ -type f -regex '.*$(NAME)[\.exe]*'`; do \
		upx --brute $$binary; \
	done;	

.PHONY: clean
clean: ## remove all build artifacts
	@rm -rf dist
	@rm -rf fetch/server.key fetch/server.crt

.PHONY: install
install: ## [deprecated] install to a PREFIX (default: /usr/local/bin)
	@make .piped
ifneq ($(shell id -u), 0)
	@echo -e "$(red)You must be root to perform this action.$(reset)"
else
ifneq (x86_64, $(shell uname -m))
	@echo -e "$(red) You must be running on x86_64 Linux to perform this action.$(reset)"
endif	
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
ifeq ($(PLATFORM),)
	$(eval PLATFORM=linux/amd64)
endif
	@echo "Installing $(PLATFORM)/$(NAME) to $(PREFIX)/$(NAME)..."
	@cp dist/$(PLATFORM)/$(NAME) $(PREFIX)
	@chmod 755 $(PREFIX)/$(NAME)
endif
	@rm -f .piped

.PHONY: uninstall
uninstall: ## [deprecated] remove from a PREFIX (default: /usr/local/bin)
ifneq ($(shell id -u), 0)
	@echo -e "You must be root to perform this action."
else
ifneq (x86_64, $(shell uname -m))
	@echo -e "You must be running on x86_64 Linux to perform this action."
endif	
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
	@echo "Uninstalling $(PREFIX)/$(NAME)..."
	@rm -rf $(PREFIX)/$(NAME)
endif

.PHONY: deb
deb: ## package in DEB format the given PLATFORM (default: linux/amd64)
ifeq (, $(shell which nfpm))
	@echo "Need to install nFPM first..."
	@go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
endif
ifeq ($(PLATFORM),)
	$(eval PLATFORM=linux/amd64)
endif
	$(eval GOOS=$(shell echo $(PLATFORM) | cut -d '/' -f 1))
	$(eval GOARCH=$(shell echo $(PLATFORM) | cut -d '/' -f 2))
	@NAME=$(NAME) VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager deb --target dist/$(PLATFORM)/

.PHONY: rpm
rpm: ## package in RPM format the given PLATFORM (default: linux/amd64)
ifeq (, $(shell which nfpm))
	@echo "Need to install nFPM first..."
	@go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
endif
ifeq ($(PLATFORM),)
	$(eval PLATFORM=linux/amd64)
endif
	$(eval GOOS=$(shell echo $(PLATFORM) | cut -d '/' -f 1))
	$(eval GOARCH=$(shell echo $(PLATFORM) | cut -d '/' -f 2))
	@NAME=$(NAME) VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager rpm --target dist/$(PLATFORM)/

.PHONY: apk
apk: ## package in APK format the given PLATFORM (default: linux/amd64)
ifeq (, $(shell which nfpm))
	@echo "Need to install nFPM first..."
	@go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
endif
ifeq ($(PLATFORM),)
	$(eval PLATFORM=linux/amd64)
endif
	$(eval GOOS=$(shell echo $(PLATFORM) | cut -d '/' -f 1))
	$(eval GOARCH=$(shell echo $(PLATFORM) | cut -d '/' -f 2))
	@NAME=$(NAME) VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager apk --target dist/$(PLATFORM)/

.PHONY: container
container: ## create a Docker container to run containerised builds
	@docker build -t golang-1.22.1-with-tools .

.PHONY: docker-prompt
docker-prompt: ## run a bash in the container to run builds
	$(eval USER=$(shell id -u))
	$(eval GROUP=$(shell id -g))
	@docker run -it \
	--rm \
	--volume /etc/passwd:/etc/passwd:ro \
	--volume /etc/group:/etc/group:ro \
	--volume "$(PWD)":/usr/src/ \
	--user $(USER):$(GROUP) \
	-w /usr/src/ \
	golang-1.22.1-with-tools \
	/bin/bash 


.PHONY: help
help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: howto
howto: ## show how to use this Makefile in your Golang project
	@echo -e "In order to use this Make rules file, simply create a Makefile"
	@echo -e "in the root of your project, with the following $(red)mandatory$(reset) contents:"
	@echo 
	@echo -e "NAME := KoolApp $(green)# replace with the name of your executable$(reset) "
	@echo -e "DESCRIPTION := KoolApp provides a cool way to do things. $(green)# replace with a description of your application$(reset) "
	@echo -e "COPYRIGHT := 2024 © Johanna Doe $(green)# replace with proper year @ your name$(reset) "
	@echo -e "LICENSE := MIT $(green)# replace with a license to your liking...$(reset) "
	@echo -e "LICENSE_URL := https://opensource.org/license/mit/ $(green)# ...and set the URL accordingly$(reset) "
	@echo -e "VERSION_MAJOR := 1 $(green)# replace with the major version$(reset)"
	@echo -e "VERSION_MINOR := 0 $(green)# replace with the minor version$(reset) "
	@echo -e "VERSION_PATCH := 2 $(green)# replace with the patch or revision$(reset)"
	@echo -e 'VERSION := $$(VERSION_MAJOR).$$(VERSION_MINOR).$$(VERSION_PATCH) $(green)# leave it like this unless you need to override$(reset) '
	@echo -e "MAINTAINER := johanna.doe@example.com $(green)# replace with the email of the maintainer$(reset) "
	@echo -e "VENDOR := koolsoft@example.com $(green)# replace with the email of the vendor$(reset) "
	@echo -e "PRODUCER_URL := https://github.com/koolsoft/ $(green)# replace with the URL of the software producer$(reset)"
	@echo -e 'DOWNLOAD_URL := $$(PRODUCER_URL)$$(NAME) $(green)# leave it like this unless you need to override$(reset)'
	@echo 
	@echo -e "include rules.mk $(green)# this is where the Make rules are imported$(reset)"
	@echo 
	@echo -e "$(green)$(bold)After$(reset) these lines you can add whatever targets you need."
	@echo -e "Please notice that $(magenta)rules.mk$(reset) will set the default target to $(magenta)linux/amd64$(reset)."




# this in an internal task used to detect whether the main Make
# instance is running in an interactive shell or redirected/piped
# to file; only the main Make process will run it and create the
# temporary .piped file, which will then be included by child Make
# instances.
.piped:
	@echo "checking if piped..."
	@[ -t 1 ] && piped=0 || piped=1 ; echo "piped=$${piped}" > .piped
