import Foundation
import Testing
@testable import KanbanMacOS

struct OpenAPIProjectsClientTests {
    @Test
    func flexibleDateTranscoderDecodesFractionalRFC3339() throws {
        let transcoder = FlexibleRFC3339DateTranscoder()
        let parsed = try transcoder.decode("2026-02-20T15:42:48.982082Z")
        #expect(parsed.timeIntervalSince1970 > 0)
    }

    @Test
    func flexibleDateTranscoderDecodesPlainRFC3339() throws {
        let transcoder = FlexibleRFC3339DateTranscoder()
        let parsed = try transcoder.decode("2026-02-20T15:42:48Z")
        #expect(parsed.timeIntervalSince1970 > 0)
    }
}
