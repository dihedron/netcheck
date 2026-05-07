#
# The following targets are used to create a new release of the application;
# they extract the latest tag (in the format vX.Y.Z), increment the major,
# minor or patch/revision version number, and create a new tag with the new version.
#
.PHONY: release-major
release-major: ## create a new major release (e.g. v1.2.3 -> v2.0.0)
	$(eval OLD_VERSION=$(shell git describe --tags --abbrev=0 || echo "v0.0.0"))
	$(eval NEW_VERSION=$(shell echo $(OLD_VERSION) | awk -F. '{print $$1"."$$2"."$$3}' | awk -F. '{print "v"$$1+1".0.0"}' | sed 's/vv/v/g'))
	@echo "New major release: $(OLD_VERSION) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release version $(NEW_VERSION)"
	@git push origin tag $(NEW_VERSION)

.PHONY: release-minor
release-minor: ## create a new minor release (e.g. v1.2.3 -> v1.3.0)
	$(eval OLD_VERSION=$(shell git describe --tags --abbrev=0 || echo "v0.0.0"))
	$(eval NEW_VERSION=$(shell echo $(OLD_VERSION) | awk -F. '{print $$1"."$$2"."$$3}' | awk -F. '{print "v"$$1"."$$2+1".0"}' | sed 's/vv/v/g'))
	@echo "New minor release: $(OLD_VERSION) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release version $(NEW_VERSION)"
	@git push origin tag $(NEW_VERSION)

.PHONY: release-patch
release-patch: ## create a new patch/revision release (e.g. v1.2.3 -> v1.2.4)
	$(eval OLD_VERSION=$(shell git describe --tags --abbrev=0 || echo "v0.0.0"))
	$(eval NEW_VERSION=$(shell echo $(OLD_VERSION) | awk -F. '{print $$1"."$$2"."$$3}' | awk -F. '{print "v"$$1"."$$2"."$$3+1}' | sed 's/vv/v/g'))
	@echo "New revision release: $(OLD_VERSION) -> $(NEW_VERSION)"
	@git tag -a $(NEW_VERSION) -m "Release version $(NEW_VERSION)"
	@git push origin tag $(NEW_VERSION)
