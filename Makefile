# OpenDecree Makefile
#
# Prerequisites: Go 1.25+, Docker, Make. All code generators run in Docker.
# Run `make help` to see available targets.
#
# Workflow: modify specs → make generate → make pre-commit → commit
# Full CI:  make all (generate + lint + test + build)

# --- Configuration ---

BINARY_NAME := decree-server
BUILD_DIR := bin
TOOLS_IMAGE := decree-tools
TOOLS_SENTINEL := .tools-image-built
DOCKER_RUN_TOOLS := docker run --rm -u $(shell id -u):$(shell id -g) -e HOME=/tmp -v $(CURDIR):/workspace -w /workspace $(TOOLS_IMAGE)
GOLANGCI_LINT_VERSION := v2.8.0

# Version injection for binaries.
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
SERVER_LDFLAGS := -X github.com/opendecree/decree/internal/version.Version=$(GIT_VERSION) -X github.com/opendecree/decree/internal/version.Commit=$(GIT_COMMIT)
CLI_LDFLAGS := -X main.cliVersion=$(GIT_VERSION) -X main.cliCommit=$(GIT_COMMIT)

# Module list for multi-module operations.
SDK_MODULES := sdk/configclient sdk/adminclient sdk/configwatcher sdk/tools

.PHONY: all generate generate-proto generate-sqlc test lint build image migrate e2e examples bench bench-e2e docs docs-api docs-cli docs-man docs-serve docs-deploy pre-commit clean tools help demo-gif validate-meta-schemas

all: generate lint test build

# --- Docker tools ---

## tools: Build the tools Docker image (only when Dockerfile.tools changes)
tools: $(TOOLS_SENTINEL)
$(TOOLS_SENTINEL): build/Dockerfile.tools
	docker build -t $(TOOLS_IMAGE) -f build/Dockerfile.tools .
	@touch $(TOOLS_SENTINEL)

# --- Code generation ---

## generate: Generate code from protobuf and SQL specs (Docker)
generate: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) sh -c 'buf generate && cd db && sqlc generate'
	python3 scripts/patch-openapi.py

## generate-proto: Generate Go code from protobuf definitions only (Docker)
generate-proto: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) buf generate
	python3 scripts/patch-openapi.py

## generate-sqlc: Generate Go code from SQL queries only (Docker)
generate-sqlc: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) bash -c "cd db && sqlc generate"

# --- Build ---

## build: Build service + CLI binaries to bin/
build:
	go build -ldflags '$(SERVER_LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree .

## image: Build the Docker image
image:
	docker build -t $(BINARY_NAME) -f build/Dockerfile .

# --- Quality ---

## lint: Run all linters (Go + protobuf)
lint: lint-go lint-proto

## lint-go: Run golangci-lint
lint-go:
	golangci-lint run ./...

## lint-proto: Run buf lint + breaking change detection (Docker)
lint-proto: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) sh -c 'buf lint && buf breaking --against ".git#branch=main"'

## test: Run unit tests across all modules
test:
	go test ./... -race -count=1
	@for mod in $(SDK_MODULES) cmd/decree; do (cd $$mod && go test ./... -race -count=1) || exit 1; done

## validate-meta-schemas: Validate canonical schema/config YAMLs against the v0.1.0 meta-schemas
validate-meta-schemas:
	python3 scripts/validate-meta-schemas.py

## pre-commit: Run all before-commit checks (build, vet, format, lint, test, coverage)
pre-commit:
	@echo "=== Build ==="
	go build ./...
	@for mod in $(SDK_MODULES) cmd/decree; do (cd $$mod && go build ./...) || exit 1; done
	@echo "=== Vet ==="
	go vet ./...
	@for mod in $(SDK_MODULES) cmd/decree; do (cd $$mod && go vet ./...) || exit 1; done
	@echo "=== Format ==="
	@unformatted=$$(gofumpt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; echo "$$unformatted"; \
		echo "Run: gofumpt -w ."; exit 1; \
	fi
	@echo "=== Lint ==="
	golangci-lint run ./...
	@echo "=== Test ==="
	go test ./... -race -count=1
	@for mod in $(SDK_MODULES) cmd/decree; do (cd $$mod && go test ./... -race -count=1) || exit 1; done
	@echo "=== Coverage ==="
	./scripts/check-coverage.sh
	@echo ""
	@echo "✓ All pre-commit checks passed"

# --- Database ---

## migrate: Run database migrations up (Docker)
migrate: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) goose -dir db/migrations postgres "$${DB_WRITE_URL:-postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable}" up

## migrate-down: Roll back the last migration (Docker)
migrate-down: $(TOOLS_SENTINEL)
	$(DOCKER_RUN_TOOLS) goose -dir db/migrations postgres "$${DB_WRITE_URL:-postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable}" down

# --- Integration tests ---

## e2e: Run end-to-end tests (docker compose lifecycle)
e2e:
	docker compose up -d --wait service
	cd e2e && go test -tags=e2e -v -race -count=1 ./... || (cd .. && docker compose down -v && exit 1)
	docker compose down -v

## e2e-coverage: Run e2e against a -cover instrumented server and report coverage on files excluded from unit-test total
e2e-coverage:
	./scripts/e2e-coverage.sh

## examples: Run SDK examples (docker compose lifecycle)
examples:
	docker compose up -d --wait service
	cd examples && make setup && make test || (cd .. && docker compose down -v && exit 1)
	docker compose down -v

# --- Benchmarks ---

## bench: Run unit benchmarks
bench:
	go test ./internal/... -bench=. -benchmem -count=3 -run=^$$

## bench-e2e: Run e2e benchmarks (docker compose lifecycle)
bench-e2e:
	docker compose up -d --wait service
	cd e2e && go test -tags=e2e -bench=. -benchmem -count=3 -run=^$$ -timeout=300s ./... || (cd .. && docker compose down -v && exit 1)
	docker compose down -v

# --- Documentation ---

## docs: Generate all documentation (API + CLI + man pages)
docs: docs-api docs-cli docs-man

## docs-api: Generate proto API reference markdown (Docker)
docs-api: $(TOOLS_SENTINEL)
	@mkdir -p docs/api
	$(DOCKER_RUN_TOOLS) buf generate --template buf.gen.doc.yaml

## docs-cli: Generate CLI reference markdown
docs-cli:
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree . && cd ../.. && $(BUILD_DIR)/decree gen-docs docs/cli

## docs-man: Generate man pages
docs-man:
	cd cmd/decree && go build -ldflags '$(CLI_LDFLAGS)' -o ../../$(BUILD_DIR)/decree . && cd ../.. && $(BUILD_DIR)/decree gen-man docs/man

# --- Demo recording ---

## demo-gif: Render assets/demo.tape to assets/demo.gif (needs docker + running service at :9090)
demo-gif:
	docker exec decree-postgres-1 psql -U centralconfig -d centralconfig -c "TRUNCATE tenants, schemas, schema_versions, config_values CASCADE;" >/dev/null
	docker run --rm --network host \
	  -v $(CURDIR):/vhs \
	  -v $(CURDIR)/bin/decree:/usr/local/bin/decree:ro \
	  ghcr.io/charmbracelet/vhs /vhs/assets/demo.tape
	docker run --rm -v $(CURDIR)/assets:/w alpine chown -R $(shell id -u):$(shell id -g) /w

# --- Maintenance ---

## clean: Remove build artifacts and generated code
clean:
	rm -rf $(BUILD_DIR)
	$(DOCKER_RUN_TOOLS) rm -rf api/centralconfig/ internal/storage/dbstore/

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
