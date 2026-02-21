import Foundation
import Observation

@MainActor
@Observable
public final class ProjectsViewModel {
    public private(set) var projects: [ProjectSummary] = []
    public private(set) var cards: [KanbanCardSummary] = []
    public var selectedProjectSlug: String?
    public var alertMessage: String?
    public private(set) var selectedCardNumber: Int?
    public private(set) var cardDetails: KanbanCardDetails?
    public var cardDetailsErrorMessage: String?
    public private(set) var isCardDetailsLoading = false
    public private(set) var reviewReasonPromptVisible = false
    public private(set) var reviewReasonTargetStatus: String?
    public var reviewReasonInput = ""
    public var reviewReasonErrorMessage: String?
    public private(set) var isReviewTransitionInFlight = false

    private let store: any ProjectsStoreProtocol
    private var watchTask: Task<Void, Never>?
    private var cardDetailsRequestToken = 0

    public init(store: any ProjectsStoreProtocol) {
        self.store = store
    }

    public func load() async {
        do {
            projects = try await store.initialLoad()
            if let selectedProjectSlug, !projects.contains(where: { $0.slug == selectedProjectSlug }) {
                self.selectedProjectSlug = nil
                closeCardDetails()
            }
            await reloadCards()
            watchTask?.cancel()
            watchTask = Task { [store] in
                do {
                    for try await update in store.startWatching() {
                        await self.apply(update: update)
                    }
                } catch {
                    await MainActor.run {
                        self.alertMessage = "Project stream failed: \(error.localizedDescription)"
                    }
                }
            }
        } catch {
            alertMessage = "Failed to load projects: \(error.localizedDescription)"
        }
    }

    public func selectProject(slug: String?) async {
        selectedProjectSlug = slug
        closeCardDetails()
        await reloadCards()
    }

    public func selectCard(number: Int) async {
        guard let selectedProjectSlug else {
            return
        }

        selectedCardNumber = number
        cardDetails = nil
        cardDetailsErrorMessage = nil
        isCardDetailsLoading = true
        cardDetailsRequestToken += 1
        let token = cardDetailsRequestToken

        do {
            let details = try await store.loadCard(projectSlug: selectedProjectSlug, number: number)
            guard token == cardDetailsRequestToken else {
                return
            }
            cardDetails = details
        } catch {
            guard token == cardDetailsRequestToken else {
                return
            }
            cardDetailsErrorMessage = "Failed to load card details: \(error.localizedDescription)"
        }

        if token == cardDetailsRequestToken {
            isCardDetailsLoading = false
        }
    }

    public func retrySelectedCard() async {
        guard let selectedCardNumber else {
            return
        }
        await selectCard(number: selectedCardNumber)
    }

    public func closeCardDetails() {
        cardDetailsRequestToken += 1
        selectedCardNumber = nil
        cardDetails = nil
        cardDetailsErrorMessage = nil
        isCardDetailsLoading = false
        closeReviewReasonPrompt()
    }

    public func requestReviewTransition(to status: String) async {
        guard let card = cardDetails, card.status == KanbanLaneStatus.review.rawValue else {
            return
        }
        if status == KanbanLaneStatus.todo.rawValue || status == KanbanLaneStatus.doing.rawValue {
            reviewReasonPromptVisible = true
            reviewReasonTargetStatus = status
            reviewReasonInput = ""
            reviewReasonErrorMessage = nil
            return
        }
        if status == KanbanLaneStatus.done.rawValue {
            await executeReviewTransition(to: status, reason: nil)
        }
    }

    public func submitReviewReason(_ reason: String) async {
        guard let target = reviewReasonTargetStatus,
              target == KanbanLaneStatus.todo.rawValue || target == KanbanLaneStatus.doing.rawValue else {
            return
        }
        let trimmed = reason.trimmingCharacters(in: .whitespacesAndNewlines)
        reviewReasonInput = reason
        if trimmed.isEmpty {
            reviewReasonErrorMessage = "Reason is required"
            return
        }
        await executeReviewTransition(to: target, reason: trimmed)
    }

