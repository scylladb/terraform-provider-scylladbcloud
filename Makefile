SHELL:=/bin/bash

GOPATH?=$(shell go env GOPATH)

build:
	CGO_ENABLED=0 go build -o terraform-provider-scylladbcloud .

fmt:
	go tool gofumpt -w -l .

generate:
	go generate ./...

lint: $(GOPATH)/bin/golangci-lint
	go tool gofumpt -l -d -e .
	$(GOPATH)/bin/golangci-lint run ./...

# Install golangci-lint following https://golangci-lint.run/docs/welcome/install/local/.
# go tool is not recommended.
$(GOPATH)/bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOPATH)/bin v2.7.2

.PHONY: build fmt generate lint
