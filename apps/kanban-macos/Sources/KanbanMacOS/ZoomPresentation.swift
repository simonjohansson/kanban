import Foundation

struct ZoomPresentation {
    let scale: Double

    init(scale: Double) {
        self.scale = min(max(scale, ZoomController.minScale), ZoomController.maxScale)
    }

    func scaled(_ value: Double) -> Double {
        value * scale
    }
}
