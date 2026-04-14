BINARY_NAME := decree-server
BUILD_DIR := bin
TOOLS_IMAGE := decree-tools
TOOLS_SENTINEL := .tools-image-built
DOCKER_RUN_TOOLS := docker run --rm -u $(shell id -u):$(shell id -g) -e HOME=/tmp -v $(CURDIR):/workspace -w /workspace $(TOOLS_IMAGE)
MKDOCS_IMAGE := $(MKDOCS_IMAGE):9.7.6
GOLANGCI_LINT_VERSION := v2.8.0

.PHONY: all generate generate-proto generate-sqlc test lint build image migrate e2e bench bench-e2e docs docs-api docs-cli docs-serve docs-deploy clean tools help

all: generate lint test build

## tools: Build the tools Docker image (only when Dockerfile.tools changes)
tools: $(TOOLS_SENTINEL)
$(TOOLS_SENTINEL): build/Dockerfile.tools
	docker build -t $(TOOLS_IMAGE) -f build/Dockerfile.tools .
	@touch $(TOOLS_SENTINEL)

## generate: Generate code from protobuf and SQL specs
generate: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) sh -c 'buf generate && cd db && sqlc generate'
	cp docs/api/openapi.swagger.json cmd/server/openapi.json

## generate-proto: Generate Go code from protobuf definitions
generate-proto: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) buf generate
	cp docs/api/openapi.swagger.json cmd/server/openapi.json

## generate-sqlc: Generate Go code from SQL queries
generate-sqlc: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) bash -c "cd db && sqlc generate"

## test: Run unit tests (all modules)
test:
	go test ./... -race -count=1
	cd sdk/configclient && go test ./... -race -count=1
	cd sdk/adminclient && go test ./... -race -count=1
	cd sdk/configwatcher && go test ./... -race -count=1
	cd sdk/tools && go test ./... -race -count=1
	cd cmd/decree && go test ./... -race -count=1

## lint: Run linters
lint: lint-go lint-proto

## lint-go: Run Go linter
lint-go:
	golangci-lint run ./...

## lint-proto: Run protobuf linter and breaking change detection
lint-proto: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) sh -c 'buf lint && buf breaking --against ".git#branch=main"'

## build: Build the service binary
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
SERVER_LDFLAGS := -X github.com/opendecree/decree/internal/version.Version=$(GIT_VERSION) -X github.com/opendecree/decree/internal/version.Commit=$(GIT_COMMIT)
CLI_LDFLAGS := -X main.cliVersion=$(GIT_VERSION) -X main.cliCommit=$(GIT_COMMIT)

build:
	go build -ldflags '$(SERVER_LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree .

## image: Build the Docker image
image:
	docker build -t $(BINARY_NAME) -f build/Dockerfile .

## migrate: Run database migrations
migrate: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) goose -dir db/migrations postgres "$${DB_WRITE_URL:-postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable}" up

## migrate-down: Roll back the last migration
migrate-down: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) goose -dir db/migrations postgres "$${DB_WRITE_URL:-postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable}" down

## e2e: Run end-to-end tests (docker compose → migrate → test → teardown)
e2e:
	docker compose up -d --wait service
	cd e2e && go test -tags=e2e -v -race -count=1 ./... || (cd .. && docker compose down -v && exit 1)
	docker compose down -v

## bench: Run unit benchmarks
bench:
	go test ./internal/... -bench=. -benchmem -count=3 -run=^$

## bench-e2e: Run e2e benchmarks (docker compose → migrate → bench → teardown)
bench-e2e:
	docker compose up -d --wait service
	cd e2e && go test -tags=e2e -bench=. -benchmem -count=3 -run=^$ -timeout=300s ./... || (cd .. && docker compose down -v && exit 1)
	docker compose down -v

## docs: Generate all documentation (API + CLI + man pages)
docs: docs-api docs-cli docs-man

## docs-api: Generate proto API reference markdown
docs-api: $(TOOLS_SENTINEL)
	@mkdir -p docs/api
	$(DOCKER_RUN_TOOLS) buf generate --template buf.gen.doc.yaml

## docs-cli: Generate CLI reference markdown
docs-cli:
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree . && cd ../.. && $(BUILD_DIR)/decree gen-docs docs/cli

## docs-man: Generate man pages
docs-man:
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree . && cd ../.. && $(BUILD_DIR)/decree gen-man docs/man

## docs-serve: Local MkDocs preview (Docker) at http://localhost:8000
docs-serve:
	docker run --rm -p 8000:8000 -v $(CURDIR):/docs $(MKDOCS_IMAGE) serve --dev-addr=0.0.0.0:8000

## docs-deploy: Deploy docs to GitHub Pages
docs-deploy:
	docker run --rm -v $(CURDIR):/docs $(MKDOCS_IMAGE) gh-deploy --force

## clean: Remove build artifacts and generated code
clean:
	rm -rf $(BUILD_DIR)
	$(DOCKER_RUN_TOOLS) rm -rf api/centralconfig/ internal/storage/dbstore/

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
