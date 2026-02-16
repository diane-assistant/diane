import SwiftUI

/// First-launch screen for macOS where users choose how to connect to Diane.
/// Options: Local (Unix socket, auto-start daemon) or Remote (HTTP to a remote server).
struct MacServerSetupView: View {
    @Bindable var config: ServerConfiguration
    var onConfigured: () -> Void
    
    @State private var selectedMode: ConnectionMode? = nil
    
    // Remote fields
    @State private var hostInput: String = ""
    @State private var portInput: String = "9090"
    @State private var apiKeyInput: String = ""
    @State private var isTesting = false
    @State private var testResult: TestResult?
    
    enum TestResult {
        case success
        case failure(String)
    }
    
    var body: some View {
        VStack(spacing: 0) {
            Spacer()
            
            VStack(spacing: 24) {
                // Header
                VStack(spacing: 8) {
                    Image(systemName: "recordingtape.circle.fill")
                        .font(.system(size: 48))
                        .foregroundStyle(.primary)
                    Text("Welcome to Diane")
                        .font(.title.bold())
                    Text("Choose how to connect to your Diane server")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                
                // Mode selection cards
                HStack(spacing: 16) {
                    modeCard(
                        mode: .local,
                        icon: "desktopcomputer",
                        title: "Local Server",
                        description: "Connect to Diane running on this Mac via Unix socket"
                    )
                    
                    modeCard(
                        mode: .remote,
                        icon: "network",
                        title: "Remote Server",
                        description: "Connect to Diane running on another machine via HTTP"
                    )
                }
                .padding(.horizontal, 16)
                
                // Remote server form (shown when remote is selected)
                if selectedMode == .remote {
                    remoteConfigForm
                        .transition(.opacity.combined(with: .move(edge: .bottom)))
                }
                
                // Action button
                if selectedMode == .local {
                    Button(action: configureLocal) {
                        Text("Continue with Local Server")
                            .frame(maxWidth: 280)
                            .padding(.vertical, 8)
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                }
            }
            .frame(maxWidth: 520)
            
            Spacer()
            Spacer()
        }
        .frame(minWidth: 600, minHeight: 400)
        .onAppear {
            // Pre-fill from existing config if available
            if !config.host.isEmpty {
                hostInput = config.host
            }
            if config.port > 0 {
                portInput = String(config.port)
            }
            if !config.apiKey.isEmpty {
                apiKeyInput = config.apiKey
            }
        }
    }
    
    // MARK: - Mode Card
    
    private func modeCard(mode: ConnectionMode, icon: String, title: String, description: String) -> some View {
        Button {
            withAnimation(.easeInOut(duration: 0.2)) {
                selectedMode = mode
                testResult = nil
            }
        } label: {
            VStack(spacing: 12) {
                Image(systemName: icon)
                    .font(.system(size: 28))
                    .foregroundStyle(selectedMode == mode ? .white : .secondary)
                
                Text(title)
                    .font(.headline)
                    .foregroundStyle(selectedMode == mode ? .white : .primary)
                
                Text(description)
                    .font(.caption)
                    .foregroundStyle(selectedMode == mode ? .white.opacity(0.8) : .secondary)
                    .multilineTextAlignment(.center)
                    .lineLimit(2)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 20)
            .padding(.horizontal, 16)
            .background(
                RoundedRectangle(cornerRadius: 10)
                    .fill(selectedMode == mode ? Color.accentColor : Color.primary.opacity(0.05))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 10)
                    .stroke(selectedMode == mode ? Color.accentColor : Color.primary.opacity(0.1), lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
    }
    
    // MARK: - Remote Config Form
    
    private var remoteConfigForm: some View {
        VStack(spacing: 12) {
            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Host")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextField("hostname or IP address", text: $hostInput)
                        .textFieldStyle(.roundedBorder)
                        .disableAutocorrection(true)
                }
                
                VStack(alignment: .leading, spacing: 4) {
                    Text("Port")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextField("9090", text: $portInput)
                        .textFieldStyle(.roundedBorder)
                        .frame(width: 80)
                }
            }
            
            VStack(alignment: .leading, spacing: 4) {
                Text("API Key (optional)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                SecureField("Leave empty if not required", text: $apiKeyInput)
                    .textFieldStyle(.roundedBorder)
            }
            
            Text("Connects over plain HTTP — use Tailscale or a VPN for secure access")
                .font(.caption2)
                .foregroundStyle(.tertiary)
            
            // Connect button
            Button(action: testRemoteConnection) {
                HStack {
                    if isTesting {
                        ProgressView()
                            .controlSize(.small)
                    }
                    Text(isTesting ? "Connecting..." : "Connect")
                }
                .frame(maxWidth: 280)
                .padding(.vertical, 8)
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)
            .disabled(hostInput.trimmingCharacters(in: .whitespaces).isEmpty || isTesting)
            
            // Result message
            if let result = testResult {
                switch result {
                case .success:
                    Label("Connected successfully", systemImage: "checkmark.circle.fill")
                        .foregroundStyle(.green)
                        .font(.subheadline)
                case .failure(let message):
                    Label(message, systemImage: "exclamationmark.triangle.fill")
                        .foregroundStyle(.red)
                        .font(.subheadline)
                }
            }
        }
        .padding(.horizontal, 16)
    }
    
    // MARK: - Actions
    
    private func configureLocal() {
        config.setLocal()
        onConfigured()
    }
    
    private func testRemoteConnection() {
        let host = hostInput.trimmingCharacters(in: .whitespaces)
        guard !host.isEmpty else { return }
        
        let port = Int(portInput) ?? 9090
        
        guard let url = URL(string: "http://\(host):\(port)") else {
            testResult = .failure("Invalid address")
            return
        }
        
        isTesting = true
        testResult = nil
        
        Task {
            let key = apiKeyInput.trimmingCharacters(in: .whitespaces)
            
            // Use a direct health check with error details instead of the simple bool
            do {
                var request = URLRequest(url: url.appendingPathComponent("health"))
                request.httpMethod = "GET"
                request.timeoutInterval = 10
                if !key.isEmpty {
                    request.setValue("Bearer \(key)", forHTTPHeaderField: "Authorization")
                }
                
                let (_, response) = try await URLSession.shared.data(for: request)
                
                if let httpResponse = response as? HTTPURLResponse {
                    if (200...299).contains(httpResponse.statusCode) {
                        config.setRemote(host: host, port: port, apiKey: key)
                        testResult = .success
                        
                        // Small delay so user sees success before transition
                        try? await Task.sleep(nanoseconds: 500_000_000)
                        onConfigured()
                    } else if httpResponse.statusCode == 401 {
                        testResult = .failure("Authentication failed (401). Check your API key.")
                    } else if httpResponse.statusCode == 403 {
                        testResult = .failure("Access forbidden (403). Check your API key.")
                    } else {
                        testResult = .failure("Server returned status \(httpResponse.statusCode)")
                    }
                } else {
                    testResult = .failure("Invalid response from server")
                }
            } catch let error as NSError {
                if error.domain == NSURLErrorDomain {
                    switch error.code {
                    case NSURLErrorTimedOut:
                        testResult = .failure("Connection timed out. Check host and port.")
                    case NSURLErrorCannotConnectToHost:
                        testResult = .failure("Cannot connect to \(host):\(port). Is the server running?")
                    case NSURLErrorCannotFindHost:
                        testResult = .failure("Cannot resolve hostname '\(host)'")
                    case NSURLErrorAppTransportSecurityRequiresSecureConnection:
                        testResult = .failure("App Transport Security blocked the request. Plain HTTP is not allowed.")
                    default:
                        testResult = .failure("Connection error: \(error.localizedDescription)")
                    }
                } else {
                    testResult = .failure("Error: \(error.localizedDescription)")
                }
            }
            
            isTesting = false
        }
    }
}
