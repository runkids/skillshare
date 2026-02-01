# REST API Rate Limiting

## Rate Limit Headers

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Max requests per window |
| `X-RateLimit-Remaining` | Requests left in window |
| `X-RateLimit-Reset` | Unix timestamp when window resets |
| `Retry-After` | Seconds until retry (on 429) |

## 429 Too Many Requests

```bash
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 60

{
  "error": "RATE_LIMIT_EXCEEDED",
  "message": "Too many requests. Try again in 60 seconds.",
  "retryAfter": 60
}
```

## Common Rate Limit Strategies

### 1. Fixed Window

```
100 requests per minute
Window: 00:00 - 00:59
```

**Pros:** Simple
**Cons:** Burst at window edges

### 2. Sliding Window

```
100 requests per rolling 60 seconds
```

**Pros:** Smoother distribution
**Cons:** More complex to implement

### 3. Token Bucket

```
Bucket size: 100 tokens
Refill rate: 10 tokens/second
```

**Pros:** Allows controlled bursts
**Cons:** More complex

## Client-Side Handling

```javascript
async function fetchWithRetry(url, options = {}, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    const response = await fetch(url, options);
    
    if (response.status === 429) {
      const retryAfter = response.headers.get('Retry-After') || 60;
      console.log(`Rate limited. Retrying in ${retryAfter}s...`);
      await sleep(retryAfter * 1000);
      continue;
    }
    
    return response;
  }
  throw new Error('Max retries exceeded');
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
```

## Exponential Backoff

```javascript
async function exponentialBackoff(fn, maxRetries = 5) {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      return await fn();
    } catch (error) {
      if (error.status !== 429 || attempt === maxRetries - 1) {
        throw error;
      }
      
      const delay = Math.min(1000 * Math.pow(2, attempt), 32000);
      const jitter = Math.random() * 1000;
      await sleep(delay + jitter);
    }
  }
}
```

## Server-Side Implementation

### Express (express-rate-limit)

```javascript
const rateLimit = require('express-rate-limit');

const limiter = rateLimit({
  windowMs: 60 * 1000, // 1 minute
  max: 100,
  standardHeaders: true,
  legacyHeaders: false,
  message: {
    error: 'RATE_LIMIT_EXCEEDED',
    message: 'Too many requests'
  }
});

app.use('/api/', limiter);
```

### Redis-based (for distributed systems)

```javascript
const Redis = require('ioredis');
const redis = new Redis();

async function checkRateLimit(key, limit, windowSec) {
  const current = await redis.incr(key);
  
  if (current === 1) {
    await redis.expire(key, windowSec);
  }
  
  return {
    allowed: current <= limit,
    remaining: Math.max(0, limit - current),
    reset: await redis.ttl(key)
  };
}
```

## Best Practices

| Practice | Description |
|----------|-------------|
| **Return headers** | Always include rate limit info |
| **Document limits** | State limits in API docs |
| **Graceful 429** | Provide retry-after info |
| **Different tiers** | Higher limits for paid plans |
| **Key by user** | Not just IP (for auth'd APIs) |
| **Separate limits** | Per endpoint if needed |
