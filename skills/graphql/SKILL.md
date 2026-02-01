---
name: graphql
version: 1.0.0
description: GraphQL API development and interaction skill. Use when building GraphQL schemas, writing queries/mutations, introspecting APIs, or debugging GraphQL endpoints.
argument-hint: "[query|mutation|introspect] [endpoint] [--variables]"
---

# GraphQL Skill

Build, query, and debug GraphQL APIs effectively.

## Quick Reference

```bash
# Introspect a GraphQL endpoint
curl -X POST <endpoint> \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}'

# Execute a query
curl -X POST <endpoint> \
  -H "Content-Type: application/json" \
  -d '{"query": "query { users { id name } }"}'

# Execute a mutation with variables
curl -X POST <endpoint> \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation($input: CreateUserInput!) { createUser(input: $input) { id } }", "variables": {"input": {"name": "John"}}}'
```

## Schema Design Patterns

### Type Definitions
```graphql
type User {
  id: ID!
  name: String!
  email: String!
  posts: [Post!]!
  createdAt: DateTime!
}

type Post {
  id: ID!
  title: String!
  content: String!
  author: User!
}

input CreateUserInput {
  name: String!
  email: String!
}

type Query {
  user(id: ID!): User
  users(first: Int, after: String): UserConnection!
}

type Mutation {
  createUser(input: CreateUserInput!): User!
  updateUser(id: ID!, input: UpdateUserInput!): User!
  deleteUser(id: ID!): Boolean!
}
```

### Pagination (Relay-style)
```graphql
type UserConnection {
  edges: [UserEdge!]!
  pageInfo: PageInfo!
  totalCount: Int!
}

type UserEdge {
  node: User!
  cursor: String!
}

type PageInfo {
  hasNextPage: Boolean!
  hasPreviousPage: Boolean!
  startCursor: String
  endCursor: String
}
```

## Best Practices

| Practice | Description |
|----------|-------------|
| **Nullable by default** | Only use `!` when field is guaranteed non-null |
| **Use Input types** | Wrap mutation arguments in Input types |
| **Descriptive names** | `createUser`, not `addUser` or `newUser` |
| **Relay pagination** | Use Connection pattern for lists |
| **Error handling** | Use union types for error states |
| **Batching** | Use DataLoader to prevent N+1 queries |

## Error Handling Pattern

```graphql
union CreateUserResult = User | ValidationError | AuthError

type ValidationError {
  field: String!
  message: String!
}

type AuthError {
  message: String!
}

type Mutation {
  createUser(input: CreateUserInput!): CreateUserResult!
}
```

## References

| Topic | File |
|-------|------|
| Introspection | [introspection.md](references/introspection.md) |
| Subscriptions | [subscriptions.md](references/subscriptions.md) |
| Authentication | [auth.md](references/auth.md) |
| Performance | [performance.md](references/performance.md) |
