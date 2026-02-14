import SwiftUI

@main
struct CatalogApp: App {
    var body: some Scene {
        WindowGroup {
            CatalogContentView()
                .frame(minWidth: 1000, minHeight: 700)
        }
        .windowStyle(.titleBar)
        .defaultSize(width: 1200, height: 800)
    }
}
