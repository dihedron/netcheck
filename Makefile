NAME := netcheck
DESCRIPTION := Simple probe to check network connectivity.
COPYRIGHT := 2024 © Andrea Funtò
LICENSE := MIT
LICENSE_URL := https://opensource.org/license/mit/
VERSION_MAJOR := 1
VERSION_MINOR := 0
VERSION_PATCH := 5
VERSION=$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)
MAINTAINER=dihedron.dev@gmail.com
VENDOR=dihedron.dev@gmail.com
PRODUCER_URL=https://github.com/dihedron/
DOWNLOAD_URL=$(PRODUCER_URL)netcheck

METADATA_PACKAGE=$$(grep "module .*" go.mod | sed 's/module //gi')/version

_RULES_MK_MINIMUM_VERSION=202412050855
#_RULES_MK_ENABLE_CGO=1
#_RULES_MK_ENABLE_GOGEN=1
#_RULES_MK_ENABLE_RACE=1
#_RULES_MK_STATIC_LINK=1
#_RULES_MK_ENABLE_NETGO=1
#_RULES_MK_FORCE_DEP_REBUILD=0

include rules.mk

# Add custom targets below...

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
