import Foundation
import Testing
@testable import KanbanMacOS

struct ProjectsViewModelTests {
    @Test
    @MainActor
    func loadPopulatesProjects() async throws {
        let store = StoreStub(result: .success([
            .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
        ]), cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo")]], cardDetails: [:], stream: .updates([]))
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
        let store = StoreStub(result: .failure(URLError(.notConnectedToInternet)), cards: [:], cardDetails: [:], stream: .updates([]))
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
            cardDetails: [:],
            stream: .failure(URLError(.networkConnectionLost))
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.load()
        try? await Task.sleep(for: .milliseconds(50))

        #expect(viewModel.alertMessage != nil)
    }

    @Test
    @MainActor
    func selectCardLoadsDetails() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: "feature/task-a",
            status: "Todo",
            description: [
                .init(timestamp: "2026-02-20T10:00:00Z", body: "First description"),
            ],
            todos: [
                .init(id: 1, text: "Write tests", completed: false),
            ],
            comments: [
                .init(timestamp: "2026-02-20T10:01:00Z", body: "First comment"),
            ]
        )
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([])
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)

        #expect(viewModel.selectedCardNumber == 1)
        #expect(viewModel.cardDetails?.title == "Task A")
        #expect(viewModel.cardDetails?.branch == "feature/task-a")
        #expect(viewModel.cardDetails?.description.first?.body == "First description")
        #expect(viewModel.cardDetails?.todos.first?.text == "Write tests")
        #expect(viewModel.cardDetails?.comments.first?.body == "First comment")
        #expect(viewModel.cardDetailsErrorMessage == nil)
        #expect(viewModel.isCardDetailsLoading == false)
    }

    @Test
    @MainActor
    func selectingAnotherCardReplacesDetails() async throws {
        let first = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: "feature/task-a",
            status: "Todo",
            description: [],
            todos: [],
            comments: []
        )
        let second = KanbanCardDetails(
            id: "alpha/card-2",
            number: 2,
            projectSlug: "alpha",
            title: "Task B",
            branch: "feature/task-b",
            status: "Todo",
            description: [],
            todos: [],
            comments: []
        )
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: [
                "alpha": [
                    .init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo"),
                    .init(id: "alpha/card-2", number: 2, projectSlug: "alpha", title: "Task B", status: "Todo"),
                ],
            ],
            cardDetails: [
                "alpha/1": .success(first),
                "alpha/2": .success(second),
            ],
            stream: .updates([])
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        await viewModel.selectCard(number: 2)

        #expect(viewModel.selectedCardNumber == 2)
        #expect(viewModel.cardDetails?.title == "Task B")
        #expect(viewModel.cardDetails?.branch == "feature/task-b")
    }

    @Test
    @MainActor
    func selectCardFailureSetsInlineError() async throws {
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo")]],
            cardDetails: ["alpha/1": .failure(URLError(.badServerResponse))],
            stream: .updates([])
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)

        #expect(viewModel.selectedCardNumber == 1)
        #expect(viewModel.cardDetails == nil)
        #expect(viewModel.cardDetailsErrorMessage != nil)
        #expect(viewModel.isCardDetailsLoading == false)
    }

    @Test
    @MainActor
    func closeCardDetailsClearsSelection() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: nil,
            status: "Todo",
            description: [],
            todos: [],
            comments: []
        )
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Todo")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([])
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        viewModel.closeCardDetails()

        #expect(viewModel.selectedCardNumber == nil)
        #expect(viewModel.cardDetails == nil)
        #expect(viewModel.cardDetailsErrorMessage == nil)
    }

    @Test
    @MainActor
    func reviewMoveToDoingOpensReasonPromptWithoutMutatingCard() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: nil,
            status: "Review",
            description: [],
            comments: []
        )
        let recorder = CallRecorder()
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Review")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([]),
            recorder: recorder
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        await viewModel.requestReviewTransition(to: "Doing")

        #expect(viewModel.reviewReasonPromptVisible == true)
        #expect(viewModel.reviewReasonTargetStatus == "Doing")
        #expect(await recorder.snapshot().isEmpty)
    }

    @Test
    @MainActor
    func submittingBlankReviewReasonShowsValidationError() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: nil,
            status: "Review",
            description: [],
            comments: []
        )
        let recorder = CallRecorder()
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Review")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([]),
            recorder: recorder
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        await viewModel.requestReviewTransition(to: "Doing")
        await viewModel.submitReviewReason("   ")

        #expect(viewModel.reviewReasonPromptVisible == true)
        #expect(viewModel.reviewReasonErrorMessage == "Reason is required")
        #expect(await recorder.snapshot().isEmpty)
    }

    @Test
    @MainActor
    func successfulReviewMoveToDoingAddsCommentInFixedFormat() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: nil,
            status: "Review",
            description: [],
            comments: []
        )
        let recorder = CallRecorder()
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Review")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([]),
            recorder: recorder
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        await viewModel.requestReviewTransition(to: "Doing")
        await viewModel.submitReviewReason("Address QA feedback")

        #expect(viewModel.reviewReasonPromptVisible == false)
        #expect(viewModel.reviewReasonErrorMessage == nil)
        #expect(viewModel.selectedCardNumber == nil)
        #expect(viewModel.cardDetails == nil)
        #expect(await recorder.snapshot() == [
            "move alpha/1 -> Doing",
            "comment alpha/1 -> Moved back to Doing: Address QA feedback",
        ])
    }

    @Test
    @MainActor
    func commentFailureAfterMoveRollsCardBackToReview() async throws {
        let details = KanbanCardDetails(
            id: "alpha/card-1",
            number: 1,
            projectSlug: "alpha",
            title: "Task A",
            branch: nil,
            status: "Review",
            description: [],
            comments: []
        )
        let recorder = CallRecorder()
        let store = StoreStub(
            result: .success([
                .init(name: "Alpha", slug: "alpha", localPath: nil, remoteURL: nil),
            ]),
            cards: ["alpha": [.init(id: "alpha/card-1", number: 1, projectSlug: "alpha", title: "Task A", status: "Review")]],
            cardDetails: ["alpha/1": .success(details)],
            stream: .updates([]),
            commentFailures: ["alpha/1/Moved back to Doing: Will fail"],
            recorder: recorder
        )
        let viewModel = ProjectsViewModel(store: store)

        await viewModel.selectProject(slug: "alpha")
        await viewModel.load()
        await viewModel.selectCard(number: 1)
        await viewModel.requestReviewTransition(to: "Doing")
        await viewModel.submitReviewReason("Will fail")

        #expect(viewModel.reviewReasonPromptVisible == false)
        #expect(viewModel.alertMessage == "Failed to add transition reason")
        #expect(await recorder.snapshot() == [
            "move alpha/1 -> Doing",
            "comment alpha/1 -> Moved back to Doing: Will fail",
            "move alpha/1 -> Review",
        ])
    }
}

