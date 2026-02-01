# OpenAPI / Swagger Specification

## Basic Structure (OpenAPI 3.0)

```yaml
openapi: 3.0.3
info:
  title: My API
  version: 1.0.0
  description: API description
  
servers:
  - url: https://api.example.com/v1
    description: Production
  - url: https://staging-api.example.com/v1
    description: Staging

paths:
  /users:
    get:
      summary: List users
      tags: [Users]
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            default: 1
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UserList'
    post:
      summary: Create user
      tags: [Users]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateUserInput'
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '400':
          $ref: '#/components/responses/BadRequest'

  /users/{id}:
    get:
      summary: Get user by ID
      tags: [Users]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '404':
          $ref: '#/components/responses/NotFound'

components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        email:
          type: string
          format: email
        createdAt:
          type: string
          format: date-time
      required: [id, name, email]
      
    CreateUserInput:
      type: object
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
      required: [name, email]
      
    UserList:
      type: object
      properties:
        data:
          type: array
          items:
            $ref: '#/components/schemas/User'
        meta:
          $ref: '#/components/schemas/PaginationMeta'
          
    PaginationMeta:
      type: object
      properties:
        total:
          type: integer
        page:
          type: integer
        perPage:
          type: integer
          
    Error:
      type: object
      properties:
        code:
          type: string
        message:
          type: string
      required: [code, message]

  responses:
    BadRequest:
      description: Bad Request
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    NotFound:
      description: Not Found
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
            
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

security:
  - bearerAuth: []
```

## Tools

| Tool | Purpose | Command |
|------|---------|---------|
| **Swagger Editor** | Edit specs | `docker run -p 8080:8080 swaggerapi/swagger-editor` |
| **Swagger UI** | View docs | `docker run -p 8080:8080 -e SWAGGER_JSON=/spec.yaml swaggerapi/swagger-ui` |
| **openapi-generator** | Generate code | `openapi-generator generate -i spec.yaml -g typescript-axios -o ./client` |
| **Redoc** | Beautiful docs | `npx redoc-cli bundle spec.yaml` |

## Validation

```bash
# Validate spec
npx @apidevtools/swagger-cli validate openapi.yaml

# Lint spec
npx @stoplight/spectral-cli lint openapi.yaml
```

## Generate from Code

### Express + swagger-jsdoc
```javascript
const swaggerJSDoc = require('swagger-jsdoc');

const options = {
  definition: {
    openapi: '3.0.0',
    info: { title: 'My API', version: '1.0.0' },
  },
  apis: ['./routes/*.js'],
};

const spec = swaggerJSDoc(options);
```

### FastAPI (Python)
```python
# Automatic OpenAPI generation
from fastapi import FastAPI
app = FastAPI()

# Access at /openapi.json
```
