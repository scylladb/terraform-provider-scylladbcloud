GOPATH?=$(shell go env GOPATH)

build:
	CGO_ENABLED=0 go build -o terraform-provider-scylladbcloud .

# Dev mode (local Siren login, loopback-only endpoints) exists only in this build.
build-dev:
	CGO_ENABLED=0 go build -tags dev -o $(GOPATH)/bin/terraform-provider-scylladbcloud .

# Fails if dev mode code reaches the release binary.
check-release:
	@! go list -deps . | grep -q '/internal/devauth$$' || { echo "release build imports internal/devauth" >&2; exit 1; }
	@CGO_ENABLED=0 go build -o $(or $(TMPDIR),/tmp)/tfprovider-release-check . && \
		! grep -q -a 'dev_siren_endpoint' $(or $(TMPDIR),/tmp)/tfprovider-release-check || { echo "release binary contains dev mode" >&2; exit 1; }
	@echo "release build has no dev mode"

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
	go test -timeout=1m -race -run="$(value run)" $(pkgs)

test-dev: run?=.*
test-dev: pkgs?=./...
test-dev:
	go test -tags dev -timeout=1m -race -run="$(value run)" $(pkgs)

testacc: run?=.*
testacc: pkgs?=./...
testacc:
	TF_ACC=1 TF_ACC_LOG=DEBUG TF_LOG=DEBUG go test -v -timeout=30m -parallel=3 -race -run="$(value run)" $(pkgs)

# Acceptance tests against the local stack (scripts/local/stack.sh up).
testacc-local: run?=TestAccScyllaDBCloudCluster_encryptionAtRestDisabled
testacc-local: pkgs?=./internal/provider/
testacc-local:
	scripts/local/stack.sh status >/dev/null
	TF_ACC=1 SCYLLADB_CLOUD_DEV_MODE=1 SCYLLADB_CLOUD_ENDPOINT=http://localhost:3011 \
		SCYLLADB_CLOUD_TEST_AWS_NODE_TYPE=$${SCYLLADB_CLOUD_TEST_AWS_NODE_TYPE:-$(if $(filter arm64,$(shell uname -m)),i8g.large,i3.large)} \
		go test -tags dev -v -timeout=60m -parallel=1 -run="$(value run)" $(pkgs)

list-testacc:
	@find . -name '*.go' -print0 | xargs -0 grep -Eoh "func TestAcc\w*" | sed 's/func //'

# Install golangci-lint following https://golangci-lint.run/docs/welcome/install/local/.
# go tool is not recommended.
# Pin to specific commit SHA aligned with the requested version.
# The install.sh script performs checksum verification of the downloaded binary.
$(GOPATH)/bin/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/9f61b0f53f80672872fced07b6874397c3ed197b/install.sh | sh -s -- -b $(GOPATH)/bin v2.7.2

.PHONY: build build-dev check-release fmt generate lint test test-dev testacc testacc-local
