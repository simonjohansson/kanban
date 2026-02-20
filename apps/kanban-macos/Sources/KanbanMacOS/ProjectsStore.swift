import Foundation

public protocol ProjectsAPIClient: Sendable {
    func listProjects() async throws -> [ProjectSummary]
}

public protocol ProjectEventStream: Sendable {
    var stream: AsyncThrowingStream<ProjectEvent, Error> { get }
}

public protocol ProjectsStoreProtocol: Sendable {
    func initialLoad() async throws -> [ProjectSummary]
    func startWatching() -> AsyncThrowingStream<[ProjectSummary], Error>
}

public final class ProjectsStore: ProjectsStoreProtocol {
    private let apiClient: any ProjectsAPIClient
    private let eventStream: any ProjectEventStream

    public init(apiClient: any ProjectsAPIClient, eventStream: any ProjectEventStream) {
        self.apiClient = apiClient
        self.eventStream = eventStream
    }

    public func initialLoad() async throws -> [ProjectSummary] {
        try await apiClient.listProjects().sorted(by: sortProjects(lhs:rhs:))
    }

    public func startWatching() -> AsyncThrowingStream<[ProjectSummary], Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    for try await event in eventStream.stream {
                        guard event.type == "project.created" || event.type == "project.deleted" else {
                            continue
                        }
                        let latest = try await apiClient.listProjects().sorted(by: sortProjects(lhs:rhs:))
                        continuation.yield(latest)
                    }
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }

            continuation.onTermination = { _ in
                task.cancel()
            }
        }
    }
}

private func sortProjects(lhs: ProjectSummary, rhs: ProjectSummary) -> Bool {
    if lhs.name == rhs.name {
        return lhs.slug < rhs.slug
    }
    return lhs.name < rhs.name
}
