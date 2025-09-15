#
# This value is updated each time a new feature is added
# to the help.mk targets and build rules file.
#
_HELP_MK_CURRENT_VERSION := 20250911151410
ifeq ($(_HELP_MK_MINIMUM_VERSION),)
	_HELP_MK_MINIMUM_VERSION := 0
endif

#
# Test if minimum help.mk version requirement is met
#
ifneq ($(shell test $(_HELP_MK_CURRENT_VERSION) -ge $(_HELP_MK_MINIMUM_VERSION); echo $$?),0)
	@echo "minimum help.mk version requirement not met (expected at least $(_HELP_MK_MINIMUM_VERSION), got $(_HELP_MK_CURRENT_VERSION))" && exit 1
endif

.PHONY: help
help: ## show this help message for all targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nusage:\n  make \033[36m\033[0m<target>\nwhere target is one of:\n"} /^[$$()% a-zA-Z_-]+:.*?##/ { printf "    \033[36m%-35s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@rm -f .piped

