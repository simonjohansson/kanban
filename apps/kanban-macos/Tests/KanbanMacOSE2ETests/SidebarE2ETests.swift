import Darwin
import Foundation
import KanbanAPI
import OpenAPIRuntime
import OpenAPIURLSession
import Testing

@Suite(.serialized)
struct SidebarE2ETests {
    @Test
    func appShowsLanesAndReflectsCardsAcrossProjectSwitchingAndMoves() async throws {
        guard ProcessInfo.processInfo.environment["KANBAN_RUN_SWIFT_E2E"] == "1" else {
            return
        }

        let harness = try E2EHarness()
        defer { harness.stop() }

        try harness.startBackend()
        try await harness.waitForBackendReady()

        try harness.startApp()

        let initialState = try await harness.waitForSidebarState()
        #expect(initialState.projects.isEmpty)
        #expect(initialState.selectedProjectSlug == nil)
        #expect(initialState.cardsByStatus?["Todo"]?.isEmpty ?? true)
        #expect(initialState.cardsByStatus?["Doing"]?.isEmpty ?? true)
        #expect(initialState.cardsByStatus?["Review"]?.isEmpty ?? true)
        #expect(initialState.cardsByStatus?["Done"]?.isEmpty ?? true)

        let configuration = Configuration(dateTranscoder: FlexibleE2EDateTranscoder())
        let client = Client(serverURL: harness.serverURL, configuration: configuration, transport: URLSessionTransport())
        let firstCreateOutput = try await client.createProject(
            body: .json(
                Components.Schemas.CreateProjectRequest(
                    name: "Swift E2E Project One"
                )
            )
        )
        guard case .created(let firstCreated) = firstCreateOutput else {
            Issue.record("expected first createProject to return .created")
            return
        }

        let secondCreateOutput = try await client.createProject(
            body: .json(
                Components.Schemas.CreateProjectRequest(
                    name: "Swift E2E Project Two"
                )
            )
        )
        guard case .created(let secondCreated) = secondCreateOutput else {
            Issue.record("expected second createProject to return .created")
            return
        }

        let firstSlug = try firstCreated.body.json.slug
        let secondSlug = try secondCreated.body.json.slug
        let projectsState = try await harness.waitForSidebarProjects(named: ["Swift E2E Project One", "Swift E2E Project Two"])
        #expect(projectsState.projects.count == 2)

        try harness.clickProject(slug: firstSlug)
        _ = try await harness.waitForSelectedProject(slug: firstSlug)

        let firstCardNumber = try await harness.createCard(projectSlug: firstSlug, title: "Swift First Card", status: "Todo")
        _ = try await harness.waitForLaneContains(status: "Todo", title: "Swift First Card")

        try harness.clickProject(slug: secondSlug)
        _ = try await harness.waitForSelectedProject(slug: secondSlug)
        _ = try await harness.waitForBoardEmpty()

        _ = try await harness.createCard(projectSlug: secondSlug, title: "Swift Second Card", status: "Todo")
        let secondLaneState = try await harness.waitForLaneContains(status: "Todo", title: "Swift Second Card")
        #expect(secondLaneState.cardsByStatus?["Todo"]?.contains("Swift First Card") == false)

        try harness.clickProject(slug: firstSlug)
        _ = try await harness.waitForSelectedProject(slug: firstSlug)
        let firstLaneState = try await harness.waitForLaneContains(status: "Todo", title: "Swift First Card")
        #expect(firstLaneState.cardsByStatus?["Todo"]?.contains("Swift Second Card") == false)

        try await harness.moveCard(projectSlug: firstSlug, cardNumber: firstCardNumber, status: "Done")
        _ = try await harness.waitForLaneContains(status: "Done", title: "Swift First Card")
        let movedState = try await harness.waitForSidebarState()
        #expect(movedState.cardsByStatus?["Todo"]?.contains("Swift First Card") == false)
    }
}

private struct SidebarState: Codable {
    let projects: [String]
    let selectedProjectSlug: String?
    let cardsByStatus: [String: [String]]?

    enum CodingKeys: String, CodingKey {
        case projects
        case selectedProjectSlug = "selected_project_slug"
        case cardsByStatus = "cards_by_status"
    }
}

