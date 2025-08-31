#!/bin/bash
set -e

echo "Generating documentation..."

# Generate Go documentation
echo "Generating Go docs..."
godoc -http=:6060 &
GODOC_PID=$!

# Wait a bit for godoc to start
sleep 2

# Generate API documentation using godoc
mkdir -p docs/generated
curl -s http://localhost:6060/pkg/github.com/bhatti/grpc-header-mapper/headermapper/ > docs/generated/api.html

# Kill godoc
kill $GODOC_PID

# Generate examples documentation
echo "Generating examples documentation..."
find examples/ -name "*.go" -exec go doc -all {} \; > docs/generated/examples.txt

# Generate benchmark results
echo "Running benchmarks for documentation..."
go test -bench=. -benchmem ./... > docs/generated/benchmarks.txt

echo "Documentation generation complete!"
