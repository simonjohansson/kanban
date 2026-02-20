import Foundation

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

public struct ProjectEvent: Equatable, Sendable {
    public let type: String
    public let projectSlug: String

    public init(type: String, projectSlug: String) {
        self.type = type
        self.projectSlug = projectSlug
    }
}
