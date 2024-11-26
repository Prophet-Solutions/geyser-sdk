#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status

# Define directories
PROTO_DIR="/app/yellowstone-grpc/yellowstone-grpc-proto/proto"
OUTPUT_DIR="/app/pb"

# Update the yellowstone-grpc repository to the latest commit
cd /app/yellowstone-grpc
git pull origin master

# Create the output directory
mkdir -p "$OUTPUT_DIR"

# Generate Go code from .proto files
protoc -I "$PROTO_DIR" \
  --go_out="$OUTPUT_DIR" --go_opt=paths=source_relative \
  --go-grpc_out="$OUTPUT_DIR" --go-grpc_opt=paths=source_relative \
  "$PROTO_DIR"/*.proto

echo "Protobuf generation completed successfully."
