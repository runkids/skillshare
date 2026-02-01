# gRPC Error Handling

## Standard Status Codes

| Code | Name | When to Use |
|------|------|-------------|
| 0 | OK | Success |
| 1 | CANCELLED | Operation cancelled by client |
| 2 | UNKNOWN | Unknown error |
| 3 | INVALID_ARGUMENT | Bad request parameters |
| 4 | DEADLINE_EXCEEDED | Timeout |
| 5 | NOT_FOUND | Resource doesn't exist |
| 6 | ALREADY_EXISTS | Resource already exists |
| 7 | PERMISSION_DENIED | No permission (authenticated) |
| 8 | RESOURCE_EXHAUSTED | Rate limit, quota exceeded |
| 9 | FAILED_PRECONDITION | System not in required state |
| 10 | ABORTED | Operation aborted (concurrency) |
| 11 | OUT_OF_RANGE | Invalid range (pagination) |
| 12 | UNIMPLEMENTED | Method not implemented |
| 13 | INTERNAL | Internal server error |
| 14 | UNAVAILABLE | Service temporarily unavailable |
| 15 | DATA_LOSS | Unrecoverable data loss |
| 16 | UNAUTHENTICATED | Not authenticated |

## Returning Errors

### Go
```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // Validation error
    if req.Id == "" {
        return nil, status.Errorf(codes.InvalidArgument, "user id is required")
    }
    
    // Not found
    user, err := s.db.FindUser(req.Id)
    if err == ErrNotFound {
        return nil, status.Errorf(codes.NotFound, "user %s not found", req.Id)
    }
    
    // Internal error
    if err != nil {
        return nil, status.Errorf(codes.Internal, "database error: %v", err)
    }
    
    return user, nil
}
```

### Python
```python
import grpc

def GetUser(self, request, context):
    if not request.id:
        context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user id is required")
    
    user = self.db.find_user(request.id)
    if not user:
        context.abort(grpc.StatusCode.NOT_FOUND, f"user {request.id} not found")
    
    return user
```

## Rich Error Details

### Proto Definition
```protobuf
import "google/rpc/error_details.proto";

// Use with google.rpc.Status
```

### Server (Go)
```go
import (
    "google.golang.org/genproto/googleapis/rpc/errdetails"
    "google.golang.org/grpc/status"
)

func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    var violations []*errdetails.BadRequest_FieldViolation
    
    if req.Email == "" {
        violations = append(violations, &errdetails.BadRequest_FieldViolation{
            Field:       "email",
            Description: "email is required",
        })
    }
    
    if req.Name == "" {
        violations = append(violations, &errdetails.BadRequest_FieldViolation{
            Field:       "name",
            Description: "name is required",
        })
    }
    
    if len(violations) > 0 {
        st := status.New(codes.InvalidArgument, "validation failed")
        br := &errdetails.BadRequest{FieldViolations: violations}
        st, _ = st.WithDetails(br)
        return nil, st.Err()
    }
    
    // Continue with creation...
}
```

### Client (Go)
```go
user, err := client.CreateUser(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if !ok {
        log.Fatalf("unknown error: %v", err)
    }
    
    log.Printf("Error: %s (code: %s)", st.Message(), st.Code())
    
    for _, detail := range st.Details() {
        switch t := detail.(type) {
        case *errdetails.BadRequest:
            for _, violation := range t.GetFieldViolations() {
                log.Printf("  - %s: %s", violation.Field, violation.Description)
            }
        case *errdetails.RetryInfo:
            log.Printf("  Retry after: %v", t.RetryDelay.AsDuration())
        }
    }
}
```

## Error Types for Different Scenarios

```go
// User input errors → INVALID_ARGUMENT
status.Errorf(codes.InvalidArgument, "invalid email format")

// Authentication → UNAUTHENTICATED
status.Errorf(codes.Unauthenticated, "invalid or expired token")

// Authorization → PERMISSION_DENIED
status.Errorf(codes.PermissionDenied, "admin access required")

// Resource not found → NOT_FOUND
status.Errorf(codes.NotFound, "user %s not found", id)

// Conflict → ALREADY_EXISTS or ABORTED
status.Errorf(codes.AlreadyExists, "email already registered")

// Rate limiting → RESOURCE_EXHAUSTED
status.Errorf(codes.ResourceExhausted, "rate limit exceeded")

// Temporary failure → UNAVAILABLE
status.Errorf(codes.Unavailable, "database connection lost")

// Bug → INTERNAL
status.Errorf(codes.Internal, "unexpected error: %v", err)
```

## Client Retry Logic

```go
import "google.golang.org/grpc/codes"

func isRetryable(code codes.Code) bool {
    switch code {
    case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted:
        return true
    default:
        return false
    }
}
```
