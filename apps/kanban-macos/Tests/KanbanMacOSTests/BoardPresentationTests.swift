import Testing
@testable import KanbanMacOS

struct BoardPresentationTests {
    @Test
    func laneWidthUsesAllAvailableHorizontalSpace() {
        let containerWidth: Double = 1200
        let expected = (containerWidth - (BoardPresentation.horizontalPadding * 2) - (BoardPresentation.laneSpacing * 3)) / 4

        let actual = BoardPresentation.laneWidth(containerWidth: containerWidth)

        #expect(actual == expected)
    }

    @Test
    func laneWidthNeverGoesNegative() {
        let actual = BoardPresentation.laneWidth(containerWidth: 0)

        #expect(actual == 0)
    }

    @Test
    func cardTitleColorHasStrongContrastAgainstBackground() {
        let title = BoardPresentation.cardTitleRGB.luminance
        let background = BoardPresentation.cardBackgroundRGB.luminance

        #expect(background - title >= 0.35)
    }
}
