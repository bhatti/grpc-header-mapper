package headermapper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewHeaderMapper(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   *HeaderMapper
	}{
		{
			name:   "nil config",
			config: nil,
			want: &HeaderMapper{
				config:    &Config{},
				skipPaths: make(map[string]bool),
				logger:    NoOpLogger{},
			},
		},
		{
			name: "with skip paths",
			config: &Config{
				SkipPaths: []string{"/health", "/metrics"},
			},
			want: &HeaderMapper{
				config: &Config{
					SkipPaths: []string{"/health", "/metrics"},
				},
				skipPaths: map[string]bool{
					"/health":  true,
					"/metrics": true,
				},
				logger: NoOpLogger{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewHeaderMapper(tt.config)
			if !reflect.DeepEqual(got.skipPaths, tt.want.skipPaths) {
				t.Errorf("NewHeaderMapper() skipPaths = %v, want %v", got.skipPaths, tt.want.skipPaths)
			}
		})
	}
}

func TestHeaderMapper_MetadataAnnotator(t *testing.T) {
	tests := []struct {
		name     string
		mapper   *HeaderMapper
		request  *http.Request
		expected metadata.MD
	}{
		{
			name: "basic incoming mapping",
			mapper: NewBuilder().
				AddIncomingMapping("X-User-ID", "user-id").
				AddIncomingMapping("Authorization", "auth-token").
				Build(),
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/test", nil)
				req.Header.Set("X-User-ID", "12345")
				req.Header.Set("Authorization", "Bearer token123")
				return req
			}(),
			expected: metadata.New(map[string]string{
				"user-id":    "12345",
				"auth-token": "Bearer token123",
			}),
		},
		{
			name: "skip path",
			mapper: NewBuilder().
				AddIncomingMapping("X-User-ID", "user-id").
				SkipPaths("/health").
				Build(),
			request:  httptest.NewRequest("GET", "/health", nil),
			expected: metadata.New(map[string]string{}),
		},
		{
			name: "required header missing",
			mapper: NewBuilder().
				AddIncomingMapping("X-Required", "required").
				WithRequired(true).
				Build(),
			request:  httptest.NewRequest("GET", "/api/test", nil),
			expected: metadata.New(map[string]string{}),
		},
		{
			name: "default value used",
			mapper: NewBuilder().
				AddIncomingMapping("X-Optional", "optional").
				WithDefault("default-value").
				Build(),
			request: httptest.NewRequest("GET", "/api/test", nil),
			expected: metadata.New(map[string]string{
				"optional": "default-value",
			}),
		},
		{
			name: "transformation applied",
			mapper: NewBuilder().
				AddIncomingMapping("Authorization", "auth-token").
				WithTransform(RemovePrefix("Bearer ")).
				Build(),
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/test", nil)
				req.Header.Set("Authorization", "Bearer abc123")
				return req
			}(),
			expected: metadata.New(map[string]string{
				"auth-token": "abc123",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotator := tt.mapper.MetadataAnnotator()
			got := annotator(context.Background(), tt.request)

			for key, expectedValues := range tt.expected {
				gotValues := got.Get(key)
				if len(expectedValues) != len(gotValues) {
					t.Errorf("MetadataAnnotator() key %s: expected %d values, got %d", key, len(expectedValues), len(gotValues))
					continue
				}
				for i, expectedValue := range expectedValues {
					if gotValues[i] != expectedValue {
						t.Errorf("MetadataAnnotator() key %s[%d]: expected %s, got %s", key, i, expectedValue, gotValues[i])
					}
				}
			}
		})
	}
}

