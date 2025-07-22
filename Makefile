.PHONY: build clean install test test-race test-coverage lint fmt vet check help

# Default target
all: build

# Build the application
build:
	go build -o sim

# Build for multiple platforms
build-all:
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -o dist/sim-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -o dist/sim-darwin-arm64
	GOOS=linux GOARCH=amd64 go build -o dist/sim-linux-amd64
	GOOS=windows GOARCH=amd64 go build -o dist/sim-windows-amd64.exe

# Clean build artifacts
clean:
	rm -f sim
	rm -rf dist/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test ./...

# Run tests with race detection
test-race:
	go test -race ./...

# Run tests with coverage
test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Run go vet
vet:
	go vet ./...

# Run all checks (used in CI)
check: fmt vet lint test-race

# Install to system (requires appropriate permissions)
install: build
	cp sim /usr/local/bin/

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  test          - Run tests"
	@echo "  test-race     - Run tests with race detection"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  check         - Run all checks (fmt, vet, lint, test-race)"
	@echo "  install       - Install to /usr/local/bin"
	@echo "  help          - Show this help"