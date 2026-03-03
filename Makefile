.PHONY: build build-all test lint clean help

BUILD_DIR := build
BINARY_NAME := okms-k8s-encryption-provider
DIST_DIR := $(BUILD_DIR)/dist
MAIN_PKG := ./cmd/okms-k8s-encryption-provider

# Supported architectures for cross-compilation
ARCHITECTURES := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

all: lint build test

# Build the application for current architecture
build:
	@echo "Building okms-k8s-encryption-provider for current architecture..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Build for multiple architectures
build-all: clean
	@echo "Building okms-k8s-encryption-provider for multiple architectures..."
	@mkdir -p $(DIST_DIR)
	@for arch in $(ARCHITECTURES); do \
		os=$${arch%/*}; \
		cpu=$${arch#*/}; \
		output=$(DIST_DIR)/$(BINARY_NAME)-$${os}-$${cpu}; \
		if [ "$$os" = "windows" ]; then \
			output=$${output}.exe; \
		fi; \
		echo "Building for $$arch -> $$output"; \
		GOOS=$$os GOARCH=$$cpu go build -o $$output -ldflags "-X main.version=$(GIT_VERSION)" $(MAIN_PKG); \
	done
	@echo "Build complete. Artifacts in $(DIST_DIR)/"

# Lint code with golangci-lint
lint:
	@echo "Running golangci-lint..."
	@mkdir -p $(BUILD_DIR)
	golangci-lint run --output.json.path=$(BUILD_DIR)/lint-report.json --output.text.path=stdout

# Run tests with coverage and junit report
test:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	go test -v -json -coverprofile=$(BUILD_DIR)/coverage.out ./... > $(BUILD_DIR)/test-results.json
	gotestsum --junitfile=$(BUILD_DIR)/junit.xml --raw-command -- cat $(BUILD_DIR)/test-results.json
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

# Display help message
help:
	@echo "Available targets:"
	@echo "  all            - Run lint,  build, and test"
	@echo "  lint           - Run golangci-lint code linter"
	@echo "  build          - Build the application for current architecture"
	@echo "  build-all      - Build for multiple architectures (linux/darwin/windows, amd64/arm64)"
	@echo "  test           - Run unit tests with coverage and junit report"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Display this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  GIT_VERSION    - Version/tag to embed in binary (default: git describe or dev)"
