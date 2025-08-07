# Makefile for Cartographer

BINARY_NAME=cartographer
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILD_DIR=build
LDFLAGS=-ldflags="-X main.version=$(VERSION)"

# Targets
.PHONY: all deps update-deps clean lint test coverhtml build docker

all: deps test build

deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@echo "Dependencies installed."

update-deps:
	@echo "Updating dependencies..."
	@go get -u
	@go mod tidy
	@echo "Finished Updating Dependencies."

add-helm-repos:
	@echo "Adding necessary Helm repos..."
	@helm repo add bitnami https://charts.bitnami.com/bitnami
	@echo "Finished Adding necessary Helm repos."

clean:
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned build artifacts."

# lint:
# 	@golangci-lint run
# 	@echo "Linting complete."

test: add-helm-repos
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo "Tests passed."

coverhtml: test
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated at coverage.html"

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
