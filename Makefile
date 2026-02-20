.PHONY: test test-backend test-swift test-swift-e2e build-swift-app run-swift-app

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
