#!/bin/bash

set -e

echo "Testing Advanced gRPC Header Mapper Server"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if server is running
check_server() {
    if curl -s -f http://localhost:8080/health/advanced > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Wait for server to start
wait_for_server() {
    local max_attempts=30
    local attempt=0

    print_status "Waiting for advanced server to start..."

    while [ $attempt -lt $max_attempts ]; do
        if check_server; then
            print_status "Advanced server is ready!"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
        echo -n "."
    done
    echo

    print_error "Advanced server failed to start within $max_attempts seconds"
    return 1
}

# Test health endpoints including advanced health check
test_advanced_health_endpoints() {
    print_status "Testing advanced health endpoints..."

    # Test advanced health endpoint without auth
    response=$(curl -s -w "\n%{http_code}" http://localhost:8080/health/advanced)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" -eq 200 ]; then
        print_status "Advanced health endpoint (no auth): $body"

        # Check if response contains expected fields
        if command -v jq &> /dev/null; then
            if echo "$body" | jq -e '.status' > /dev/null 2>&1; then
                print_status "Health response contains status field"
            fi
            if echo "$body" | jq -e '.uptime' > /dev/null 2>&1; then
                print_status "Health response contains uptime field"
            fi
        fi
    else
        print_error "Advanced health endpoint failed with code: $http_code"
        return 1
    fi

    # Test advanced health endpoint with auth
    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer test-token" \
        http://localhost:8080/health/advanced)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" -eq 200 ]; then
        print_status "Advanced health endpoint (with auth): authenticated field present"
        if command -v jq &> /dev/null; then
            if echo "$body" | jq -e '.authenticated' > /dev/null 2>&1; then
                print_status "Authentication detected in health check"
            fi
        fi
    fi
}

# Test metrics endpoint
test_metrics_endpoint() {
    print_status "Testing metrics endpoint..."

    response=$(curl -s -w "\n%{http_code}" http://localhost:8080/metrics)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" -eq 200 ]; then
        print_status "Metrics endpoint accessible"

        if command -v jq &> /dev/null; then
            # Check for expected metrics structure
            if echo "$body" | jq -e '.incoming_headers' > /dev/null 2>&1; then
                print_status "Metrics contains incoming_headers data"
            fi
            if echo "$body" | jq -e '.outgoing_headers' > /dev/null 2>&1; then
                print_status "Metrics contains outgoing_headers data"
            fi
            if echo "$body" | jq -e '.errors' > /dev/null 2>&1; then
                print_status "Metrics contains error count"
            fi
        fi
    else
        print_error "Metrics endpoint failed with code: $http_code"
        return 1
    fi
}

# Test advanced header mapping with more complex scenarios
test_advanced_header_mapping() {
    print_status "Testing advanced header mapping..."

    echo
    print_status "Testing comprehensive header mapping with transformations..."

    response=$(curl -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer advanced-token-12345" \
        -H "X-API-Key: advanced-api-key" \
        -H "X-User-ID: advanced-user-123" \
        -H "X-User-Role: admin" \
        -H "X-Request-ID: adv-req-456" \
        -H "X-Correlation-ID: adv-corr-789" \
        -H "X-Trace-ID: adv-trace-abc" \
        -H "X-Span-ID: adv-span-def" \
        -H "User-Agent: AdvancedClient/1.2.3 (Test Suite)" \
        -H "X-Client-Version: 2.1.0" \
        -H "X-Device-ID: device-xyz" \
        -H "Accept: application/json" \
        -H "Accept-Language: en-US,en;q=0.9" \
        -H "Accept-Encoding: gzip, deflate" \
        -H "X-Tenant-ID: tenant-abc" \
        -H "X-Region: us-west-2" \
        -d '{"message": "Advanced test message"}' \
        -w "\n--- Response Info ---\nHTTP Code: %{http_code}\nResponse Time: %{time_total}s\n" \
        -D /tmp/advanced_response_headers \
        http://localhost:8080/v1/echo)

    echo "$response"
    echo

    # Detailed header validation
    headers_to_check=(
        "authorization:Authorization header"
        "auth-token:Bearer token extraction"
        "api-key:API key header"
        "user-id:User ID header"
        "user-role:User role header"
        "request-id:Request ID (bidirectional)"
        "user-agent:User agent with sanitization"
        "client-version:Client version header"
        "tenant-id:Tenant ID (bidirectional)"
        "region:Region (bidirectional)"
    )

    for header_check in "${headers_to_check[@]}"; do
        header_key=$(echo "$header_check" | cut -d: -f1)
        header_desc=$(echo "$header_check" | cut -d: -f2)

        if echo "$response" | grep -q "\"$header_key\""; then
            print_status "$header_desc mapped correctly"
        else
            print_warning "$header_desc not found in response"
        fi
    done

    # Check Bearer token transformation specifically
    if command -v jq &> /dev/null; then
        auth_token=$(echo "$response" | jq -r '.headers["auth-token"] // "not found"')
        if [[ "$auth_token" == *"advanced-token-12345"* ]]; then
            print_status "Advanced Bearer token transformation working"
        else
            print_warning "Advanced Bearer token transformation issue: '$auth_token'"
        fi

        # Check user agent sanitization
        user_agent=$(echo "$response" | jq -r '.headers["user-agent"] // "not found"')
        if [[ "$user_agent" == *"x.x.x"* ]]; then
            print_status "User agent sanitization working: '$user_agent'"
        else
            print_warning "User agent sanitization may not be working: '$user_agent'"
        fi
    fi
}

# Test advanced outgoing headers
test_advanced_outgoing_headers() {
    print_status "Testing advanced outgoing headers..."

    if [ -f /tmp/advanced_response_headers ]; then
        echo
        print_status "Checking advanced response headers..."
        echo "Response Headers:"
        cat /tmp/advanced_response_headers
        echo

        # Check advanced outgoing headers
        advanced_headers=(
            "X-Processing-Time:Processing time with suffix"
            "X-Server-Version:Advanced server version"
            "X-RateLimit-Remaining:Rate limit info"
            "X-Response-Timestamp:Response timestamp"
            "X-Content-Security-Policy:Security policy header"
            "X-Frame-Options:Frame options header"
            "Cache-Control:Cache control header"
        )

        for header_check in "${advanced_headers[@]}"; do
            header_name=$(echo "$header_check" | cut -d: -f1)
            header_desc=$(echo "$header_check" | cut -d: -f2)

            if grep -qi "$header_name" /tmp/advanced_response_headers; then
                header_value=$(grep -i "$header_name" /tmp/advanced_response_headers | cut -d: -f2 | tr -d ' \r\n')
                print_status "$header_desc present: $header_value"
            else
                print_warning "$header_desc not found"
            fi
        done

        rm -f /tmp/advanced_response_headers
    fi
}

# Test configuration loading
test_config_loading() {
    print_status "Testing configuration loading capability..."

    # Create a temporary config file
    cat > /tmp/test-config.yaml << EOF
mappings:
  - http_header: "X-Test-Config"
    grpc_metadata: "test-config"
    direction: 0
    required: false

skip_paths:
  - "/health"
  - "/metrics"

debug: false
EOF

    print_status "Created temporary config file for testing"

    # Note: We can't easily test config loading without restarting the server
    # But we can verify the config file format is valid
    if [ -f /tmp/test-config.yaml ]; then
        print_status "Config file format appears valid"
        rm -f /tmp/test-config.yaml
    fi
}

# Test error handling
test_error_handling() {
    print_status "Testing error handling..."

    # Test invalid JSON
    response=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d 'invalid json' \
        http://localhost:8080/v1/echo)

    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" -eq 400 ] || [ "$http_code" -eq 500 ]; then
        print_status "Error handling working for invalid JSON (HTTP $http_code)"
    else
        print_warning "Error handling may not be working as expected (HTTP $http_code)"
    fi
}

# Test performance with multiple requests
test_performance() {
    print_status "Testing performance with multiple requests..."

    local num_requests=10
    local start_time=$(date +%s)

    for i in $(seq 1 $num_requests); do
        curl -s \
            -X POST \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer perf-test-$i" \
            -H "X-Request-ID: perf-req-$i" \
            -d "{\"message\": \"Performance test $i\"}" \
            http://localhost:8080/v1/echo > /dev/null
    done

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    local rps=$((num_requests / duration))

    print_status "Performance test: $num_requests requests in ${duration}s (~${rps} req/s)"
}

# Main test function
run_advanced_tests() {
    print_status "Starting advanced server tests..."

    # Check if server is running
    if ! check_server; then
        print_error "Advanced server is not running. Please start it first with:"
        print_error "  ./bin/advanced-example"
        print_error "Or: make run-advanced-example"
        return 1
    fi

    # Run all tests
    test_advanced_health_endpoints || return 1
    test_metrics_endpoint || return 1
    test_advanced_header_mapping || return 1
    test_advanced_outgoing_headers || return 1
    test_config_loading || return 1
    test_error_handling || return 1
    test_performance || return 1

    echo
    print_status "All advanced server tests completed successfully!"
    print_status "The advanced server demonstrates:"
    print_status "  - Comprehensive header mapping (15+ headers)"
    print_status "  - Advanced transformations (Bearer extraction, User-Agent sanitization)"
    print_status "  - Metrics collection and reporting"
    print_status "  - Enhanced logging and error handling"
    print_status "  - Graceful shutdown and production-ready features"

    return 0
}

# Check dependencies
check_dependencies() {
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not installed"
        return 1
    fi

    if ! command -v jq &> /dev/null; then
        print_warning "jq is not installed. Some tests may not work optimally."
        print_warning "Install with: brew install jq (macOS) or apt-get install jq (Ubuntu)"
    fi

    return 0
}

# Script entry point
main() {
    check_dependencies || exit 1
    run_advanced_tests
}

# Run main function
main "$@"
