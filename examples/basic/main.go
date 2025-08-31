package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	"github.com/bhatti/grpc-header-mapper/headermapper"
	pb "github.com/bhatti/grpc-header-mapper/test/testdata/proto"
)

// BasicLogger provides simple logging with header visibility
type BasicLogger struct {
	prefix string
}

func NewBasicLogger(prefix string) *BasicLogger {
	return &BasicLogger{prefix: prefix}
}

func (l *BasicLogger) Debug(args ...interface{}) {
	log.Printf("[DEBUG] [%s] %v", l.prefix, args)
}

func (l *BasicLogger) Info(args ...interface{}) {
	log.Printf("[INFO] [%s] %v", l.prefix, args)
}

func (l *BasicLogger) Warn(args ...interface{}) {
	log.Printf("[WARN] [%s] %v", l.prefix, args)
}

func (l *BasicLogger) Error(args ...interface{}) {
	log.Printf("[ERROR] [%s] %v", l.prefix, args)
}

// BasicServer implements the test service with header logging
type BasicServer struct {
	pb.UnimplementedTestServiceServer
	logger *BasicLogger
}

func NewBasicServer() *BasicServer {
	return &BasicServer{
		logger: NewBasicLogger("BasicServer"),
	}
}

func (s *BasicServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.logger.Info("=== Processing Echo Request ===")

	// Extract headers from metadata for demonstration
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		s.logger.Warn("No incoming metadata found")
		md = metadata.New(map[string]string{})
	}

	s.logger.Info("📥 INCOMING HEADERS (mapped to gRPC metadata):")
	headers := make(map[string]string)
	for key, values := range md {
		if len(values) > 0 {
			headers[key] = values[0]
			s.logger.Info("  ✅ %s: %s", key, values[0])
		}
	}

	if len(headers) == 0 {
		s.logger.Info("  (no mapped headers found)")
	}

	// Create some outgoing metadata to test response header mapping
	outgoingMD := metadata.New(map[string]string{
		"processing-time-ms":   "42",
		"server-version":       "v1.0.0-basic",
		"rate-limit-remaining": "100",
	})

	s.logger.Info("📤 OUTGOING HEADERS (will be mapped to HTTP response headers):")
	for key, values := range outgoingMD {
		if len(values) > 0 {
			s.logger.Info("  ✅ %s: %s", key, values[0])
		}
	}

	// Send outgoing headers
	if err := grpc.SendHeader(ctx, outgoingMD); err != nil {
		s.logger.Error("Failed to send outgoing headers:", err)
	}

	response := &pb.EchoResponse{
		Message:   "Basic Echo: " + req.Message,
		Headers:   headers,
		Timestamp: 1234567890, // Simple timestamp
	}

	s.logger.Info("✅ Echo request processed successfully")
	return response, nil
}

