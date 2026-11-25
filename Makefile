GOPATH?=$(shell go env GOPATH)
GOLANGCI_LINT_VERSION:=v2.15.0
GOLANGCI_LINT_INSTALL_SHA:=25c5369a93aacb8cccea0d46e1569ca4a194564d
GOLANGCI_LINT:=$(GOPATH)/bin/golangci-lint-$(GOLANGCI_LINT_VERSION)

build:
	CGO_ENABLED=0 go build -o terraform-provider-scylladbcloud .

fmt:
	go tool gofumpt -w -l .

generate:
	go generate ./...

lint: $(GOLANGCI_LINT)
	go tool gofumpt -l -d -e .
	$(GOLANGCI_LINT) run ./...
	go vet ./...

test: run?=.*
test: pkgs?=./...
test:
	go test -timeout=1m -race -run="$(value run)" $(pkgs)

testacc: run?=.*
testacc: pkgs?=./...
testacc:
	TF_ACC=1 TF_ACC_LOG=DEBUG TF_LOG=DEBUG go test -v -timeout=30m -parallel=3 -race -run="$(value run)" $(pkgs)

# Plans a release without building it: version, notes and checks.
# Example: make release-preview BUMP=patch SUMMARY="Maintenance release."
BUMP ?= auto
release-preview: export BUMP := $(BUMP)
release-preview: export SUMMARY := $(SUMMARY)
release-preview:
	./scripts/release-plan.sh

list-testacc:
	@find . -name '*.go' -print0 | xargs -0 grep -Eoh "func TestAcc\w*" | sed 's/func //'

# Install golangci-lint following https://golangci-lint.run/docs/welcome/install/local/.
# go tool is not recommended.
# Pin to specific commit SHA aligned with the requested version.
# The install.sh script performs checksum verification of the downloaded binary.
# The version is in the file name, so a version change installs the new binary.
# The temporary directory keeps any other golangci-lint in $(GOPATH)/bin as it is.
$(GOLANGCI_LINT):
	tmp=$$(mktemp -d) && \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_LINT_INSTALL_SHA)/install.sh | sh -s -- -b $$tmp $(GOLANGCI_LINT_VERSION) && \
	mkdir -p $(dir $@) && \
	mv $$tmp/golangci-lint $@ && \
	rmdir $$tmp

.PHONY: build fmt generate lint test testacc release-preview