private final class E2EHarness {
    let serverURL: URL

    private let repoRoot: URL
    private let appDirectory: URL
    private let backendDirectory: URL
    private let stateFileURL: URL
    private let selectFileURL: URL
    private let homeDirectory: URL
    private let cardsPath: URL
    private let sqlitePath: URL
    private let address: String

    private var backendProcess: Process?
    private var appProcess: Process?

    init() throws {
        repoRoot = try Self.findRepoRoot()
        appDirectory = repoRoot.appendingPathComponent("apps/kanban-macos", isDirectory: true)
        backendDirectory = repoRoot.appendingPathComponent("backend", isDirectory: true)

        let port = try Self.reservePort()
        address = "127.0.0.1:\(port)"
        serverURL = URL(string: "http://\(address)")!

        let tempRoot = FileManager.default.temporaryDirectory
            .appendingPathComponent("kanban-swift-e2e-\(UUID().uuidString)", isDirectory: true)
        homeDirectory = tempRoot.appendingPathComponent("home", isDirectory: true)
        let configDir = homeDirectory
            .appendingPathComponent(".config", isDirectory: true)
            .appendingPathComponent("kanban", isDirectory: true)
        cardsPath = tempRoot.appendingPathComponent("cards", isDirectory: true)
        sqlitePath = tempRoot.appendingPathComponent("projection.db", isDirectory: false)
        stateFileURL = tempRoot.appendingPathComponent("sidebar-state.json", isDirectory: false)
        selectFileURL = tempRoot.appendingPathComponent("sidebar-select.txt", isDirectory: false)

        try FileManager.default.createDirectory(at: configDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: cardsPath, withIntermediateDirectories: true)
        try writeConfig(configDirectory: configDir)
    }

