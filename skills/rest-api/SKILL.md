---
name: rest-api
version: 1.0.0
description: REST API development and interaction skill. Use when designing RESTful APIs, making HTTP requests, debugging endpoints, or working with OpenAPI/Swagger specs.
argument-hint: "[GET|POST|PUT|DELETE] [endpoint] [--headers] [--data]"
---

# REST API Skill

Design, build, and interact with RESTful APIs.

## Quick Reference

```bash
# GET request
curl -s https://api.example.com/users | python3 -m json.tool

# GET with query params
curl -s "https://api.example.com/users?page=1&limit=10"

# POST with JSON
curl -X POST https://api.example.com/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John", "email": "john@example.com"}'

# PUT update
curl -X PUT https://api.example.com/users/123 \
  -H "Content-Type: application/json" \
  -d '{"name": "John Updated"}'

# DELETE
curl -X DELETE https://api.example.com/users/123

# With authentication
curl -H "Authorization: Bearer <token>" https://api.example.com/me
```

## HTTP Methods

| Method | Usage | Idempotent | Safe |
|--------|-------|------------|------|
| `GET` | Retrieve resource(s) | ✅ | ✅ |
| `POST` | Create resource | ❌ | ❌ |
| `PUT` | Replace resource | ✅ | ❌ |
| `PATCH` | Partial update | ❌ | ❌ |
| `DELETE` | Remove resource | ✅ | ❌ |
| `HEAD` | Get headers only | ✅ | ✅ |
| `OPTIONS` | Get allowed methods | ✅ | ✅ |

## Status Codes

| Range | Meaning | Common Codes |
|-------|---------|--------------|
| 2xx | Success | 200 OK, 201 Created, 204 No Content |
| 3xx | Redirect | 301 Moved, 304 Not Modified |
| 4xx | Client Error | 400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found, 422 Unprocessable |
| 5xx | Server Error | 500 Internal, 502 Bad Gateway, 503 Unavailable |

## URL Design Best Practices

```
# Resources (nouns, plural)
GET    /users              # List users
GET    /users/123          # Get user 123
POST   /users              # Create user
PUT    /users/123          # Replace user 123
PATCH  /users/123          # Update user 123
DELETE /users/123          # Delete user 123

# Nested resources
GET    /users/123/posts    # User's posts
POST   /users/123/posts    # Create post for user

# Filtering & pagination
GET    /users?status=active&role=admin
GET    /users?page=2&per_page=20
GET    /users?sort=created_at&order=desc

# Actions (when CRUD doesn't fit)
POST   /users/123/activate
POST   /orders/456/cancel
```

## Request/Response Examples

### Successful Response
```json
{
  "data": {
    "id": "123",
    "type": "user",
    "attributes": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  }
}
```

### Error Response
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input",
    "details": [
      {"field": "email", "message": "Invalid email format"}
    ]
  }
}
```

### Pagination Response
```json
{
  "data": [...],
  "meta": {
    "total": 100,
    "page": 1,
    "per_page": 20,
    "total_pages": 5
  },
  "links": {
    "self": "/users?page=1",
    "next": "/users?page=2",
    "last": "/users?page=5"
  }
}
```

## Headers

| Header | Purpose | Example |
|--------|---------|---------|
| `Content-Type` | Request body format | `application/json` |
| `Accept` | Expected response format | `application/json` |
| `Authorization` | Auth credentials | `Bearer <token>` |
| `X-Request-ID` | Request tracing | `uuid` |
| `Cache-Control` | Caching behavior | `max-age=3600` |
| `ETag` | Resource version | `"abc123"` |

## References

| Topic | File |
|-------|------|
| OpenAPI/Swagger | [openapi.md](references/openapi.md) |
| Authentication | [auth.md](references/auth.md) |
| Versioning | [versioning.md](references/versioning.md) |
| Rate Limiting | [rate-limiting.md](references/rate-limiting.md) |
