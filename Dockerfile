# Use the official Golang image as the base
FROM golang:1.23-alpine

# Set the working directory inside the container
WORKDIR /app

# Install necessary packages
RUN apk add --no-cache git bash protobuf

# Install protoc Go plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add GOPATH/bin to PATH
ENV PATH="/root/go/bin:${PATH}"

# Clone the yellowstone-grpc repository
RUN git clone https://github.com/rpcpool/yellowstone-grpc.git

# Copy your generate script into the container
COPY generate.sh /app/generate.sh

# Make the script executable
RUN chmod +x /app/generate.sh

# Define the entrypoint command
ENTRYPOINT ["/app/generate.sh"]
