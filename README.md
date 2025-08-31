# gRPC Header Mapper

[![Go Reference](https://pkg.go.dev/badge/github.com/bhatti/grpc-header-mapper.svg)](https://pkg.go.dev/github.com/bhatti/grpc-header-mapper)
[![Go Report Card](https://goreportcard.com/badge/github.com/bhatti/grpc-header-mapper)](https://goreportcard.com/report/github.com/bhatti/grpc-header-mapper)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://github.com/bhatti/grpc-header-mapper/workflows/CI/badge.svg)](https://github.com/bhatti/grpc-header-mapper/actions)
[![codecov](https://codecov.io/gh/bhatti/grpc-header-mapper/branch/main/graph/badge.svg)](https://codecov.io/gh/bhatti/grpc-header-mapper)

A high-performance Go library for mapping HTTP headers to gRPC metadata and vice versa when using grpc-gateway. Designed for production environments with comprehensive configuration options, transformations, and monitoring capabilities.

## Features

- **Bidirectional Mapping**: HTTP headers ↔ gRPC metadata with configurable directions
- **Flexible Configuration**: Programmatic builder API or declarative YAML/JSON config
- **High Performance**: Optimized for production with minimal allocations
- **Transformations**: Built-in and custom header value transformations
- **Path Filtering**: Skip header mapping for specific routes (health checks, metrics)
- **Monitoring Ready**: Built-in metrics and debug logging
- **Well Tested**: Comprehensive test suite with benchmarks
- **Production Ready**: Used in production environments

## Installation

```bash
go get github.com/bhatti/grpc-header-mapper
```

## Quick Start

### Try It Out

```bash
# Clone and set up
git clone https://github.com/bhatti/grpc-header-mapper.git
cd grpc-header-mapper
make setup

# Run the basic example
make run-basic-example

# In another terminal, test it
make test-basic-server

# Or test everything automatically  
make test-basic-integration
```

### Basic Usage

```go
package main

import (
    "github.com/bhatti/grpc-header-mapper/headermapper"
    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func main() {
    // Create mapper with fluent builder API
    mapper := headermapper.NewBuilder().
        AddIncomingMapping("Authorization", "authorization").
        AddBidirectionalMapping("X-Request-ID", "request-id").
        SkipPaths("/health", "/metrics").
        Build()

    // Use with grpc-gateway
    mux := headermapper.CreateGatewayMux(mapper)
    
    // Or configure manually
    mux := runtime.NewServeMux(
        runtime.WithMetadata(mapper.MetadataAnnotator()),
        runtime.WithForwardResponseOption(mapper.ResponseModifier()),
        runtime.WithIncomingHeaderMatcher(mapper.HeaderMatcher()),
    )
}
```

### Complete Server Example

```go
package main

import (
    "context"
    "log"
    "net"
    "net/http"

    "google.golang.org/grpc"
    "github.com/bhatti/grpc-header-mapper/headermapper"
)

func main() {
    // Configure header mapping
    mapper := headermapper.NewBuilder().
        // Authentication
        AddIncomingMapping("Authorization", "authorization").WithRequired(true).
        AddIncomingMapping("X-API-Key", "api-key").
        
        // Request tracking
        AddBidirectionalMapping("X-Request-ID", "request-id").
        AddBidirectionalMapping("X-Trace-ID", "trace-id").
        
        // Response headers
        AddOutgoingMapping("processing-time-ms", "X-Processing-Time").
        AddOutgoingMapping("server-version", "X-Server-Version").
        
        // Transformations
        AddIncomingMapping("Authorization", "auth-token").
        WithTransform(headermapper.ChainTransforms(
            headermapper.TrimSpace,
            headermapper.RemovePrefix("Bearer "),
        )).
        
        SkipPaths("/health", "/metrics").
        Debug(true).
        Build()

    // gRPC server with interceptors
    grpcServer := grpc.NewServer(
        grpc.UnaryInterceptor(mapper.UnaryServerInterceptor()),
        grpc.StreamInterceptor(mapper.StreamServerInterceptor()),
    )

    // HTTP gateway
    mux := headermapper.CreateGatewayMux(mapper)
    
    // Start servers...
}
```

## Configuration

### Builder Pattern (Recommended)

```go
mapper := headermapper.NewBuilder().
    // Incoming: HTTP → gRPC
    AddIncomingMapping("X-User-ID", "user-id").
    WithRequired(true).
    WithDefault("anonymous").
    
    // Outgoing: gRPC → HTTP  
    AddOutgoingMapping("response-time", "X-Response-Time").
    WithTransform(headermapper.AddPrefix("Duration: ")).
    
    // Bidirectional: Both directions
    AddBidirectionalMapping("X-Request-ID", "request-id").
    
    // Configuration options
    SkipPaths("/health", "/metrics").
    CaseSensitive(false).
    OverwriteExisting(true).
    Debug(false).
    Build()
```

### YAML Configuration

```yaml
# config.yaml
mappings:
  - http_header: "Authorization"
    grpc_metadata: "authorization"
    direction: 0  # 0=Incoming, 1=Outgoing, 2=Bidirectional
    required: true
    
  - http_header: "X-Request-ID"
    grpc_metadata: "request-id"
    direction: 2
    default_value: "generated-id"

skip_paths: ["/health", "/metrics"]
case_sensitive: false
overwrite_existing: true
debug: false
```

```go
// Load from file
config, err := headermapper.LoadConfigFromFile("config.yaml")
if err != nil {
    log.Fatal(err)
}

mapper := headermapper.NewHeaderMapper(config)
```

### Struct Configuration

```go
config := &headermapper.Config{
    Mappings: []headermapper.HeaderMapping{
        {
            HTTPHeader:   "Authorization",
            GRPCMetadata: "authorization",
            Direction:    headermapper.Incoming,
            Required:     true,
        },
        {
            HTTPHeader:   "X-Request-ID",
            GRPCMetadata: "request-id", 
            Direction:    headermapper.Bidirectional,
            DefaultValue: "auto-generated",
        },
    },
    SkipPaths: []string{"/health"},
    Debug:     true,
}

mapper := headermapper.NewHeaderMapper(config)
```

## Transformations

### Built-in Transformations

```go
// Basic transformations
headermapper.ToLower          // "HELLO" → "hello"
headermapper.ToUpper          // "hello" → "HELLO"  
headermapper.TrimSpace        // "  hello  " → "hello"
headermapper.AddPrefix("X-") // "value" → "X-value"
headermapper.RemovePrefix("Bearer ") // "Bearer token" → "token"

// Advanced transformations
headermapper.Normalize        // Trim + lowercase
headermapper.ExtractBearerToken // Extract token from "Bearer <token>"
headermapper.MaskSensitive(3) // Show first/last 3 chars: "abcdefghijk" → "abc*****ijk"
headermapper.Truncate(10)     // Limit length to 10 characters
```

### Chaining Transformations

```go
mapper := headermapper.NewBuilder().
    AddIncomingMapping("authorization", "auth-token").
    WithTransform(headermapper.ChainTransforms(
        headermapper.TrimSpace,
        headermapper.RemovePrefix("Bearer "),
        headermapper.ToLower,
        headermapper.Truncate(50),
    )).
    Build()
```

### Custom Transformations

```go
// Custom transformation function
func customTransform(value string) string {
    // Your custom logic here
    return processValue(value)
}

mapper := headermapper.NewBuilder().
    AddIncomingMapping("custom-header", "custom-metadata").
    WithTransform(customTransform).
    Build()
```

## Predefined Mappings

### Common Headers

```go
config := &headermapper.Config{
    Mappings: headermapper.CommonMappings(),
}
// Includes: User-Agent, Authorization, Content-Type, Accept, X-Request-ID, X-Correlation-ID
```

### Authentication Headers

```go
config := &headermapper.Config{
    Mappings: headermapper.AuthMappings(),
}
// Includes: Authorization (required), X-API-Key, X-User-ID
```

### Tracing Headers

```go
config := &headermapper.Config{
    Mappings: headermapper.TracingMappings(),
}
// Includes: X-Trace-ID, X-Span-ID, X-Request-ID, X-Correlation-ID
```

### Combining Mappings

```go
config := &headermapper.Config{
    Mappings: append(
        headermapper.CommonMappings(),
        append(headermapper.AuthMappings(), headermapper.TracingMappings()...)...,
    ),
}
```

## Integration

### gRPC Interceptors

```go
grpcServer := grpc.NewServer(
    grpc.UnaryInterceptor(mapper.UnaryServerInterceptor()),
    grpc.StreamInterceptor(mapper.StreamServerInterceptor()),
)
```

### Manual Gateway Setup

```go
mux := runtime.NewServeMux(
    runtime.WithIncomingHeaderMatcher(mapper.HeaderMatcher()),
    runtime.WithMetadata(mapper.MetadataAnnotator()),
    runtime.WithForwardResponseOption(mapper.ResponseModifier()),
)
```

### Custom Logger

```go
type MyLogger struct{}

func (l MyLogger) Debug(args ...interface{}) { /* implementation */ }
func (l MyLogger) Info(args ...interface{})  { /* implementation */ }
func (l MyLogger) Warn(args ...interface{})  { /* implementation */ }
func (l MyLogger) Error(args ...interface{}) { /* implementation */ }

mapper := headermapper.NewBuilder().Build()
mapper.SetLogger(MyLogger{})
```

## Examples

The project includes two comprehensive examples demonstrating different usage patterns:

### Basic Example
- **[Basic Usage](examples/basic/)** - Clean implementation with essential header mapping
- Demonstrates core functionality with clear logging
- Perfect for getting started and understanding the concepts

### Advanced Example
- **[Advanced Configuration](examples/advanced/)** - Production-ready implementation
- Includes metrics collection, custom logging, and error handling
- Shows complex transformations and configuration loading
- Demonstrates graceful shutdown and monitoring endpoints

### Additional Resources
- **[Docker Setup](examples/docker/)** - Containerized deployment examples
- **[Configuration Files](examples/config/)** - YAML/JSON configuration examples
- **[Test Scripts](scripts/)** - Automated testing and validation scripts

### Running Examples

```bash
# Run basic example with clear logging
make run-basic-example

# Test it automatically (starts server, tests, stops)
make test-basic-integration

# Run advanced example with metrics
make run-advanced-example

# Test advanced features
make test-advanced-integration
```

## Testing

### Running Tests

```bash
# Core library tests
make test           # Unit tests with race detection and coverage
make bench          # Benchmark tests for performance validation  
make coverage       # Generate HTML coverage report

# Integration testing (automated)
make test-basic-integration     # Test basic example end-to-end
make test-advanced-integration  # Test advanced example with all features
make test-integration          # Default integration test (basic)

# Manual testing (requires server running in another terminal)
make run-basic-example         # Terminal 1: Start server
make test-basic-server         # Terminal 2: Run tests
```

### Test Coverage

The library includes comprehensive test coverage:
- Unit tests for all core functionality
- Integration tests with real HTTP/gRPC communication
- Benchmark tests for performance validation
- Example server validation with automated scripts

### Writing Tests

```go
func TestHeaderMapping(t *testing.T) {
    mapper := headermapper.NewBuilder().
        AddIncomingMapping("X-User-ID", "user-id").
        Build()

    req := httptest.NewRequest("GET", "/api/test", nil)
    req.Header.Set("X-User-ID", "12345")

    md := mapper.MetadataAnnotator()(context.Background(), req)
    
    userID := md.Get("user-id")
    assert.Equal(t, "12345", userID[0])
}
```

## Monitoring & Debugging

### Debug Logging

```go
mapper := headermapper.NewBuilder().
    Debug(true).  // Enable debug logging
    Build()
```

### Statistics

```go
stats := mapper.GetStats()
fmt.Printf("Incoming mappings: %d\n", stats.IncomingMappings)
fmt.Printf("Outgoing mappings: %d\n", stats.OutgoingMappings)
fmt.Printf("Failed mappings: %d\n", stats.FailedMappings)
```

## Performance

Optimized for high-throughput production environments:

```
BenchmarkMetadataAnnotator-8    2000000    750 ns/op    240 B/op    6 allocs/op
BenchmarkHeaderMatcher-8        5000000    300 ns/op     80 B/op    2 allocs/op  
BenchmarkTransformations-8     10000000    150 ns/op     32 B/op    1 allocs/op
```

## Development

### Setup

```bash
git clone https://github.com/bhatti/grpc-header-mapper.git
cd grpc-header-mapper
make setup  # Complete development environment setup
```

### Available Commands

```bash
# Build and Development
make build          # Build the library
make examples       # Build all example binaries (basic, advanced)
make clean          # Clean build artifacts and generated proto files

# Testing
make test           # Run unit tests with coverage
make bench          # Run benchmark tests
make coverage       # Generate HTML coverage report
make lint           # Run linter (golangci-lint)
make fmt            # Format code

# Running Examples
make run-basic-example     # Run basic example server
make run-advanced-example  # Run advanced example server with metrics
make run-example          # Run basic example (default)

# Testing Examples  
make test-basic-integration     # Full automated test of basic example
make test-advanced-integration  # Full automated test of advanced example
make test-integration          # Full automated test (default: basic)

# Manual Testing (requires server running)
make test-basic-server      # Test basic server functionality
make test-advanced-server   # Test advanced server functionality

# CI/CD
make ci             # Run all CI checks (fmt, vet, lint, test)
make pre-commit     # Run pre-commit checks
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) - HTTP gateway for gRPC
- [gRPC-Go](https://github.com/grpc/grpc-go) - Go implementation of gRPC

---

**Built with ❤️ for the Go and gRPC community**