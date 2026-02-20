# Kanban Monorepo TODO

## Background
You want a multi-repo kanban system where a single backend manages cards for multiple projects. Each project is associated with git metadata (local path and/or remote URL), while the actual kanban card storage lives outside tracked repos, in backend-managed markdown files. The markdown files are the source of truth.

The long-term system has four components:
1. Backend API (store and manage cards)
2. CLI (LLM-friendly primary interface)
3. Swift app (user-facing)
4. Web app (user-facing)

Backend phase is complete and no longer treated as MVP-only. Current focus is productionizing the CLI as a separate module.

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

28. Preferred implementation order.
- Answer: `e2e/black-box tests -> unit tests -> implementation` (enforced in AGENTS.md).

29. CLI packaging and runtime behavior.
- Answer: `kb` as separate module at `/Users/simonjohansson/src/kanban/cli`, HTTP-only, non-interactive.
- Answer: Config precedence is `flags > env > ~/.config/kb/config.yaml`.
- Answer: First run writes defaults (localhost backend URL, sqlite/cards path under `~/.local/state/kanban`) and rewrites config when fields are missing.
- Answer: CLI commands include project/card/watch/primer set with explicit `--project` for card operations.
- Answer: `watch` streams all websocket events unless `--project` filter is set.

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
1. Update backend/public client surface for CLI consumption (`gen/client`, project delete endpoint).
2. Create `/Users/simonjohansson/src/kanban/cli` as separate Go module (`kb`).
3. Build CLI in required order:
- black-box/e2e harness first (compile CLI + run backend process + multi-project flows + watch assertions with logging)
- unit tests second
- implementation third
4. Add monorepo workspace and automation targets for CLI/backend testing.
5. Refactor after green tests.

## Active Work Tracking (Before Implementation)
### CLI rewrite from scratch (Cobra-only)
- Context:
  - User requested complete reset of `/Users/simonjohansson/src/kanban/cli`.
  - No legacy parser or facade code should remain.
  - `kb` with no args must show help.
  - `primer` remains a dedicated command.
- Plan:
  1. Recreate `cli` module from scratch with clean structure.
  2. Write failing black-box tests for help and core command flows.
  3. Write focused unit tests for formatter/config/websocket URL helpers.
  4. Implement Cobra command tree and command handlers.
  5. Run full CLI tests and adjust docs/make targets as needed.
- Checklist:
  - [x] Remove old `cli` implementation and recreate empty module directory.
  - [x] Add failing black-box tests for `kb --help`, `kb` (no args), and `primer`.
  - [x] Add failing command tests for `project` and `card` flows against test server.
  - [x] Implement Cobra root command with persistent flags and extensive help text.
  - [x] Implement `project`, `card`, `watch`, and `primer` commands via generated API client.
  - [x] Add/update unit tests for output/error formatting and websocket URL handling.
  - [x] Run CLI unit + black-box tests until green.

### CLI UX consistency polish (Cobra)
- Context:
  - User requested command naming/help UX consistency improvements after rewrite.
- Plan:
  1. Define a consistent alias strategy for nouns and common verbs.
  2. Add shorthand flags for repeated card/project inputs.
  3. Normalize help text wording and examples.
  4. Validate with black-box tests.
- Checklist:
  - [x] Add failing black-box tests for aliases and shorthand flags.
  - [x] Add failing black-box tests for help wording/alias visibility.
  - [x] Implement aliases and shorthand flags in Cobra command tree.
  - [x] Normalize command short/long help strings and examples.
  - [x] Run full CLI test suite until green.

### Config ownership: move storage paths to backend
- Context:
  - `sqlite_path` and `cards_path` should be backend-scoped configuration, not CLI-scoped.
  - Desired flow: run backend directly with storage configuration, while CLI stays HTTP-only.
