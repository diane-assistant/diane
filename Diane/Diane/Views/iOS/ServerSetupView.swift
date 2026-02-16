import SwiftUI

/// First-launch screen for configuring the Diane server connection.
/// Supports pairing code (recommended) or manual API key entry.
struct ServerSetupView: View {
    @Bindable var config: ServerConfiguration
    var onConnected: () -> Void

    @State private var hostInput: String = ""
    @State private var portInput: String = "9090"
    @State private var pairingCodeInput: String = ""
    @State private var apiKeyInput: String = ""
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
        NavigationStack {
            ScrollView {
                VStack(spacing: 28) {
                    // Header
                    VStack(spacing: 8) {
                        Image(systemName: "server.rack")
                            .font(.system(size: 48))
                            .foregroundStyle(.secondary)
                        Text("Connect to Diane")
                            .font(.title.bold())
                        Text("Enter your server address and authenticate")
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                    }
                    .padding(.top, 24)

                    // Server address
                    VStack(spacing: 12) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Host")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            TextField("my-mac or my-mac.tail12345.ts.net", text: $hostInput)
                                .textFieldStyle(.roundedBorder)
                                .textContentType(.URL)
                                .autocorrectionDisabled()
                                .textInputAutocapitalization(.never)
                                .keyboardType(.URL)
                        }

                        VStack(alignment: .leading, spacing: 4) {
                            Text("Port")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            TextField("9090", text: $portInput)
                                .textFieldStyle(.roundedBorder)
                                .keyboardType(.numberPad)
                        }
                    }
                    .padding(.horizontal, 24)

                    // Auth method picker
                    VStack(spacing: 12) {
                        Picker("Authentication", selection: $authMethod) {
                            ForEach(AuthMethod.allCases, id: \.self) { method in
                                Text(method.rawValue).tag(method)
                            }
                        }
                        .pickerStyle(.segmented)
                        .onChange(of: authMethod) { _, _ in
                            testResult = nil
                        }

                        // Auth fields
                        if authMethod == .pairingCode {
                            VStack(alignment: .leading, spacing: 4) {
                                Text("Pairing Code")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                TextField("Run 'diane pair' on your server", text: $pairingCodeInput)
                                    .textFieldStyle(.roundedBorder)
                                    .font(.system(.body, design: .monospaced))
                                    .keyboardType(.numberPad)
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
                    }
                    .padding(.horizontal, 24)

                    Text("Connects over plain HTTP \u{2014} use Tailscale or a VPN for secure access")
                        .font(.caption2)
                        .foregroundStyle(.tertiary)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, 24)

                    // Connect button
                    Button(action: connectRemote) {
                        HStack {
                            if isTesting {
                                ProgressView()
                                    .controlSize(.small)
                            }
                            Text(isTesting ? "Connecting..." : "Connect")
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(connectButtonDisabled)
                    .padding(.horizontal, 24)

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
                                .multilineTextAlignment(.center)
                                .padding(.horizontal, 24)
                        }
                    }

                    Spacer(minLength: 40)
                }
            }
            .navigationTitle("Setup")
            .navigationBarTitleDisplayMode(.inline)
        }
        .onAppear {
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

    private func connectRemote() {
        if authMethod == .pairingCode {
            pairWithCode()
        } else {
            testDirectConnection()
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
                        onConnected()
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
                    if let json = try? JSONSerialization.jsonObject(with: data) as? [String: String],
                       let errorMsg = json["error"] {
                        testResult = .failure(errorMsg)
                    } else {
                        testResult = .failure("Server returned status \(httpResponse.statusCode)")
                    }
                }
            } catch let error as NSError {
                handleConnectionError(error, host: host, port: port)
            }

            isTesting = false
        }
    }

    /// Direct connection with an API key (existing flow).
    private func testDirectConnection() {
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
                        onConnected()
                    } else if httpResponse.statusCode == 401 {
                        testResult = .failure("Unauthorized. Check your API key.")
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
                testResult = .failure("App Transport Security blocked the connection.")
            default:
                testResult = .failure("Connection error: \(error.localizedDescription)")
            }
        } else {
            testResult = .failure("Error: \(error.localizedDescription)")
        }
    }
}
