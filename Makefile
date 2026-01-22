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
	go vet ./...

test: run?=.*
test: pkgs?=./...
test:
	go test -timeout=1m -race -run="$(run)" $(pkgs)

testacc: run?=.*
testacc: pkgs?=./...
testacc:
	TF_ACC=1 TF_ACC_LOG=DEBUG TF_LOG=DEBUG go test -timeout=30m -parallel=10 -race -run="$(run)" $(pkgs)

test: run?=.*
test: pkgs?=./...
test:
	go test -timeout=5m -race -run="$(run)" $(pkgs)

testacc: run?=.*
testacc: pkgs?=./...
testacc:
	TF_ACC=1 TF_ACC_LOG=DEBUG TF_LOG=DEBUG go test -timeout=15m -parallel=10 -race -run="$(run)" $(pkgs)

# Install golangci-lint following https://golangci-lint.run/docs/welcome/install/local/.
# go tool is not recommended.
# Pin to specific commit SHA aligned with the requested version.
# The install.sh script performs checksum verification of the downloaded binary.
$(GOPATH)/bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/9f61b0f53f80672872fced07b6874397c3ed197b/install.sh | sh -s -- -b $(GOPATH)/bin v2.7.2

.PHONY: build fmt generate lint test testacc