- Plan:
  1. Remove storage-path settings from CLI config/env/flags/help.
  2. Ensure backend owns and documents storage-path config (`sqlite` + cards storage path).
  3. Add/adjust root launch UX so running backend with scoped config is straightforward.
  4. Validate with backend and CLI tests.
- Checklist:
  - [x] Remove `sqlite_path`/`cards_path` from CLI `Config`, env parsing, persistent flags, and primer/help output.
  - [x] Keep CLI config focused on `server_url` + output mode.
  - [ ] Add backend tasks for explicit storage config surface (flags/env/docs) scoped to backend runtime.
  - [ ] Decide/implement backend naming consistency for cards storage (`--cards-path` alias vs existing `--data-dir` semantics).
  - [ ] Add backend tests for storage path configuration and startup behavior.
  - [ ] Add root-level run guidance/target (or launcher) for starting backend with storage options.

### Primer quality improvements
- Context:
  - Primer should be optimized for machine/LLM agent usage with explicit execution contracts.
- Checklist:
  - [x] Add failing tests for richer text/JSON primer content.
  - [x] Rewrite primer text into machine-oriented sections (system prompt, rules, templates).
  - [x] Extend JSON primer payload with execution rules, command templates, and agent prompt.
  - [x] Run CLI tests and verify primer output.

### Primer feedback hardening
- Context:
  - External LLM review identified missing operational details required for robust agent chaining.
- Checklist:
  - [ ] Add failing tests for response shapes and ID semantics.
  - [ ] Add failing tests for JSON error shape and delete semantics.
  - [ ] Add failing tests for `desc` semantics and project command support matrix.
  - [ ] Add failing tests for watch event schema and status-at-creation rule.
  - [ ] Update primer text/json payload to include all above details.
  - [ ] Run CLI tests until green.

### Backend service-layer refactor (strict sync)
- Goal: remove orchestration from Huma handlers and centralize markdown+projection+events in service layer.
- Policy: markdown is authoritative; if projection sync fails in request path, return `500` and do not emit success event.
- Execution checklist:
  - [x] Add failing service tests for orchestration and error typing.
  - [x] Implement `internal/service` with typed errors and write/read use-cases.
  - [x] Wire `internal/server` to call service methods only for business operations.
  - [x] Add centralized `service error -> huma error` mapping helper.
  - [x] Run backend tests and resolve regressions.

### CLI workstream status (carry-over, do not lose track)
- Completed:
  - [x] Create separate CLI module at `/Users/simonjohansson/src/kanban/cli`.
  - [x] Add monorepo workspace at `/Users/simonjohansson/src/kanban/go.work`.
  - [x] Add black-box CLI e2e harness skeleton at `/Users/simonjohansson/src/kanban/cli/e2e_blackbox_test.go` (expected red until CLI exists).
  - [x] Move generated OpenAPI Go client to backend public package `/Users/simonjohansson/src/kanban/backend/gen/client` for cross-module import.
- Remaining:
  - [x] Run CLI e2e to capture explicit red baseline.
  - [x] Add CLI unit tests (config precedence, output format, error JSON, watch stream behavior).
  - [x] Implement `kb` commands with OpenAPI client usage.
  - [x] Add make targets for CLI e2e with backend lifecycle management and logs.
  - [x] Run full monorepo tests.

### Active follow-up: Primer/LLM contract hardening
- Goal: make `kb primer` explicit enough for deterministic LLM chaining without guessing.
- Checklist:
  - [x] Document command response shapes (JSON examples/fields).
  - [x] Document card ID semantics (`--id` numeric per-project sequence and composite ID format).
  - [x] Document JSON error shape and handling expectations.
  - [x] Clarify soft vs hard delete behavior.
  - [x] Clarify `card describe` semantics vs `card get`.
  - [x] Clarify project command coverage (what exists and what does not).
  - [x] Document watch event JSON shape.
  - [x] Document allowed statuses at create/move.
  - [x] Re-run CLI + monorepo tests.

