# Geyser SDK

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Geyser SDK is a Go client library for interacting with the Geyser API. It provides both gRPC and WebSocket clients to connect to Geyser nodes and subscribe to Solana blockchain events such as transactions, accounts, slots, blocks, and more.

## Features

- **gRPC Client**: Connect to Geyser nodes using gRPC and subscribe to various Solana blockchain events.
- **Enhanced WebSocket Client**: Use Geyser Enhanced WebSocket connections to subscribe to events in real-time.
- **Event Subscriptions**: Subscribe to transactions, accounts, slots, blocks, block metadata, and entries.
- **Data Conversion**: Convert Geyser transaction and block data into Solana Go SDK types.
- **Automated Protobuf Generation**: Use Docker to automate the generation of Go code from protobuf definitions.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
  - [gRPC Client](#grpc-client)
  - [Enhanced WebSocket Client](#enhanced-websocket-client)
- [Protobuf Generation](#protobuf-generation)
- [Contributing](#contributing)
- [License](#license)

## Installation

To use the Geyser SDK in your Go project, add it to your module dependencies:

```bash
go get github.com/Prophet-Solutions/geyser-sdk
```

Ensure you have Go version **1.20** or higher installed.

## Usage

### gRPC Client

#### Connecting to a Geyser Node

Create a new gRPC client by specifying the context, gRPC server URL, and optional compression and metadata. The client allows you to interact with the Geyser node using gRPC calls.

#### Subscribing to Events

Use the client to create a stream and subscribe to various events like accounts, transactions, slots, blocks, and more. You can manage subscriptions by adding, modifying, or removing them as needed.

#### Handling Updates

Set up channels to receive updates and errors from the stream. Implement your logic to process incoming data and handle any errors that occur during the streaming process.

#### Closing the Client

Always close the client when it's no longer needed to gracefully terminate the connection and free up resources.

### Enhanced WebSocket Client

#### Connecting to a Geyser Enhanced WebSocket

Instantiate the Enhanced WebSocket client with the Geyser node's WebSocket endpoint (tested with Helius). Establish the connection to start interacting with the node in real-time.

#### Subscribing to Transactions and Accounts

Subscribe to transactions and account updates by specifying filters and options. The client supports various subscription methods to cater to your application's needs.

#### Handling Updates

Listen to the channels provided by the subscriptions to receive real-time updates. Process the data according to your application's requirements.

#### Closing the Enhanced WebSocket Client

Close the Enhanced WebSocket client to terminate the connection and ensure that all resources are properly released.

## Protobuf Generation

The Geyser SDK uses protobuf definitions from the [yellowstone-grpc](https://github.com/rpcpool/yellowstone-grpc) project. A Docker setup is provided to automate the generation of Go code from the protobuf files.

### Using Docker to Generate Protobuf Code

1. **Build the Docker Image**

   Build the Docker image using the provided `Dockerfile`.

   ```bash
   docker build -t geyser-sdk-protoc .
   ```

2. **Run the Protobuf Generation**

   Run the Docker container to generate the protobuf code.

   ```bash
   docker run --rm -v $(pwd)/pb:/app/pb geyser-sdk-protoc
   ```

   This command mounts the local `pb` directory to the container and outputs the generated Go files there.

### Files

- **Dockerfile**

  Sets up the Docker environment with Go and the necessary tools to generate protobuf files.

- **generate.sh**

  A script that pulls the latest protobuf definitions and generates the Go code.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on GitHub to contribute to the project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
