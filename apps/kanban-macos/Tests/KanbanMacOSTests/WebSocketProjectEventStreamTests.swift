import Foundation
import Testing
@testable import KanbanMacOS

struct WebSocketProjectEventStreamTests {
    @Test
    func decodeProjectEventPayloadParsesKnownEventKinds() throws {
        let payload = Data(#"{"type":"project.created","project":"alpha","timestamp":"2026-02-21T15:00:00Z"}"#.utf8)

        let event = try decodeProjectEventPayload(payload)

        #expect(event.type == .project_period_created)
        #expect(event.projectSlug == "alpha")
    }

    @Test
    func decodeProjectEventPayloadRejectsUnknownEventKinds() throws {
        let payload = Data(#"{"type":"project.renamed","project":"alpha","timestamp":"2026-02-21T15:00:00Z"}"#.utf8)

        #expect(throws: Error.self) {
            _ = try decodeProjectEventPayload(payload)
        }
    }

    @Test
    func decodeProjectEventPayloadRejectsMalformedPayloads() throws {
        let payload = Data(#"{"type":"project.created","project":123,"timestamp":"2026-02-21T15:00:00Z"}"#.utf8)

        #expect(throws: Error.self) {
            _ = try decodeProjectEventPayload(payload)
        }
    }
}