### Active follow-up: Unified config file for backend + CLI
- Goal: move runtime config to shared `~/.config/kanban/config.yaml` and namespace fields cleanly.
- Desired shape:
  - top-level: `server_url`
  - `backend`: `sqlite_path`, `cards_path`
  - `cli`: `output`
- Checklist:
  - [x] Add/update shared config schema + loader tests (failing first).
  - [x] Wire backend to read shared config defaults from shared path.

### Active follow-up: Swift app e2e UI harness
- Goal: add a real process-level Swift e2e test that proves sidebar updates from backend state.
- Requested acceptance flow:
  1. Start backend on random port with random storage paths.
  2. Start Swift app and assert initial project list is empty.
  3. Create a project via API client/CLI.
  4. Assert the project appears in the app sidebar list.
- Implementation approach:
  - Keep SwiftPM setup and add a dedicated e2e test target.
  - Launch real backend process (`go run`) from test with temp cards/sqlite locations.
  - Launch real app process from test with isolated HOME/config pointing to backend URL.
  - Add a small test-only sidebar state probe (env-gated) so tests can assert rendered list model deterministically.
  - Add make target for running Swift e2e tests directly.
- Checklist:
  - [x] Add failing Swift e2e test for initial empty project list.
  - [x] Add failing Swift e2e test step for create-project then sidebar update.
  - [x] Implement app env-gated sidebar probe output.
  - [x] Implement e2e process harness helpers (backend/app lifecycle, random port, temp dirs, log forwarding).
  - [x] Add make target for swift e2e tests.
  - [x] Run swift e2e + full swift tests until green.
  - [x] Force e2e app process to isolated backend via `KANBAN_SERVER_URL` override (avoid using user local backend/config).
  - [x] Extend e2e flow to create two projects and verify both appear in sidebar.
  - [x] Extend e2e flow to simulate row click and verify selected project highlight state.
  - [x] Add failing unit tests for sidebar probe selection command/state payload.
  - [x] Implement test-only sidebar selection hook used by e2e click simulation.
  - [x] Re-run Swift unit + e2e tests until green.
  - [x] Wire CLI to read shared config defaults from shared path.
  - [x] Keep precedence behavior with flags/env overriding file values.
  - [x] Add/adjust tests for path + namespaced field behavior.
  - [x] Re-run CLI + backend + monorepo tests.

### Active follow-up: Single unified `kanban` binary (backend + CLI)
- Goal: replace separate backend+CLI executables with one binary named `kanban`.
- Product decisions:
  - Root command shows help by default.
  - `serve` subcommand starts backend server.
  - Existing CLI operations remain over HTTP via generated OpenAPI client.
  - Precedence remains `flags > env > config`.
  - Break immediately to unified binary (no compatibility wrappers).
  - Update related tooling/docs/tests (including Swift app harness references).
- Plan:
  1. Add failing tests for unified command behavior (`kanban` root help and `kanban serve` process flow).
  2. Port CLI runtime into backend module and rename command surface from `kb` to `kanban`.
  3. Add `serve` command in same command tree using backend runtime config defaults.
  4. Switch backend e2e tests/build paths to `cmd/kanban`.
  5. Move/update CLI unit/e2e tests into backend module.
  6. Update root Makefile/go.work/docs and Swift e2e harness backend launch command.
  7. Run full backend + Swift tests.
- Checklist:
  - [x] Add failing unified-binary e2e tests in backend for root help and CLI command flows.
  - [x] Implement `cmd/kanban` entrypoint with root help default.
  - [x] Implement `serve` subcommand and migrate backend startup logic.
  - [x] Port CLI command package into backend module and rename to `kanban`.
  - [x] Update primer/templates/help text from `kb` to `kanban`.
  - [x] Update backend/Swift test harnesses to start server via `kanban serve`.
  - [x] Update root Makefile and `go.work` for single-binary workflow.
  - [x] Remove `cli` module from active monorepo workspace/test flow (`go.work` + root `Makefile`).
  - [x] Run and pass backend tests + Swift tests/e2e.

