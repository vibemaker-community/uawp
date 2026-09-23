.PHONY: fmt test race vet check build validate-release-version

GO ?= go
GOFMT ?= gofmt
VERSION ?= dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
SOURCE_DATE_EPOCH ?= $(shell git show -s --format=%ct HEAD 2>/dev/null)
RELEASE_TAG ?= $(shell git tag --points-at HEAD 2>/dev/null | head -n 1)
BUILD_OUTPUT ?= bin/uawp
NORMALIZED_VERSION := $(patsubst v%,%,$(VERSION))
BUILT_AT := $(shell if [ -n "$(SOURCE_DATE_EPOCH)" ]; then date -u -r "$(SOURCE_DATE_EPOCH)" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d "@$(SOURCE_DATE_EPOCH)" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null; fi)
BUILDINFO_PACKAGE := github.com/vibemaker-community/uawp/internal/buildinfo
LDFLAGS := -buildid= -X $(BUILDINFO_PACKAGE).Version=$(NORMALIZED_VERSION) -X $(BUILDINFO_PACKAGE).Commit=$(COMMIT) -X $(BUILDINFO_PACKAGE).BuiltAt=$(BUILT_AT)

fmt:
	test -z "$$($(GOFMT) -l .)"

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: fmt test race vet

validate-release-version:
	@if [ -n "$(RELEASE_TAG)" ] && [ "$(RELEASE_TAG)" != "v$(NORMALIZED_VERSION)" ]; then \
		echo "release tag $(RELEASE_TAG) does not match version v$(NORMALIZED_VERSION)" >&2; \
		exit 1; \
	fi

build: validate-release-version
	@mkdir -p "$(dir $(BUILD_OUTPUT))"
	CGO_ENABLED=0 $(GO) build -trimpath -buildvcs=false -ldflags '$(LDFLAGS)' -o "$(BUILD_OUTPUT)" ./cmd/uawp
