# c64u Makefile
# Build automation for the C64 Ultimate CLI tool

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build variables
BINARY_NAME := c64u
BUILD_DIR := build
DIST_DIR := dist
GO_FILES := $(shell find . -name '*.go' -type f)

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Default target
.PHONY: all
all: build

# Build the binary for current platform
.PHONY: build
build: $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(GO_FILES)
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/c64u

# Install to ~/bin or /usr/local/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	@if [ -d "$$HOME/bin" ]; then \
		cp $(BUILD_DIR)/$(BINARY_NAME) $$HOME/bin/; \
		echo "Installed to $$HOME/bin/$(BINARY_NAME)"; \
	elif [ -w /usr/local/bin ]; then \
		cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/; \
		echo "Installed to /usr/local/bin/$(BINARY_NAME)"; \
	else \
		echo "Error: Neither ~/bin nor /usr/local/bin is writable"; \
		echo "Try: sudo make install"; \
		exit 1; \
	fi

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html

# Build for all platforms
.PHONY: release
release: clean
	@echo "Building release binaries..."
	@mkdir -p $(DIST_DIR)

	@echo "Building for macOS (Intel)..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/c64u

	@echo "Building for macOS (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/c64u

	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/c64u

	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/c64u

	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/c64u

	@echo "Release binaries built in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  brew install golangci-lint"; \
		echo "  or visit https://golangci-lint.run/usage/install/"; \
	fi

# Run the binary (for testing)
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Development build with verbose output
.PHONY: dev
dev:
	@echo "Building development version..."
	go build -v -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/c64u

# Show help
.PHONY: help
help:
	@echo "c64u Makefile targets:"
	@echo ""
	@echo "  make build          - Build binary for current platform"
	@echo "  make install        - Install binary to ~/bin or /usr/local/bin"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make release        - Build for all platforms"
	@echo "  make deps           - Download and tidy dependencies"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter (requires golangci-lint)"
	@echo "  make run            - Build and run the binary"
	@echo "  make dev            - Build development version with verbose output"
	@echo "  make help           - Show this help message"
	@echo ""
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