### Active follow-up: macOS Swift app foundation
- Goal: create a native macOS SwiftUI app with left project pane + empty right pane, backed by API + websocket updates.
- Location: `/Users/simonjohansson/src/kanban/apps/kanban-macos`
- Scope for this step:
  - main window with `NavigationSplitView`
  - sidebar listing projects from backend
  - right pane placeholder (empty state)
  - config from `~/.config/kanban/config.yaml` (`server_url` fallback to localhost)
  - websocket subscription and project list refresh on project events
  - row hover tooltip showing local/remote paths when available
  - alert on load/stream failures
  - generated API client from OpenAPI and used by app
- TDD checklist:
  - [x] Add failing tests for config loading/fallback behavior.
  - [x] Add failing tests for project store (initial load + websocket-triggered refresh).
  - [x] Add failing tests for view model state + alert behavior.
  - [x] Scaffold Swift package app + OpenAPI generation target.
  - [x] Implement minimal app/services to satisfy tests.
  - [x] Run Swift package tests and fix regressions.
  - [x] Add root Makefile target to compile and launch the Swift app.

### Active follow-up: Web frontend (Svelte) served by backend
- Goal: add a Svelte frontend that can run standalone in Node for development/testing and also be embedded/served by Go backend in production.
- Confirmed decisions:
  - Path/name: `/Users/simonjohansson/src/kanban/apps/kanban-web`.
  - Stack: Svelte + TypeScript.
  - Node: latest LTS.
  - Runtime mode: choose easiest standalone mode.
  - Backend routing: serve frontend at `/` with SPA fallback for unknown non-API routes.
  - Scope for now: minimal parity with Swift shell (left project pane + empty main pane + realtime project updates via websocket).
  - Browser tests: Playwright headless by default.
  - API integration: use generated client from backend OpenAPI.
- Requirements:
  - Frontend can be run independently (dev server + browser e2e tests).
  - Backend serves built frontend when running `kanban serve`.
  - Keep API communication over HTTP/WebSocket against backend endpoints.
- Plan:
  1. Scaffold frontend project in monorepo (`/Users/simonjohansson/src/kanban/apps/kanban-web`) using current Svelte/SvelteKit.
  2. Add standalone runtime:
  - `npm run dev` for local development.
  - production build output suitable for backend embedding.
  3. Add backend static-serving layer:
  - use `go:embed` for built assets.
  - serve app shell/assets from backend routes without breaking API routes (`/projects`, `/ws`, `/openapi.yaml`, etc.).
  4. Add frontend browser e2e tests (Playwright) that open real browser and validate critical flows.
  5. Add Make targets for frontend install/build/test and embed-refresh workflow.
  6. Add docs for standalone frontend vs embedded backend mode.
- Checklist:
  - [x] Add failing backend test for serving frontend index/assets from embedded bundle.
  - [x] Add failing frontend browser e2e smoke test (load app, fetch projects, render state).
  - [x] Scaffold Svelte app with API client wiring and websocket subscription baseline.
  - [x] Implement backend embedded static file serving and route fallback.
  - [x] Add Make targets for `frontend-dev`, `frontend-build`, `frontend-test-e2e`.
  - [x] Ensure `kanban serve` serves embedded frontend build.
  - [x] Run backend + frontend + browser e2e tests until green.

### Active follow-up: Coverage uplift to 80%
- Goal: raise backend statement coverage from current ~39.3% to at least 80%.
- Focus gaps (current low areas):
  - `internal/service`
  - `internal/kanban`
  - untested command/flow branches and error paths.
- Plan:
  1. Add targeted unit tests for `internal/service` methods and error mapping paths.
  2. Add command-level tests for `internal/kanban` (`serve`, flag precedence, output/error branches).
  3. Add additional backend e2e/error-path tests to cover branch-heavy handlers.
  4. Measure coverage after each batch and prioritize lowest packages first.
