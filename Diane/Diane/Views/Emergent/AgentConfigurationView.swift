import SwiftUI

// 2.1 Build the SwiftUI form for Custom Agent configuration
struct AgentConfigurationView: View {
    // 2.2 Wire the configuration form to EmergentAgentService
    @EnvironmentObject var agentService: EmergentAgentService
    
    @State private var name: String = ""
    @State private var persona: String = ""
    @State private var tools: [String] = []
    
    // 2.3 Handle states
    @State private var agentState: EmergentAgentState = .pending
    @State private var errorMessage: String? = nil
    
    var body: some View {
        VStack(spacing: Spacing.large.rawValue) {
            DetailSection(title: "Agent Configuration") {
                VStack(spacing: Spacing.medium.rawValue) {
                    TextField("Agent Name", text: $name)
                        .textFieldStyle(.roundedBorder)
                        .padding(.horizontal, Padding.small.rawValue)
                    
                    TextEditor(text: $persona)
                        .frame(height: 100)
                        .border(Color.gray.opacity(0.2))
                        .padding(.horizontal, Padding.small.rawValue)
                        .overlay(
                            RoundedRectangle(cornerRadius: CornerRadius.medium.rawValue)
                                .stroke(Color.gray.opacity(0.2), lineWidth: 1)
                        )
                }
                .padding(.vertical, Padding.section.rawValue)
            }
            
            // 2.4 Use standard Spacing.large for loading and error states during deployment
            if agentState == .deploying {
                ProgressView("Deploying Agent...")
                    .padding(Spacing.large.rawValue)
            } else if let error = errorMessage {
                Text(error)
                    .foregroundColor(.red)
                    .padding(Spacing.large.rawValue)
            } else {
                HStack(spacing: Spacing.medium.rawValue) {
                    Button("Save Agent") {
                        Task { await saveAgent() }
                    }
                    .buttonStyle(.borderedProminent)
                    
                    Button("Deploy") {
                        Task { await deployAgent() }
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.green)
                }
            }
            Spacer()
        }
        .padding(Padding.standard.rawValue)
    }
    
    func saveAgent() async {
        let newAgent = EmergentAgentConfig(id: UUID().uuidString, name: name, persona: persona, tools: tools, state: agentState)
        do {
            _ = try await agentService.saveAgent(config: newAgent)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
    
    func deployAgent() async {
        let newAgent = EmergentAgentConfig(id: UUID().uuidString, name: name, persona: persona, tools: tools, state: agentState)
        agentState = .deploying
        do {
            _ = try await agentService.saveAgent(config: newAgent)
            try await agentService.deployAgent(id: newAgent.id)
            agentState = .running
        } catch {
            agentState = .error
            errorMessage = error.localizedDescription
        }
    }
}
