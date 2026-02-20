import Foundation
import Testing
@testable import KanbanMacOS

@Suite(.serialized)
struct AppConfigTests {
    @Test
    func prefersServerURLFromEnvironment() throws {
        setenv("KANBAN_SERVER_URL", "http://127.0.0.1:9911", 1)
        defer { unsetenv("KANBAN_SERVER_URL") }

        let home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)

        let loaded = try AppConfig.load(homeDirectory: home.path(percentEncoded: false))
        #expect(loaded.serverURL.absoluteString == "http://127.0.0.1:9911")
    }

    @Test
    func loadsServerURLFromSharedConfig() throws {
        let home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)

        let configPath = home
            .appendingPathComponent(".config", isDirectory: true)
            .appendingPathComponent("kanban", isDirectory: true)
            .appendingPathComponent("config.yaml", isDirectory: false)
        try FileManager.default.createDirectory(at: configPath.deletingLastPathComponent(), withIntermediateDirectories: true)
        try """
        server_url: http://127.0.0.1:9010
        backend:
          sqlite_path: /tmp/projection.db
          cards_path: /tmp/cards
        cli:
          output: json
        """.write(to: configPath, atomically: true, encoding: .utf8)

        let loaded = try AppConfig.load(homeDirectory: home.path(percentEncoded: false))
        #expect(loaded.serverURL.absoluteString == "http://127.0.0.1:9010")
    }

    @Test
    func fallsBackToLocalhostWhenConfigMissing() throws {
        let home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)

        let loaded = try AppConfig.load(homeDirectory: home.path(percentEncoded: false))
        #expect(loaded.serverURL.absoluteString == "http://127.0.0.1:8080")
    }
}
