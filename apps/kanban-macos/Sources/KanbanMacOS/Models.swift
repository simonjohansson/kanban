import Foundation
import KanbanAPI

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
    public let todosCount: Int
    public let todosCompletedCount: Int
    public let acceptanceCriteriaCount: Int
    public let acceptanceCriteriaCompletedCount: Int

    public init(
        id: String,
        number: Int,
        projectSlug: String,
        title: String,
        branch: String? = nil,
        status: String,
        todosCount: Int = 0,
        todosCompletedCount: Int = 0,
        acceptanceCriteriaCount: Int = 0,
        acceptanceCriteriaCompletedCount: Int = 0
    ) {
        self.id = id
        self.number = number
        self.projectSlug = projectSlug
        self.title = title
        self.branch = branch
        self.status = status
        self.todosCount = todosCount
        self.todosCompletedCount = todosCompletedCount
        self.acceptanceCriteriaCount = acceptanceCriteriaCount
        self.acceptanceCriteriaCompletedCount = acceptanceCriteriaCompletedCount
    }
}

public struct KanbanCardTextEvent: Equatable, Sendable {
    public let timestamp: String
    public let body: String

    public init(timestamp: String, body: String) {
        self.timestamp = timestamp
        self.body = body
    }
}

public struct KanbanTodo: Equatable, Sendable {
    public let id: Int
    public let text: String
    public let completed: Bool

    public init(id: Int, text: String, completed: Bool) {
        self.id = id
        self.text = text
        self.completed = completed
    }
}

public struct KanbanAcceptanceCriterion: Equatable, Sendable {
    public let id: Int
    public let text: String
    public let completed: Bool

    public init(id: Int, text: String, completed: Bool) {
        self.id = id
        self.text = text
        self.completed = completed
    }
}

public struct KanbanCardDetails: Equatable, Sendable {
    public let id: String
    public let number: Int
    public let projectSlug: String
    public let title: String
    public let branch: String?
    public let status: String
    public let description: [KanbanCardTextEvent]
    public let todos: [KanbanTodo]
    public let acceptanceCriteria: [KanbanAcceptanceCriterion]
    public let comments: [KanbanCardTextEvent]

    public init(
        id: String,
        number: Int,
        projectSlug: String,
        title: String,
        branch: String? = nil,
        status: String,
        description: [KanbanCardTextEvent],
        todos: [KanbanTodo],
        acceptanceCriteria: [KanbanAcceptanceCriterion] = [],
        comments: [KanbanCardTextEvent]
    ) {
        self.id = id
        self.number = number
        self.projectSlug = projectSlug
        self.title = title
        self.branch = branch
        self.status = status
        self.description = description
        self.todos = todos
        self.acceptanceCriteria = acceptanceCriteria
        self.comments = comments
    }
}

public typealias WebSocketEventType = Components.Schemas.WebsocketEventType

public struct ProjectEvent: Equatable, Sendable {
    public let type: WebSocketEventType
    public let projectSlug: String?

    public init(type: WebSocketEventType, projectSlug: String?) {
        self.type = type
        self.projectSlug = projectSlug
    }
}
