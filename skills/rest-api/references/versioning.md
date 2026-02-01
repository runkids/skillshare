# REST API Versioning

## Versioning Strategies

### 1. URL Path Versioning (Recommended)

```
GET /v1/users
GET /v2/users
```

**Pros:** Clear, cacheable, easy routing
**Cons:** URL pollution

```javascript
// Express routing
const v1Router = require('./routes/v1');
const v2Router = require('./routes/v2');

app.use('/v1', v1Router);
app.use('/v2', v2Router);
```

### 2. Header Versioning

```bash
curl -H "Accept: application/vnd.api+json; version=2" \
  https://api.example.com/users

# Or custom header
curl -H "API-Version: 2" https://api.example.com/users
```

**Pros:** Clean URLs
**Cons:** Hidden, harder to test

### 3. Query Parameter

```
GET /users?version=2
```

**Pros:** Easy to test
**Cons:** Awkward, pollutes query params

### 4. Content Negotiation (Media Type)

```bash
curl -H "Accept: application/vnd.company.v2+json" \
  https://api.example.com/users
```

**Pros:** RESTful, flexible
**Cons:** Complex, less discoverable

## Version Lifecycle

| Stage | Description | Duration |
|-------|-------------|----------|
| **Current** | Active development | Ongoing |
| **Supported** | Bug fixes only | 6-12 months |
| **Deprecated** | Warnings, no fixes | 3-6 months |
| **Sunset** | Removed | - |

## Deprecation Headers

```http
HTTP/1.1 200 OK
Deprecation: Sun, 01 Jan 2025 00:00:00 GMT
Sunset: Sun, 01 Jul 2025 00:00:00 GMT
Link: <https://api.example.com/v2/users>; rel="successor-version"
```

## Backward Compatibility Tips

### Adding Fields (Safe)
```json
// v1
{ "name": "John" }

// v2 - adds email (backward compatible)
{ "name": "John", "email": "john@example.com" }
```

### Changing Field Names (Breaking)
```json
// Instead of renaming, add alias
{
  "name": "John",           // Keep old
  "fullName": "John Doe"    // Add new
}
```

### Removing Fields
```json
// 1. Deprecate first (return null, add warning)
{ "oldField": null, "newField": "value" }

// 2. Remove in next major version
{ "newField": "value" }
```

## Migration Guide Template

```markdown
# Migrating from v1 to v2

## Breaking Changes
- `user.name` renamed to `user.fullName`
- Removed `user.legacyId` field

## New Features
- Added `user.avatar` field
- New `/users/search` endpoint

## Deprecations
- `/users/find` deprecated, use `/users/search`

## Migration Steps
1. Update client to use new field names
2. Test with v2 endpoint
3. Remove v1 references
```

## Semantic Versioning for APIs

```
v{MAJOR}.{MINOR}

MAJOR: Breaking changes
MINOR: Backward-compatible additions

Examples:
- v1 → v2: Breaking change
- v1.1: New optional field
- v1.2: New endpoint
```
