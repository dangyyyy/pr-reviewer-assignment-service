.PHONY: build run test clean docker-build docker-up docker-down docker-logs lint help test-api

build:
	@echo "Building application..."
	@go build -o bin/server ./cmd/server

run: build
	@echo "Running application..."
	@./bin/server

test:
	@echo "Running tests..."
	@go test -v ./...

test-api:
	@echo "Testing API..."
	@./scripts/test_api.sh

clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean

docker-build:
	@echo "Building Docker image..."
	@docker-compose build

docker-up:
	@echo "Starting services..."
	@docker-compose up -d

docker-down:
	@echo "Stopping services..."
	@docker-compose down

docker-logs:
	@docker-compose logs -f app

lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

fmt:
	@echo "Formatting code..."
	@go fmt ./...

help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application locally"
	@echo "  test          - Run tests"
	@echo "  test-api      - Run API integration tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-up     - Start services with docker-compose"
	@echo "  docker-down   - Stop services"
	@echo "  docker-logs   - View application logs"
	@echo "  lint          - Run linter"
	@echo "  deps          - Download dependencies"
	@echo "  fmt           - Format code"
	@echo "  help          - Show this help message"
