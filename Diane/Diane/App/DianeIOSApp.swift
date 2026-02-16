import SwiftUI

@main
struct DianeIOSApp: App {
    @State private var config = ServerConfiguration()
    @State private var client: DianeHTTPClient?
    @State private var monitor: IOSStatusMonitor?
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            Group {
                if config.isConfigured, let client, let monitor {
                    IOSContentView()
                        .environment(\.dianeClient, client)
                        .environment(monitor)
                        .environment(config)
                } else {
                    ServerSetupView(config: config) {
                        setupClient()
                    }
                }
            }
            .onChange(of: scenePhase) { _, newPhase in
                monitor?.handleScenePhase(newPhase)
            }
            .onAppear {
                if config.isConfigured {
                    setupClient()
                }
            }
        }
    }

    private func setupClient() {
        guard let url = config.baseURL else { return }
        let key = config.apiKey.isEmpty ? nil : config.apiKey
        let newClient = DianeHTTPClient(baseURL: url, apiKey: key)
        let newMonitor = IOSStatusMonitor(client: newClient)
        self.client = newClient
        self.monitor = newMonitor
        newMonitor.startPolling()
    }
}
