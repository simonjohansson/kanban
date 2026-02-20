import Foundation
import Testing
@testable import KanbanMacOS

@Suite(.serialized)
struct SidebarStateProbeTests {
    @Test
    func writesProjectNamesToJSONFile() throws {
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("kanban-sidebar-probe-tests-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        let output = dir.appendingPathComponent("sidebar-state.json", isDirectory: false)
        let probe = SidebarStateProbe(outputURL: output, selectionInputURL: nil)
        let projects = [
            ProjectSummary(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ProjectSummary(name: "Beta", slug: "beta", localPath: nil, remoteURL: nil),
        ]

        probe.write(
            projects: projects,
            selectedProjectSlug: "beta",
            cardsByStatus: [
                "Todo": ["Task A"],
                "Doing": [],
                "Review": [],
                "Done": [],
            ]
        )

        let raw = try Data(contentsOf: output)
        let payload = try JSONDecoder().decode(SidebarStateProbePayload.self, from: raw)
        #expect(payload.projects == ["Alpha", "Beta"])
        #expect(payload.selectedProjectSlug == "beta")
        #expect(payload.cardsByStatus["Todo"] == ["Task A"])
    }

    @Test
    func fromEnvironmentReadsConfiguredPath() {
        let stateKey = "KANBAN_E2E_SIDEBAR_STATE_PATH"
        let statePath = "/tmp/kanban-sidebar-state.json"
        let selectKey = "KANBAN_E2E_SIDEBAR_SELECT_PATH"
        let selectPath = "/tmp/kanban-sidebar-select.txt"
        setenv(stateKey, statePath, 1)
        setenv(selectKey, selectPath, 1)
        defer {
            unsetenv(stateKey)
            unsetenv(selectKey)
        }

        let probe = SidebarStateProbe.fromEnvironment()
        #expect(probe.outputURL?.path(percentEncoded: false) == statePath)
        #expect(probe.selectionInputURL?.path(percentEncoded: false) == selectPath)
    }

    @Test
    func consumeSelectionRequestReadsAndClearsFile() throws {
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("kanban-sidebar-probe-tests-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        let selectPath = dir.appendingPathComponent("select.txt", isDirectory: false)
        try "beta\n".write(to: selectPath, atomically: true, encoding: .utf8)

        let probe = SidebarStateProbe(outputURL: nil, selectionInputURL: selectPath)
        #expect(probe.consumeSelectionRequest() == "beta")
        #expect(probe.consumeSelectionRequest() == nil)
    }
}
