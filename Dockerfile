# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS builder
WORKDIR /src

# Cache deps first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the rest
COPY . .

# Build static linux binary
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/app /app/app
USER appuser
ENTRYPOINT ["/app/app"]

