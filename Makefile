# Makefile for Cartographer

BINARY_NAME=cartographer
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_DIR=build
LDFLAGS=-ldflags="-X main.version=$(VERSION)"

# Targets
.PHONY: all clean lint test build docker deps

all: deps lint test build

deps:
	go get -u github.com/spf13/cobra
	go get -u github.com/spf13/viper
	go get -u k8s.io/apimachinery
	go get -u sigs.k8s.io/yaml
	go mod tidy

clean:
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned build artifacts."

lint:
	@golangci-lint run
	@echo "Linting complete."

test:
	@go test ./...
	@echo "Tests passed."

# Build for Linux and Mac ARM
build:
	@mkdir -p $(BUILD_DIR)/linux
	@mkdir -p $(BUILD_DIR)/darwin_arm64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux/$(BINARY_NAME) .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin_arm64/$(BINARY_NAME) .
	@echo "Build complete."

# Use a multi-stage Dockerfile for a minimal runtime image
docker: build
	docker build --platform linux/amd64 -t cartographer:$(VERSION) .
