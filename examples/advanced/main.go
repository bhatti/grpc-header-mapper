package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/bhatti/grpc-header-mapper/headermapper"
	pb "github.com/bhatti/grpc-header-mapper/test/testdata/proto"
)

// AdvancedLogger implements structured logging with different levels
type AdvancedLogger struct {
	prefix string
}

func NewAdvancedLogger(prefix string) *AdvancedLogger {
	return &AdvancedLogger{prefix: prefix}
}

func (l *AdvancedLogger) Debug(args ...interface{}) {
	log.Printf("[DEBUG] [%s] %v", l.prefix, fmt.Sprint(args...))
}

func (l *AdvancedLogger) Info(args ...interface{}) {
	log.Printf("[INFO] [%s] %v", l.prefix, fmt.Sprint(args...))
}

func (l *AdvancedLogger) Warn(args ...interface{}) {
	log.Printf("[WARN] [%s] %v", l.prefix, fmt.Sprint(args...))
}

func (l *AdvancedLogger) Error(args ...interface{}) {
	log.Printf("[ERROR] [%s] %v", l.prefix, fmt.Sprint(args...))
}

// MetricsCollector collects header mapping metrics
type MetricsCollector struct {
	incomingHeaders map[string]int64
	outgoingHeaders map[string]int64
	errors          int64
	mutex           sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		incomingHeaders: make(map[string]int64),
		outgoingHeaders: make(map[string]int64),
	}
}

func (m *MetricsCollector) IncrementIncoming(header string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.incomingHeaders[header]++
}

func (m *MetricsCollector) IncrementOutgoing(header string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.outgoingHeaders[header]++
}

func (m *MetricsCollector) IncrementErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errors++
}

func (m *MetricsCollector) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"incoming_headers": m.incomingHeaders,
		"outgoing_headers": m.outgoingHeaders,
		"errors":           m.errors,
	}
}

// AdvancedServer implements the test service with enhanced header processing
type AdvancedServer struct {
	pb.UnimplementedTestServiceServer
	metrics *MetricsCollector
	logger  *AdvancedLogger
}

func NewAdvancedServer() *AdvancedServer {
	return &AdvancedServer{
		metrics: NewMetricsCollector(),
		logger:  NewAdvancedLogger("AdvancedServer"),
	}
}

func (s *AdvancedServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	startTime := time.Now()

	// Extract and log incoming metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		s.logger.Warn("No metadata found in context")
	}

	// Process headers and collect metrics
	headers := make(map[string]string)
	for key, values := range md {
		if len(values) > 0 {
			headers[key] = values[0]
			s.metrics.IncrementIncoming(key)
			s.logger.Debug("Incoming header:", key, "=", values[0])
		}
	}

	// Add processing time and server info to outgoing metadata
	processingTime := time.Since(startTime)

	// Create outgoing metadata
	outgoingMD := metadata.New(map[string]string{
		"processing-time-ms":   strconv.FormatInt(processingTime.Milliseconds(), 10),
		"server-version":       "v2.0.0-advanced",
		"rate-limit-remaining": "99",
		"response-timestamp":   strconv.FormatInt(time.Now().Unix(), 10),
	})

	// Track outgoing headers
	for key := range outgoingMD {
		s.metrics.IncrementOutgoing(key)
	}

	// Send outgoing metadata
	if err := grpc.SendHeader(ctx, outgoingMD); err != nil {
		s.logger.Error("Failed to send header:", err)
		s.metrics.IncrementErrors()
	}

	// Validate required headers
	if authToken := headers["authorization"]; authToken == "" {
		s.logger.Warn("Missing authorization header")
	} else {
		s.logger.Info("Authorized request from token:", authToken[:min(10, len(authToken))]+"...")
	}

	// Check for user identification
	userID := headers["user-id"]
	if userID == "" {
		userID = "anonymous"
	}

	response := &pb.EchoResponse{
		Message:   fmt.Sprintf("Advanced Echo: %s (from user: %s)", req.Message, userID),
		Headers:   headers,
		Timestamp: time.Now().Unix(),
	}

	s.logger.Info("Processed echo request in", processingTime)
	return response, nil
}

func (s *AdvancedServer) GetMetrics() map[string]interface{} {
	return s.metrics.GetStats()
}

// Custom transformations for advanced example
func advancedBearerTokenExtractor(value string) string {
	// Extract token and validate format
	token := headermapper.RemovePrefix("Bearer ")(headermapper.TrimSpace(value))
	if len(token) < 10 {
		return "invalid-token"
	}
	return token
}

