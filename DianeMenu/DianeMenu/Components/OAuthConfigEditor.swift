import SwiftUI

struct OAuthConfigEditor: View {
    @Binding var config: OAuthConfig?
    
    @State private var enabled: Bool
    @State private var provider: String
    @State private var clientID: String
    @State private var clientSecret: String
    @State private var scopes: [String]
    @State private var deviceAuthURL: String
    @State private var tokenURL: String
    
    init(config: Binding<OAuthConfig?>) {
        self._config = config
        
        let initialConfig = config.wrappedValue
        self._enabled = State(initialValue: initialConfig != nil)
        self._provider = State(initialValue: initialConfig?.provider ?? "")
        self._clientID = State(initialValue: initialConfig?.clientID ?? "")
        self._clientSecret = State(initialValue: initialConfig?.clientSecret ?? "")
        self._scopes = State(initialValue: initialConfig?.scopes ?? [])
        self._deviceAuthURL = State(initialValue: initialConfig?.deviceAuthURL ?? "")
        self._tokenURL = State(initialValue: initialConfig?.tokenURL ?? "")
    }
    
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Toggle("Enable OAuth", isOn: $enabled)
                .onChange(of: enabled) { _, newValue in
                    updateConfig()
                }
            
            if enabled {
                VStack(alignment: .leading, spacing: 8) {
                    TextField("Provider (e.g., github, google)", text: $provider)
                        .textFieldStyle(.roundedBorder)
                        .onChange(of: provider) { _, _ in updateConfig() }
                    
                    TextField("Client ID", text: $clientID)
                        .textFieldStyle(.roundedBorder)
                        .onChange(of: clientID) { _, _ in updateConfig() }
                    
                    SecureField("Client Secret", text: $clientSecret)
                        .textFieldStyle(.roundedBorder)
                        .onChange(of: clientSecret) { _, _ in updateConfig() }
                    
                    StringArrayEditor(
                        items: $scopes,
                        title: "Scopes",
                        placeholder: "Add scope (e.g., user:email)"
                    )
                    .onChange(of: scopes) { _, _ in updateConfig() }
                    
                    TextField("Device Auth URL (optional)", text: $deviceAuthURL)
                        .textFieldStyle(.roundedBorder)
                        .onChange(of: deviceAuthURL) { _, _ in updateConfig() }
                    
                    TextField("Token URL (optional)", text: $tokenURL)
                        .textFieldStyle(.roundedBorder)
                        .onChange(of: tokenURL) { _, _ in updateConfig() }
                }
                .padding(.leading, 16)
            }
        }
    }
    
    private func updateConfig() {
        if enabled && !provider.isEmpty {
            config = OAuthConfig(
                provider: provider,
                clientID: clientID.isEmpty ? nil : clientID,
                clientSecret: clientSecret.isEmpty ? nil : clientSecret,
                scopes: scopes.isEmpty ? nil : scopes,
                deviceAuthURL: deviceAuthURL.isEmpty ? nil : deviceAuthURL,
                tokenURL: tokenURL.isEmpty ? nil : tokenURL
            )
        } else {
            config = nil
        }
    }
}
