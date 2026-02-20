import Testing
@testable import KanbanMacOS

struct ZoomPresentationTests {
    @Test
    func scaledValueGrowsWithZoom() {
        let presentation = ZoomPresentation(scale: 1.2)
        #expect(presentation.scaled(10) == 12)
    }

    @Test
    func scaleIsClampedToControllerBounds() {
        let low = ZoomPresentation(scale: 0.2)
        let high = ZoomPresentation(scale: 2.4)

        #expect(low.scale == ZoomController.minScale)
        #expect(high.scale == ZoomController.maxScale)
    }
}
