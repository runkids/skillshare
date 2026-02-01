# GraphQL Performance Optimization

## N+1 Query Problem

### The Problem

```graphql
query {
  posts {      # 1 query
    author {   # N queries (one per post)
      name
    }
  }
}
```

### Solution: DataLoader

```javascript
import DataLoader from 'dataloader';

// Create loader
const userLoader = new DataLoader(async (userIds) => {
  const users = await User.findByIds(userIds);
  const userMap = new Map(users.map(u => [u.id, u]));
  return userIds.map(id => userMap.get(id));
});

// In resolver
const resolvers = {
  Post: {
    author: (post, _, { loaders }) => {
      return loaders.user.load(post.authorId);
    },
  },
};

// Context setup (new loader per request)
const context = () => ({
  loaders: {
    user: new DataLoader(batchUsers),
  },
});
```

## Query Complexity Analysis

```javascript
import { createComplexityLimitRule } from 'graphql-validation-complexity';

const complexityRule = createComplexityLimitRule(1000, {
  scalarCost: 1,
  objectCost: 10,
  listFactor: 20,
  introspectionListFactor: 2,
});

const server = new ApolloServer({
  typeDefs,
  resolvers,
  validationRules: [complexityRule],
});
```

### Manual Complexity

```graphql
type Query {
  users(first: Int!): [User!]! @complexity(value: 10, multipliers: ["first"])
}
```

## Query Depth Limiting

```javascript
import depthLimit from 'graphql-depth-limit';

const server = new ApolloServer({
  typeDefs,
  resolvers,
  validationRules: [depthLimit(10)],
});
```

## Caching

### Response Caching (Apollo)

```graphql
type Query {
  user(id: ID!): User @cacheControl(maxAge: 60)
}

type User @cacheControl(maxAge: 120) {
  id: ID!
  name: String!
  email: String! @cacheControl(maxAge: 0)  # Never cache
}
```

### Persisted Queries

```javascript
// Client sends hash instead of full query
{
  "extensions": {
    "persistedQuery": {
      "version": 1,
      "sha256Hash": "abc123..."
    }
  },
  "variables": { "id": "1" }
}
```

## Pagination Best Practices

### Cursor-based (Recommended)

```graphql
query {
  users(first: 20, after: "cursor123") {
    edges {
      node { id name }
      cursor
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
```

### Limit page size

```javascript
const resolvers = {
  Query: {
    users: (_, { first = 20 }) => {
      const limit = Math.min(first, 100); // Cap at 100
      return User.findMany({ take: limit });
    },
  },
};
```

## Field Selection Optimization

```javascript
import graphqlFields from 'graphql-fields';

const resolvers = {
  Query: {
    users: (_, args, context, info) => {
      const requestedFields = graphqlFields(info);
      
      // Only select requested columns
      const select = Object.keys(requestedFields);
      return User.findMany({ select });
    },
  },
};
```

## Monitoring

- **Apollo Studio**: Built-in tracing and analytics
- **OpenTelemetry**: Distributed tracing
- **Custom plugins**: Log slow queries

```javascript
const slowQueryPlugin = {
  requestDidStart() {
    const start = Date.now();
    return {
      willSendResponse({ request }) {
        const duration = Date.now() - start;
        if (duration > 1000) {
          console.warn(`Slow query (${duration}ms):`, request.query);
        }
      },
    };
  },
};
```
