// Package headermapper provides middleware for mapping HTTP headers to gRPC metadata
// and vice versa when using grpc-gateway.
package headermapper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// MappingDirection defines the direction of header mapping
type MappingDirection int

const (
	// Incoming maps HTTP headers to gRPC metadata (HTTP -> gRPC)
	Incoming MappingDirection = iota
	// Outgoing maps gRPC metadata to HTTP headers (gRPC -> HTTP)
	Outgoing
	// Bidirectional maps in both directions
	Bidirectional
)

// TransformFunc is a function that transforms header values
type TransformFunc func(value string) string

// HeaderMapping defines how to map between HTTP headers and gRPC metadata
type HeaderMapping struct {
	// HTTPHeader is the HTTP header name (case-insensitive)
	HTTPHeader string `json:"http_header" yaml:"http_header"`
	// GRPCMetadata is the gRPC metadata key (case-sensitive)
	GRPCMetadata string `json:"grpc_metadata" yaml:"grpc_metadata"`
	// Direction specifies mapping direction
	Direction MappingDirection `json:"direction" yaml:"direction"`
	// Transform is an optional transformation function
	Transform TransformFunc `json:"-" yaml:"-"`
	// Required indicates if this header is required
	Required bool `json:"required" yaml:"required"`
	// DefaultValue is used when header is missing and Required is false
	DefaultValue string `json:"default_value" yaml:"default_value"`
}

// Config holds the configuration for header mapping
type Config struct {
	// Mappings defines the header mappings
	Mappings []HeaderMapping `json:"mappings" yaml:"mappings"`
	// SkipPaths defines paths to skip header mapping
	SkipPaths []string `json:"skip_paths" yaml:"skip_paths"`
	// CaseSensitive determines if HTTP header matching is case-sensitive
	CaseSensitive bool `json:"case_sensitive" yaml:"case_sensitive"`
	// OverwriteExisting determines if existing metadata should be overwritten
	OverwriteExisting bool `json:"overwrite_existing" yaml:"overwrite_existing"`
	// Debug enables debug logging
	Debug bool `json:"debug" yaml:"debug"`
}

// HeaderMapper provides header mapping functionality
type HeaderMapper struct {
	config    *Config
	skipPaths map[string]bool
	logger    Logger
}

// Logger interface for logging (can be implemented by any logger)
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
}

// NoOpLogger is a no-operation logger
type NoOpLogger struct{}

func (n NoOpLogger) Debug(args ...interface{}) {}
func (n NoOpLogger) Info(args ...interface{})  {}
func (n NoOpLogger) Warn(args ...interface{})  {}
func (n NoOpLogger) Error(args ...interface{}) {}

// NewHeaderMapper creates a new HeaderMapper with the given configuration
func NewHeaderMapper(config *Config) *HeaderMapper {
	if config == nil {
		config = &Config{}
	}

	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	return &HeaderMapper{
		config:    config,
		skipPaths: skipPaths,
		logger:    NoOpLogger{},
	}
}

// SetLogger sets a custom logger
func (hm *HeaderMapper) SetLogger(logger Logger) {
	hm.logger = logger
}

// MetadataAnnotator creates a metadata annotator for incoming requests
func (hm *HeaderMapper) MetadataAnnotator() func(context.Context, *http.Request) metadata.MD {
	return func(ctx context.Context, req *http.Request) metadata.MD {
		if hm.skipPaths[req.URL.Path] {
			return metadata.New(map[string]string{})
		}

		md := metadata.New(map[string]string{})

		for _, mapping := range hm.config.Mappings {
			if mapping.Direction == Outgoing {
				continue
			}

			hm.mapIncomingHeader(req, md, mapping)
		}

		if hm.config.Debug {
			hm.logger.Debug("Mapped incoming headers:", md)
		}

		return md
	}
}

// ResponseModifier creates a response modifier for outgoing responses
func (hm *HeaderMapper) ResponseModifier() func(context.Context, http.ResponseWriter, proto.Message) error {
	return func(ctx context.Context, w http.ResponseWriter, msg proto.Message) error {
		md, ok := runtime.ServerMetadataFromContext(ctx)
		if !ok {
			return nil
		}

		for _, mapping := range hm.config.Mappings {
			if mapping.Direction == Incoming {
				continue
			}

			hm.mapOutgoingHeader(md.HeaderMD, w, mapping)
		}

		if hm.config.Debug {
			hm.logger.Debug("Mapped outgoing headers to response")
		}

		return nil
	}
}

