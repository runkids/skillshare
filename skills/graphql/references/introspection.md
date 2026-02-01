# GraphQL Introspection

## Full Schema Introspection

```graphql
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type { ...TypeRef }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
        }
      }
    }
  }
}
```

## Quick Type Lookup

```graphql
# List all types
query { __schema { types { name kind } } }

# Get specific type details
query { __type(name: "User") { 
  name 
  fields { name type { name kind } } 
}}

# Get all queries
query { __schema { queryType { fields { name } } } }

# Get all mutations
query { __schema { mutationType { fields { name } } } }
```

## cURL Examples

```bash
# Get all type names
curl -s -X POST https://api.example.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}' | python3 -m json.tool

# Get User type fields
curl -s -X POST https://api.example.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __type(name: \"User\") { fields { name type { name } } } }"}'
```

## Tools

- **GraphQL Playground**: Interactive IDE
- **GraphiQL**: In-browser GraphQL IDE
- **Insomnia**: API client with GraphQL support
- **Apollo Studio**: Schema management and monitoring
