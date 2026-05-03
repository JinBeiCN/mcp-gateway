.PHONY: build test run clean lint

APP_NAME = mcp-gateway
MODULE = github.com/JinBeiCN/mcp-gateway
VERSION = 1.0.0
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

build:
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/mcp-gateway/

build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/mcp-gateway/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-amd64 ./cmd/mcp-gateway/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-arm64 ./cmd/mcp-gateway/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe ./cmd/mcp-gateway/

run:
	go run ./cmd/mcp-gateway/ --config config.example.yaml

test:
	go test -v -cover ./...

test-race:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -rf bin/

validate:
	go run ./cmd/mcp-gateway/ --config config.example.yaml --validate

fmt:
	go fmt ./...

deps:
	go mod tidy
	go mod verify
