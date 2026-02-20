import Foundation

struct ZoomController {
    static let minScale: Double = 0.6
    static let maxScale: Double = 1.6
    static let step: Double = 0.1

    var scale: Double

    init(scale: Double = 1.0) {
        self.scale = Self.clamp(scale)
    }

    mutating func zoomIn() {
        scale = Self.clamp(scale + Self.step)
    }

    mutating func zoomOut() {
        scale = Self.clamp(scale - Self.step)
    }

    mutating func reset() {
        scale = 1.0
    }

    private static func clamp(_ value: Double) -> Double {
        min(max(value, minScale), maxScale)
    }
}
