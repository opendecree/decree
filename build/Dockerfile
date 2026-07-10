# Build stage
# golang:1.26-bookworm
FROM golang@sha256:079e59808d2d252516e27e3f3a9c003740dee7f75e55aa71528766d52bcfc16a AS builder

WORKDIR /app

# Cache dependencies — this layer only rebuilds when go.mod/go.sum change.
COPY go.mod go.sum ./
COPY api/go.mod api/go.sum ./api/
RUN go mod download

# Build — this layer rebuilds on any source change, but deps are cached.
ARG VERSION=dev
ARG COMMIT=unknown
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/opendecree/decree/internal/version.Version=${VERSION} -X github.com/opendecree/decree/internal/version.Commit=${COMMIT}" \
    -o /bin/decree-server ./cmd/server

# Runtime stage
# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:9c346e4be81b5ca7ff31a0d89eaeade58b0f95cfd3baed1f36083ddb47ca3160

COPY --from=builder /bin/decree-server /bin/decree-server

USER nonroot:nonroot

ENTRYPOINT ["/bin/decree-server"]
