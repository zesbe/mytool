
# Build configuration
APP_NAME := mytool
VERSION := 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Output directory
DIST_DIR := dist

.PHONY: all build clean test run release checksums

# Default target
all: build

# Build for current platform
build:
	@echo "Building $(APP_NAME) v$(VERSION)..."
	go build $(LDFLAGS) -o $(APP_NAME) .
	@echo "Done! Run ./$(APP_NAME) to start"

# Run the app
run: build
	./$(APP_NAME)

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf $(DIST_DIR)
	rm -f $(APP_NAME)
	rm -f $(APP_NAME).exe

# Build for all platforms
release: clean
	@echo "Building releases for v$(VERSION)..."
	@mkdir -p $(DIST_DIR)

	@echo "  → Linux AMD64"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .

	@echo "  → Linux ARM64"
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 .

	@echo "  → macOS AMD64"
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .

	@echo "  → macOS ARM64 (Apple Silicon)"
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .

	@echo "  → Windows AMD64"
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .

	@echo ""
	@echo "Releases built in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

# Generate checksums
checksums: release
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && \
	for f in *; do \
		sha256sum "$$f" > "$$f.sha256"; \
	done
	@echo "Checksums generated!"
	@ls -la $(DIST_DIR)/*.sha256

# Install locally
install: build
	@echo "Installing to ~/.local/bin/$(APP_NAME)..."
	@mkdir -p ~/.local/bin
	@cp $(APP_NAME) ~/.local/bin/$(APP_NAME)
	@echo "Installed! Make sure ~/.local/bin is in your PATH"

# Show help
help:
	@echo "Available targets:"
	@echo "  make build     - Build for current platform"
	@echo "  make run       - Build and run"
	@echo "  make test      - Run tests"
	@echo "  make release   - Build for all platforms"
	@echo "  make checksums - Generate SHA256 checksums"
	@echo "  make install   - Install to ~/.local/bin"
	@echo "  make clean     - Remove build artifacts"