private struct StoreStub: ProjectsStoreProtocol {
    let result: Result<[ProjectSummary], Error>
    let cards: [String: [KanbanCardSummary]]
    let cardDetails: [String: Result<KanbanCardDetails, Error>]
    let stream: StreamBehavior
    let moveFailures: Set<String>
    let commentFailures: Set<String>
    let recorder: CallRecorder

    init(
        result: Result<[ProjectSummary], Error>,
        cards: [String: [KanbanCardSummary]],
        cardDetails: [String: Result<KanbanCardDetails, Error>],
        stream: StreamBehavior,
        moveFailures: Set<String> = [],
        commentFailures: Set<String> = [],
        recorder: CallRecorder = CallRecorder()
    ) {
        self.result = result
        self.cards = cards
        self.cardDetails = cardDetails
        self.stream = stream
        self.moveFailures = moveFailures
        self.commentFailures = commentFailures
        self.recorder = recorder
    }

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

    func loadCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails {
        switch cardDetails["\(projectSlug)/\(number)"] {
        case .success(let value):
            return value
        case .failure(let error):
            throw error
        case .none:
            throw URLError(.fileDoesNotExist)
        }
    }

    func moveCard(projectSlug: String, number: Int, status: String) async throws {
        await recorder.record("move \(projectSlug)/\(number) -> \(status)")
        if moveFailures.contains("\(projectSlug)/\(number)/\(status)") {
            throw URLError(.badServerResponse)
        }
    }

    func commentOnCard(projectSlug: String, number: Int, body: String) async throws {
        await recorder.record("comment \(projectSlug)/\(number) -> \(body)")
        if commentFailures.contains("\(projectSlug)/\(number)/\(body)") {
            throw URLError(.badServerResponse)
        }
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

private actor CallRecorder {
    private var calls: [String] = []

    func record(_ value: String) {
        calls.append(value)
    }

    func snapshot() -> [String] {
        calls
    }
}
