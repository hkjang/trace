# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS web-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS go-builder
ARG VERSION=0.2.0-dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/frontend/dist/ /src/internal/web/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/hkjang/trace/internal/version.Version=${VERSION} -X github.com/hkjang/trace/internal/version.Commit=${COMMIT} -X github.com/hkjang/trace/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/trace ./cmd/trace

FROM alpine:3.23
ARG VERSION=0.2.0-dev
LABEL org.opencontainers.image.title="trace" \
      org.opencontainers.image.description="Temporal decision intelligence service" \
      org.opencontainers.image.source="https://github.com/hkjang/trace" \
      org.opencontainers.image.version="${VERSION}"
RUN apk add --no-cache ca-certificates tzdata && addgroup -S trace && adduser -S -G trace trace
COPY --from=go-builder /out/trace /usr/local/bin/trace
USER trace:trace
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/trace"]
