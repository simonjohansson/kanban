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
        }
    }

    private func shouldRefreshCards(for event: ProjectEvent) -> Bool {
        guard let selected = selectedProjectSlug else {
            return false
        }
        if event.type.hasPrefix("project.") {
            return true
        }
        return event.projectSlug == nil || event.projectSlug == selected
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
}