func userAgentSanitizer(value string) string {
	// Sanitize user agent to remove version numbers but keep basic info
	return headermapper.RegexReplace(`\d+\.\d+(\.\d+)*`, "x.x.x")(value)
}

func timestampGenerator(value string) string {
	// Generate current timestamp if value is empty
	if headermapper.TrimSpace(value) == "" {
		return strconv.FormatInt(time.Now().Unix(), 10)
	}
	return value
}

func createAdvancedMapper() *headermapper.HeaderMapper {
	// Load configuration from environment or use defaults
	configPath := os.Getenv("HEADER_MAPPER_CONFIG")

	var mapper *headermapper.HeaderMapper

	if configPath != "" {
		// Load from configuration file
		config, err := headermapper.LoadConfigFromFile(configPath)
		if err != nil {
			log.Printf("Failed to load config from %s: %v, using default configuration", configPath, err)
			mapper = createDefaultAdvancedMapper()
		} else {
			log.Printf("Loaded configuration from %s", configPath)
			mapper = headermapper.NewHeaderMapper(config)
		}
	} else {
		mapper = createDefaultAdvancedMapper()
	}

	// Set advanced logger
	mapper.SetLogger(NewAdvancedLogger("HeaderMapper"))

	return mapper
}

func createDefaultAdvancedMapper() *headermapper.HeaderMapper {
	return headermapper.NewBuilder().
		// Authentication with advanced transformations
		AddIncomingMapping("Authorization", "authorization").WithRequired(false).
		AddIncomingMapping("authorization", "auth-token").
		WithTransform(advancedBearerTokenExtractor).

		// API Key with validation
		AddIncomingMapping("X-API-Key", "api-key").
		WithTransform(headermapper.ChainTransforms(
			headermapper.TrimSpace,
			headermapper.DefaultIfEmpty("missing-api-key"),
		)).

		// User identification
		AddIncomingMapping("X-User-ID", "user-id").
		WithDefault("anonymous").
		AddIncomingMapping("X-User-Role", "user-role").
		WithDefault("guest").

		// Request tracking (bidirectional)
		AddBidirectionalMapping("X-Request-ID", "request-id").
		WithTransform(headermapper.DefaultIfEmpty("auto-generated-"+strconv.FormatInt(time.Now().UnixNano(), 10))).
		AddBidirectionalMapping("X-Correlation-ID", "correlation-id").
		AddBidirectionalMapping("X-Trace-ID", "trace-id").
		AddBidirectionalMapping("X-Span-ID", "span-id").

		// Client information
		AddIncomingMapping("User-Agent", "user-agent").
		WithTransform(userAgentSanitizer).
		AddIncomingMapping("X-Client-Version", "client-version").
		AddIncomingMapping("X-Device-ID", "device-id").

		// Content negotiation
		AddBidirectionalMapping("Content-Type", "content-type").
		WithDefault("application/json").
		AddIncomingMapping("Accept", "accept").
		AddIncomingMapping("Accept-Language", "accept-language").
		AddIncomingMapping("Accept-Encoding", "accept-encoding").

		// Response headers with transformations
		AddOutgoingMapping("processing-time-ms", "X-Processing-Time").
		WithTransform(headermapper.AddSuffix("ms")).
		AddOutgoingMapping("server-version", "X-Server-Version").
		WithDefault("v2.0.0").
		AddOutgoingMapping("rate-limit-remaining", "X-RateLimit-Remaining").
		AddOutgoingMapping("response-timestamp", "X-Response-Timestamp").
		WithTransform(timestampGenerator).

		// Security headers
		AddOutgoingMapping("security-policy", "X-Content-Security-Policy").
		WithDefault("default-src 'self'").
		AddOutgoingMapping("frame-options", "X-Frame-Options").
		WithDefault("DENY").

		// Custom business headers
		AddBidirectionalMapping("X-Tenant-ID", "tenant-id").
		AddBidirectionalMapping("X-Region", "region").
		AddOutgoingMapping("cache-control-value", "Cache-Control").
		WithDefault("no-cache").

		// Skip paths for health checks and admin endpoints
		SkipPaths("/health", "/ready", "/metrics", "/admin", "/debug").

		// Enable debug mode in development
		Debug(os.Getenv("DEBUG") == "true").

		// Allow overwriting existing headers
		OverwriteExisting(true).
		Build()
}

