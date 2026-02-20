import Testing
@testable import KanbanMacOS

struct ZoomControllerTests {
    @Test
    func zoomInIncreasesScaleByStep() {
        var zoom = ZoomController()

        zoom.zoomIn()

        #expect(zoom.scale == 1.1)
    }

    @Test
    func zoomOutDecreasesScaleByStep() {
        var zoom = ZoomController()

        zoom.zoomOut()

        #expect(zoom.scale == 0.9)
    }

    @Test
    func zoomInClampsAtMaximum() {
        var zoom = ZoomController(scale: 1.58)

        zoom.zoomIn()

        #expect(zoom.scale == ZoomController.maxScale)
    }

    @Test
    func zoomOutClampsAtMinimum() {
        var zoom = ZoomController(scale: 0.62)

        zoom.zoomOut()

        #expect(zoom.scale == ZoomController.minScale)
    }
}
