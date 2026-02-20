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
