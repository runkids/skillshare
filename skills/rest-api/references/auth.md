# REST API Authentication

## Authentication Methods

### 1. Bearer Token (JWT)

```bash
# Request
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  https://api.example.com/users

# Login to get token
curl -X POST https://api.example.com/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "secret"}'

# Response
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600
}
```

### 2. API Key

```bash
# Header
curl -H "X-API-Key: your-api-key" https://api.example.com/data

# Query param (less secure)
curl "https://api.example.com/data?api_key=your-api-key"
```

### 3. Basic Auth

```bash
curl -u username:password https://api.example.com/users

# Or with header
curl -H "Authorization: Basic $(echo -n 'user:pass' | base64)" \
  https://api.example.com/users
```

### 4. OAuth 2.0

```bash
# Authorization Code Flow
# Step 1: Redirect user to authorize
https://auth.example.com/authorize?
  response_type=code&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=https://yourapp.com/callback&
  scope=read+write&
  state=random_state

# Step 2: Exchange code for token
curl -X POST https://auth.example.com/token \
  -d "grant_type=authorization_code" \
  -d "code=AUTHORIZATION_CODE" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_SECRET" \
  -d "redirect_uri=https://yourapp.com/callback"

# Step 3: Use access token
curl -H "Authorization: Bearer ACCESS_TOKEN" \
  https://api.example.com/me
```

## Token Refresh

```bash
curl -X POST https://api.example.com/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "your-refresh-token"}'
```

## Security Best Practices

| Practice | Description |
|----------|-------------|
| **HTTPS only** | Never send credentials over HTTP |
| **Short-lived tokens** | Access tokens: 15min-1hr |
| **Secure refresh** | Refresh tokens: rotate on use |
| **Rate limiting** | Prevent brute force attacks |
| **Token revocation** | Implement logout/invalidation |
| **Scope limitation** | Request minimal permissions |

## Error Responses

```json
// 401 Unauthorized
{
  "error": "UNAUTHORIZED",
  "message": "Invalid or expired token"
}

// 403 Forbidden
{
  "error": "FORBIDDEN", 
  "message": "Insufficient permissions"
}
```

## CORS for Browser Clients

```javascript
// Server-side headers
Access-Control-Allow-Origin: https://yourapp.com
Access-Control-Allow-Headers: Authorization, Content-Type
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Credentials: true
```
