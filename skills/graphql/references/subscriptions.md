# GraphQL Subscriptions

## Schema Definition

```graphql
type Subscription {
  messageAdded(channelId: ID!): Message!
  userStatusChanged(userId: ID!): UserStatus!
  orderUpdated(orderId: ID!): Order!
}

type Message {
  id: ID!
  content: String!
  author: User!
  createdAt: DateTime!
}
```

## Client Implementation (JavaScript)

### Using graphql-ws

```javascript
import { createClient } from 'graphql-ws';

const client = createClient({
  url: 'wss://api.example.com/graphql',
  connectionParams: {
    authToken: 'your-token',
  },
});

// Subscribe
const unsubscribe = client.subscribe(
  {
    query: `subscription ($channelId: ID!) {
      messageAdded(channelId: $channelId) {
        id
        content
        author { name }
      }
    }`,
    variables: { channelId: '123' },
  },
  {
    next: (data) => console.log('New message:', data),
    error: (err) => console.error('Error:', err),
    complete: () => console.log('Subscription complete'),
  }
);

// Later: unsubscribe
unsubscribe();
```

### Using Apollo Client

```javascript
import { ApolloClient, InMemoryCache, split, HttpLink } from '@apollo/client';
import { GraphQLWsLink } from '@apollo/client/link/subscriptions';
import { getMainDefinition } from '@apollo/client/utilities';
import { createClient } from 'graphql-ws';

const httpLink = new HttpLink({ uri: 'https://api.example.com/graphql' });

const wsLink = new GraphQLWsLink(createClient({
  url: 'wss://api.example.com/graphql',
}));

const splitLink = split(
  ({ query }) => {
    const definition = getMainDefinition(query);
    return (
      definition.kind === 'OperationDefinition' &&
      definition.operation === 'subscription'
    );
  },
  wsLink,
  httpLink,
);

const client = new ApolloClient({
  link: splitLink,
  cache: new InMemoryCache(),
});
```

## Server Implementation (Node.js)

```javascript
import { createServer } from 'http';
import { WebSocketServer } from 'ws';
import { useServer } from 'graphql-ws/lib/use/ws';
import { schema } from './schema';
import { PubSub } from 'graphql-subscriptions';

const pubsub = new PubSub();

const resolvers = {
  Subscription: {
    messageAdded: {
      subscribe: (_, { channelId }) => 
        pubsub.asyncIterator([`MESSAGE_ADDED_${channelId}`]),
    },
  },
  Mutation: {
    sendMessage: async (_, { channelId, content }, { user }) => {
      const message = await createMessage({ channelId, content, authorId: user.id });
      pubsub.publish(`MESSAGE_ADDED_${channelId}`, { messageAdded: message });
      return message;
    },
  },
};

const server = createServer();
const wsServer = new WebSocketServer({ server, path: '/graphql' });
useServer({ schema }, wsServer);

server.listen(4000);
```

## Best Practices

1. **Use Redis PubSub** for production (not in-memory PubSub)
2. **Implement connection authentication** via `connectionParams`
3. **Handle reconnection** on the client side
4. **Rate limit subscriptions** to prevent abuse
5. **Use filters** to send only relevant updates
