import Foundation

public enum KanbanLaneStatus: String, CaseIterable, Sendable {
    case todo = "Todo"
    case doing = "Doing"
    case review = "Review"
    case done = "Done"
}

public struct ProjectSummary: Equatable, Identifiable, Sendable {
    public var id: String { slug }

    public let name: String
    public let slug: String
    public let localPath: String?
    public let remoteURL: String?

    public init(name: String, slug: String, localPath: String?, remoteURL: String?) {
        self.name = name
        self.slug = slug
        self.localPath = localPath
        self.remoteURL = remoteURL
    }
}

public struct KanbanCardSummary: Equatable, Identifiable, Sendable {
    public let id: String
    public let number: Int
    public let projectSlug: String
    public let title: String
    public let branch: String?
    public let status: String

    public init(id: String, number: Int, projectSlug: String, title: String, branch: String? = nil, status: String) {
        self.id = id
        self.number = number
        self.projectSlug = projectSlug
        self.title = title
        self.branch = branch
        self.status = status
    }
}

public struct ProjectEvent: Equatable, Sendable {
    public let type: String
    public let projectSlug: String?

    public init(type: String, projectSlug: String?) {
        self.type = type
        self.projectSlug = projectSlug
    }
}
