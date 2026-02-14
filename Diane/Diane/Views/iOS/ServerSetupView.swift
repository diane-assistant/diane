import SwiftUI

/// First-launch screen for configuring the Diane server connection.
/// Shows host/port fields with a Connect button that tests the connection.
struct ServerSetupView: View {
    @Bindable var config: ServerConfiguration
    var onConnected: () -> Void

    @State private var hostInput: String = ""
    @State private var portInput: String = "8080"
    @State private var isTesting = false
    @State private var testResult: TestResult?

    enum TestResult {
        case success
        case failure(String)
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 32) {
                Spacer()

                // Header
                VStack(spacing: 8) {
                    Image(systemName: "server.rack")
                        .font(.system(size: 48))
                        .foregroundStyle(.secondary)
                    Text("Connect to Diane")
                        .font(.title.bold())
                    Text("Enter your Diane server address to get started")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                }

                // Form
                VStack(spacing: 16) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Host")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        TextField("my-mac or my-mac.tail12345.ts.net", text: $hostInput)
                            .textFieldStyle(.roundedBorder)
                            .textContentType(.URL)
                            .autocorrectionDisabled()
                            #if os(iOS)
                            .textInputAutocapitalization(.never)
                            .keyboardType(.URL)
                            #endif
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Port")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        TextField("8080", text: $portInput)
                            .textFieldStyle(.roundedBorder)
                            #if os(iOS)
                            .keyboardType(.numberPad)
                            #endif
                    }

                    Text("Connects over plain HTTP — use Tailscale VPN for secure access")
                        .font(.caption2)
                        .foregroundStyle(.tertiary)
                        .multilineTextAlignment(.center)
                }
                .padding(.horizontal, 24)

                // Connect button
                Button(action: testConnection) {
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
                .disabled(hostInput.trimmingCharacters(in: .whitespaces).isEmpty || isTesting)
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

                Spacer()
                Spacer()
            }
            .navigationTitle("Setup")
            .navigationBarTitleDisplayMode(.inline)
        }
        .onAppear {
            // Pre-fill from existing config if available
            if !config.host.isEmpty {
                hostInput = config.host
            }
            if config.port > 0 {
                portInput = String(config.port)
            }
        }
    }

    private func testConnection() {
        let host = hostInput.trimmingCharacters(in: .whitespaces)
        guard !host.isEmpty else { return }

        let port = Int(portInput) ?? 8080

        guard let url = URL(string: "http://\(host):\(port)") else {
            testResult = .failure("Invalid address")
            return
        }

        isTesting = true
        testResult = nil

        Task {
            let client = DianeHTTPClient(baseURL: url)
            let healthy = await client.health()

            if healthy {
                config.host = host
                config.port = port
                testResult = .success

                // Small delay so user sees success before transition
                try? await Task.sleep(nanoseconds: 500_000_000)
                onConnected()
            } else {
                testResult = .failure("Could not reach server at \(host):\(port)")
            }

            isTesting = false
        }
    }
}
