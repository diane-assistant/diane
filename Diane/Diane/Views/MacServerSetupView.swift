import SwiftUI

/// First-launch screen for macOS where users choose how to connect to Diane.
/// Options: Local (Unix socket, auto-start daemon) or Remote (HTTP to a remote server).
/// Remote supports pairing code (recommended) or manual API key entry.
struct MacServerSetupView: View {
    @Bindable var config: ServerConfiguration
    var onConfigured: () -> Void
    
    @State private var selectedMode: ConnectionMode? = nil
    
    // Remote fields
    @State private var hostInput: String = ""
    @State private var portInput: String = "9090"
    @State private var apiKeyInput: String = ""
    @State private var pairingCodeInput: String = ""
    @State private var authMethod: AuthMethod = .pairingCode
    @State private var isTesting = false
    @State private var testResult: TestResult?
    
    enum AuthMethod: String, CaseIterable {
        case pairingCode = "Pairing Code"
        case apiKey = "API Key"
    }
    
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
                authMethod = .apiKey
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
            // Host and port
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
            
            // Auth method picker
            Picker("Authentication", selection: $authMethod) {
                ForEach(AuthMethod.allCases, id: \.self) { method in
                    Text(method.rawValue).tag(method)
                }
            }
            .pickerStyle(.segmented)
            .onChange(of: authMethod) { _, _ in
                testResult = nil
            }
            
            // Auth fields based on method
            if authMethod == .pairingCode {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Pairing Code")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextField("Run 'diane pair' on your server", text: $pairingCodeInput)
                        .textFieldStyle(.roundedBorder)
                        .font(.system(.body, design: .monospaced))
                    Text("Run diane pair on your server to get a 6-digit code")
                        .font(.caption2)
                        .foregroundStyle(.tertiary)
                }
            } else {
                VStack(alignment: .leading, spacing: 4) {
                    Text("API Key")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    SecureField("Leave empty if not required", text: $apiKeyInput)
                        .textFieldStyle(.roundedBorder)
                }
            }
            
            Text("Connects over plain HTTP \u{2014} use Tailscale or a VPN for secure access")
                .font(.caption2)
                .foregroundStyle(.tertiary)
            
            // Connect button
            Button(action: connectRemote) {
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
            .disabled(connectButtonDisabled)
            
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
    
    private var connectButtonDisabled: Bool {
        if isTesting { return true }
        if hostInput.trimmingCharacters(in: .whitespaces).isEmpty { return true }
        if authMethod == .pairingCode {
            let code = pairingCodeInput.replacingOccurrences(of: " ", with: "")
            if code.isEmpty { return true }
        }
        return false
    }
    
    // MARK: - Actions
    
    private func configureLocal() {
        config.setLocal()
        onConfigured()
    }
    
    private func connectRemote() {
        if authMethod == .pairingCode {
            pairWithCode()
        } else {
            testRemoteConnection()
        }
    }
    
    /// Pair using a time-based code: POST /pair with the code, receive the API key.
    private func pairWithCode() {
        let host = hostInput.trimmingCharacters(in: .whitespaces)
        guard !host.isEmpty else { return }
        
        let port = Int(portInput) ?? 9090
        let code = pairingCodeInput.replacingOccurrences(of: " ", with: "")
        
        guard !code.isEmpty else {
            testResult = .failure("Enter the pairing code from 'diane pair'")
            return
        }
        
        guard let baseURL = URL(string: "http://\(host):\(port)") else {
            testResult = .failure("Invalid address")
            return
        }
        
        isTesting = true
        testResult = nil
        
        Task {
            do {
                // Step 1: Exchange pairing code for API key
                let pairURL = baseURL.appendingPathComponent("pair")
                var request = URLRequest(url: pairURL)
                request.httpMethod = "POST"
                request.timeoutInterval = 10
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")
                
                let body = ["code": code]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)
                
                let (data, response) = try await URLSession.shared.data(for: request)
                
                guard let httpResponse = response as? HTTPURLResponse else {
                    testResult = .failure("Invalid response from server")
                    isTesting = false
                    return
                }
                
                if httpResponse.statusCode == 200 {
                    // Parse the API key from response
                    guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: String],
                          let apiKey = json["api_key"], !apiKey.isEmpty else {
                        testResult = .failure("Server returned invalid pairing response")
                        isTesting = false
                        return
                    }
                    
                    // Step 2: Verify the API key works with a health check
                    var healthRequest = URLRequest(url: baseURL.appendingPathComponent("health"))
                    healthRequest.httpMethod = "GET"
                    healthRequest.timeoutInterval = 10
                    healthRequest.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
                    
                    let (_, healthResponse) = try await URLSession.shared.data(for: healthRequest)
                    
                    if let healthHTTP = healthResponse as? HTTPURLResponse,
                       (200...299).contains(healthHTTP.statusCode) {
                        config.setRemote(host: host, port: port, apiKey: apiKey)
                        testResult = .success
                        
                        try? await Task.sleep(nanoseconds: 500_000_000)
                        onConfigured()
                    } else {
                        testResult = .failure("Pairing succeeded but health check failed")
                    }
                } else if httpResponse.statusCode == 401 {
                    testResult = .failure("Invalid or expired pairing code. Run 'diane pair' again.")
                } else if httpResponse.statusCode == 429 {
                    testResult = .failure("Too many attempts. Wait a moment and try again.")
                } else if httpResponse.statusCode == 503 {
                    testResult = .failure("Pairing not available (no API key configured on server)")
                } else {
                    // Try to parse error message
                    if let json = try? JSONSerialization.jsonObject(with: data) as? [String: String],
                       let errorMsg = json["error"] {
                        testResult = .failure(errorMsg)
                    } else {
                        testResult = .failure("Server returned status \(httpResponse.statusCode)")
                    }
                }
            } catch let error as NSError {
                handleConnectionError(error, host: host, port: Int(portInput) ?? 9090)
            }
            
            isTesting = false
        }
    }
    
    /// Direct connection with an API key (existing flow).
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
                handleConnectionError(error, host: host, port: port)
            }
            
            isTesting = false
        }
    }
    
    private func handleConnectionError(_ error: NSError, host: String, port: Int) {
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
}
