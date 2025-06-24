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

.PHONY: deb
deb: nfpm-deb ## build a DEB package

.PHONY: rpm
rpm: nfpm-rpm ## build a RPM package

.PHONY: apk
apk: nfpm-apk ## build a APK package

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
