import SwiftUI

@Observable
final class CatalogTheme {
    var spacing: CGFloat = 8
    var padding: CGFloat = 16
    var fontSize: CGFloat = 13
    var cornerRadius: CGFloat = 8
    var accentColor: Color = .accentColor
    var backgroundColor: Color = .clear
    var canvasWidth: CGFloat = 800
    var canvasHeight: CGFloat = 600

    struct SizePreset: Identifiable {
        let id: String
        let name: String
        let width: CGFloat
        let height: CGFloat
    }

    static let sizePresets: [SizePreset] = [
        SizePreset(id: "compact", name: "Compact", width: 400, height: 300),
        SizePreset(id: "default", name: "Default", width: 800, height: 600),
        SizePreset(id: "wide", name: "Wide", width: 1200, height: 800),
    ]

    func applyPreset(_ preset: SizePreset) {
        canvasWidth = preset.width
        canvasHeight = preset.height
    }

    func reset() {
        spacing = 8
        padding = 16
        fontSize = 13
        cornerRadius = 8
        accentColor = .accentColor
        backgroundColor = .clear
        canvasWidth = 800
        canvasHeight = 600
    }
}
