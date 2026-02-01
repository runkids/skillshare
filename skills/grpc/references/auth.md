# gRPC Authentication

## Authentication Methods

### 1. Token-Based (JWT/Bearer)

#### Client
```go
// Per-call credentials
type tokenAuth struct {
    token string
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
    return map[string]string{
        "authorization": "Bearer " + t.token,
    }, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
    return true
}

// Usage
conn, err := grpc.Dial(address,
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
    grpc.WithPerRPCCredentials(tokenAuth{token: "your-jwt-token"}),
)
```

#### Server Interceptor
```go
func authInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing metadata")
    }
    
    auth := md.Get("authorization")
    if len(auth) == 0 {
        return nil, status.Error(codes.Unauthenticated, "missing auth token")
    }
    
    token := strings.TrimPrefix(auth[0], "Bearer ")
    user, err := validateToken(token)
    if err != nil {
        return nil, status.Error(codes.Unauthenticated, "invalid token")
    }
    
    // Add user to context
    ctx = context.WithValue(ctx, "user", user)
    return handler(ctx, req)
}

// Register
server := grpc.NewServer(
    grpc.UnaryInterceptor(authInterceptor),
)
```

### 2. mTLS (Mutual TLS)

```go
// Server
cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
certPool := x509.NewCertPool()
ca, _ := ioutil.ReadFile("ca.crt")
certPool.AppendCertsFromPEM(ca)

creds := credentials.NewTLS(&tls.Config{
    Certificates: []tls.Certificate{cert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    certPool,
})

server := grpc.NewServer(grpc.Creds(creds))

// Client
cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
certPool := x509.NewCertPool()
ca, _ := ioutil.ReadFile("ca.crt")
certPool.AppendCertsFromPEM(ca)

creds := credentials.NewTLS(&tls.Config{
    Certificates: []tls.Certificate{cert},
    RootCAs:      certPool,
})

conn, _ := grpc.Dial(address, grpc.WithTransportCredentials(creds))
```

### 3. API Key

```go
// Client metadata
ctx := metadata.AppendToOutgoingContext(ctx, "x-api-key", "your-api-key")
response, err := client.GetUser(ctx, request)

// Server extraction
md, _ := metadata.FromIncomingContext(ctx)
apiKey := md.Get("x-api-key")
```

## grpcurl with Auth

```bash
# Bearer token
grpcurl -H "Authorization: Bearer <token>" \
  localhost:50051 user.UserService/GetUser

# API Key
grpcurl -H "X-API-Key: your-key" \
  localhost:50051 user.UserService/GetUser

# mTLS
grpcurl -cert client.crt -key client.key -cacert ca.crt \
  api.example.com:443 user.UserService/GetUser
```

## Streaming Interceptor

```go
func streamAuthInterceptor(
    srv interface{},
    ss grpc.ServerStream,
    info *grpc.StreamServerInfo,
    handler grpc.StreamHandler,
) error {
    md, ok := metadata.FromIncomingContext(ss.Context())
    if !ok {
        return status.Error(codes.Unauthenticated, "missing metadata")
    }
    
    // Validate auth...
    
    // Wrap stream with new context
    wrapped := &wrappedStream{
        ServerStream: ss,
        ctx:          context.WithValue(ss.Context(), "user", user),
    }
    
    return handler(srv, wrapped)
}

type wrappedStream struct {
    grpc.ServerStream
    ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
    return w.ctx
}
```

## Best Practices

| Practice | Description |
|----------|-------------|
| **Always use TLS** | Never plaintext in production |
| **Rotate tokens** | Short-lived access tokens |
| **Validate on server** | Never trust client data |
| **Rate limit** | Prevent abuse |
| **Audit logging** | Log auth events |
