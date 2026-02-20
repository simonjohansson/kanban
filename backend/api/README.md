# OpenAPI

The backend contract is generated from Huma-registered API operations and served by the backend at `GET /openapi.yaml`.
The checked-in spec snapshot lives at `/Users/simonjohansson/src/kanban/backend/api/openapi.yaml`.

## Sync From API

```bash
cd /Users/simonjohansson/src/kanban/backend
make openapi-sync
```

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
2. Run `make openapi-sync` and commit the updated `openapi.yaml`.
3. Regenerate clients.
4. Run client compile/tests in consuming repos.
