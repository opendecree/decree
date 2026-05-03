# Build stage
# golang:1.26-bookworm
FROM golang@sha256:47ce5636e9936b2c5cbf708925578ef386b4f8872aec74a67bd13a627d242b19 AS builder

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
    -o /bin/decree ./cmd/server

# Runtime stage
# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838

COPY --from=builder /bin/decree /bin/decree

USER nonroot:nonroot

ENTRYPOINT ["/bin/decree"]