// HeaderMatcher creates a header matcher for grpc-gateway
func (hm *HeaderMapper) HeaderMatcher() func(string) (string, bool) {
	// Create a map for quick lookup
	headerMap := make(map[string]string)
	for _, mapping := range hm.config.Mappings {
		if mapping.Direction != Outgoing {
			key := mapping.HTTPHeader
			if !hm.config.CaseSensitive {
				key = strings.ToLower(key)
			}
			headerMap[key] = mapping.GRPCMetadata
		}
	}

	return func(key string) (string, bool) {
		searchKey := key
		if !hm.config.CaseSensitive {
			searchKey = strings.ToLower(key)
		}

		if grpcKey, exists := headerMap[searchKey]; exists {
			return grpcKey, true
		}

		// Fallback to default behavior
		defaultKey, defaultExists := runtime.DefaultHeaderMatcher(key)
		if !defaultExists || defaultKey == "" {
			// Manual fallback - convert to grpc-metadata format
			defaultKey = "grpc-metadata-" + strings.ToLower(strings.ReplaceAll(key, "_", "-"))
		}
		return defaultKey, true
	}
}

// UnaryServerInterceptor creates a gRPC unary server interceptor
func (hm *HeaderMapper) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if hm.skipPaths[info.FullMethod] {
			return handler(ctx, req)
		}

		// Process metadata
		newCtx := hm.processIncomingMetadata(ctx)

		return handler(newCtx, req)
	}
}

// StreamServerInterceptor creates a gRPC stream server interceptor
func (hm *HeaderMapper) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if hm.skipPaths[info.FullMethod] {
			return handler(srv, ss)
		}

		// Wrap the server stream to process metadata
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          hm.processIncomingMetadata(ss.Context()),
		}

		return handler(srv, wrappedStream)
	}
}

// mapIncomingHeader maps a single incoming HTTP header to gRPC metadata
func (hm *HeaderMapper) mapIncomingHeader(req *http.Request, md metadata.MD, mapping HeaderMapping) {
	headerValue := req.Header.Get(mapping.HTTPHeader)

	if headerValue == "" && mapping.DefaultValue != "" {
		headerValue = mapping.DefaultValue
	}

	if headerValue == "" && mapping.Required {
		hm.logger.Warn("Required header missing:", mapping.HTTPHeader)
		return
	}

	if headerValue == "" {
		return
	}

	// Apply transformation if provided
	if mapping.Transform != nil {
		headerValue = mapping.Transform(headerValue)
	}

	// Check if we should overwrite existing metadata
	if !hm.config.OverwriteExisting && len(md.Get(mapping.GRPCMetadata)) > 0 {
		return
	}

	md.Set(mapping.GRPCMetadata, headerValue)
}

// mapOutgoingHeader maps a single outgoing gRPC metadata to HTTP header
func (hm *HeaderMapper) mapOutgoingHeader(md metadata.MD, w http.ResponseWriter, mapping HeaderMapping) {
	values := md.Get(mapping.GRPCMetadata)
	if len(values) == 0 {
		if mapping.DefaultValue != "" {
			values = []string{mapping.DefaultValue}
		} else if mapping.Required {
			hm.logger.Warn("Required metadata missing:", mapping.GRPCMetadata)
			return
		} else {
			return
		}
	}

	headerValue := values[0] // Use first value

	// Apply transformation if provided
	if mapping.Transform != nil {
		headerValue = mapping.Transform(headerValue)
	}

	// Check if we should overwrite existing headers
	if !hm.config.OverwriteExisting && w.Header().Get(mapping.HTTPHeader) != "" {
		return
	}

	w.Header().Set(mapping.HTTPHeader, headerValue)
}

