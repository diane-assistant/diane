import SwiftUI

// 2.1 Build the SwiftUI form for Custom Agent configuration
struct AgentConfigurationView: View {
    // 2.2 Wire the configuration form to EmergentAgentService
    @EnvironmentObject var agentService: EmergentAgentService
    
    @State private var name: String = ""
    @State private var persona: String = ""
    @State private var tools: [String] = []
    
    // Workspace Config state
    @State private var wsEnabled = false
    @State private var wsBaseImage = ""
    @State private var wsProvider = ""
    @State private var wsConnectRepo = false
    @State private var wsRepoType: EmergentRepoSourceType = .fixed
    @State private var wsRepoUrl = ""
    @State private var wsRepoBranch = ""
    @State private var wsCpu = ""
    @State private var wsMemory = ""
    @State private var wsDisk = ""
    @State private var wsSetupCommands: [String] = []
    @State private var wsTools: [String] = []
    @State private var wsCheckoutOnStart = false
    @State private var wsError: String? = nil
    @State private var availableImages: [EmergentWorkspaceImageDTO] = []
    
    // 2.3 Handle states
    @State private var agentState: EmergentAgentState = .pending
    @State private var errorMessage: String? = nil
    
    var body: some View {
        ScrollView {
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
            
            DetailSection(title: "Workspace Configuration") {
                VStack(alignment: .leading, spacing: Spacing.medium.rawValue) {
                    Toggle("Enable Workspace", isOn: $wsEnabled)
                    
                    if let err = wsError {
                        Text(err).font(.caption).foregroundStyle(.red)
                    }
                    
                    if wsEnabled {
                        Divider().padding(.vertical, 8)
                        
                        HStack {
                            Text("Base Image")
                                .frame(width: 100, alignment: .leading)
                            
                            if availableImages.isEmpty {
                                TextField("e.g. ubuntu:latest", text: $wsBaseImage)
                                    .textFieldStyle(.roundedBorder)
                            } else {
                                Picker("", selection: $wsBaseImage) {
                                    Text("Select an image...").tag("")
                                    ForEach(availableImages, id: \.id) { img in
                                        Text(img.name).tag(img.dockerRef ?? img.name)
                                    }
                                }
                                .pickerStyle(.menu)
                            }
                        }
                        
                        HStack {
                            Text("Provider")
                                .frame(width: 100, alignment: .leading)
                            Picker("", selection: $wsProvider) {
                                Text("Auto").tag("")
                                Text("Firecracker").tag("firecracker")
                                Text("gVisor").tag("gvisor")
                                Text("E2B").tag("e2b")
                            }
                            .pickerStyle(.menu)
                        }
                        
                        Divider().padding(.vertical, 4)
                        
                        Toggle("Connect Repository", isOn: $wsConnectRepo.animation())
                            .padding(.vertical, 4)
                        
                        if wsConnectRepo {
                            VStack(alignment: .leading, spacing: 8) {
                                HStack {
                                    Text("Source Type")
                                        .frame(width: 100, alignment: .leading)
                                    Picker("", selection: $wsRepoType) {
                                        Text("Fixed URL").tag(EmergentRepoSourceType.fixed)
                                        Text("Task Context").tag(EmergentRepoSourceType.taskContext)
                                    }
                                    .pickerStyle(.segmented)
                                }
                                
                                if wsRepoType == .fixed {
                                    HStack {
                                        Text("Repository URL")
                                            .frame(width: 100, alignment: .leading)
                                        TextField("https://github.com/...", text: $wsRepoUrl)
                                            .textFieldStyle(.roundedBorder)
                                    }
                                }
                                
                                HStack {
                                    Text("Branch")
                                        .frame(width: 100, alignment: .leading)
                                    TextField("main", text: $wsRepoBranch)
                                        .textFieldStyle(.roundedBorder)
                                }
                                
                                Toggle("Checkout on Start", isOn: $wsCheckoutOnStart)
                                    .padding(.leading, 108)
                            }
                            .padding()
                            .background(Color.secondary.opacity(0.05))
                            .cornerRadius(8)
                        }
                        
                        Divider().padding(.vertical, 4)
                        
                        Text("Resource Limits")
                            .font(.headline)
                            .padding(.top, 4)
                        
                        HStack(spacing: 16) {
                            HStack {
                                Text("CPU")
                                    .frame(width: 50, alignment: .leading)
                                TextField("Cores", text: $wsCpu)
                                    .textFieldStyle(.roundedBorder)
                            }
                            HStack {
                                Text("Memory")
                                    .frame(width: 50, alignment: .leading)
                                TextField("e.g. 4G", text: $wsMemory)
                                    .textFieldStyle(.roundedBorder)
                            }
                            HStack {
                                Text("Disk")
                                    .frame(width: 40, alignment: .leading)
                                TextField("e.g. 10G", text: $wsDisk)
                                    .textFieldStyle(.roundedBorder)
                            }
                        }
                        .padding(.leading, 108)
                        
                        Divider().padding(.vertical, 4)
                        
                        StringArrayEditor(items: $wsSetupCommands, title: "Setup Commands", placeholder: "e.g. npm install")
                        StringArrayEditor(items: $wsTools, title: "Workspace Tools", placeholder: "e.g. git")
                    }
                }
                .padding(.vertical, Padding.section.rawValue)
            }
            .task {
                do {
                    let images = try await EmergentAdminClient.shared.getWorkspaceImages()
                    availableImages = images.filter { $0.status == "ready" }
                } catch {}
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
        .frame(minWidth: 500, minHeight: 600)
    }
    
    func saveAgent() async {
        let newAgent = EmergentAgentConfig(id: UUID().uuidString, name: name, persona: persona, tools: tools, state: agentState)
        do {
            let saved = try await agentService.saveAgent(config: newAgent)
            try await saveWorkspaceConfig(agentId: saved.id)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
    
    func deployAgent() async {
        let newAgent = EmergentAgentConfig(id: UUID().uuidString, name: name, persona: persona, tools: tools, state: agentState)
        agentState = .deploying
        do {
            let saved = try await agentService.saveAgent(config: newAgent)
            try await saveWorkspaceConfig(agentId: saved.id)
            try await agentService.deployAgent(id: saved.id)
            agentState = .running
        } catch {
            agentState = .error
            errorMessage = error.localizedDescription
        }
    }
    
    private func saveWorkspaceConfig(agentId: String) async throws {
        wsError = nil
        let config = EmergentAgentWorkspaceConfig(
            enabled: wsEnabled,
            baseImage: wsEnabled ? (wsBaseImage.isEmpty ? nil : wsBaseImage) : nil,
            provider: wsEnabled ? (wsProvider.isEmpty ? nil : wsProvider) : nil,
            repoSource: wsEnabled ? (wsConnectRepo ? EmergentRepoSourceConfig(type: wsRepoType, url: wsRepoUrl.isEmpty ? nil : wsRepoUrl, branch: wsRepoBranch.isEmpty ? nil : wsRepoBranch) : EmergentRepoSourceConfig(type: .none)) : nil,
            resourceLimits: wsEnabled ? EmergentResourceLimits(cpu: wsCpu.isEmpty ? nil : wsCpu, memory: wsMemory.isEmpty ? nil : wsMemory, disk: wsDisk.isEmpty ? nil : wsDisk) : nil,
            setupCommands: wsEnabled ? wsSetupCommands : nil,
            tools: wsEnabled ? wsTools : nil,
            checkoutOnStart: wsEnabled ? wsCheckoutOnStart : nil
        )
        do {
            _ = try await EmergentAdminClient.shared.updateWorkspaceConfig(definitionId: agentId, config: config)
        } catch {
            wsError = "Workspace Config Error: \(error.localizedDescription)"
            throw error
        }
    }
}
