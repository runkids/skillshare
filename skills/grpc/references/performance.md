# gRPC Performance Optimization

## Connection Management

### Connection Pooling

```go
// gRPC manages connection pooling automatically
// But you can tune it

conn, err := grpc.Dial(address,
    grpc.WithDefaultServiceConfig(`{
        "loadBalancingPolicy": "round_robin",
        "healthCheckConfig": {
            "serviceName": ""
        }
    }`),
)
```

### Keepalive Settings

```go
import "google.golang.org/grpc/keepalive"

// Client
conn, err := grpc.Dial(address,
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
        Time:                10 * time.Second, // Ping interval
        Timeout:             3 * time.Second,  // Wait for pong
        PermitWithoutStream: true,             // Ping even without active streams
    }),
)

// Server
server := grpc.NewServer(
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute,
        MaxConnectionAge:      30 * time.Minute,
        MaxConnectionAgeGrace: 5 * time.Second,
        Time:                  10 * time.Second,
        Timeout:               3 * time.Second,
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             5 * time.Second,
        PermitWithoutStream: true,
    }),
)
```

## Message Size

```go
// Default max: 4MB
// Increase if needed

// Client
conn, err := grpc.Dial(address,
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(10*1024*1024), // 10MB
        grpc.MaxCallSendMsgSize(10*1024*1024),
    ),
)

// Server
server := grpc.NewServer(
    grpc.MaxRecvMsgSize(10*1024*1024),
    grpc.MaxSendMsgSize(10*1024*1024),
)
```

## Compression

```go
import "google.golang.org/grpc/encoding/gzip"

// Client - per call
response, err := client.GetData(ctx, request, grpc.UseCompressor(gzip.Name))

// Client - default for all calls
conn, err := grpc.Dial(address,
    grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
)

// Server - automatically handles compressed requests
// No special configuration needed
```

## Load Balancing

### Client-Side Load Balancing

```go
// DNS resolver with round-robin
conn, err := grpc.Dial(
    "dns:///my-service.example.com:50051",
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
)
```

### Server-Side (with proxy)

```yaml
# Envoy configuration example
listeners:
  - address:
      socket_address:
        address: 0.0.0.0
        port_value: 50051
    filter_chains:
      - filters:
          - name: envoy.filters.network.http_connection_manager
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
              codec_type: AUTO
              stat_prefix: grpc
              route_config:
                virtual_hosts:
                  - name: grpc_service
                    domains: ["*"]
                    routes:
                      - match: { prefix: "/" }
                        route: { cluster: grpc_backend }
              http_filters:
                - name: envoy.filters.http.router
```

## Streaming Performance

### Buffer Settings

```go
// Server - increase window size for high throughput
server := grpc.NewServer(
    grpc.InitialWindowSize(1 << 20),     // 1MB
    grpc.InitialConnWindowSize(1 << 20), // 1MB
)

// Client
conn, err := grpc.Dial(address,
    grpc.WithInitialWindowSize(1 << 20),
    grpc.WithInitialConnWindowSize(1 << 20),
)
```

### Batch Processing

```go
// Instead of sending one message at a time
for _, item := range items {
    stream.Send(&pb.Item{...}) // Slow!
}

// Batch multiple items
batch := &pb.ItemBatch{Items: items}
stream.Send(batch) // Faster!
```

## Benchmarking

```bash
# Using ghz
ghz --insecure \
    --proto ./user.proto \
    --call user.UserService.GetUser \
    -d '{"id": "123"}' \
    -c 100 \       # Concurrent workers
    -n 10000 \     # Total requests
    localhost:50051

# Output includes:
# - Requests/sec
# - Latency percentiles
# - Error rate
```

## Monitoring

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// Server with OpenTelemetry
server := grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)

// Client
conn, err := grpc.Dial(address,
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

## Best Practices Summary

| Aspect | Recommendation |
|--------|----------------|
| **Connections** | Reuse connections, use keepalive |
| **Messages** | Keep small, use streaming for large data |
| **Compression** | Enable for large payloads |
| **Timeouts** | Always set deadlines |
| **Retries** | Implement with backoff |
| **Monitoring** | Use OpenTelemetry/Prometheus |