- Checklist:
  - [ ] Add failing tests for uncovered `internal/service` methods (list/get/move/comment/append/delete/rebuild).
  - [ ] Add failing tests for uncovered `internal/kanban` run/serve branches.
  - [ ] Add failing tests for remaining backend handler error branches.
  - [ ] Reach >=80% backend statement coverage and record report.

### Active follow-up: Kanban lanes in Swift + Web (status-only board)
- Goal: both `/Users/simonjohansson/src/kanban/apps/kanban-macos` and `/Users/simonjohansson/src/kanban/apps/kanban-web` should render a normal 4-lane board (`Todo`, `Doing`, `Review`, `Done`) for the selected project.
- Requested behavior:
  - Cards are created from API/CLI (no in-app create UI in this step).
  - Lanes are keyed by `status`.
  - When switching projects, lane content reflects the selected project only.
  - If cards are created/moved via API/CLI while app is open, views update automatically.
  - E2E assertions should verify card titles in lanes.
- Additional request:
  - Remove `column` field usage and move toward status-only board semantics.
- Plan:
  1. Add failing web e2e coverage for card create/move + project switching.
  2. Add failing Swift e2e coverage for card create/move + project switching.
  3. Implement web board lanes + selected-project card loading + websocket-triggered refresh.
  4. Implement Swift board lanes + selected-project card loading + websocket-triggered refresh.
  5. Remove `column` usage in app layers and align with status-only lane logic.
  6. Run app unit/e2e suites and backend tests impacted by API interactions.
- Checklist:
  - [x] Add failing frontend e2e for 4-lane board create/switch/switch-back flow.
  - [x] Add failing frontend e2e for move-via-API reflected in lanes.
  - [x] Add failing Swift e2e for 4-lane board create/switch/switch-back flow.
  - [x] Add failing Swift e2e for move-via-API reflected in lanes.
  - [x] Implement web board view and card refresh wiring.
  - [x] Implement Swift board view and card refresh wiring.
  - [x] Remove `column` usage from app-layer lane logic.
  - [x] Run all relevant tests until green.

## Proposed Backend Refactor (Service Layer)
### Problem
- Current Huma handlers orchestrate too much: markdown write, sqlite projection update, logging, and websocket event publishing.
- This creates duplication and increases the chance of inconsistent behavior across endpoints.

### Target Architecture
1. Transport layer (`internal/server`)
- Responsibility: HTTP/OpenAPI contracts only.
- Parse/validate request payloads and path/query params.
- Call service methods.
- Map domain/app errors to HTTP errors.
- No direct calls to markdown/sqlite/event hub.

2. Application layer (`internal/app` or `internal/service`)
- Single orchestration boundary for use cases.
- Example methods:
  - `CreateProject`
  - `DeleteProject`
  - `CreateCard`
  - `MoveCard`
  - `CommentCard`
  - `AppendDescription`
  - `DeleteCard`
  - `RebuildProjection`
- Performs:
  - markdown source-of-truth mutation
  - sqlite projection sync
  - structured logging
  - websocket event publication
- Returns typed errors and operation results.

3. Infrastructure layer (`internal/store`, websocket hub)
- Markdown store remains source of truth.
- SQLite projection remains derived read model.
- Event hub remains pub/sub transport.
- Exposed through interfaces consumed by service layer.

4. Error model (`internal/app/errors.go`)
- Introduce typed/sentinel errors (not found, conflict, validation, internal).
- Server layer uses one error-to-Huma mapper to remove per-handler branching duplication.

### Consistency Strategy
- Keep markdown as authoritative write target.
- Service synchronizes projection in the same request path.
- If projection update fails, return error and emit high-signal logs; projection can be repaired via existing rebuild endpoint.
- Optional future step: background reconcile queue for projection failures.

