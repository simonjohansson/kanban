import Foundation
import Testing
@testable import KanbanMacOS

struct ProjectsStoreTests {
    @Test
    func initialLoadFetchesProjects() async throws {
        let api = APIStub(projectResponses: [
            [.init(name: "Alpha", slug: "alpha", localPath: "/tmp/alpha", remoteURL: nil)],
        ])
        let events = EventStreamStub(events: [])
        let store = ProjectsStore(apiClient: api, eventStream: events)

        let projects = try await store.initialLoad()
        #expect(projects.count == 1)
        #expect(projects.first?.name == "Alpha")
    }

    @Test
    func projectEventsTriggerReload() async throws {
        let api = APIStub(projectResponses: [
            [.init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil)],
            [
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
                .init(name: "Beta", slug: "beta", localPath: nil, remoteURL: nil),
            ],
        ])
        let events = EventStreamStub(events: [
            .init(type: "project.created", projectSlug: "beta"),
        ])
        let store = ProjectsStore(apiClient: api, eventStream: events)

        _ = try await store.initialLoad()
        let updates = store.startWatching()
        let next = try await updates.first(where: { _ in true })

        #expect(next?.count == 2)
        #expect(next?.contains(where: { $0.slug == "beta" }) == true)
    }
}

private actor APIStub: ProjectsAPIClient {
    private var responses: [[ProjectSummary]]

    init(projectResponses: [[ProjectSummary]]) {
        self.responses = projectResponses
    }

    func listProjects() async throws -> [ProjectSummary] {
        if responses.isEmpty { return [] }
        return responses.removeFirst()
    }
}

private struct EventStreamStub: ProjectEventStream {
    let events: [ProjectEvent]

    var stream: AsyncThrowingStream<ProjectEvent, Error> {
        AsyncThrowingStream { continuation in
            for event in events {
                continuation.yield(event)
            }
            continuation.finish()
        }
    }
}
