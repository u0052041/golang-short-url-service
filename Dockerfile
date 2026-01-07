# Build stage
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 先留預設值，讓你本機沒帶參數也能 build；之後 CI 再注入
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
  -ldflags="-w -s -X main.version=$VERSION" \
  -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

WORKDIR /app
COPY --from=builder /app/server .
USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/server"]
