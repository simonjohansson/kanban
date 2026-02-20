import Foundation

public struct WebSocketProjectEventStream: ProjectEventStream {
    private let serverURL: URL
    private let session: URLSession

    public init(serverURL: URL, session: URLSession = .shared) {
        self.serverURL = serverURL
        self.session = session
    }

    public var stream: AsyncThrowingStream<ProjectEvent, Error> {
        AsyncThrowingStream { continuation in
            guard let wsURL = websocketURL(from: serverURL) else {
                continuation.finish(throwing: URLError(.badURL))
                return
            }

            let task = session.webSocketTask(with: wsURL)
            task.resume()

            let worker = Task {
                defer {
                    task.cancel(with: .normalClosure, reason: nil)
                }

                while !Task.isCancelled {
                    do {
                        let message = try await task.receive()
                        let payload: Data
                        switch message {
                        case .data(let data):
                            payload = data
                        case .string(let text):
                            payload = Data(text.utf8)
                        @unknown default:
                            continue
                        }

                        if let raw = try? JSONDecoder().decode(RawEvent.self, from: payload),
                           let project = raw.project?.trimmingCharacters(in: .whitespacesAndNewlines),
                           !project.isEmpty
                        {
                            continuation.yield(ProjectEvent(type: raw.type, projectSlug: project))
                        }
                    } catch {
                        continuation.finish(throwing: error)
                        return
                    }
                }
                continuation.finish()
            }

            continuation.onTermination = { _ in
                worker.cancel()
            }
        }
    }
}

private struct RawEvent: Decodable {
    let type: String
    let project: String?
}

private func websocketURL(from baseURL: URL) -> URL? {
    guard var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false) else {
        return nil
    }
    components.path = "/ws"
    components.query = nil
    switch components.scheme {
    case "https":
        components.scheme = "wss"
    default:
        components.scheme = "ws"
    }
    return components.url
}
