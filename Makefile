NAME := netcheck
DESCRIPTION := Simple probe to check network connectivity.
COPYRIGHT := 2024 © Andrea Funtò
LICENSE := MIT
LICENSE_URL := https://opensource.org/license/mit/
VERSION_MAJOR := 1
VERSION_MINOR := 0
VERSION_PATCH := 2
VERSION=$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)
MAINTAINER=dihedron.dev@gmail.com
VENDOR=dihedron.dev@gmail.com
LICENSE="MIT"
RELEASE=1
PRODUCER_URL=https://github.com/dihedron/
DOWNLOAD_URL=$(PRODUCER_URL)netcheck

SHELL := /bin/bash

platforms="$$(go tool dist list)"
module := $$(grep "module .*" go.mod | sed 's/module //gi')
package := $(module)/version
now := $$(date --rfc-3339=seconds)


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
	@go mod tidy
ifeq (, $(shell which govulncheck))
	@go install golang.org/x/vuln/cmd/govulncheck@latest
endif
ifeq ($(DOCKER),true)
	$(eval cvsflags=-buildvcs=false)
endif
	@govulncheck ./...
	@go generate ./...    
	@for platform in "$(platforms)"; do \
		if test "$(@)" = "$$platform"; then \
			echo "Building target $(@)..."; \
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
	
.PHONY: compress
compress: ## compress all the executables with UPX (good quality)
ifeq (, $(shell which upx))
	@echo "Need to install UPX first..."
	@sudo apt install upx
endif	
	@for binary in `find dist/ -type f -regex '.*netcheck[\.exe]*'`; do \
		upx -9 $$binary; \
	done;	

.PHONY: extra-compress
extra-compress: ## compress all the executables with UPX (best quality, slooow!)
ifeq (, $(shell which upx))
	@echo "Need to install UPX first..."
	@sudo apt install upx
endif	
	@for binary in `find dist/ -type f -regex '.*netcheck[\.exe]*'`; do \
		upx --brute $$binary; \
	done;	

.PHONY: clean
clean: ## remove all build artifacts
	@rm -rf dist
	@rm -rf fetch/server.key fetch/server.crt

.PHONY: install
install: ## [deprecated] install to a PREFIX (default: /usr/local/bin)
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
else
ifneq (x86_64, $(shell uname -m))
	@echo "You must be running on x86_64 Linux to perform this action."
endif	
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
ifeq ($(PLATFORM),)
	$(eval PLATFORM=linux/amd64)
endif
	@echo "Installing $(PLATFORM)/netcheck to $(PREFIX)/netcheck..."
	@cp dist/$(PLATFORM)/netcheck $(PREFIX)
	@chmod 755 $(PREFIX)/netcheck
endif

.PHONY: uninstall
uninstall: ## [deprecated] remove from a PREFIX (default: /usr/local/bin)
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
else
ifneq (x86_64, $(shell uname -m))
	@echo "You must be running on x86_64 Linux to perform this action."
endif	
ifeq ($(PREFIX),)
	$(eval PREFIX="/usr/local/bin")
endif
	@echo "Uninstalling $(PREFIX)/netcheck..."
	@rm -rf $(PREFIX)/netcheck
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
	@VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager deb --target dist/$(PLATFORM)/

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
	@VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager rpm --target dist/$(PLATFORM)/

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
	@VERSION=$(VERSION) GOOS=$(GOOS) GOARCH=$(GOARCH) PLATFORM=$(PLATFORM) nfpm package --packager apk --target dist/$(PLATFORM)/

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

.PHONY: run-redis
run-redis: fetch-redis
	@docker run --name myredis -p6379:6379 redis

.PHONY: fetch-redis
fetch-redis:
	@docker pull redis:latest

.PHONY: run-consul
run-consul: fetch-consul
	@docker run --name myconsul -p8501:8501 consul

.PHONY: fetch-consul
fetch-consul:
	@docker pull hashicorp/consul:latest

.PHONY: self-signed-cert
self-signed-cert:
	openssl req -x509 -newkey rsa:4096 -keyout fetch/server.key -out fetch/server.crt -sha256 -days 3650 -nodes -subj "/C=XX/ST=StateName/L=CityName/O=CompanyName/OU=CompanySectionName/CN=CommonNameOrHostname"

.PHONY: help
help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)