### Migration Plan (TDD)
1. Add failing service-layer unit tests for one operation (`DeleteProject`) proving:
- markdown delete called
- projection delete called
- event published
- errors mapped by type
2. Implement minimal service + interfaces to satisfy tests.
3. Add failing server tests ensuring handlers delegate to service (no store/projection direct usage).
4. Migrate one endpoint (`DELETE /projects/{project}`) to service and keep tests green.
5. Repeat per endpoint until all write operations are service-backed.
6. Centralize error mapping in one helper and remove repetitive handler error branches.
7. Refactor read paths (`listCards`) to use service query methods for symmetry.
8. Run full backend suite + e2e to confirm no API contract regressions.

### Benefits
- Smaller, clearer handlers.
- Single orchestration path for markdown+projection+events.
- Easier testing (unit-test orchestration without HTTP).
- Lower risk of drift between endpoints.

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
- [x] Refactor backend to service-layer orchestration (`internal/service`) with typed app errors.
- [x] Keep strict sync semantics for writes (projection sync failure returns `500`).
- [x] Build CLI `kb` as separate module using backend generated OpenAPI client.
- [x] Add CLI black-box e2e tests and CLI unit tests.
- [x] Add monorepo/root and CLI make targets for running tests easily.

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
- [x] Added generated OpenAPI client at `/Users/simonjohansson/src/kanban/backend/gen/client/client.gen.go`.
- [x] Added generated-client e2e flow test at `/Users/simonjohansson/src/kanban/backend/e2e_generated_client_test.go`.
- [x] Added backend logging (startup/shutdown, HTTP request logs, and operation-level action logs).
- [x] Added e2e backend log streaming to test output for black-box and generated-client tests.
- [x] Migrated backend API registration/contract to Huma with chi adapter.
- [x] Added OpenAPI export command at `/Users/simonjohansson/src/kanban/backend/cmd/export-openapi/main.go`.
- [x] Updated OpenAPI workflow so `make openapi-sync` exports spec from running API registration.
- [x] Migrated SQLite projection layer to `sqlc`-generated queries.
- [x] Added integration/e2e tests (in-process + black-box process) with filesystem + sqlite assertions.
- [x] Full backend test suite is green.
- [x] Increased backend server package coverage to `83.6%`.
- [x] Introduced application service layer (`/Users/simonjohansson/src/kanban/backend/internal/service`) and migrated server handlers to delegate business logic.
- [x] Implemented CLI binary entrypoint at `/Users/simonjohansson/src/kanban/cli/cmd/kb/main.go`.
- [x] Implemented CLI config/command runtime at `/Users/simonjohansson/src/kanban/cli/internal/kb`.
- [x] Ensured CLI config precedence `flags > env > config`, with first-run and missing-field auto-fill config writes.
- [x] Verified full monorepo test run via root `make test` is green.
- [x] Added CLI module local backend linkage in `/Users/simonjohansson/src/kanban/cli/go.mod` (`require` + `replace`) so generated client imports resolve without remote fetches.
- [x] Ran CLI dependency upgrade/tidy pass (`go get -u` on direct deps + `go mod tidy`) and verified tests stay green.
- [x] Hardened `kb primer` output (templates + response/error/event/semantics contract) for LLM reliability and added primer regression tests.
- [x] Unified runtime config location to `~/.config/kanban/config.yaml` with shared schema (`server_url`, `backend.*`, `cli.output`) and wired backend+CLI to read it.
- [x] Fixed backend entrypoint so `go run cmd/kanban-backend/main.go` works (moved runtime config helpers into `main.go`).
- [x] Fixed `kb watch` interrupt handling so Ctrl-C reliably exits immediately (added regression test).
- [x] Added first macOS Swift app foundation at `/Users/simonjohansson/src/kanban/apps/kanban-macos` with generated OpenAPI client, sidebar project list, and websocket-triggered refresh flow.

## Open Questions
- None currently blocking implementation.
