

_APPLICATION_NAME := netcheck
_APPLICATION_DESCRIPTION := Simple probe to check network connectivity.
_APPLICATION_COPYRIGHT := 2025 © Andrea Funtò
_APPLICATION_LICENSE := MIT
_APPLICATION_LICENSE_URL := https://opensource.org/license/mit/
_APPLICATION_VERSION_MAJOR := 1
_APPLICATION_VERSION_MINOR := 1
_APPLICATION_VERSION_PATCH := 5
_APPLICATION_VERSION=$(_APPLICATION_VERSION_MAJOR).$(_APPLICATION_VERSION_MINOR).$(_APPLICATION_VERSION_PATCH)
_APPLICATION_MAINTAINER=dihedron.dev@gmail.com
_APPLICATION_VENDOR=dihedron.dev@gmail.com
_APPLICATION_PRODUCER_URL=https://github.com/dihedron/
_APPLICATION_DOWNLOAD_URL=$(_APPLICATION_PRODUCER_URL)$(_APPLICATION_NAME)
_APPLICATION_METADATA_PACKAGE=$$(grep "module .*" go.mod | sed 's/module //gi')/metadata
#_APPLICATION_DOTENV_VAR_NAME=

_GOLANG_MK_FLAG_ENABLE_CGO=0
_GOLANG_MK_FLAG_ENABLE_GOGEN=0
_GOLANG_MK_FLAG_ENABLE_RACE=0
#_GOLANG_MK_FLAG_STATIC_LINK=1
#_GOLANG_MK_FLAG_ENABLE_NETGO=1
#_GOLANG_MK_FLAG_STRIP_SYMBOLS=1
#_GOLANG_MK_FLAG_STRIP_DBG_INFO=1
#_GOLANG_MK_FLAG_FORCE_DEP_REBUILD=1
#_GOLANG_MK_FLAG_OMIT_VCS_INFO=1


include golang.mk
include nfpm.mk
include help.mk
include piped.mk

# Add custom targets below...

#
# compile is the default target; it builds the 
# application for the default platform (linux/amd64)
#
.DEFAULT_GOAL := compile

.PHONY: compile 
compile: linux/amd64 ## build for the default linux/amd64 platform

.PHONY: clean 
clean: golang-clean ## remove all build artifacts

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
