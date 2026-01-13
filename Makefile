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
# Pin to specific commit SHA (instead of mutable HEAD) for supply-chain security.
# The install.sh script performs checksum verification of the downloaded binary.
$(GOPATH)/bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/3a6eaa424b519c8f0b6376c9a4006934c4aac72a/install.sh | sh -s -- -b $(GOPATH)/bin v2.7.2

.PHONY: build fmt generate lint
