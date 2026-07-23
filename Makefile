.PHONY: build build-all clean install test test-race test-coverage lint fmt vet check help cam-build cam-clean

# Default target
all: build

# Extract version from config.yaml
VERSION := $(shell grep 'version:' config.yaml | awk '{print $$2}' | tr -d '"')
LDFLAGS := -ldflags "-X github.com/annurdien/sim-cli/cmd.Version=$(VERSION)"

UNAME_S := $(shell uname -s)

# Build the application (with embedded cam binaries on macOS)
build:
ifeq ($(UNAME_S),Darwin)
	@cd Iris && ./Scripts/build.sh
	@printf "  \033[2m›\033[0m Compiling sim..."
	@go build -tags cam_embed $(LDFLAGS) -o sim
	@printf "\r  \033[32m✓\033[0m Compiling sim   \n"
	@printf "\n  \033[1m\033[32msim\033[0m built successfully.\n\n"
else
	@printf "  \033[2m›\033[0m Compiling sim..."
	@go build $(LDFLAGS) -o sim
	@printf "\r  \033[32m✓\033[0m Compiling sim   \n"
	@printf "\n  \033[1m\033[32msim\033[0m built successfully.\n\n"
endif

# Build for multiple platforms
build-all:
	@mkdir -p dist
ifeq ($(UNAME_S),Darwin)
	@cd Iris && ./Scripts/build.sh
	@GOOS=darwin GOARCH=amd64 go build -tags cam_embed $(LDFLAGS) -o dist/sim-darwin-amd64
	@GOOS=darwin GOARCH=arm64 go build -tags cam_embed $(LDFLAGS) -o dist/sim-darwin-arm64
else
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/sim-darwin-amd64
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/sim-darwin-arm64
endif
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/sim-linux-amd64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/sim-windows-amd64.exe

# Clean build artifacts
clean:
	rm -f sim
	rm -rf dist/
	rm -rf cmd/assets/
	rm -f coverage.out coverage.html

# Build Iris (FrameHost + IrisInject dylib)
cam-build:
	@cd Iris && ./Scripts/build.sh

# Clean Iris build artifacts
cam-clean:
	cd Iris && swift package clean
	rm -rf Iris/.build/injector
	rm -rf cmd/assets/

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
	@echo "  cam-build     - Build Iris (FrameHost + IrisInject dylib)"
	@echo "  cam-clean     - Clean Iris build artifacts"
	@echo "  help          - Show this help"