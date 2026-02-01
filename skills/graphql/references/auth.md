# GraphQL Authentication & Authorization

## Authentication Methods

### Bearer Token (JWT)

```bash
curl -X POST https://api.example.com/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -d '{"query": "{ me { id name } }"}'
```

### API Key

```bash
curl -X POST https://api.example.com/graphql \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"query": "{ users { id } }"}'
```

## Server-Side Context

```javascript
const server = new ApolloServer({
  typeDefs,
  resolvers,
  context: async ({ req }) => {
    const token = req.headers.authorization?.replace('Bearer ', '');
    
    if (token) {
      try {
        const user = await verifyToken(token);
        return { user };
      } catch (e) {
        // Invalid token - continue as unauthenticated
      }
    }
    
    return { user: null };
  },
});
```

## Authorization Patterns

### Field-Level Authorization

```javascript
const resolvers = {
  User: {
    email: (user, _, { currentUser }) => {
      // Only show email to the user themselves or admins
      if (currentUser?.id === user.id || currentUser?.role === 'ADMIN') {
        return user.email;
      }
      return null;
    },
  },
};
```

### Directive-Based

```graphql
directive @auth(requires: Role = USER) on FIELD_DEFINITION

enum Role {
  ADMIN
  USER
  GUEST
}

type Query {
  users: [User!]! @auth(requires: ADMIN)
  me: User @auth(requires: USER)
  publicPosts: [Post!]!
}
```

```javascript
class AuthDirective extends SchemaDirectiveVisitor {
  visitFieldDefinition(field) {
    const { requires } = this.args;
    const originalResolve = field.resolve;
    
    field.resolve = async function (...args) {
      const context = args[2];
      
      if (!context.user) {
        throw new AuthenticationError('Not authenticated');
      }
      
      if (!hasRole(context.user, requires)) {
        throw new ForbiddenError('Not authorized');
      }
      
      return originalResolve.apply(this, args);
    };
  }
}
```

### Shield Library

```javascript
import { shield, rule, allow, deny } from 'graphql-shield';

const isAuthenticated = rule()((parent, args, { user }) => !!user);
const isAdmin = rule()((parent, args, { user }) => user?.role === 'ADMIN');
const isOwner = rule()((parent, args, { user }) => parent.userId === user?.id);

const permissions = shield({
  Query: {
    users: isAdmin,
    me: isAuthenticated,
    publicPosts: allow,
  },
  Mutation: {
    createPost: isAuthenticated,
    deleteUser: isAdmin,
  },
  User: {
    email: or(isOwner, isAdmin),
  },
});
```

## Error Handling

```javascript
import { AuthenticationError, ForbiddenError } from 'apollo-server';

// In resolvers
if (!context.user) {
  throw new AuthenticationError('You must be logged in');
}

if (context.user.role !== 'ADMIN') {
  throw new ForbiddenError('Admin access required');
}
```

Response format:
```json
{
  "errors": [{
    "message": "You must be logged in",
    "extensions": {
      "code": "UNAUTHENTICATED"
    }
  }]
}
```