// createAdvancedInterceptor adds additional processing beyond header mapping
func createAdvancedInterceptor(server *AdvancedServer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Add request context information
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := getFirstValue(md, "request-id")
		userID := getFirstValue(md, "user-id")

		server.logger.Info("Processing request", info.FullMethod, "for user", userID, "with request-id", requestID)

		// Call the handler
		resp, err := handler(ctx, req)

		// Log the result
		duration := time.Since(startTime)
		if err != nil {
			server.logger.Error("Request failed:", err, "duration:", duration)
			server.metrics.IncrementErrors()
		} else {
			server.logger.Info("Request completed successfully, duration:", duration)
		}

		return resp, err
	}
}

func getFirstValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// setupMetricsEndpoint adds a metrics endpoint to the HTTP server
func setupMetricsEndpoint(mux *runtime.ServeMux, server *AdvancedServer) {
	mux.HandlePath("GET", "/metrics", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")

		metrics := server.GetMetrics()
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
			return
		}
	})

	// Add advanced health check with header validation
	mux.HandlePath("GET", "/health/advanced", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")

		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "v2.0.0-advanced",
			"uptime":    time.Since(startTime).Seconds(),
		}

		// Check if required headers are present for health
		if auth := r.Header.Get("Authorization"); auth != "" {
			health["authenticated"] = true
		}

		json.NewEncoder(w).Encode(health)
	})
}

var startTime = time.Now()

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Advanced gRPC Header Mapper Example Server...")

	// Create advanced server
	server := NewAdvancedServer()

	// Create sophisticated header mapper
	mapper := createAdvancedMapper()

	// Validate configuration
	if err := mapper.Validate(); err != nil {
		log.Fatalf("Invalid header mapper configuration: %v", err)
	}

	server.logger.Info("Header mapper configuration validated successfully")

	// Create gRPC server with multiple interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			mapper.UnaryServerInterceptor(),   // Header mapping
			createAdvancedInterceptor(server), // Advanced logging and metrics
		),
		grpc.ChainStreamInterceptor(
			mapper.StreamServerInterceptor(),
		),
	)

	// Register services
	pb.RegisterTestServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	server.logger.Info("gRPC services registered")

	// Set up HTTP gateway with advanced configuration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create gateway with custom options
	mux := headermapper.CreateGatewayMux(mapper,
		runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
			server.logger.Error("Gateway error:", err)
			server.metrics.IncrementErrors()

			// Custom error response
			w.Header().Set("Content-Type", "application/json")

			st := status.Convert(err)
			statusCode := runtime.HTTPStatusFromCode(st.Code())
			w.WriteHeader(statusCode)

			errorResp := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    st.Code(),
					"message": st.Message(),
					"details": st.Proto().GetDetails(),
				},
				"timestamp": time.Now().Unix(),
				"path":      r.URL.Path,
			}

			json.NewEncoder(w).Encode(errorResp)
		}),
	)

	// Connect to gRPC server
	conn, err := grpc.DialContext(
		ctx,
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to dial gRPC server: %v", err)
	}
	defer conn.Close()

	// Register gateway handler
	if err := pb.RegisterTestServiceHandler(ctx, mux, conn); err != nil {
		log.Fatalf("Failed to register gateway: %v", err)
	}

	server.logger.Info("HTTP gateway registered")

	// Setup additional endpoints
	setupMetricsEndpoint(mux, server)

	// Start servers with graceful shutdown
	var wg sync.WaitGroup
	wg.Add(2)

	// Start gRPC server
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", ":9090")
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}

		server.logger.Info("gRPC server listening on :9090")
		if err := grpcServer.Serve(lis); err != nil {
			server.logger.Error("gRPC server error:", err)
		}
	}()

	// Start HTTP server with advanced configuration
	go func() {
		defer wg.Done()

		httpServer := &http.Server{
			Addr:    ":8080",
			Handler: mux,
			// Add timeouts for production use
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		server.logger.Info("HTTP gateway listening on :8080")
		server.logger.Info("Advanced endpoints available:")
		server.logger.Info("  GET  /metrics - Server metrics")
		server.logger.Info("  GET  /health/advanced - Advanced health check")
		server.logger.Info("  POST /v1/echo - Echo service with header mapping")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.logger.Error("HTTP server error:", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c

	server.logger.Info("Received signal:", sig, "- initiating graceful shutdown...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	server.logger.Info("Stopping gRPC server...")
	grpcServer.GracefulStop()

	server.logger.Info("Stopping HTTP server...")
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		server.logger.Info("All servers stopped gracefully")
	case <-shutdownCtx.Done():
		server.logger.Error("Shutdown timeout exceeded, forcing exit")
	}

	// Print final metrics
	finalMetrics := server.GetMetrics()
	server.logger.Info("Final metrics:", finalMetrics)
}