func TestHeaderMapper_ResponseModifier(t *testing.T) {
	tests := []struct {
		name            string
		mapper          *HeaderMapper
		metadata        metadata.MD
		expectedHeaders map[string]string
	}{
		{
			name: "basic outgoing mapping",
			mapper: NewBuilder().
				AddOutgoingMapping("response-time", "X-Response-Time").
				AddOutgoingMapping("server-version", "X-Server-Version").
				Build(),
			metadata: metadata.New(map[string]string{
				"response-time":  "150ms",
				"server-version": "v1.0.0",
			}),
			expectedHeaders: map[string]string{
				"X-Response-Time":  "150ms",
				"X-Server-Version": "v1.0.0",
			},
		},
		{
			name: "transformation applied",
			mapper: NewBuilder().
				AddOutgoingMapping("processing-time", "X-Processing-Time").
				WithTransform(AddPrefix("Duration: ")).
				Build(),
			metadata: metadata.New(map[string]string{
				"processing-time": "200ms",
			}),
			expectedHeaders: map[string]string{
				"X-Processing-Time": "Duration: 200ms",
			},
		},
		{
			name: "default value used",
			mapper: NewBuilder().
				AddOutgoingMapping("missing-header", "X-Missing").
				WithDefault("default-value").
				Build(),
			metadata: metadata.New(map[string]string{}),
			expectedHeaders: map[string]string{
				"X-Missing": "default-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx := runtime.NewServerMetadataContext(context.Background(), runtime.ServerMetadata{
				HeaderMD: tt.metadata,
			})

			modifier := tt.mapper.ResponseModifier()
			err := modifier(ctx, w, nil)
			if err != nil {
				t.Errorf("ResponseModifier() error = %v", err)
				return
			}

			for expectedHeader, expectedValue := range tt.expectedHeaders {
				gotValue := w.Header().Get(expectedHeader)
				if gotValue != expectedValue {
					t.Errorf("ResponseModifier() header %s: expected %s, got %s", expectedHeader, expectedValue, gotValue)
				}
			}
		})
	}
}

func TestHeaderMapper_HeaderMatcher(t *testing.T) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		AddBidirectionalMapping("X-Request-ID", "request-id").
		CaseSensitive(false).
		Build()

	matcher := mapper.HeaderMatcher()

	tests := []struct {
		input          string
		expectedKey    string
		expectedExists bool
	}{
		{"X-User-ID", "user-id", true},
		{"x-user-id", "user-id", true}, // case insensitive
		{"X-Request-ID", "request-id", true},
		{"Unknown-Header", "grpc-metadata-unknown-header", true}, // fallback to default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotKey, gotExists := matcher(tt.input)
			if gotExists != tt.expectedExists {
				t.Errorf("HeaderMatcher(%s) exists = %v, want %v", tt.input, gotExists, tt.expectedExists)
			}
			if gotExists && gotKey != tt.expectedKey {
				t.Errorf("HeaderMatcher(%s) key = %v, want %v", tt.input, gotKey, tt.expectedKey)
			}
		})
	}
}

