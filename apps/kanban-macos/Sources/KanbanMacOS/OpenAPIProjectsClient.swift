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
                    status: $0.status
                )
            }
        default:
            throw URLError(.badServerResponse)
        }
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
