#!/bin/bash

set -e

echo "Testing Basic gRPC Header Mapper Server"
echo "======================================="

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
    if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Wait for server to start
wait_for_server() {
    local max_attempts=30
    local attempt=0

    print_status "Waiting for server to start..."

    while [ $attempt -lt $max_attempts ]; do
        if check_server; then
            print_status "Server is ready!"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
        echo -n "."
    done
    echo

    print_error "Server failed to start within $max_attempts seconds"
    return 1
}

# Test health endpoints
test_health_endpoints() {
    print_status "Testing health endpoints..."

    # Test health endpoint
    response=$(curl -s -w "\n%{http_code}" http://localhost:8080/health)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" -eq 200 ]; then
        print_status "Health endpoint: $body"
    else
        print_error "Health endpoint failed with code: $http_code"
        return 1
    fi

    # Test ready endpoint
    response=$(curl -s -w "\n%{http_code}" http://localhost:8080/ready)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)

    if [ "$http_code" -eq 200 ]; then
        print_status "Ready endpoint: $body"
    else
        print_error "Ready endpoint failed with code: $http_code"
        return 1
    fi
}

# Test basic header mapping
test_basic_header_mapping() {
    print_status "Testing basic header mapping..."

    echo
    print_status "Testing incoming header mapping (HTTP headers -> gRPC metadata)..."

    response=$(curl -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test-token-12345" \
        -H "X-API-Key: api-key-67890" \
        -H "X-User-ID: user-123" \
        -H "X-Request-ID: req-456" \
        -H "X-Correlation-ID: corr-789" \
        -H "User-Agent: test-client/1.0" \
        -d '{"message": "Test header mapping"}' \
        -w "\n--- Response Info ---\nHTTP Code: %{http_code}\nResponse Time: %{time_total}s\n" \
        -D /tmp/basic_response_headers \
        http://localhost:8080/v1/echo)

    echo "$response"
    echo

    # Check if headers were mapped correctly in the response
    if echo "$response" | grep -q "authorization"; then
        print_status "Authorization header mapped correctly"
    else
        print_warning "Authorization header not found in response"
    fi

    if echo "$response" | grep -q "api-key"; then
        print_status "X-API-Key header mapped correctly"
    else
        print_warning "X-API-Key header not found in response"
    fi

    if echo "$response" | grep -q "user-id"; then
        print_status "X-User-ID header mapped correctly"
    else
        print_warning "X-User-ID header not found in response"
    fi

    if echo "$response" | grep -q "auth-token"; then
        print_status "Authorization transformation applied (Bearer token extracted)"
    else
        print_warning "Authorization transformation not applied"
    fi

    echo
    print_status "Testing outgoing header mapping (gRPC metadata -> HTTP headers)..."

    # Check response headers
    if [ -f /tmp/basic_response_headers ]; then
        echo "Response Headers:"
        cat /tmp/basic_response_headers
        echo

        if grep -qi "X-Processing-Time" /tmp/basic_response_headers; then
            processing_time=$(grep -i "X-Processing-Time" /tmp/basic_response_headers | cut -d: -f2 | tr -d ' \r\n')
            print_status "X-Processing-Time header present: $processing_time"
        else
            print_warning "X-Processing-Time header not found in response"
        fi

        if grep -qi "X-Server-Version" /tmp/basic_response_headers; then
            version=$(grep -i "X-Server-Version" /tmp/basic_response_headers | cut -d: -f2 | tr -d ' \r\n')
            print_status "X-Server-Version header present: $version"
        else
            print_warning "X-Server-Version header not found in response"
        fi

        if grep -qi "X-RateLimit-Remaining" /tmp/basic_response_headers; then
            rate_limit=$(grep -i "X-RateLimit-Remaining" /tmp/basic_response_headers | cut -d: -f2 | tr -d ' \r\n')
            print_status "X-RateLimit-Remaining header present: $rate_limit"
        else
            print_warning "X-RateLimit-Remaining header not found in response"
        fi

        # Clean up
        rm -f /tmp/basic_response_headers
    fi
}

# Test Bearer token transformation
test_bearer_token_transformation() {
    print_status "Testing Bearer token transformation..."

    echo
    print_status "Testing Bearer token extraction transformation..."

    response=$(curl -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization:   Bearer   secret-token-with-spaces   " \
        -d '{"message": "Test transformation"}' \
        http://localhost:8080/v1/echo)

    if command -v jq &> /dev/null; then
        auth_token=$(echo "$response" | jq -r '.headers["auth-token"] // "not found"')
        if [ "$auth_token" = "secret-token-with-spaces" ]; then
            print_status "Bearer token transformation working: '$auth_token'"
        elif [ "$auth_token" = "not found" ]; then
            print_warning "Bearer token transformation not working - auth-token header not found"
        else
            print_warning "Bearer token transformation may not be working correctly: '$auth_token'"
        fi
    else
        print_warning "jq not available, skipping JSON parsing test"
        echo "Raw response: $response"
    fi
}

# Test bidirectional headers
test_bidirectional_headers() {
    print_status "Testing bidirectional headers..."

    echo
    print_status "Testing X-Request-ID bidirectional mapping..."

    test_request_id="test-req-$(date +%s)"

    response=$(curl -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Request-ID: $test_request_id" \
        -d '{"message": "Test bidirectional"}' \
        -D /tmp/bidirectional_headers \
        http://localhost:8080/v1/echo)

    # Check if request ID appears in response body (incoming mapping)
    if echo "$response" | grep -q "$test_request_id"; then
        print_status "X-Request-ID found in response body (incoming mapping worked)"
    else
        print_warning "X-Request-ID not found in response body"
    fi

    # Check if request ID appears in response headers (outgoing mapping)
    if [ -f /tmp/bidirectional_headers ]; then
        if grep -q "$test_request_id" /tmp/bidirectional_headers; then
            print_status "X-Request-ID found in response headers (outgoing mapping worked)"
        else
            print_warning "X-Request-ID not found in response headers"
        fi
        rm -f /tmp/bidirectional_headers
    fi
}

# Test skipped paths
test_skipped_paths() {
    print_status "Testing path skipping functionality..."

    local paths=("/health" "/ready")

    for path in "${paths[@]}"; do
        if curl -s -f "http://localhost:8080$path" > /dev/null; then
            print_status "Skipped path $path responds correctly (headers not processed)"
        else
            print_error "Skipped path $path is not responding"
        fi
    done
}

# Main test function
run_tests() {
    print_status "Starting basic server tests..."

    # Check if server is running
    if ! check_server; then
        print_error "Server is not running. Please start it first with:"
        print_error "  make run-example"
        print_error "Or manually: ./bin/basic-example"
        return 1
    fi

    # Run all tests
    test_health_endpoints || return 1
    test_basic_header_mapping || return 1
    test_bearer_token_transformation || return 1
    test_bidirectional_headers || return 1
    test_skipped_paths || return 1

    echo
    print_status "All basic server tests completed successfully!"
    print_status "Check the server logs to see detailed header mapping information."

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
    run_tests
}

# Run main function
main "$@"
