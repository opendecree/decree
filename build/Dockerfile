# Build stage
FROM golang:1.24-bookworm AS builder

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
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/decree /bin/decree

USER nonroot:nonroot

ENTRYPOINT ["/bin/decree"]
