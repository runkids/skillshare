# gRPC Streaming

## Server Streaming

Client sends one request, server sends multiple responses.

### Proto Definition
```protobuf
service StockService {
  rpc StreamPrices(StockRequest) returns (stream StockPrice);
}

message StockRequest {
  repeated string symbols = 1;
}

message StockPrice {
  string symbol = 1;
  double price = 2;
  google.protobuf.Timestamp timestamp = 3;
}
```

### Server (Go)
```go
func (s *server) StreamPrices(req *pb.StockRequest, stream pb.StockService_StreamPricesServer) error {
    for {
        for _, symbol := range req.Symbols {
            price := getPrice(symbol)
            if err := stream.Send(&pb.StockPrice{
                Symbol: symbol,
                Price:  price,
                Timestamp: timestamppb.Now(),
            }); err != nil {
                return err
            }
        }
        time.Sleep(time.Second)
    }
}
```

### Client (Go)
```go
stream, err := client.StreamPrices(ctx, &pb.StockRequest{
    Symbols: []string{"AAPL", "GOOGL"},
})

for {
    price, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Printf("%s: $%.2f\n", price.Symbol, price.Price)
}
```

## Client Streaming

Client sends multiple messages, server responds once.

### Proto Definition
```protobuf
service FileService {
  rpc UploadFile(stream FileChunk) returns (UploadResponse);
}

message FileChunk {
  bytes content = 1;
  string filename = 2;
}

message UploadResponse {
  string file_id = 1;
  int64 bytes_received = 2;
}
```

### Server (Go)
```go
func (s *server) UploadFile(stream pb.FileService_UploadFileServer) error {
    var totalBytes int64
    var filename string
    
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            return stream.SendAndClose(&pb.UploadResponse{
                FileId:        generateID(),
                BytesReceived: totalBytes,
            })
        }
        if err != nil {
            return err
        }
        
        filename = chunk.Filename
        totalBytes += int64(len(chunk.Content))
        // Write chunk to storage...
    }
}
```

### Client (Go)
```go
stream, err := client.UploadFile(ctx)

file, _ := os.Open("large-file.bin")
buf := make([]byte, 1024*1024) // 1MB chunks

for {
    n, err := file.Read(buf)
    if err == io.EOF {
        break
    }
    
    stream.Send(&pb.FileChunk{
        Content:  buf[:n],
        Filename: "large-file.bin",
    })
}

response, err := stream.CloseAndRecv()
```

## Bidirectional Streaming

Both client and server stream messages independently.

### Proto Definition
```protobuf
service ChatService {
  rpc Chat(stream ChatMessage) returns (stream ChatMessage);
}

message ChatMessage {
  string user = 1;
  string content = 2;
  google.protobuf.Timestamp timestamp = 3;
}
```

### Server (Go)
```go
func (s *server) Chat(stream pb.ChatService_ChatServer) error {
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        // Broadcast to all connected clients
        response := &pb.ChatMessage{
            User:      "Server",
            Content:   fmt.Sprintf("Received: %s", msg.Content),
            Timestamp: timestamppb.Now(),
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
    }
}
```

### Client (Go)
```go
stream, err := client.Chat(ctx)

// Send in goroutine
go func() {
    for _, msg := range messages {
        stream.Send(&pb.ChatMessage{
            User:    "Alice",
            Content: msg,
        })
    }
    stream.CloseSend()
}()

// Receive
for {
    msg, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("[%s]: %s\n", msg.User, msg.Content)
}
```

## grpcurl with Streaming

```bash
# Server streaming
grpcurl -plaintext -d '{"symbols": ["AAPL"]}' \
  localhost:50051 stock.StockService/StreamPrices

# Client streaming (from stdin)
echo '{"content": "chunk1"}
{"content": "chunk2"}' | \
grpcurl -plaintext -d @ localhost:50051 file.FileService/UploadFile

# Bidirectional (interactive)
grpcurl -plaintext -d @ localhost:50051 chat.ChatService/Chat
```

## Best Practices

1. **Handle EOF** - Always check for `io.EOF` in receive loops
2. **Graceful shutdown** - Use `CloseSend()` on client
3. **Backpressure** - Implement flow control for high-volume streams
4. **Keepalive** - Configure keepalive for long-lived streams
5. **Timeouts** - Use context deadlines appropriately
