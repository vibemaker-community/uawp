.PHONY: fmt test race vet check

GO ?= go
GOFMT ?= gofmt

fmt:
	test -z "$$($(GOFMT) -l .)"

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: fmt test race vet
