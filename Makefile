VERSION := $(shell tr -d '[:space:]' < VERSION)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: test build web image release-image

test:
	go test ./...
	cd frontend && npm run lint

web:
	cd frontend && npm ci && npm run build
	find internal/web/dist -mindepth 1 -type f -delete
	cp -a frontend/dist/. internal/web/dist/

build: web
	go build -trimpath -ldflags="-X github.com/hkjang/trace/internal/version.Version=$(VERSION) -X github.com/hkjang/trace/internal/version.Commit=$(COMMIT) -X github.com/hkjang/trace/internal/version.BuildTime=$(BUILD_TIME)" -o trace ./cmd/trace

image:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) -t trace:v$(VERSION) .

release-image:
	./scripts/release-image.sh v$(VERSION)
