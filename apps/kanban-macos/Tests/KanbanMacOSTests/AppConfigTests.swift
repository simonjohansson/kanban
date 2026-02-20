import Foundation
import Testing
@testable import KanbanMacOS

struct AppConfigTests {
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
