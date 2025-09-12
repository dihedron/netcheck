# Add custom targets below...

#
# compile is the default target; it builds the 
# application for the default platform (linux/amd64)
#
.DEFAULT_GOAL := compile

.PHONY: compile 
compile: goreleaser-dev ## build for the default linux/amd64 platform

.PHONY: snapshot 
snapshot: goreleaser-snapshot ## build a snapshot version for the supported platforms

.PHONY: release 
release: goreleaser-release ## build a release version (requires a valid tag)

.PHONY: clean 
clean: #clean the binary directory 
	@rm -rf dist

#.PHONY: install
#install: ## install the plugin locally
#	@echo Installing Linux/AMD64 provider to ./_test/plugins/${_GORELEASER_MK_VARS_PLUGIN_ADDRESS}/${_GORELEASER_MK_VARS_VERSION}/linux_amd64...
#	@mkdir -p ./_test/plugins/${_GORELEASER_MK_VARS_PLUGIN_ADDRESS}/${_GORELEASER_MK_VARS_VERSION}/linux_amd64
#	@mv dist/terraform-provider-os_linux_amd64_v1/terraform-provider-os ./_test/plugins/${_GORELEASER_MK_VARS_PLUGIN_ADDRESS}/${_GORELEASER_MK_VARS_VERSION}/linux_amd64/

#.PHONY: uninstall
#uninstall: # uninstall the plugin locally
#	@echo Removing Linux/AMD64 provider from ./_test/plugins/${_GORELEASER_MK_VARS_PLUGIN_ADDRESS}/${_GORELEASER_MK_VARS_VERSION}/linux_amd64...
#	@rm -rf ./_test/plugins/${_GORELEASER_MK_VARS_PLUGIN_ADDRESS}/${_GORELEASER_MK_VARS_VERSION}/linux_amd64