func createBasicMapper() *headermapper.HeaderMapper {
	logger := NewBasicLogger("HeaderMapper")

	mapper := headermapper.NewBuilder().
		// Authentication headers
		AddIncomingMapping("Authorization", "authorization").WithRequired(false).
		AddIncomingMapping("X-API-Key", "api-key").
		AddIncomingMapping("X-User-ID", "user-id").

		// Request tracking (bidirectional)
		AddBidirectionalMapping("X-Request-ID", "request-id").
		AddBidirectionalMapping("X-Correlation-ID", "correlation-id").
		AddBidirectionalMapping("X-Trace-ID", "trace-id").

		// Response headers
		AddOutgoingMapping("processing-time-ms", "X-Processing-Time").
		AddOutgoingMapping("server-version", "X-Server-Version").WithDefault("v1.0.0").
		AddOutgoingMapping("rate-limit-remaining", "X-RateLimit-Remaining").

		// Content headers
		AddBidirectionalMapping("Content-Type", "content-type").
		AddIncomingMapping("Accept", "accept").
		AddIncomingMapping("User-Agent", "user-agent").

		// Custom transformation: Extract Bearer token
		AddIncomingMapping("authorization", "auth-token").
		WithTransform(headermapper.ChainTransforms(
			headermapper.TrimSpace,
			headermapper.RemovePrefix("Bearer "),
		)).

		// Skip health checks and metrics
		SkipPaths("/health", "/metrics", "/ready").
		Debug(true).
		Build()

	// Set custom logger
	mapper.SetLogger(logger)

	logger.Info("📋 Header Mapping Configuration:")
	logger.Info("  Incoming: Authorization → authorization")
	logger.Info("  Incoming: X-API-Key → api-key")
	logger.Info("  Incoming: X-User-ID → user-id")
	logger.Info("  Incoming: Authorization → auth-token (with Bearer extraction)")
	logger.Info("  Bidirectional: X-Request-ID ↔ request-id")
	logger.Info("  Bidirectional: X-Correlation-ID ↔ correlation-id")
	logger.Info("  Bidirectional: X-Trace-ID ↔ trace-id")
	logger.Info("  Outgoing: processing-time-ms → X-Processing-Time")
	logger.Info("  Outgoing: server-version → X-Server-Version")
	logger.Info("  Outgoing: rate-limit-remaining → X-RateLimit-Remaining")

	return mapper
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Starting Basic gRPC Header Mapper Example Server...")

	// Create server
	server := NewBasicServer()

	// Create header mapper
	mapper := createBasicMapper()

	// Validate configuration
	if err := mapper.Validate(); err != nil {
		log.Fatalf("❌ Invalid header mapper configuration: %v", err)
	}

	server.logger.Info("✅ Header mapper configuration validated")

	// Set up gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(mapper.UnaryServerInterceptor()),
		grpc.StreamInterceptor(mapper.StreamServerInterceptor()),
	)

	// Register services
	pb.RegisterTestServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	server.logger.Info("✅ gRPC services registered")

	// Set up HTTP gateway
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := headermapper.CreateGatewayMux(mapper)

	// Connect to gRPC server
	conn, err := grpc.DialContext(
		ctx,
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("❌ Failed to dial server: %v", err)
	}
	defer conn.Close()

	// Register gateway
	if err := pb.RegisterTestServiceHandler(ctx, mux, conn); err != nil {
		log.Fatalf("❌ Failed to register gateway: %v", err)
	}

	server.logger.Info("✅ HTTP gateway registered")

	// Add health check endpoints
	mux.HandlePath("GET", "/health", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	mux.HandlePath("GET", "/ready", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Start servers with graceful shutdown
	var wg sync.WaitGroup
	wg.Add(2)

	// Start gRPC server
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", ":9090")
		if err != nil {
			log.Fatalf("❌ Failed to listen on gRPC port: %v", err)
		}

		log.Println("🎯 gRPC server listening on :9090")
		if err := grpcServer.Serve(lis); err != nil {
			server.logger.Error("gRPC server error:", err)
		}
	}()

	// Start HTTP server
	go func() {
		defer wg.Done()
		httpServer := &http.Server{
			Addr:    ":8080",
			Handler: mux,
		}

		log.Println("🌐 HTTP gateway listening on :8080")
		log.Println("📖 Available endpoints:")
		log.Println("  GET  /health - Health check")
		log.Println("  GET  /ready - Readiness check")
		log.Println("  POST /v1/echo - Echo service with header mapping")
		log.Println("")
		log.Println("🧪 Test the server with:")
		log.Println(`  curl -X POST -H "Content-Type: application/json" \`)
		log.Println(`       -H "Authorization: Bearer secret-token" \`)
		log.Println(`       -H "X-User-ID: user-123" \`)
		log.Println(`       -H "X-Request-ID: req-456" \`)
		log.Println(`       -d '{"message": "Hello World"}' \`)
		log.Println(`       http://localhost:8080/v1/echo`)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.logger.Error("HTTP server error:", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c

	log.Printf("📴 Received signal %v - initiating graceful shutdown...", sig)

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop gRPC server gracefully
	log.Println("🛑 Stopping gRPC server...")
	grpcServer.GracefulStop()

	// Cancel context for HTTP server
	log.Println("🛑 Stopping HTTP server...")
	cancel()

	// Wait for both servers to stop or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ All servers stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("⏰ Shutdown timeout exceeded, servers may still be stopping...")
	}

	log.Println("👋 Basic example server stopped")
}
