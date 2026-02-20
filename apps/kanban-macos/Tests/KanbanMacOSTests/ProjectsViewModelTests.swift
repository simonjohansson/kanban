import Foundation
import Testing
@testable import KanbanMacOS

struct ProjectsViewModelTests {
    @Test
    @MainActor
    func loadPopulatesProjects() async throws {
        let store = StoreStub(result: .success([
            .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
        ]), cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo")]], stream: .updates([]))
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()

        #expect(viewModel.projects.count == 1)
        #expect(viewModel.cards.count == 1)
        #expect(viewModel.alertMessage == nil)
    }

    @Test
    @MainActor
    func loadFailureSetsAlertMessage() async throws {
        let store = StoreStub(result: .failure(URLError(.notConnectedToInternet)), cards: [:], stream: .updates([]))
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.load()

        #expect(viewModel.projects.isEmpty)
        #expect(viewModel.alertMessage != nil)
    }

    @Test
    @MainActor
    func streamFailureSetsAlertMessage() async throws {
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: [:],
            stream: .failure(URLError(.networkConnectionLost))
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.load()
        try? await Task.sleep(for: .milliseconds(50))

        #expect(viewModel.alertMessage != nil)
    }
}

private struct StoreStub: ProjectsStoreProtocol {
    let result: Result<[ProjectSummary], Error>
    let cards: [String: [KanbanCardSummary]]
    let stream: StreamBehavior

    func initialLoad() async throws -> [ProjectSummary] {
        switch result {
        case .success(let projects):
            return projects
        case .failure(let error):
            throw error
        }
    }

    func loadCards(projectSlug: String) async throws -> [KanbanCardSummary] {
        cards[projectSlug] ?? []
    }

    func startWatching() -> AsyncThrowingStream<StoreUpdate, Error> {
        AsyncThrowingStream { continuation in
            switch stream {
            case .updates(let values):
                for value in values {
                    continuation.yield(value)
                }
                continuation.finish()
            case .failure(let error):
                continuation.finish(throwing: error)
            }
        }
    }
}

private enum StreamBehavior {
    case updates([StoreUpdate])
    case failure(Error)
}
