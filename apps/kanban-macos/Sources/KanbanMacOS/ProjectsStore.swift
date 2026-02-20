import Foundation

public protocol ProjectsAPIClient: Sendable {
    func listProjects() async throws -> [ProjectSummary]
    func listCards(projectSlug: String) async throws -> [KanbanCardSummary]
    func getCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails
}

public protocol ProjectEventStream: Sendable {
    var stream: AsyncThrowingStream<ProjectEvent, Error> { get }
}

public struct StoreUpdate: Sendable {
    public let projects: [ProjectSummary]
    public let event: ProjectEvent

    public init(projects: [ProjectSummary], event: ProjectEvent) {
        self.projects = projects
        self.event = event
    }
}

public protocol ProjectsStoreProtocol: Sendable {
    func initialLoad() async throws -> [ProjectSummary]
    func loadCards(projectSlug: String) async throws -> [KanbanCardSummary]
    func loadCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails
    func startWatching() -> AsyncThrowingStream<StoreUpdate, Error>
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

    public func loadCards(projectSlug: String) async throws -> [KanbanCardSummary] {
        try await apiClient.listCards(projectSlug: projectSlug).sorted(by: sortCards(lhs:rhs:))
    }

    public func loadCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails {
        try await apiClient.getCard(projectSlug: projectSlug, number: number)
    }

    public func startWatching() -> AsyncThrowingStream<StoreUpdate, Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    for try await event in eventStream.stream {
                        let latest = try await apiClient.listProjects().sorted(by: sortProjects(lhs:rhs:))
                        continuation.yield(StoreUpdate(projects: latest, event: event))
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

private func sortCards(lhs: KanbanCardSummary, rhs: KanbanCardSummary) -> Bool {
    lhs.number < rhs.number
}
