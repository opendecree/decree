#!/bin/bash
# Installs development tools pinned to the same versions as build/Dockerfile.tools.
# Runs during Codespaces prebuild (onCreateCommand) so the result is cached.
set -euo pipefail

# Proto toolchain
go install github.com/bufbuild/buf/cmd/buf@v1.66.1
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.3
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.27.3

# DB toolchain
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
go install github.com/pressly/goose/v3/cmd/goose@v3.27.0

# Go linter
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b "$(go env GOPATH)/bin" v2.11.4

# Release toolchain
go install github.com/goreleaser/goreleaser/v2@latest

# Protobuf compiler (IDE support)
sudo apt-get update -qq && sudo apt-get install -y -qq protobuf-compiler
