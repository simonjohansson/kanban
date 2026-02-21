import Foundation

struct SidebarStateProbe {
    let outputURL: URL?
    let selectionInputURL: URL?

    static func fromEnvironment(environment: [String: String] = ProcessInfo.processInfo.environment) -> Self {
        let outputURL: URL?
        if let raw = environment["KANBAN_E2E_SIDEBAR_STATE_PATH"]?
            .trimmingCharacters(in: .whitespacesAndNewlines),
            !raw.isEmpty {
            outputURL = URL(fileURLWithPath: raw, isDirectory: false)
        } else {
            outputURL = nil
        }

        let selectionInputURL: URL?
        if let raw = environment["KANBAN_E2E_SIDEBAR_SELECT_PATH"]?
            .trimmingCharacters(in: .whitespacesAndNewlines),
            !raw.isEmpty {
            selectionInputURL = URL(fileURLWithPath: raw, isDirectory: false)
        } else {
            selectionInputURL = nil
        }

        return SidebarStateProbe(outputURL: outputURL, selectionInputURL: selectionInputURL)
    }

    func write(
        projects: [ProjectSummary],
        selectedProjectSlug: String?,
        cardsByStatus: [String: [String]],
        cardsByStatusDetailed: [String: [SidebarCardStateProbe]],
        cardDetailsVisible: Bool,
        cardDetails: SidebarCardDetailsStateProbe?,
        cardDetailsError: String?,
        reviewReasonPromptVisible: Bool,
        reviewReasonTargetStatus: String?,
        reviewReasonError: String?
    ) {
        guard let outputURL else {
            return
        }

        let payload = SidebarStateProbePayload(
            projects: projects.map(\.name),
            selectedProjectSlug: selectedProjectSlug,
            cardsByStatus: cardsByStatus,
            cardsByStatusDetailed: cardsByStatusDetailed,
            cardDetailsVisible: cardDetailsVisible,
            cardDetails: cardDetails,
            cardDetailsError: cardDetailsError,
            reviewReasonPromptVisible: reviewReasonPromptVisible,
            reviewReasonTargetStatus: reviewReasonTargetStatus,
            reviewReasonError: reviewReasonError
        )
        guard let raw = try? JSONEncoder().encode(payload) else {
            return
        }
        try? raw.write(to: outputURL, options: [.atomic])
    }

    func consumeSelectionRequest() -> String? {
        guard let selectionInputURL else {
            return nil
        }
        guard let raw = try? String(contentsOf: selectionInputURL, encoding: .utf8) else {
            return nil
        }
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        try? FileManager.default.removeItem(at: selectionInputURL)
        return trimmed.isEmpty ? nil : trimmed
    }
}

struct SidebarStateProbePayload: Codable, Equatable {
    let projects: [String]
    let selectedProjectSlug: String?
    let cardsByStatus: [String: [String]]
    let cardsByStatusDetailed: [String: [SidebarCardStateProbe]]
    let cardDetailsVisible: Bool
    let cardDetails: SidebarCardDetailsStateProbe?
    let cardDetailsError: String?
    let reviewReasonPromptVisible: Bool
    let reviewReasonTargetStatus: String?
    let reviewReasonError: String?

    enum CodingKeys: String, CodingKey {
        case projects
        case selectedProjectSlug = "selected_project_slug"
        case cardsByStatus = "cards_by_status"
        case cardsByStatusDetailed = "cards_by_status_detailed"
        case cardDetailsVisible = "card_details_visible"
        case cardDetails = "card_details"
        case cardDetailsError = "card_details_error"
        case reviewReasonPromptVisible = "review_reason_prompt_visible"
        case reviewReasonTargetStatus = "review_reason_target_status"
        case reviewReasonError = "review_reason_error"
    }
}

struct SidebarCardStateProbe: Codable, Equatable {
    let title: String
    let branch: String?
}

struct SidebarCardDetailsStateProbe: Codable, Equatable {
    let title: String
    let branch: String?
    let descriptionBodies: [String]
    let commentBodies: [String]

    enum CodingKeys: String, CodingKey {
        case title
        case branch
        case descriptionBodies = "description_bodies"
        case commentBodies = "comment_bodies"
    }
}
