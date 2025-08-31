.PHONY: test lint fmt vet build clean examples bench coverage proto proto-clean googleapis setup run-example run-basic-example run-advanced-example test-server test-basic-server test-advanced-server test-integration test-basic-integration test-advanced-integration

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=grpc-header-mapper

# Proto parameters
PROTO_DIR=test/testdata/proto
PROTO_FILES=$(PROTO_DIR)/test.proto
PROTOC=protoc

# Generate proto files (depends on googleapis)
proto: googleapis
	@echo "Generating proto files..."
	@mkdir -p $(PROTO_DIR)
	$(PROTOC) \
		--proto_path=$(PROTO_DIR) \
		--proto_path=third_party/googleapis \
		--go_out=$(PROTO_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_DIR) \
		--go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(PROTO_DIR) \
		--grpc-gateway_opt=paths=source_relative \
		$(PROTO_FILES)
	@echo "Proto files generated successfully!"

# Clean generated proto files
proto-clean:
	@echo "Cleaning generated proto files..."
	@rm -f $(PROTO_DIR)/*.pb.go
	@echo "Generated proto files cleaned!"

# Install googleapis proto files (needed for grpc-gateway annotations)
googleapis:
	@echo "Setting up googleapis proto files..."
	@mkdir -p third_party/googleapis/google/api
	@curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto -o third_party/googleapis/google/api/annotations.proto
	@curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto -o third_party/googleapis/google/api/http.proto
	@curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/httpbody.proto -o third_party/googleapis/google/api/httpbody.proto
	@echo "googleapis proto files downloaded!"

# Build (depends on proto generation)
build: proto
	$(GOBUILD) -v ./...

# Test (depends on proto generation)
test: proto
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Benchmark
bench: proto
	$(GOTEST) -bench=. -benchmem ./...

# Coverage
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	$(GOCMD) fmt ./...

# Vet
vet: proto
	$(GOCMD) vet ./...

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) verify

# Tidy
tidy: proto
	$(GOMOD) tidy

# Clean
clean: proto-clean
	$(GOCLEAN)
	rm -f coverage.out coverage.html
	rm -rf bin/

# Examples - Build all example binaries
examples: proto
	@mkdir -p bin
	$(GOBUILD) -o bin/example-server cmd/example-server/main.go
	$(GOBUILD) -o bin/basic-example examples/basic/main.go
	$(GOBUILD) -o bin/advanced-example examples/advanced/main.go
	@echo "All example binaries built successfully!"

# Run the basic example server
run-basic-example: examples
	@echo "Starting basic example server..."
	@echo "Use Ctrl+C to stop gracefully"
	./bin/basic-example

# Run the advanced example server
run-advanced-example: examples
	@echo "Starting advanced example server..."
	@echo "Use Ctrl+C to stop gracefully"
	./bin/advanced-example

# Run the example server (backward compatibility - runs basic)
run-example: run-basic-example

# Test the basic server (requires server to be running)
test-basic-server:
	@echo "Testing basic server functionality..."
	@mkdir -p scripts
	@chmod +x scripts/test-basic-server.sh
	@./scripts/test-basic-server.sh

# Test the advanced server (requires server to be running)
test-advanced-server:
	@echo "Testing advanced server functionality..."
	@mkdir -p scripts
	@chmod +x scripts/test-advanced-server.sh
	@./scripts/test-advanced-server.sh

# Test the running server (backward compatibility - tests basic server)
test-server: test-basic-server

# Run basic server and test it automatically
test-basic-integration: examples
	@echo "Running basic server integration tests..."
	@echo "Starting server in background..."
	@./bin/basic-example & \
	SERVER_PID=$$!; \
	echo "Server PID: $$SERVER_PID"; \
	sleep 5; \
	mkdir -p scripts; \
	chmod +x scripts/test-basic-server.sh; \
	echo "Running tests..."; \
	./scripts/test-basic-server.sh; \
	TEST_RESULT=$$?; \
	echo "Stopping server..."; \
	kill $$SERVER_PID 2>/dev/null || kill -9 $$SERVER_PID 2>/dev/null; \
	wait $$SERVER_PID 2>/dev/null; \
	echo "Integration tests completed with result: $$TEST_RESULT"; \
	exit $$TEST_RESULT

# Run advanced server and test it automatically
test-advanced-integration: examples
	@echo "Running advanced server integration tests..."
	@echo "Starting advanced server in background..."
	@./bin/advanced-example & \
	SERVER_PID=$$!; \
	echo "Server PID: $$SERVER_PID"; \
	sleep 5; \
	mkdir -p scripts; \
	chmod +x scripts/test-advanced-server.sh; \
	echo "Running tests..."; \
	./scripts/test-advanced-server.sh; \
	TEST_RESULT=$$?; \
	echo "Stopping server..."; \
	kill $$SERVER_PID 2>/dev/null || kill -9 $$SERVER_PID 2>/dev/null; \
	wait $$SERVER_PID 2>/dev/null; \
	echo "Advanced integration tests completed with result: $$TEST_RESULT"; \
	exit $$TEST_RESULT

# Run server and test it automatically (backward compatibility - uses basic)
test-integration: test-basic-integration

# Install all development tools
tools:
	@echo "Installing development tools..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin
	$(GOGET) github.com/goreleaser/goreleaser@latest
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GOGET) github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	@echo "All tools installed successfully!"

# Development setup (run this first)
setup: googleapis tools proto
	@echo "Development environment setup complete!"

# CI tasks
ci: deps proto fmt vet lint test

# Release (requires goreleaser)
release:
	goreleaser release --clean

# Pre-commit hook
pre-commit: fmt vet lint test

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Setup and Development:"
	@echo "  setup     - Set up development environment (run this first)"
	@echo "  tools     - Install development tools"
	@echo "  googleapis - Download googleapis proto files"
	@echo "  proto     - Generate proto files (includes grpc-gateway)"
	@echo ""
	@echo "Build and Test:"
	@echo "  build     - Build the library (generates proto first)"
	@echo "  test      - Run tests with race detection"
	@echo "  bench     - Run benchmarks"
	@echo "  coverage  - Generate test coverage report"
	@echo "  lint      - Run linter"
	@echo "  fmt       - Format code"
	@echo "  vet       - Run go vet"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps      - Download dependencies"
	@echo "  tidy      - Tidy go modules"
	@echo "  clean     - Clean build artifacts and generated proto files"
	@echo ""
	@echo "Examples:"
	@echo "  examples  - Build all example binaries"
	@echo "  run-basic-example - Build and run the basic example server"
	@echo "  run-advanced-example - Build and run the advanced example server"
	@echo "  run-example - Build and run the basic example server (default)"
	@echo ""
	@echo "Testing:"
	@echo "  test-basic-server - Test the running basic server (server must be running)"
	@echo "  test-advanced-server - Test the running advanced server (server must be running)"
	@echo "  test-server - Test the running basic server (backward compatibility)"
	@echo ""
	@echo "Integration Testing (automated):"
	@echo "  test-basic-integration - Run basic server and test it automatically"
	@echo "  test-advanced-integration - Run advanced server and test it automatically"
	@echo "  test-integration - Run basic server integration tests (default)"
	@echo ""
	@echo "CI/CD:"
	@echo "  ci        - Run all CI checks"
	@echo "  release   - Create a release"
	@echo "  pre-commit - Run pre-commit checks"
	@echo "  help      - Show this help"
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup                    # First time setup"
	@echo "  make run-basic-example        # Run basic server"
	@echo "  make test-basic-integration   # Test everything automatically"