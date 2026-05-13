.PHONY: build test clean run-scan run-agent run-server lint fmt help

# Build variables
BINARY_NAME=pgaioptimizer
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

## help: Show this help message
help:
	@echo "pgaioptimizer - PostgreSQL AI Performance Analyzer"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Build the binary"
	@echo "  make test          Run tests"
	@echo "  make lint          Run linter"
	@echo "  make fmt           Format code"
	@echo "  make clean         Remove build artifacts"
	@echo "  make build-all     Cross-compile for all platforms"
	@echo "  make docker        Build Docker image"
	@echo "  make web           Build frontend"
	@echo ""

## build: Build the binary for current platform
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/pgaioptimizer/

## test: Run all tests
test:
	go test -v -race -count=1 ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format Go code
fmt:
	go fmt ./...
	goimports -w .

## clean: Remove build artifacts
clean:
	rm -rf bin/ dist/
	rm -f $(BINARY_NAME)

## build-all: Cross-compile for all platforms
build-all: clean
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_amd64 ./cmd/pgaioptimizer/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_arm64 ./cmd/pgaioptimizer/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_darwin_amd64 ./cmd/pgaioptimizer/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_darwin_arm64 ./cmd/pgaioptimizer/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_windows_amd64.exe ./cmd/pgaioptimizer/

## web: Build frontend
web:
	cd web && npm install && npm run build

## docker: Build Docker image
docker:
	docker build -t $(BINARY_NAME):$(VERSION) .

## run-scan: Run a local scan (requires PG_DSN env var)
run-scan: build
	./bin/$(BINARY_NAME) scan --dsn "$(PG_DSN)"
