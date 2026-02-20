.PHONY: test test-backend test-swift test-swift-e2e build-swift-app run-swift-app run-backend frontend-install frontend-dev frontend-build frontend-build-embed frontend-test-e2e

KANBAN_ADDR ?= 127.0.0.1:8080
KANBAN_CARDS_PATH ?= $(HOME)/.local/state/kanban/cards
KANBAN_SQLITE_PATH ?= $(HOME)/.local/state/kanban/projection.db

test: test-backend test-swift

test-backend:
	$(MAKE) -C backend test

test-swift:
	cd apps/kanban-macos && swift test

test-swift-e2e:
	cd apps/kanban-macos && swift build --product KanbanMacOS
	cd apps/kanban-macos && KANBAN_RUN_SWIFT_E2E=1 swift test --filter SidebarE2ETests

build-swift-app:
	cd apps/kanban-macos && swift build

run-swift-app:
	cd apps/kanban-macos && swift run KanbanMacOS

run-backend:
	cd backend && go run ./cmd/kanban serve --addr "$(KANBAN_ADDR)" --cards-path "$(KANBAN_CARDS_PATH)" --sqlite-path "$(KANBAN_SQLITE_PATH)"

frontend-install:
	cd apps/kanban-web && npm ci

frontend-dev:
	cd apps/kanban-web && npm run dev

frontend-build:
	cd apps/kanban-web && npm run build

frontend-build-embed:
	cd apps/kanban-web && npm run build:embed

frontend-test-e2e:
	cd apps/kanban-web && npm run test:e2e -- --reporter=line
