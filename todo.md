# Kanban Monorepo TODO

## Background
You want a multi-repo kanban system where a single backend manages cards for multiple projects. Each project is associated with git metadata (local path and/or remote URL), while the actual kanban card storage lives outside tracked repos, in backend-managed markdown files. The markdown files are the source of truth.

The long-term system has four components:
1. Backend API (store and manage cards)
2. CLI (LLM-friendly primary interface)
3. Swift app (user-facing)
4. Web app (user-facing)

Current MVP scope is backend + API + full end-to-end test harness.

## What It Solves
- Centralized kanban management across many projects/repos.
- File-native persistence with human-readable markdown.
- Query performance via SQLite projection rebuilt from markdown at any time.
- Real-time notifications for clients (Swift/Web/CLI integrations).

## Confirmed Decisions (Q&A Log)
1. MVP starts with backend + API first.
- Answer: Yes.

2. Backend instance model.
- Answer: One backend manages many projects.

3. Project mapping to git metadata.
- Answer: Keep both local path and remote URL.

4. Storage location.
- Answer: Backend stores cards outside tracked project repos.

5. Required card fields for now.
- Answer: `status`, `column`, `title`, `description`, `comments`, `history`.

6. Comments/history storage format.
- Answer: Same markdown card file, sectioned event-log style.

7. Live updates between clients.
- Answer: Backend-only filesystem writes; websocket updates preferred (polling okay as backup later).

8. Concurrent edit conflict handling.
- Answer: Defer for now.

9. Auth model.
- Answer: Single-user, no auth for MVP.

10. SQLite role.
- Answer: SQLite is projection; markdown is source of truth; full rebuild must always be possible.

11. CLI behavior.
- Answer: LLM-friendly core commands + `primer` command (CLI not in current MVP implementation scope).

12. Client functionality now.
- Answer: Read-only first for Swift/Web later.

13. API style.
- Answer: REST for MVP.

14. Project creation behavior.
- Answer: Explicit `POST /projects`.

15. Card ID format.
- Answer: `<project-slug>/card-<sequential-number>`.

16. Allowed status values.
- Answer: hardcoded `Todo`, `Doing`, `Review`, `Done`.

17. Markdown format.
- Answer: YAML frontmatter + markdown body sections.

18. Description/comments/history mutation model.
- Answer: Append-only event-log sections in markdown.

19. Deletion model.
- Answer: Soft delete by default; optional hard delete with explicit force param.

20. Realtime events.
- Answer: Include operation events (`project.created`, `card.created`, `card.updated`, `card.moved`, `card.commented`, `card.deleted_soft`, `card.deleted_hard`).

21. Testing layers.
- Answer: Both in-process and real-process e2e tests.

22. Toolchain/version constraints.
- Answer: Use Go 1.26.

23. Dependency/test constraints.
- Answer: Run `go get -u` on deps and use `testify` for tests.

24. Development process constraint.
- Answer: TDD workflow (write failing test first, minimal implementation, then refactor).

25. Repo structure.
- Answer: Monorepo root with backend in `/backend`.

26. API evolution strategy suggestion.
- Answer: Add OpenAPI spec and generate clients for CLI/Swift/Web to keep clients in sync as API changes.

27. Backend framework direction.
- Answer: Migrate backend REST layer to `huma` (no longer MVP-only architecture), keep websocket on native route, and generate OpenAPI from registered operations.

## Architecture
- Monorepo root: `/Users/simonjohansson/src/kanban`
- Backend module: `/Users/simonjohansson/src/kanban/backend`
- Data layout:
  - `<data_dir>/projects/<project-slug>/project.md`
  - `<data_dir>/projects/<project-slug>/card-<n>.md`
- Markdown source of truth:
  - Project metadata in `project.md`
  - Card frontmatter + sections (`Description`, `Comments`, `History`)
- SQLite projection:
  - derived/upserted from backend operations
  - fully rebuildable from markdown snapshot
- API transport:
  - REST JSON via `huma` + websocket stream for events

## Future Enhancements
- OpenAPI contract for backend API.
- Client generation pipeline from OpenAPI for CLI/Swift/Web.
- Contract tests to ensure implementation matches OpenAPI.

## Plan
1. Lock Go module and dependencies (`go 1.26`, `testify`, upgraded deps).
2. Build API slices in TDD order:
- project lifecycle
- card lifecycle
- soft/hard delete behavior
- rebuild behavior
- websocket events
3. Build full e2e harness:
- in-process HTTP server tests
- black-box real server process tests
4. Refactor for clarity and maintainability after green tests.
5. Add OpenAPI spec and generation workflow after core API stabilizes.

## Todos
- [x] Finalize `backend/go.mod` + dependency upgrades.
- [x] Add first failing tests for project lifecycle.
- [x] Implement minimal project lifecycle API to pass tests.
- [x] Add failing tests for card lifecycle and markdown event sections.
- [x] Implement minimal card endpoints/operations.
- [x] Add failing tests for websocket event stream.
- [x] Implement websocket hub + event broadcasting.
- [x] Add failing tests for rebuild and real-process black-box harness.
- [x] Implement projection rebuild endpoint.
- [x] Refactor code while preserving green tests.
- [x] Add OpenAPI spec for backend API.
- [x] Add OpenAPI-based client generation workflow for future CLI/Swift/Web clients.
- [x] Add e2e test that uses generated OpenAPI client for full operation flow.
- [x] Run full test suite and report results.

## Done
- [x] Requirements elicitation and scope lock.
- [x] Monorepo structure started with backend module directory.
- [x] Initial backend scaffolding started (now being validated and completed in TDD flow).
- [x] Added this background/context tracker in root `todo.md`.
- [x] Noted OpenAPI + generated-client direction.
- [x] Implemented backend service entrypoint in `/Users/simonjohansson/src/kanban/backend/cmd/kanban-backend/main.go`.
- [x] Implemented REST endpoints for projects and card lifecycle.
- [x] Implemented markdown source-of-truth store with frontmatter + event-log sections.
- [x] Implemented SQLite projection and rebuild flow from markdown snapshot.
- [x] Implemented websocket event stream endpoint (`/ws`).
- [x] Added OpenAPI spec at `/Users/simonjohansson/src/kanban/backend/api/openapi.yaml` and served endpoint `GET /openapi.yaml`.
- [x] Added OpenAPI validation/client generation workflow via `/Users/simonjohansson/src/kanban/backend/Makefile`.
- [x] Added generated OpenAPI client at `/Users/simonjohansson/src/kanban/backend/internal/gen/client/client.gen.go`.
- [x] Added generated-client e2e flow test at `/Users/simonjohansson/src/kanban/backend/e2e_generated_client_test.go`.
- [x] Added backend logging (startup/shutdown, HTTP request logs, and operation-level action logs).
- [x] Added e2e backend log streaming to test output for black-box and generated-client tests.
- [x] Migrated backend API registration/contract to Huma with chi adapter.
- [x] Added OpenAPI export command at `/Users/simonjohansson/src/kanban/backend/cmd/export-openapi/main.go`.
- [x] Updated OpenAPI workflow so `make openapi-sync` exports spec from running API registration.
- [x] Added integration/e2e tests (in-process + black-box process) with filesystem + sqlite assertions.
- [x] Full backend test suite is green.
- [x] Increased backend server package coverage to `83.6%`.

## Open Questions
- None currently blocking implementation.
