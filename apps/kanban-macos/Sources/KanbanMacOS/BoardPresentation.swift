import SwiftUI

struct BoardPresentation {
    static let laneSpacing: Double = 12
    static let horizontalPadding: Double = 16
    static let verticalPadding: Double = 12

    static let cardTitleRGB = RGBColor(red: 0.12, green: 0.14, blue: 0.18)
    static let cardBackgroundRGB = RGBColor(red: 0.94, green: 0.95, blue: 0.97)

    static var cardTitleColor: Color {
        Color(red: cardTitleRGB.red, green: cardTitleRGB.green, blue: cardTitleRGB.blue)
    }

    static var cardBackgroundColor: Color {
        Color(red: cardBackgroundRGB.red, green: cardBackgroundRGB.green, blue: cardBackgroundRGB.blue)
    }

    static func laneWidth(containerWidth: Double, laneCount: Int = 4) -> Double {
        guard laneCount > 0 else {
            return 0
        }

        let totalSpacing = Double(max(0, laneCount - 1)) * laneSpacing
        let totalPadding = horizontalPadding * 2
        let usableWidth = max(0, containerWidth - totalSpacing - totalPadding)
        return usableWidth / Double(laneCount)
    }
}

struct RGBColor {
    let red: Double
    let green: Double
    let blue: Double

    var luminance: Double {
        0.2126 * red + 0.7152 * green + 0.0722 * blue
    }
}