// processIncomingMetadata processes incoming metadata based on mappings
func (hm *HeaderMapper) processIncomingMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	newMD := metadata.New(map[string]string{})

	// Copy existing metadata
	for k, v := range md {
		newMD.Set(k, v...)
	}

	// Apply mappings that might transform metadata keys/values
	for _, mapping := range hm.config.Mappings {
		if mapping.Direction == Outgoing {
			continue
		}

		// This could include additional processing logic
		// For now, metadata is already processed by MetadataAnnotator
	}

	return metadata.NewIncomingContext(ctx, newMD)
}

// wrappedServerStream wraps a grpc.ServerStream to provide custom context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// Common transformation functions

// ToLower transforms a header value to lowercase
func ToLower(value string) string {
	return strings.ToLower(value)
}

// ToUpper transforms a header value to uppercase
func ToUpper(value string) string {
	return strings.ToUpper(value)
}

// TrimSpace trims whitespace from a header value
func TrimSpace(value string) string {
	return strings.TrimSpace(value)
}

// AddPrefix adds a prefix to a header value
func AddPrefix(prefix string) TransformFunc {
	return func(value string) string {
		return prefix + value
	}
}

// RemovePrefix removes a prefix from a header value
func RemovePrefix(prefix string) TransformFunc {
	return func(value string) string {
		return strings.TrimPrefix(value, prefix)
	}
}

// ChainTransforms chains multiple transformation functions
func ChainTransforms(transforms ...TransformFunc) TransformFunc {
	return func(value string) string {
		result := value
		for _, transform := range transforms {
			if transform != nil {
				result = transform(result)
			}
		}
		return result
	}
}

// Builder provides a fluent API for creating HeaderMapper configurations

// Builder helps build HeaderMapper configurations
type Builder struct {
	config *Config
}

// NewBuilder creates a new configuration builder
func NewBuilder() *Builder {
	return &Builder{
		config: &Config{
			Mappings: make([]HeaderMapping, 0),
		},
	}
}

// AddMapping adds a header mapping
func (b *Builder) AddMapping(httpHeader, grpcMetadata string, direction MappingDirection) *Builder {
	b.config.Mappings = append(b.config.Mappings, HeaderMapping{
		HTTPHeader:   httpHeader,
		GRPCMetadata: grpcMetadata,
		Direction:    direction,
	})
	return b
}

// AddIncomingMapping adds an incoming header mapping (HTTP -> gRPC)
func (b *Builder) AddIncomingMapping(httpHeader, grpcMetadata string) *Builder {
	return b.AddMapping(httpHeader, grpcMetadata, Incoming)
}

// AddOutgoingMapping adds an outgoing header mapping (gRPC -> HTTP)
func (b *Builder) AddOutgoingMapping(grpcMetadata, httpHeader string) *Builder {
	return b.AddMapping(httpHeader, grpcMetadata, Outgoing)
}

// AddBidirectionalMapping adds a bidirectional header mapping
func (b *Builder) AddBidirectionalMapping(httpHeader, grpcMetadata string) *Builder {
	return b.AddMapping(httpHeader, grpcMetadata, Bidirectional)
}

// WithTransform sets a transformation function for the last added mapping
func (b *Builder) WithTransform(transform TransformFunc) *Builder {
	if len(b.config.Mappings) > 0 {
		b.config.Mappings[len(b.config.Mappings)-1].Transform = transform
	}
	return b
}

// WithRequired marks the last added mapping as required
func (b *Builder) WithRequired(required bool) *Builder {
	if len(b.config.Mappings) > 0 {
		b.config.Mappings[len(b.config.Mappings)-1].Required = required
	}
	return b
}

// WithDefault sets a default value for the last added mapping
func (b *Builder) WithDefault(defaultValue string) *Builder {
	if len(b.config.Mappings) > 0 {
		b.config.Mappings[len(b.config.Mappings)-1].DefaultValue = defaultValue
	}
	return b
}

// SkipPaths sets paths to skip header mapping
func (b *Builder) SkipPaths(paths ...string) *Builder {
	b.config.SkipPaths = paths
	return b
}

// CaseSensitive sets case sensitivity for header matching
func (b *Builder) CaseSensitive(caseSensitive bool) *Builder {
	b.config.CaseSensitive = caseSensitive
	return b
}

// OverwriteExisting sets whether to overwrite existing headers/metadata
func (b *Builder) OverwriteExisting(overwrite bool) *Builder {
	b.config.OverwriteExisting = overwrite
	return b
}

