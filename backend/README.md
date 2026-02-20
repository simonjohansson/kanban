# Kanban Backend

Go backend for the multi-project kanban system.

Framework: `huma` (with `chi` adapter). The OpenAPI document is generated from registered operations and served at `GET /openapi.yaml`.
SQLite projection queries are generated with `sqlc`.

## Run

```bash
cd /Users/simonjohansson/src/kanban/backend
go run ./cmd/kanban serve --addr 127.0.0.1:8080 --cards-path /tmp/kanban-data/cards --sqlite-path /tmp/kanban-data/projection.db
```

Storage/runtime config sources (highest precedence first):
1. flags: `kanban serve --cards-path ... --sqlite-path ... --addr ...`
2. env: `KANBAN_CARDS_PATH`, `KANBAN_SQLITE_PATH`, `KANBAN_SERVER_URL`
3. config file: `~/.config/kanban/config.yaml`

`--data-dir` is still accepted as a deprecated alias for `--cards-path`.

## Sample API Flow (curl)

Set base URL:

```bash
BASE_URL="http://127.0.0.1:8080"
```

1. Add a project

```bash
curl -sS -X POST "$BASE_URL/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Demo Project",
    "local_path": "/Users/simonjohansson/src/some-repo",
    "remote_url": "https://github.com/example/some-repo.git"
  }'
```

2. Show projects (includes the one you just added)

```bash
curl -sS "$BASE_URL/projects"
```

3. Add a card to the project (`demo-project`)

```bash
curl -sS -X POST "$BASE_URL/projects/demo-project/cards" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Set up CI",
    "description": "Create initial CI pipeline",
    "status": "Todo"
  }'
```

4. Show the card (`card-1`)

```bash
curl -sS "$BASE_URL/projects/demo-project/cards/1"
```

5. Add a comment on the card

```bash
curl -sS -X POST "$BASE_URL/projects/demo-project/cards/1/comments" \
  -H "Content-Type: application/json" \
  -d '{
    "body": "Please include macOS + Linux jobs"
  }'
```

6. Update (append) description

```bash
curl -sS -X PATCH "$BASE_URL/projects/demo-project/cards/1/description" \
  -H "Content-Type: application/json" \
  -d '{
    "body": "Also add branch protection checks"
  }'
```

7. Show the card again

```bash
curl -sS "$BASE_URL/projects/demo-project/cards/1"
```

## Useful Endpoints

- `GET /health`
- `GET /client-config`
- `GET /openapi.yaml`
- `GET /ws`
- `POST /admin/rebuild`

## OpenAPI And Client Generation

```bash
cd /Users/simonjohansson/src/kanban/backend
make openapi-sync
make openapi-validate
make openapi-gen-go-client
```

Generated client output:
- `/Users/simonjohansson/src/kanban/backend/gen/client/client.gen.go`

## SQLC Generation

```bash
cd /Users/simonjohansson/src/kanban/backend
make sqlc-generate
```

Inputs:
- `/Users/simonjohansson/src/kanban/backend/sqlc.yaml`
- `/Users/simonjohansson/src/kanban/backend/internal/store/sql/schema.sql`
- `/Users/simonjohansson/src/kanban/backend/internal/store/sql/queries.sql`

Generated output:
- `/Users/simonjohansson/src/kanban/backend/internal/store/sqlcgen`
