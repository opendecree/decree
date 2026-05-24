# Build stage — golang:1.26-bookworm
FROM golang@sha256:313faae491b410a35402c05d35e7518ae99103d957308e940e1ae2cfa0aac29b AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /bin/jwks-server .

# Runtime stage — distroless (no shell; health check uses built-in subcommand)
# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838

COPY --from=builder /bin/jwks-server /bin/jwks-server

USER nonroot:nonroot

ENTRYPOINT ["/bin/jwks-server"]
