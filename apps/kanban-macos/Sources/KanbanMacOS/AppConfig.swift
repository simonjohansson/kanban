import Foundation

public struct AppConfig: Equatable, Sendable {
    public let serverURL: URL

    public init(serverURL: URL) {
        self.serverURL = serverURL
    }

    public static func load(homeDirectory: String) throws -> Self {
        if let fromEnv = ProcessInfo.processInfo.environment["KANBAN_SERVER_URL"]?
            .trimmingCharacters(in: .whitespacesAndNewlines),
           !fromEnv.isEmpty,
           let parsed = URL(string: fromEnv),
           parsed.scheme != nil,
           parsed.host != nil
        {
            return AppConfig(serverURL: parsed)
        }

        let configURL = URL(fileURLWithPath: homeDirectory, isDirectory: true)
            .appendingPathComponent(".config", isDirectory: true)
            .appendingPathComponent("kanban", isDirectory: true)
            .appendingPathComponent("config.yaml", isDirectory: false)

        guard let data = try? Data(contentsOf: configURL),
              let text = String(data: data, encoding: .utf8)
        else {
            return .fallback
        }

        for line in text.split(whereSeparator: \.isNewline) {
            let raw = line.trimmingCharacters(in: .whitespaces)
            guard raw.hasPrefix("server_url:") else { continue }
            let value = raw.replacingOccurrences(of: "server_url:", with: "").trimmingCharacters(in: .whitespaces)
            if let parsed = URL(string: value), parsed.scheme != nil, parsed.host != nil {
                return AppConfig(serverURL: parsed)
            }
        }

        return .fallback
    }

    public static var fallback: Self {
        AppConfig(serverURL: URL(string: "http://127.0.0.1:8080")!)
    }
}
