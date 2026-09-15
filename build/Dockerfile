# Build stage
# golang:1.26-bookworm
FROM golang@sha256:f44f6e88636cfb311f9ebace870ded69d943f227bb3cb27d32ffd84ea18c43ea AS builder

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
FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2

COPY --from=builder /bin/decree-server /bin/decree-server

USER nonroot:nonroot

ENTRYPOINT ["/bin/decree-server"]
