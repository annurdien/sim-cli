.PHONY: build clean install test help

# Default target
all: build

# Build the application
build:
	go build -o sim

# Build for multiple platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -o dist/sim-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -o dist/sim-darwin-arm64
	GOOS=linux GOARCH=amd64 go build -o dist/sim-linux-amd64
	GOOS=windows GOARCH=amd64 go build -o dist/sim-windows-amd64.exe

# Clean build artifacts
clean:
	rm -f sim
	rm -rf dist/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test ./...

# Install to system (requires appropriate permissions)
install: build
	cp sim /usr/local/bin/

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  clean      - Clean build artifacts"
	@echo "  deps       - Install dependencies"
	@echo "  test       - Run tests"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  help       - Show this help"