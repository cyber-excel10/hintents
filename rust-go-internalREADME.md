# Hintens

[![CI](https://github.com/dotandev/hintens/workflows/CI/badge.svg)](https://github.com/dotandev/hintens/actions)
[![Documentation](https://img.shields.io/badge/docs-rustdoc-blue.svg)](https://docs.rs/hintens)
[![Go Reference](https://pkg.go.dev/badge/github.com/dotandev/hintens-go)](https://pkg.go.dev/github.com/dotandev/hintens-go)

Hintens is a hybrid Rust-Go system that combines Rust's performance with Go's concurrency model through efficient IPC.

## Architecture

### IPC Layer Communication

The diagram below illustrates how Rust and Go components communicate through the IPC layer:

```mermaid
sequenceDiagram
    participant RustApp as Rust Application
    participant RustIPC as Rust IPC Layer<br/>(hintens-rs)
    participant IPCChannel as IPC Channel<br/>(Unix Domain Socket/Windows Named Pipe)
    participant GoIPC as Go IPC Layer<br/>(hintens-go)
    participant GoApp as Go Components

    Note over RustIPC,GoIPC: Shared Memory Region for Zero-Copy Transfers
    
    RustApp->>RustIPC: 1. Serialize Data (bincode)
    RustIPC->>RustIPC: 2. Create Message Header<br/>- Magic Bytes (0x4850)<br/>- Version (1.0)<br/>- Message Type<br/>- Payload Length
    
    alt Small Message (< 64KB)
        RustIPC->>IPCChannel: 3a. Write Header + Payload
        IPCChannel->>GoIPC: 4a. Forward Complete Message
    else Large Message (> 64KB)
        RustIPC->>RustIPC: 3b. Allocate Shared Memory
        RustIPC->>IPCChannel: 4b. Write Header + Shared Memory Handle
        RustIPC->>GoIPC: 5b. Transfer via Shared Memory
    end
    
    GoIPC->>GoIPC: 6. Parse Header & Validate
    GoIPC->>GoApp: 7. Deserialize & Deliver
    GoApp-->>GoIPC: 8. Process & Prepare Response
    GoIPC-->>RustIPC: 9. Return Result via Same Channel
    RustIPC-->>RustApp: 10. Deliver Response
```

### Message Structure

```mermaid
flowchart TD
    subgraph Message Structure
        direction LR
        A[Message Header<br/>16 bytes] --> B[Magic: 0x4850<br/>2 bytes]
        A --> C[Version: 1.0<br/>2 bytes]
        A --> D[Message Type<br/>4 bytes]
        A --> E[Flags<br/>2 bytes]
        A --> F[Payload Length<br/>4 bytes]
        A --> G[Checksum<br/>2 bytes]
        
        H[Message Body<br/>Variable] --> I[Serialized Data<br/>bincode format]
        H --> J[Shared Memory Handle<br/>Optional]
    end
    
    subgraph IPC Flow
        K[Rust Component] --> L[Serialize]
        L --> M{Size Check}
        M -->|< 64KB| N[Socket Write]
        M -->|> 64KB| O[Shared Memory]
        O --> P[Memory Map]
        N --> Q[Go Component]
        P --> Q
        Q --> R[Deserialize]
        R --> S[Process]
    end
```

## Protocol Details

### Message Header Format

| Field | Size | Description |
|-------|------|-------------|
| Magic | 2 bytes | Protocol identifier (0x4850 = "HP") |
| Version | 2 bytes | Protocol version (0x0100 = v1.0) |
| Message Type | 4 bytes | Request (1), Response (2), Event (3), Error (4) |
| Flags | 2 bytes | SHM flag, Compression flag, etc. |
| Payload Length | 4 bytes | Size of payload in bytes |
| Checksum | 2 bytes | CRC16 of payload |
| Request ID | 8 bytes | Unique request identifier |

### Transport Mechanisms

- **Small Messages** (<64KB): Direct socket transfer
- **Large Messages** (≥64KB): Shared memory with handle passing

## Implementation Examples

### Rust Client
```rust
use hintens::IPCClient;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = IPCClient::connect("/tmp/hintens.sock").await?;
    
    // Send small message
    let response: String = client
        .send("Hello from Rust!")
        .await?;
    
    // Send large data via shared memory
    let large_data = vec![0u8; 1024 * 1024]; // 1MB
    let result: Vec<u8> = client
        .send(large_data)
        .await?;
    
    Ok(())
}
```

### Go Client
```go
package main

import (
    "github.com/dotandev/hintens-go/ipc"
)

func main() {
    client, err := ipc.NewClient("/tmp/hintens.sock")
    if err != nil {
        panic(err)
    }
    defer client.Close()
    
    // Send small message
    response, err := client.Send([]byte("Hello from Go!"))
    if err != nil {
        panic(err)
    }
    
    // Send large data
    largeData := make([]byte, 1024*1024) // 1MB
    result, err := client.Send(largeData)
}
```

## Performance Considerations

- Zero-copy transfers for payloads >64KB using shared memory
- Connection pooling for high-throughput scenarios
- Automatic batching of small messages
- Configurable timeouts and retry policies

## Development

### Building from Source
```bash
# Clone the repository
git clone https://github.com/dotandev/hintens.git
cd hintens

# Build Rust components
cargo build --release

# Build Go components
cd go && go build ./...
```

### Running Tests
```bash
# Run Rust tests
cargo test

# Run Go tests
cd go && go test ./...

# Run integration tests
cargo test --test ipc_integration
```

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.