// Debug enables debug logging
func (b *Builder) Debug(debug bool) *Builder {
	b.config.Debug = debug
	return b
}

// Build creates the HeaderMapper
func (b *Builder) Build() *HeaderMapper {
	return NewHeaderMapper(b.config)
}

// Predefined common mappings

// CommonMappings returns commonly used header mappings
func CommonMappings() []HeaderMapping {
	return []HeaderMapping{
		{
			HTTPHeader:   "User-Agent",
			GRPCMetadata: "user-agent",
			Direction:    Incoming,
		},
		{
			HTTPHeader:   "Authorization",
			GRPCMetadata: "authorization",
			Direction:    Incoming,
		},
		{
			HTTPHeader:   "Content-Type",
			GRPCMetadata: "content-type",
			Direction:    Bidirectional,
		},
		{
			HTTPHeader:   "Accept",
			GRPCMetadata: "accept",
			Direction:    Incoming,
		},
		{
			HTTPHeader:   "X-Request-ID",
			GRPCMetadata: "x-request-id",
			Direction:    Bidirectional,
		},
		{
			HTTPHeader:   "X-Correlation-ID",
			GRPCMetadata: "x-correlation-id",
			Direction:    Bidirectional,
		},
	}
}

// AuthMappings returns authentication-related header mappings
func AuthMappings() []HeaderMapping {
	return []HeaderMapping{
		{
			HTTPHeader:   "Authorization",
			GRPCMetadata: "authorization",
			Direction:    Incoming,
			Required:     true,
		},
		{
			HTTPHeader:   "X-API-Key",
			GRPCMetadata: "x-api-key",
			Direction:    Incoming,
		},
		{
			HTTPHeader:   "X-User-ID",
			GRPCMetadata: "x-user-id",
			Direction:    Bidirectional,
		},
	}
}

// TracingMappings returns tracing-related header mappings
func TracingMappings() []HeaderMapping {
	return []HeaderMapping{
		{
			HTTPHeader:   "X-Trace-ID",
			GRPCMetadata: "x-trace-id",
			Direction:    Bidirectional,
		},
		{
			HTTPHeader:   "X-Span-ID",
			GRPCMetadata: "x-span-id",
			Direction:    Bidirectional,
		},
		{
			HTTPHeader:   "X-Request-ID",
			GRPCMetadata: "x-request-id",
			Direction:    Bidirectional,
		},
		{
			HTTPHeader:   "X-Correlation-ID",
			GRPCMetadata: "x-correlation-id",
			Direction:    Bidirectional,
		},
	}
}

// Helper functions for common use cases

// CreateGatewayMux creates a new gRPC gateway ServeMux with header mapping
func CreateGatewayMux(mapper *HeaderMapper, opts ...runtime.ServeMuxOption) *runtime.ServeMux {
	// Prepend our options
	allOpts := []runtime.ServeMuxOption{
		runtime.WithIncomingHeaderMatcher(mapper.HeaderMatcher()),
		runtime.WithMetadata(mapper.MetadataAnnotator()),
		runtime.WithForwardResponseOption(mapper.ResponseModifier()),
	}

	// Add user-provided options
	allOpts = append(allOpts, opts...)

	return runtime.NewServeMux(allOpts...)
}

// Validation functions

// Validate validates the header mapper configuration
func (hm *HeaderMapper) Validate() error {
	if hm.config == nil {
		return fmt.Errorf("configuration is nil")
	}

	for i, mapping := range hm.config.Mappings {
		if mapping.HTTPHeader == "" {
			return fmt.Errorf("mapping %d: HTTPHeader cannot be empty", i)
		}
		if mapping.GRPCMetadata == "" {
			return fmt.Errorf("mapping %d: GRPCMetadata cannot be empty", i)
		}
	}

	return nil
}

// Stats provides statistics about header mapping operations
type Stats struct {
	IncomingMappings int64
	OutgoingMappings int64
	FailedMappings   int64
	LastUpdated      time.Time
}

// GetStats returns statistics about the header mapper (placeholder for future implementation)
func (hm *HeaderMapper) GetStats() *Stats {
	return &Stats{
		LastUpdated: time.Now(),
	}
}