    public func cancelReviewReasonPrompt() {
        closeReviewReasonPrompt()
    }

    public func cards(for status: KanbanLaneStatus) -> [KanbanCardSummary] {
        cards.filter { $0.status == status.rawValue }
    }

    public func cardsByStatusMap() -> [String: [String]] {
        var map: [String: [String]] = [:]
        for status in KanbanLaneStatus.allCases {
            map[status.rawValue] = cards(for: status).map(\.title)
        }
        return map
    }

    func cardsByStatusDetailedMap() -> [String: [SidebarCardStateProbe]] {
        var map: [String: [SidebarCardStateProbe]] = [:]
        for status in KanbanLaneStatus.allCases {
            map[status.rawValue] = cards(for: status).map {
                SidebarCardStateProbe(title: $0.title, branch: $0.branch)
            }
        }
        return map
    }

    private func apply(update: StoreUpdate) async {
        projects = update.projects
        if let selected = selectedProjectSlug,
           !update.projects.contains(where: { $0.slug == selected }) {
            selectedProjectSlug = nil
            closeCardDetails()
        }
        if shouldRefreshCards(for: update.event) {
            await reloadCards()
            if let selectedCardNumber {
                await selectCard(number: selectedCardNumber)
            }
        }
    }

    private func shouldRefreshCards(for event: ProjectEvent) -> Bool {
        guard let selected = selectedProjectSlug else {
            return false
        }
        switch event.type {
        case .project_period_created, .project_period_deleted:
            return true
        case .card_period_created,
             .card_period_branch_period_updated,
             .card_period_moved,
             .card_period_commented,
             .card_period_updated,
             .card_period_todo_period_added,
             .card_period_todo_period_updated,
             .card_period_todo_period_deleted,
             .card_period_acceptance_period_added,
             .card_period_acceptance_period_updated,
             .card_period_acceptance_period_deleted,
             .card_period_deleted_soft,
             .card_period_deleted_hard,
             .resync_period_required:
            return event.projectSlug == nil || event.projectSlug == selected
        }
    }

    private func reloadCards() async {
        guard let selectedProjectSlug else {
            cards = []
            return
        }
        do {
            cards = try await store.loadCards(projectSlug: selectedProjectSlug)
            if let selectedCardNumber, !cards.contains(where: { $0.number == selectedCardNumber }) {
                closeCardDetails()
            }
        } catch {
            alertMessage = "Failed to load cards: \(error.localizedDescription)"
        }
    }

    private func closeReviewReasonPrompt() {
        reviewReasonPromptVisible = false
        reviewReasonTargetStatus = nil
        reviewReasonInput = ""
        reviewReasonErrorMessage = nil
    }

    private func refreshBoardAndSelectedCard() async {
        await reloadCards()
        if let selectedCardNumber {
            await selectCard(number: selectedCardNumber)
        }
    }

    private func executeReviewTransition(to status: String, reason: String?) async {
        guard let selectedProjectSlug, let selectedCardNumber else {
            return
        }
        isReviewTransitionInFlight = true
        defer { isReviewTransitionInFlight = false }

        do {
            try await store.moveCard(projectSlug: selectedProjectSlug, number: selectedCardNumber, status: status)
            if let reason {
                let commentBody = "Moved back to \(status): \(reason)"
                do {
                    try await store.commentOnCard(projectSlug: selectedProjectSlug, number: selectedCardNumber, body: commentBody)
                } catch {
                    try? await store.moveCard(
                        projectSlug: selectedProjectSlug,
                        number: selectedCardNumber,
                        status: KanbanLaneStatus.review.rawValue
                    )
                    alertMessage = "Failed to add transition reason"
                    closeReviewReasonPrompt()
                    await refreshBoardAndSelectedCard()
                    return
                }
            }
            closeReviewReasonPrompt()
            closeCardDetails()
        } catch {
            alertMessage = "Failed to move card: \(error.localizedDescription)"
        }

        await refreshBoardAndSelectedCard()
    }
}
