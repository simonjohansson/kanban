import Foundation
import Testing
@testable import KanbanMacOS

struct ProjectsStoreTests {
    @Test
    func initialLoadFetchesProjects() async throws {
        let api = APIStub(projectResponses: [
            [.init(name: "Alpha", slug: "alpha", localPath: "/tmp/alpha", remoteURL: nil)],
        ], cardResponses: [:])
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
        ], cardResponses: [:])
        let events = EventStreamStub(events: [
            .init(type: "project.created", projectSlug: "beta"),
        ])
        let store = ProjectsStore(apiClient: api, eventStream: events)

        _ = try await store.initialLoad()
        let updates = store.startWatching()
        let next = try await updates.first(where: { _ in true })

        #expect(next?.projects.count == 2)
        #expect(next?.projects.contains(where: { $0.slug == "beta" }) == true)
    }

    @Test
    func loadCardsFetchesProjectCards() async throws {
        let api = APIStub(
            projectResponses: [[]],
            cardResponses: [
                "alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task", status: "Todo")],
            ]
        )
        let events = EventStreamStub(events: [])
        let store = ProjectsStore(apiClient: api, eventStream: events)

        let cards = try await store.loadCards(projectSlug: "alpha")
        #expect(cards.count == 1)
        #expect(cards.first?.title == "Task")
    }
}

private actor APIStub: ProjectsAPIClient {
    private var responses: [[ProjectSummary]]
    private let cardResponses: [String: [KanbanCardSummary]]

    init(projectResponses: [[ProjectSummary]], cardResponses: [String: [KanbanCardSummary]]) {
        self.responses = projectResponses
        self.cardResponses = cardResponses
    }

    func listProjects() async throws -> [ProjectSummary] {
        if responses.isEmpty { return [] }
        return responses.removeFirst()
    }

    func listCards(projectSlug: String) async throws -> [KanbanCardSummary] {
        cardResponses[projectSlug] ?? []
    }

    func getCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails {
        KanbanCardDetails(
            id: "\(projectSlug)/card-\(number)",
            number: number,
            projectSlug: projectSlug,
            title: "Task \(number)",
            status: "Todo",
            description: [],
            comments: []
        )
    }

    func moveCard(projectSlug: String, number: Int, status: String) async throws {}

    func commentOnCard(projectSlug: String, number: Int, body: String) async throws {}
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
