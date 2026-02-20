# OpenAPI

The backend contract lives in `/Users/simonjohansson/src/kanban/backend/api/openapi.yaml` and is served by the backend at `GET /openapi.yaml`.

## Validate

```bash
cd /Users/simonjohansson/src/kanban/backend
make openapi-validate
```

## Generate Go client (for CLI)

```bash
cd /Users/simonjohansson/src/kanban/backend
make openapi-gen-go-client
```

Output:
- `/Users/simonjohansson/src/kanban/backend/internal/gen/client/client.gen.go`

## Generate other clients

Use the same OpenAPI file with your preferred generator:
- Swift: `openapi-generator-cli -g swift5`
- TypeScript/Web: `openapi-generator-cli -g typescript-fetch`

Recommended workflow for all clients:
1. Update backend routes and tests.
2. Update `openapi.yaml` in the same PR.
3. Regenerate clients.
4. Run client compile/tests in consuming repos.
