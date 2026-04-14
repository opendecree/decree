# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /bin/decree ./cmd/server

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/decree /bin/decree

USER nonroot:nonroot

ENTRYPOINT ["/bin/decree"]
