import Foundation
import KanbanAPI
import OpenAPIRuntime
import OpenAPIURLSession

public struct OpenAPIProjectsClient: ProjectsAPIClient {
    private let client: Client

    public init(serverURL: URL) throws {
        let configuration = Configuration(dateTranscoder: FlexibleRFC3339DateTranscoder())
        self.client = Client(serverURL: serverURL, configuration: configuration, transport: URLSessionTransport())
    }

    public func listProjects() async throws -> [ProjectSummary] {
        let response = try await client.listProjects()
        switch response {
        case .ok(let ok):
            let body = try ok.body.json
            return body.projects.map {
                ProjectSummary(
                    name: $0.name,
                    slug: $0.slug,
                    localPath: $0.local_path,
                    remoteURL: $0.remote_url
                )
            }
        default:
            throw URLError(.badServerResponse)
        }
    }

    public func listCards(projectSlug: String) async throws -> [KanbanCardSummary] {
        let response = try await client.listCards(path: .init(project: projectSlug))
        switch response {
        case .ok(let ok):
            let body = try ok.body.json
            return body.cards.map {
                KanbanCardSummary(
                    id: $0.id,
                    number: Int($0.number),
                    projectSlug: $0.project,
                    title: $0.title,
                    branch: $0.branch,
                    status: $0.status
                )
            }
        default:
            throw URLError(.badServerResponse)
        }
    }

    public func getCard(projectSlug: String, number: Int) async throws -> KanbanCardDetails {
        let response = try await client.getCard(path: .init(project: projectSlug, number: Int64(number)))
        switch response {
        case .ok(let ok):
            let body = try ok.body.json
            return KanbanCardDetails(
                id: body.id,
                number: Int(body.number),
                projectSlug: body.project,
                title: body.title,
                branch: body.branch,
                status: body.status,
                description: body.description.map {
                    KanbanCardTextEvent(
                        timestamp: Self.formatTimestamp($0.timestamp),
                        body: Self.normalizeEscapedNewlines($0.body)
                    )
                },
                comments: body.comments.map {
                    KanbanCardTextEvent(
                        timestamp: Self.formatTimestamp($0.timestamp),
                        body: Self.normalizeEscapedNewlines($0.body)
                    )
                }
            )
        default:
            throw URLError(.badServerResponse)
        }
    }

    public func moveCard(projectSlug: String, number: Int, status: String) async throws {
        let response = try await client.moveCard(
            path: .init(project: projectSlug, number: Int64(number)),
            body: .json(.init(status: status))
        )
        switch response {
        case .ok:
            return
        default:
            throw URLError(.badServerResponse)
        }
    }

    public func commentOnCard(projectSlug: String, number: Int, body: String) async throws {
        let response = try await client.commentCard(
            path: .init(project: projectSlug, number: Int64(number)),
            body: .json(.init(body: body))
        )
        switch response {
        case .ok:
            return
        default:
            throw URLError(.badServerResponse)
        }
    }

    private static func formatTimestamp(_ date: Date) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.string(from: date)
    }

    private static func normalizeEscapedNewlines(_ text: String) -> String {
        text.replacingOccurrences(of: "\\n", with: "\n")
    }
}

struct FlexibleRFC3339DateTranscoder: DateTranscoder {
    private let fractional: any DateTranscoder = .iso8601WithFractionalSeconds
    private let plain: any DateTranscoder = .iso8601

    func encode(_ date: Date) throws -> String {
        try fractional.encode(date)
    }

    func decode(_ dateString: String) throws -> Date {
        if let parsed = try? fractional.decode(dateString) {
            return parsed
        }
        return try plain.decode(dateString)
    }
}
