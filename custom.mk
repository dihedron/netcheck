# Add custom targets below...

#
# compile is the default target; it builds the
# application for the default platform (linux/amd64)
#
.DEFAULT_GOAL := compile

# GOLANGCI_LINT_VERSION is the golangci-lint the lint target expects: the one
# the golangci-lint workflow runs
GOLANGCI_LINT_VERSION := 2.14

.PHONY: lint
lint: ## check that the code is clean (gofmt, go vet, golangci-lint); every build runs it first
	@echo "Checking the code..."
	@unformatted="$$(gofmt -l cmd internal pkg docs)"; \
		if [ -n "$$unformatted" ]; then echo "not formatted with gofmt (run gofmt -w):" >&2; echo "$$unformatted" >&2; exit 1; fi
	@go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is not installed: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION).0" >&2; exit 1; }
	@golangci-lint version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION)\." || echo "warning: golangci-lint is not v$(GOLANGCI_LINT_VERSION), which CI runs: its findings may differ" >&2
	@golangci-lint run ./...

.PHONY: stress
stress: ## run the stress and chaos test with the race detector (20 s; NETCHECK_STRESS_DURATION, _CHECKS and _SEED tune it)
	go test -race -tags stress -run TestStress -count=1 -timeout 0 -v ./pkg/netcheck

.PHONY: soak
soak: ## the stress test for hours (8 h by default; SOAK_SECONDS sets it): meant for an overnight run
	NETCHECK_STRESS_DURATION=$(or $(SOAK_SECONDS),28800) go test -race -tags stress -run TestStress -count=1 -timeout 0 -v ./pkg/netcheck

.PHONY: compile
compile: lint go-dev ## build for the default linux/amd64 platform

.PHONY: snapshot
snapshot: signing-check lint go-snapshot ## build a snapshot version for the supported platforms (signed if GPG_FINGERPRINT and GPG_KEY_FILE are set)

.PHONY: release
release: signing-check lint ## build a release version (requires a valid tag and the packaging key), documents included
	@test -n "$(GPG_FINGERPRINT)" || { echo "a release must be signed: set GPG_FINGERPRINT and GPG_KEY_FILE" >&2; exit 1; }
	@DOCS_VERSION="$$(git describe --tags --exact-match)" docs/build.sh
	@$(MAKE) --no-print-directory go-release

.PHONY: docs
docs: ## build the release documents (PDF) into build/docs/ (needs docker or podman)
	@docs/build.sh

# signing-check: the packaging key signs the checksums file through gpg
# (GPG_FINGERPRINT) and the packages through nfpm (GPG_KEY_FILE, its armored
# private key): both or neither, so that no build is half signed
.PHONY: signing-check
signing-check:
	@if [ -n "$(GPG_FINGERPRINT)" ] && [ ! -r "$(GPG_KEY_FILE)" ]; then echo "GPG_FINGERPRINT is set but GPG_KEY_FILE is not a readable key file" >&2; exit 1; fi
	@if [ -z "$(GPG_FINGERPRINT)" ] && [ -n "$(GPG_KEY_FILE)" ]; then echo "GPG_KEY_FILE is set but GPG_FINGERPRINT is not" >&2; exit 1; fi

.PHONY: clean
clean: ## clean the binary directory
	@rm -rf dist

# CONTAINER runs the secret scanners; podman works as well
CONTAINER ?= docker
GITLEAKS_IMAGE := zricethezav/gitleaks:latest
TRUFFLEHOG_IMAGE := ghcr.io/trufflesecurity/trufflehog:latest

.PHONY: scanners
scanners: ## pull the latest secret scanners and compare their versions with CI's pins (run before every commit)
	@$(CONTAINER) pull -q $(GITLEAKS_IMAGE) >/dev/null
	@$(CONTAINER) pull -q $(TRUFFLEHOG_IMAGE) >/dev/null
	@echo "gitleaks:   latest $$($(CONTAINER) run --rm $(GITLEAKS_IMAGE) version | tr -d v), CI $$(sed -n 's/.*GITLEAKS_VERSION: *//p' .github/workflows/ci.yml)"
	@echo "TruffleHog: latest $$($(CONTAINER) run --rm $(TRUFFLEHOG_IMAGE) --version 2>&1 | awk '{print $$2}'), CI $$(sed -n 's/.*trufflehog@.*# *v//p' .github/workflows/ci.yml)"

.PHONY: secrets
secrets: scanners ## scan the history and the working tree for secrets with gitleaks and TruffleHog, as CI does (run before every commit)
	@echo "gitleaks: history..."
	@$(CONTAINER) run --rm -v "$(CURDIR)":/repo:ro $(GITLEAKS_IMAGE) git /repo --config /repo/.gitleaks.toml --no-banner --redact
	@echo "gitleaks: working tree..."
	@$(CONTAINER) run --rm -v "$(CURDIR)":/repo:ro $(GITLEAKS_IMAGE) dir /repo --config /repo/.gitleaks.toml --no-banner --redact
	@echo "TruffleHog: history..."
	@$(CONTAINER) run --rm -v "$(CURDIR)":/repo:ro $(TRUFFLEHOG_IMAGE) git file:///repo --results=verified,unknown --fail --no-update
	@echo "TruffleHog: working tree..."
	@$(CONTAINER) run --rm -v "$(CURDIR)":/repo:ro $(TRUFFLEHOG_IMAGE) filesystem /repo --exclude-paths=/repo/.trufflehog-exclude --results=verified,unknown --fail --no-update

# govulncheck runs through "go run" so that it is always built with the
# current toolchain (a binary built with an older Go cannot read newer sources);
# "-scan module" flags every vulnerable module in go.mod, called or not, as
# CI's dependency review does (-C: module scans fail in a directory without
# Go files, such as the root)
.PHONY: vuln
vuln: ## check go.mod and the code for known vulnerabilities with govulncheck (run before every commit)
	@# both always run: a finding in one must not hide the other
	@rc=0; \
	go run golang.org/x/vuln/cmd/govulncheck@latest -C cmd/netcheck -scan module || rc=1; \
	go run golang.org/x/vuln/cmd/govulncheck@latest ./... || rc=1; \
	exit $$rc

# FUZZTIME is how long make fuzz runs each fuzz target
FUZZTIME ?= 30s

.PHONY: fuzz
fuzz: ## run every fuzz target for FUZZTIME each (default 30s); go test runs their seeds anyway
	@for pkg in $$(go list ./...); do \
		for target in $$(go test -list '^Fuzz' "$$pkg" | grep '^Fuzz'); do \
			echo "$$pkg $$target"; \
			go test "$$pkg" -run '^$$' -fuzz "^$$target$$" -fuzztime $(FUZZTIME) || exit 1; \
		done; \
	done