    func startBackend() throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = [
            "go",
            "run",
            "./cmd/kanban",
            "serve",
            "--addr", address,
            "--cards-path", cardsPath.path(percentEncoded: false),
            "--sqlite-path", sqlitePath.path(percentEncoded: false),
        ]
        process.currentDirectoryURL = backendDirectory
        process.environment = ProcessInfo.processInfo.environment
        process.standardOutput = logPipe(prefix: "backend")
        process.standardError = logPipe(prefix: "backend")
        try process.run()
        backendProcess = process
    }

    func waitForBackendReady() async throws {
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = 0.5
        config.timeoutIntervalForResource = 0.5
        let session = URLSession(configuration: config)
        defer {
            session.invalidateAndCancel()
        }

        let deadline = Date().addingTimeInterval(15)
        let healthURL = serverURL.appendingPathComponent("health")
        while Date() < deadline {
            if let backendProcess, !backendProcess.isRunning {
                throw HarnessError.processFailed("backend exited before becoming healthy")
            }
            do {
                var request = URLRequest(url: healthURL)
                request.timeoutInterval = 0.5
                let (_, response) = try await session.data(for: request)
                if let http = response as? HTTPURLResponse, http.statusCode == 200 {
                    return
                }
            } catch {
                // retry until timeout
            }
            try await Task.sleep(nanoseconds: 150_000_000)
        }
        throw HarnessError.timeout("backend did not become healthy")
    }

    func startApp() throws {
        let appBinary: URL
        if let configured = ProcessInfo.processInfo.environment["KANBAN_APP_BINARY"], !configured.isEmpty {
            appBinary = URL(fileURLWithPath: configured, isDirectory: false)
        } else {
            appBinary = appDirectory
                .appendingPathComponent(".build", isDirectory: true)
                .appendingPathComponent("debug", isDirectory: true)
                .appendingPathComponent("KanbanMacOS", isDirectory: false)
        }
        guard FileManager.default.fileExists(atPath: appBinary.path(percentEncoded: false)) else {
            throw HarnessError.invalidEnvironment("app binary not found at \(appBinary.path(percentEncoded: false)); run `swift build --product KanbanMacOS` first")
        }

        let process = Process()
        process.executableURL = appBinary
        process.currentDirectoryURL = appDirectory

        var env = ProcessInfo.processInfo.environment
        env["HOME"] = homeDirectory.path(percentEncoded: false)
        env["KANBAN_SERVER_URL"] = serverURL.absoluteString
        env["KANBAN_E2E_SIDEBAR_STATE_PATH"] = stateFileURL.path(percentEncoded: false)
        env["KANBAN_E2E_SIDEBAR_SELECT_PATH"] = selectFileURL.path(percentEncoded: false)
        env["KANBAN_E2E_DISABLE_ACTIVATION"] = "1"
        process.environment = env
        process.standardOutput = logPipe(prefix: "swift-app")
        process.standardError = logPipe(prefix: "swift-app")

        try process.run()
        appProcess = process
    }

    func waitForSidebarState() async throws -> SidebarState {
        let deadline = Date().addingTimeInterval(15)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before writing sidebar state")
            }
            if let state = try readSidebarState() {
                return state
            }
            try await Task.sleep(nanoseconds: 150_000_000)
        }
        throw HarnessError.timeout("sidebar state file was not written")
    }

    func waitForSidebarProject(named name: String) async throws -> SidebarState {
        let deadline = Date().addingTimeInterval(20)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before sidebar update")
            }
            if let state = try readSidebarState(), state.projects.contains(name) {
                return state
            }
            try await Task.sleep(nanoseconds: 150_000_000)
        }
        throw HarnessError.timeout("project \(name) not present in sidebar state")
    }

    func waitForSidebarProjects(named names: [String]) async throws -> SidebarState {
        let expected = Set(names)
        let deadline = Date().addingTimeInterval(20)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before sidebar update")
            }
            if let state = try readSidebarState(), Set(state.projects) == expected {
                return state
            }
            try await Task.sleep(nanoseconds: 150_000_000)
        }
        throw HarnessError.timeout("sidebar did not match expected projects: \(names.joined(separator: ", "))")
    }

    func clickProject(slug: String) throws {
        try slug.write(to: selectFileURL, atomically: true, encoding: .utf8)
    }

    func waitForSelectedProject(slug: String) async throws -> SidebarState {
        let deadline = Date().addingTimeInterval(10)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before selection update")
            }
            if let state = try readSidebarState(), state.selectedProjectSlug == slug {
                return state
            }
            try await Task.sleep(nanoseconds: 120_000_000)
        }
        throw HarnessError.timeout("project \(slug) was not selected")
    }

    func waitForLaneContains(status: String, title: String) async throws -> SidebarState {
        let deadline = Date().addingTimeInterval(20)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before lane update")
            }
            if let state = try readSidebarState(),
               state.cardsByStatus?[status]?.contains(title) == true {
                return state
            }
            try await Task.sleep(nanoseconds: 120_000_000)
        }
        throw HarnessError.timeout("lane \(status) did not contain card \(title)")
    }

    func waitForBoardEmpty() async throws -> SidebarState {
        let deadline = Date().addingTimeInterval(12)
        while Date() < deadline {
            if let appProcess, !appProcess.isRunning {
                throw HarnessError.processFailed("swift app exited before board update")
            }
            if let state = try readSidebarState() {
                let todo = state.cardsByStatus?["Todo"] ?? []
                let doing = state.cardsByStatus?["Doing"] ?? []
                let review = state.cardsByStatus?["Review"] ?? []
                let done = state.cardsByStatus?["Done"] ?? []
                if todo.isEmpty && doing.isEmpty && review.isEmpty && done.isEmpty {
                    return state
                }
            }
            try await Task.sleep(nanoseconds: 120_000_000)
        }
        throw HarnessError.timeout("board was not empty")
    }

    func createCard(projectSlug: String, title: String, status: String) async throws -> Int {
        let url = serverURL.appendingPathComponent("projects/\(projectSlug)/cards")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "content-type")
        let payload: [String: String] = [
            "title": title,
            "description": title,
            "status": status,
        ]
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, http.statusCode == 201 else {
            throw HarnessError.processFailed("create card failed")
        }
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let number = object["number"] as? Int else {
            throw HarnessError.processFailed("create card response missing number")
        }
        return number
    }

    func moveCard(projectSlug: String, cardNumber: Int, status: String) async throws {
        let url = serverURL.appendingPathComponent("projects/\(projectSlug)/cards/\(cardNumber)/move")
        var request = URLRequest(url: url)
        request.httpMethod = "PATCH"
        request.setValue("application/json", forHTTPHeaderField: "content-type")
        request.httpBody = try JSONSerialization.data(withJSONObject: ["status": status])

        let (_, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
            throw HarnessError.processFailed("move card failed")
        }
    }

    func stop() {
        terminate(process: appProcess)
        terminate(process: backendProcess)
    }

    private func readSidebarState() throws -> SidebarState? {
        guard FileManager.default.fileExists(atPath: stateFileURL.path(percentEncoded: false)) else {
            return nil
        }
        let data = try Data(contentsOf: stateFileURL)
        return try JSONDecoder().decode(SidebarState.self, from: data)
    }

    private func writeConfig(configDirectory: URL) throws {
        let configPath = configDirectory.appendingPathComponent("config.yaml", isDirectory: false)
        let raw = """
        server_url: \(serverURL.absoluteString)
        """
        try raw.write(to: configPath, atomically: true, encoding: .utf8)
    }

    private func logPipe(prefix: String) -> Pipe {
        let pipe = Pipe()
        pipe.fileHandleForReading.readabilityHandler = { handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            if let line = String(data: data, encoding: .utf8) {
                print("[\(prefix)] \(line)", terminator: "")
            }
        }
        return pipe
    }

    private static func findRepoRoot() throws -> URL {
        var current = URL(fileURLWithPath: #filePath, isDirectory: false)
            .deletingLastPathComponent()
        while current.path != "/" {
            let backend = current.appendingPathComponent("backend", isDirectory: true).path(percentEncoded: false)
            let app = current.appendingPathComponent("apps/kanban-macos", isDirectory: true).path(percentEncoded: false)
            if FileManager.default.fileExists(atPath: backend), FileManager.default.fileExists(atPath: app) {
                return current
            }
            current.deleteLastPathComponent()
        }
        throw HarnessError.invalidEnvironment("unable to locate monorepo root")
    }

    private static func reservePort() throws -> UInt16 {
        let fd = socket(AF_INET, SOCK_STREAM, 0)
        guard fd >= 0 else {
            throw HarnessError.processFailed("socket failed: \(String(cString: strerror(errno)))")
        }
        defer { close(fd) }

        var addr = sockaddr_in()
        addr.sin_len = UInt8(MemoryLayout<sockaddr_in>.stride)
        addr.sin_family = sa_family_t(AF_INET)
        addr.sin_port = in_port_t(0)
        addr.sin_addr = in_addr(s_addr: inet_addr("127.0.0.1"))

        let bindResult = withUnsafePointer(to: &addr) { pointer -> Int32 in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                bind(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_in>.stride))
            }
        }
        guard bindResult == 0 else {
            throw HarnessError.processFailed("bind failed: \(String(cString: strerror(errno)))")
        }

        var boundAddr = sockaddr_in()
        var len = socklen_t(MemoryLayout<sockaddr_in>.stride)
        let nameResult = withUnsafeMutablePointer(to: &boundAddr) { pointer -> Int32 in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                getsockname(fd, sockaddrPtr, &len)
            }
        }
        guard nameResult == 0 else {
            throw HarnessError.processFailed("getsockname failed: \(String(cString: strerror(errno)))")
        }
        return UInt16(bigEndian: boundAddr.sin_port)
    }

    private func terminate(process: Process?) {
        guard let process else { return }
        if process.isRunning {
            process.terminate()
            let deadline = Date().addingTimeInterval(2)
            while process.isRunning && Date() < deadline {
                usleep(50_000)
            }
            if process.isRunning {
                kill(process.processIdentifier, SIGKILL)
            }
        }
    }
}

private enum HarnessError: Error {
    case timeout(String)
    case processFailed(String)
    case invalidEnvironment(String)
}

private struct FlexibleE2EDateTranscoder: DateTranscoder {
    private let fractional: any DateTranscoder = .iso8601WithFractionalSeconds
    private let plain: any DateTranscoder = .iso8601

    func encode(_ date: Date) throws -> String {
        try fractional.encode(date)
    }

    func decode(_ dateString: String) throws -> Date {
        if let parsed = try? fractional.decode(dateString) {
            return parsed
        }
        return try plain.decode(dateString)
    }
}
