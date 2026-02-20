import Foundation
import Testing
@testable import KanbanMacOS

struct ProjectsViewModelTests {
    @Test
    @MainActor
    func loadPopulatesProjects() async throws {
        let store = StoreStub(result: .success([
            .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
        ]), stream: .updates([]))
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.load()

        #expect(viewModel.projects.count == 1)
        #expect(viewModel.alertMessage == nil)
    }

    @Test
    @MainActor
    func loadFailureSetsAlertMessage() async throws {
        let store = StoreStub(result: .failure(URLError(.notConnectedToInternet)), stream: .updates([]))
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
    let stream: StreamBehavior

    func initialLoad() async throws -> [ProjectSummary] {
        switch result {
        case .success(let projects):
            return projects
        case .failure(let error):
            throw error
        }
    }

    func startWatching() -> AsyncThrowingStream<[ProjectSummary], Error> {
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
    case updates([[ProjectSummary]])
    case failure(Error)
}
