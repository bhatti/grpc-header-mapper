package headermapper

// Package headermapper provides middleware for mapping HTTP headers to gRPC metadata
// and vice versa when using grpc-gateway.
//
// This library enables seamless integration between HTTP and gRPC services by providing
// configurable, bidirectional header mapping with support for transformations,
// validation, and path-based filtering.
//
// # Basic Usage
//
//	mapper := headermapper.NewBuilder().
//		AddIncomingMapping("Authorization", "authorization").
//		AddBidirectionalMapping("X-Request-ID", "request-id").
//		Build()
//
//	mux := headermapper.CreateGatewayMux(mapper)
//
// # Features
//
//   - Bidirectional header mapping (HTTP ↔ gRPC)
//   - Configurable transformations
//   - Path-based filtering
//   - Built-in validation
//   - Debug logging support
//   - High performance design
//   - Comprehensive test coverage
//
// # Configuration
//
// The library supports both programmatic configuration via the builder pattern
// and declarative configuration via structs that can be loaded from JSON/YAML.
//
// # Transformations
//
// Header values can be transformed during mapping using built-in functions
// or custom transformation functions:
//
//	mapper := headermapper.NewBuilder().
//		AddIncomingMapping("authorization", "auth-token").
//		WithTransform(headermapper.ChainTransforms(
//			headermapper.TrimSpace,
//			headermapper.RemovePrefix("Bearer "),
//		)).
//		Build()
//
// # gRPC Integration
//
// The library provides gRPC interceptors for server-side processing:
//
//	grpcServer := grpc.NewServer(
//		grpc.UnaryInterceptor(mapper.UnaryServerInterceptor()),
//		grpc.StreamInterceptor(mapper.StreamServerInterceptor()),
//	)