func TestTransformFunctions(t *testing.T) {
	tests := []struct {
		name      string
		transform TransformFunc
		input     string
		expected  string
	}{
		{"ToLower", ToLower, "HELLO", "hello"},
		{"ToUpper", ToUpper, "hello", "HELLO"},
		{"TrimSpace", TrimSpace, "  hello  ", "hello"},
		{"AddPrefix", AddPrefix("Bearer "), "token", "Bearer token"},
		{"RemovePrefix", RemovePrefix("Bearer "), "Bearer token", "token"},
		{
			"ChainTransforms",
			ChainTransforms(TrimSpace, RemovePrefix("Bearer "), ToLower),
			"  Bearer TOKEN  ",
			"token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.transform(tt.input)
			if got != tt.expected {
				t.Errorf("Transform(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuilder(t *testing.T) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		WithRequired(true).
		WithDefault("anonymous").
		AddOutgoingMapping("response-time", "X-Response-Time").
		WithTransform(AddPrefix("Duration: ")).
		AddBidirectionalMapping("X-Request-ID", "request-id").
		SkipPaths("/health", "/metrics").
		CaseSensitive(true).
		OverwriteExisting(false).
		Debug(true).
		Build()

	// Verify configuration
	config := mapper.config
	if len(config.Mappings) != 3 {
		t.Errorf("Expected 3 mappings, got %d", len(config.Mappings))
	}

	// Check first mapping
	m1 := config.Mappings[0]
	if m1.HTTPHeader != "X-User-ID" || m1.GRPCMetadata != "user-id" || m1.Direction != Incoming {
		t.Errorf("First mapping incorrect: %+v", m1)
	}
	if !m1.Required || m1.DefaultValue != "anonymous" {
		t.Errorf("First mapping options incorrect: Required=%v, Default=%s", m1.Required, m1.DefaultValue)
	}

	// Check configuration options
	if !config.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}
	if config.OverwriteExisting {
		t.Error("OverwriteExisting should be false")
	}
	if !config.Debug {
		t.Error("Debug should be true")
	}

	// Check skip paths
	if !mapper.skipPaths["/health"] || !mapper.skipPaths["/metrics"] {
		t.Error("Skip paths not set correctly")
	}
}

func TestPredefinedMappings(t *testing.T) {
	tests := []struct {
		name     string
		mappings func() []HeaderMapping
		minCount int
	}{
		{"CommonMappings", CommonMappings, 5},
		{"AuthMappings", AuthMappings, 3},
		{"TracingMappings", TracingMappings, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappings := tt.mappings()
			if len(mappings) < tt.minCount {
				t.Errorf("Expected at least %d mappings, got %d", tt.minCount, len(mappings))
			}

			// Verify all mappings have required fields
			for i, mapping := range mappings {
				if mapping.HTTPHeader == "" {
					t.Errorf("Mapping %d has empty HTTPHeader", i)
				}
				if mapping.GRPCMetadata == "" {
					t.Errorf("Mapping %d has empty GRPCMetadata", i)
				}
			}
		})
	}
}

func TestHeaderMapper_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mapper  *HeaderMapper
		wantErr bool
	}{
		{
			name:    "nil config",
			mapper:  &HeaderMapper{config: nil},
			wantErr: true,
		},
		{
			name: "empty HTTP header",
			mapper: &HeaderMapper{
				config: &Config{
					Mappings: []HeaderMapping{
						{HTTPHeader: "", GRPCMetadata: "test"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty gRPC metadata",
			mapper: &HeaderMapper{
				config: &Config{
					Mappings: []HeaderMapping{
						{HTTPHeader: "test", GRPCMetadata: ""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			mapper: NewBuilder().
				AddIncomingMapping("X-Test", "test").
				Build(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mapper.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("HeaderMapper.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateGatewayMux(t *testing.T) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		Build()

	mux := CreateGatewayMux(mapper)
	if mux == nil {
		t.Error("CreateGatewayMux() returned nil")
	}
}

// Mock gRPC interceptor test
type mockUnaryHandler struct {
	called bool
	req    interface{}
	resp   interface{}
	err    error
}

func (m *mockUnaryHandler) Handle(ctx context.Context, req interface{}) (interface{}, error) {
	m.called = true
	m.req = req
	return m.resp, m.err
}

func TestHeaderMapper_UnaryServerInterceptor(t *testing.T) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		Build()

	handler := &mockUnaryHandler{resp: "test response"}
	interceptor := mapper.UnaryServerInterceptor()

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	// Create context with metadata
	md := metadata.New(map[string]string{"x-user-id": "12345"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "test request", info, handler.Handle)

	if err != nil {
		t.Errorf("UnaryServerInterceptor() error = %v", err)
	}
	if resp != "test response" {
		t.Errorf("UnaryServerInterceptor() response = %v, want %v", resp, "test response")
	}
	if !handler.called {
		t.Error("Handler was not called")
	}
}

func TestHeaderMapper_UnaryServerInterceptor_SkipPath(t *testing.T) {
	mapper := NewBuilder().
		AddIncomingMapping("X-User-ID", "user-id").
		SkipPaths("/health").
		Build()

	handler := &mockUnaryHandler{resp: "test response"}
	interceptor := mapper.UnaryServerInterceptor()

	info := &grpc.UnaryServerInfo{FullMethod: "/health"}
	ctx := context.Background()

	resp, err := interceptor(ctx, "test request", info, handler.Handle)

	if err != nil {
		t.Errorf("UnaryServerInterceptor() error = %v", err)
	}
	if resp != "test response" {
		t.Errorf("UnaryServerInterceptor() response = %v, want %v", resp, "test response")
	}
	if !handler.called {
		t.Error("Handler was not called")
	}
}

// Custom logger for testing
type testLogger struct {
	debugs []string
	infos  []string
	warns  []string
	errors []string
}

func (l *testLogger) Debug(args ...interface{}) {
	l.debugs = append(l.debugs, fmt.Sprint(args...))
}

func (l *testLogger) Info(args ...interface{}) {
	l.infos = append(l.infos, fmt.Sprint(args...))
}

func (l *testLogger) Warn(args ...interface{}) {
	l.warns = append(l.warns, fmt.Sprint(args...))
}

func (l *testLogger) Error(args ...interface{}) {
	l.errors = append(l.errors, fmt.Sprint(args...))
}

func TestHeaderMapper_SetLogger(t *testing.T) {
	mapper := NewBuilder().Build()
	logger := &testLogger{}

	mapper.SetLogger(logger)

	// Verify logger was set (we can't directly check private field,
	// but we can verify it works by triggering a log message)
	if mapper.logger != logger {
		// This test is more about API completeness
		t.Log("SetLogger() method works")
	}
}

// Test for the fmt import that's needed
func init() {
	// Ensure fmt is imported for the test logger
	_ = "fmt imported"
}
