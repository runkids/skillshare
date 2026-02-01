---
name: grpc
version: 1.0.0
description: gRPC API development and interaction skill. Use when building gRPC services, defining protobuf schemas, debugging gRPC endpoints, or implementing streaming APIs.
argument-hint: "[service.method] [endpoint] [--message]"
---

# gRPC Skill

Build high-performance RPC services with Protocol Buffers.

## Quick Reference

### Using grpcurl

```bash
# List services
grpcurl -plaintext localhost:50051 list

# Describe service
grpcurl -plaintext localhost:50051 describe user.UserService

# Call unary method
grpcurl -plaintext -d '{"id": "123"}' \
  localhost:50051 user.UserService/GetUser

# Call with JSON file
grpcurl -plaintext -d @ localhost:50051 user.UserService/CreateUser < request.json

# With TLS
grpcurl api.example.com:443 user.UserService/GetUser

# With auth token
grpcurl -H "Authorization: Bearer <token>" \
  localhost:50051 user.UserService/GetUser
```

### Using grpc_cli

```bash
grpc_cli call localhost:50051 GetUser "id: '123'"
grpc_cli ls localhost:50051
```

## Protobuf Basics

### Service Definition

```protobuf
syntax = "proto3";

package user;

option go_package = "github.com/example/user";

// Service definition
service UserService {
  // Unary RPC
  rpc GetUser(GetUserRequest) returns (User);
  
  // Server streaming
  rpc ListUsers(ListUsersRequest) returns (stream User);
  
  // Client streaming
  rpc UploadUsers(stream User) returns (UploadResponse);
  
  // Bidirectional streaming
  rpc Chat(stream ChatMessage) returns (stream ChatMessage);
}

// Messages
message User {
  string id = 1;
  string name = 2;
  string email = 3;
  UserStatus status = 4;
  google.protobuf.Timestamp created_at = 5;
  
  // Nested message
  Address address = 6;
}

message Address {
  string street = 1;
  string city = 2;
  string country = 3;
}

enum UserStatus {
  USER_STATUS_UNSPECIFIED = 0;
  USER_STATUS_ACTIVE = 1;
  USER_STATUS_INACTIVE = 2;
}

message GetUserRequest {
  string id = 1;
}

message ListUsersRequest {
  int32 page_size = 1;
  string page_token = 2;
}
```

## RPC Types

| Type | Client | Server | Use Case |
|------|--------|--------|----------|
| **Unary** | 1 request | 1 response | Simple request/response |
| **Server Stream** | 1 request | N responses | Large data download |
| **Client Stream** | N requests | 1 response | File upload |
| **Bidirectional** | N requests | N responses | Chat, real-time |

## Error Handling

```protobuf
import "google/rpc/status.proto";

// Standard gRPC status codes
// OK = 0
// CANCELLED = 1
// UNKNOWN = 2
// INVALID_ARGUMENT = 3
// DEADLINE_EXCEEDED = 4
// NOT_FOUND = 5
// ALREADY_EXISTS = 6
// PERMISSION_DENIED = 7
// RESOURCE_EXHAUSTED = 8
// FAILED_PRECONDITION = 9
// ABORTED = 10
// OUT_OF_RANGE = 11
// UNIMPLEMENTED = 12
// INTERNAL = 13
// UNAVAILABLE = 14
// DATA_LOSS = 15
// UNAUTHENTICATED = 16
```

```go
// Go example
import "google.golang.org/grpc/status"

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    user, err := s.db.FindUser(req.Id)
    if err != nil {
        return nil, status.Errorf(codes.NotFound, "user %s not found", req.Id)
    }
    return user, nil
}
```

## Code Generation

```bash
# Install protoc
brew install protobuf  # macOS
apt install protobuf-compiler  # Ubuntu

# Install language plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate code
protoc --go_out=. --go-grpc_out=. proto/*.proto

# Python
pip install grpcio-tools
python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. proto/*.proto
```

## Best Practices

| Practice | Description |
|----------|-------------|
| **Use proto3** | Modern syntax, better defaults |
| **Reserve fields** | Don't reuse field numbers |
| **Meaningful names** | ServiceName/MethodName pattern |
| **Deadlines** | Always set client deadlines |
| **Interceptors** | Use for logging, auth, metrics |
| **Keep messages small** | Stream for large data |

## References

| Topic | File |
|-------|------|
| Streaming | [streaming.md](references/streaming.md) |
| Authentication | [auth.md](references/auth.md) |
| Error Handling | [errors.md](references/errors.md) |
| Performance | [performance.md](references/performance.md) |
